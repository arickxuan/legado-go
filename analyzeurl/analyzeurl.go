package analyzeurl

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopro/analyzer"

	"github.com/fastschema/qjs"
)

var (
	pagePattern            = regexp.MustCompile(`\{\{page(?:\|(\d+(?:,\d+)*))?\}\}`)
	keyPattern             = regexp.MustCompile(`\{\{key\}\}`)
	jsPattern              = regexp.MustCompile(`\{\{(@js:|<js>)(.*?)(?:</js>)?\}\}`)
	exprPattern            = regexp.MustCompile(`\{\{(.+?)\}\}`)
	angleBracketPagePattern = regexp.MustCompile(`<(.*?)>`)
)

// URLOption represents the JSON option appended after a URL (comma-separated).
type URLOption struct {
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
	Charset string            `json:"charset"`
}

// AnalyzeUrl handles URL template expansion and HTTP requests.
type AnalyzeUrl struct {
	RuleUrl    string            // original rule URL template
	FinalUrl   string            // expanded URL
	Method     string            // GET or POST
	Body       string            // POST body
	HeaderMap  map[string]string // request headers
	Charset    string
	SourceUrl  string            // base URL of the book source
	SourceHeader string          // header JSON from book source
	jsCode        string            // bare @js: code from searchUrl (executed in GetStrResponse)
	jsPool        *qjs.Pool         // JS pool for inline expression evaluation
	encodedKey    string            // URL-encoded search keyword (for re-replacement after inline expr)
	page          int               // page number (for re-replacement after inline expr)
	SourceComment string            // bookSourceComment for eval in inline expressions
}

// New creates a new AnalyzeUrl with the given parameters.
func New(ruleUrl string, key string, page int, sourceUrl string, sourceHeader string, jsPools ...*qjs.Pool) *AnalyzeUrl {
	a := &AnalyzeUrl{
		RuleUrl:     ruleUrl,
		Method:      "GET",
		HeaderMap:   make(map[string]string),
		SourceUrl:   sourceUrl,
		SourceHeader: sourceHeader,
	}
	if len(jsPools) > 0 {
		a.jsPool = jsPools[0]
	}
	a.initUrl(key, page)
	return a
}

// SetComment stores the bookSourceComment for eval() in inline expressions.
// Must be called before any URL resolution that may use {{expression}} templates.
func (a *AnalyzeUrl) SetComment(comment string) {
	a.SourceComment = comment
	// Re-resolve if FinalUrl still contains unresolved {{expressions}}
	if strings.Contains(a.FinalUrl, "{{") {
		a.resolveInlineExpressions()
	}
}

// initUrl processes the URL template: replace key, page, execute JS.
// JS code is executed with nil pool (no JS support); pool is used in GetStrResponse for @js: searchUrl.
func (a *AnalyzeUrl) initUrl(key string, page int) {
	ruleUrl := a.RuleUrl

	// Replace {{key}}
	a.encodedKey = url.QueryEscape(key)
	ruleUrl = keyPattern.ReplaceAllString(ruleUrl, a.encodedKey)

	// Replace {{page}}
	a.page = page
	if page > 0 {
		ruleUrl = pagePattern.ReplaceAllStringFunc(ruleUrl, func(match string) string {
			parts := pagePattern.FindStringSubmatch(match)
			if len(parts) > 1 && parts[1] != "" {
				// Has custom page values like {{page|1,2,3}}
				pageStrs := strings.Split(parts[1], ",")
				idx := page - 1
				if idx < len(pageStrs) {
					return strings.TrimSpace(pageStrs[idx])
				}
				return strings.TrimSpace(pageStrs[len(pageStrs)-1])
			}
			return strconv.Itoa(page)
		})
	}

	// Execute bare @js: or <js> prefix (Legado searchUrl like @js:result=...)
	// These are NOT wrapped in {{...}}, so jsPattern won't catch them.
	if strings.HasPrefix(ruleUrl, "@js:") {
		a.jsCode = ruleUrl[4:]
		ruleUrl = ""
	} else if strings.HasPrefix(ruleUrl, "<js>") {
		end := strings.Index(ruleUrl, "</js>")
		if end != -1 {
			a.jsCode = ruleUrl[4:end]
			ruleUrl = strings.TrimSpace(ruleUrl[end+5:])
		}
	}

	// If URL still contains {{expression}}, store raw and defer to resolveInlineExpressions
	if strings.Contains(ruleUrl, "{{") {
		a.FinalUrl = ruleUrl
	} else {
		ruleUrl = a.resolveAngleBracketPages(ruleUrl)
		a.FinalUrl = a.processUrl(ruleUrl)
	}

	// Parse source header
	if a.SourceHeader != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(a.SourceHeader), &headers); err == nil {
			for k, v := range headers {
				if _, exists := a.HeaderMap[k]; !exists {
					a.HeaderMap[k] = v
				}
			}
		}
	}

	// Set default User-Agent if not set
	if _, ok := a.HeaderMap["User-Agent"]; !ok {
		a.HeaderMap["User-Agent"] = "Mozilla/5.0 (Linux; Android 12; Pixel 6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36"
	}
}

// findOptionSeparator finds the comma that separates URL from option JSON.
// Must be after the path part and before any JSON-like content.
func findOptionSeparator(urlStr string) int {
	// Look for ,{ or ," patterns that indicate start of option JSON
	for i := 0; i < len(urlStr); i++ {
		if urlStr[i] == ',' {
			rest := strings.TrimSpace(urlStr[i+1:])
			if strings.HasPrefix(rest, "{") || strings.HasPrefix(rest, "\"") {
				return i
			}
		}
	}
	return -1
}

// parseUrlOption parses the URL option JSON string.
func (a *AnalyzeUrl) parseUrlOption(optionStr string) {
	var opt URLOption
	// Legado uses single-quote JSON (e.g. {'method':'POST'}), convert to double-quote
	jsonStr := optionStr
	if strings.Contains(jsonStr, "'") {
		jsonStr = singleToDoubleQuote(jsonStr)
	}
	if err := json.Unmarshal([]byte(jsonStr), &opt); err != nil {
		return
	}
	if opt.Method != "" {
		a.Method = strings.ToUpper(opt.Method)
	}
	if opt.Body != "" {
		a.Body = opt.Body
	}
	if opt.Charset != "" {
		a.Charset = opt.Charset
	}
	for k, v := range opt.Headers {
		a.HeaderMap[k] = v
	}
}

// singleToDoubleQuote converts single-quoted JSON to double-quoted JSON.
// Handles Legado book source URL option format like {'method':'POST','body':'s=test'}.
func singleToDoubleQuote(s string) string {
	var b strings.Builder
	inDouble := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '"' {
			inDouble = !inDouble
			b.WriteByte(ch)
		} else if ch == '\'' && !inDouble {
			b.WriteByte('"')
		} else {
			b.WriteByte(ch)
		}
	}
	return b.String()
}

// quoteJS returns a JavaScript string literal for the given Go string.
func quoteJS(s string) string {
	var b strings.Builder
	b.WriteByte('\'')
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch ch {
		case '\\':
			b.WriteString("\\\\")
		case '\'':
			b.WriteString("\\'")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		default:
			b.WriteByte(ch)
		}
	}
	b.WriteByte('\'')
	return b.String()
}

// resolveAngleBracketPages handles Legado's <val1,val2,...> page syntax.
// page=1 uses val1, page=2 uses val2, etc. Empty values are omitted.
// Example: /book/${key}<,/${page}> with page=1 -> /book/斗破
//          /book/${key}<,/${page}> with page=2 -> /book/斗破,/2
func (a *AnalyzeUrl) resolveAngleBracketPages(url string) string {
	matches := angleBracketPagePattern.FindStringSubmatch(url)
	if len(matches) < 2 {
		return url
	}
	fullMatch := matches[0]
	pagesStr := matches[1]
	pages := strings.Split(pagesStr, ",")
	idx := a.page - 1
	if idx < 0 {
		idx = 0
	}
	var replacement string
	if idx < len(pages) {
		replacement = strings.TrimSpace(pages[idx])
	} else if len(pages) > 0 {
		replacement = strings.TrimSpace(pages[len(pages)-1])
	}
	return strings.Replace(url, fullMatch, replacement, 1)
}

// processUrl parses URL options and resolves relative URL.
func (a *AnalyzeUrl) processUrl(rawUrl string) string {
	// Handle <val1,val2,...> page syntax before URL parsing
	rawUrl = a.resolveAngleBracketPages(rawUrl)
	// Handle remaining {{page}} if present
	if a.page > 0 {
		rawUrl = pagePattern.ReplaceAllString(rawUrl, strconv.Itoa(a.page))
	}
	rawUrl = keyPattern.ReplaceAllString(rawUrl, a.encodedKey)
	urlOption := ""
	commaIdx := findOptionSeparator(rawUrl)
	if commaIdx != -1 {
		urlOption = rawUrl[commaIdx+1:]
		rawUrl = rawUrl[:commaIdx]
	}
	result := resolveURL(a.SourceUrl, rawUrl)
	if urlOption != "" {
		a.parseUrlOption(urlOption)
	}
	return result
}

// resolveInlineExpressions evaluates {{expression}} JS templates in the URL.
// If the result contains more {{key}}/{{page}}, those are replaced too.
func (a *AnalyzeUrl) resolveInlineExpressions() {
	if a.jsPool == nil || !strings.Contains(a.FinalUrl, "{{") {
		return
	}
	for i := 0; i < 5; i++ { // max 5 iterations to avoid infinite loops
		matches := exprPattern.FindStringSubmatch(a.FinalUrl)
		if len(matches) < 2 {
			break
		}
		expr := matches[1]
		// Skip simple variable names that weren't replaced (e.g. unknown {{foo}})
		if !strings.Contains(expr, "(") && !strings.Contains(expr, ".") && !strings.Contains(expr, "+") {
			break
		}
		rt, err := a.jsPool.Get()
		if err != nil {
			break
		}
		ctx := rt.Context()
		analyzer.InjectLegadoStubs(ctx)
		ctx.Global().SetPropertyStr("baseUrl", ctx.NewString(a.SourceUrl))
		ctx.Global().SetPropertyStr("result", ctx.NewString(""))
		// Provide key/page for template literals like ${key}
		decodedKey, _ := url.QueryUnescape(a.encodedKey)
		ctx.Global().SetPropertyStr("key", ctx.NewString(decodedKey))
		ctx.Global().SetPropertyStr("page", ctx.NewString(strconv.Itoa(a.page)))
		// Set source properties for eval(source.bookSourceComment) etc.
		_, _ = ctx.Eval("set_source.js", qjs.Code(
			"source.getKey = function(){return " + quoteJS(a.SourceUrl) + "};var api,v;"))
		if a.SourceComment != "" {
			_, _ = ctx.Eval("set_comment.js", qjs.Code(
				"source.bookSourceComment = " + quoteJS(a.SourceComment) + ";"))
		}
		res, evalErr := ctx.Eval("inline_expr.js", qjs.Code(expr))
		resultStr := ""
		if evalErr != nil {
			log.Printf("[resolveExpr] eval error: %v", evalErr)
		}
		if evalErr == nil && !res.IsUndefined() && !res.IsNull() {
			resultStr = res.String()
		}
		res.Free()
		// Also check global 'result' variable (set by comment eval code)
		if resultStr == "" {
			gv := ctx.Global().GetPropertyStr("result")
			if gv != nil && !gv.IsUndefined() && !gv.IsNull() {
				s := gv.String()
				if s != "" {
					resultStr = s
				}
				gv.Free()
			}
		}
		a.jsPool.Put(rt)
		if resultStr == "" {
			break
		}
		a.FinalUrl = strings.Replace(a.FinalUrl, matches[0], resultStr, 1)
		// Re-replace {{key}} and {{page}} if result contains them
		a.FinalUrl = keyPattern.ReplaceAllString(a.FinalUrl, a.encodedKey)
		a.FinalUrl = pagePattern.ReplaceAllString(a.FinalUrl, strconv.Itoa(a.page))
	}
	// If still has unresolved {{}}, don't URL-encode it
	if strings.Contains(a.FinalUrl, "{{") {
		return
	}
	a.FinalUrl = a.processUrl(a.FinalUrl)
}

// resolveJS executes bare @js: code and parses the result as URL + option.
// Legado searchUrl like: @js:url=baseUrl+"/so/{{key}}.html,{'method':'GET',...}";result=url;
func (a *AnalyzeUrl) resolveJS(jsPool *qjs.Pool) {
	if jsPool == nil {
		return
	}
	rt, err := jsPool.Get()
	if err != nil {
		return
	}
	defer jsPool.Put(rt)

	ctx := rt.Context()
	analyzer.InjectLegadoStubs(ctx)
	ctx.Global().SetPropertyStr("baseUrl", ctx.NewString(a.SourceUrl))
	ctx.Global().SetPropertyStr("result", ctx.NewString(""))

	res, err := ctx.Eval("searchUrl.js", qjs.Code(a.jsCode))
	if err != nil {
		return
	}
	defer res.Free()

	resultStr := ""
	if !res.IsUndefined() && !res.IsNull() {
		resultStr = res.String()
	}
	if resultStr == "" {
		resultVal := ctx.Global().GetPropertyStr("result")
		if resultVal != nil && !resultVal.IsUndefined() && !resultVal.IsNull() {
			resultStr = resultVal.String()
			resultVal.Free()
		}
	}
	if resultStr == "" {
		return
	}

	// Parse result: may be "URL,{'method':'POST',...}" or just "URL"
	commaIdx := findOptionSeparator(resultStr)
	if commaIdx != -1 {
		optStr := resultStr[commaIdx+1:]
		resultStr = resultStr[:commaIdx]
		a.parseUrlOption(optStr)
	}

	if resultStr != "" {
		a.FinalUrl = resolveURL(a.SourceUrl, resultStr)
	}
}

// GetStrResponse sends the HTTP request and returns the response body as string.
func (a *AnalyzeUrl) GetStrResponse(jsPool *qjs.Pool) (string, error) {
	// Execute bare @js: code from searchUrl (e.g. qidian.com)
	if a.jsCode != "" {
		a.resolveJS(jsPool)
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	var req *http.Request
	var err error

	switch a.Method {
	case "POST":
		req, err = http.NewRequest("POST", a.FinalUrl, strings.NewReader(a.Body))
		if err != nil {
			return "", err
		}
		if a.HeaderMap["Content-Type"] == "" && a.Body != "" {
			// Auto-detect content type
			if isJSON(a.Body) {
				req.Header.Set("Content-Type", "application/json")
			} else if isXML(a.Body) {
				req.Header.Set("Content-Type", "application/xml")
			} else {
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			}
		}
	default:
		req, err = http.NewRequest("GET", a.FinalUrl, nil)
		if err != nil {
			return "", err
		}
	}

	for k, v := range a.HeaderMap {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Handle charset encoding
	if a.Charset != "" && strings.ToLower(a.Charset) != "utf-8" {
		// For gbk and other encodings, we'd need golang.org/x/text
		// For now, return as-is (most sites use UTF-8)
	}

	return string(body), nil
}

// ExecuteJS executes JavaScript on the response body and returns the result.
func (a *AnalyzeUrl) ExecuteJS(jsPool *qjs.Pool, jsCode string, body string) (string, error) {
	if jsPool == nil {
		return body, nil
	}
	rt, err := jsPool.Get()
	if err != nil {
		return body, err
	}
	defer jsPool.Put(rt)

	ctx := rt.Context()
	ctx.Global().SetPropertyStr("result", ctx.NewString(body))
	ctx.Global().SetPropertyStr("baseUrl", ctx.NewString(a.FinalUrl))
	ctx.Global().SetPropertyStr("src", ctx.NewString(body))

	res, err := ctx.Eval("analyzeUrl.js", qjs.Code(jsCode))
	if err != nil {
		return body, err
	}
	defer res.Free()

	if res.IsUndefined() || res.IsNull() {
		return "", nil
	}
	return res.String(), nil
}

// resolveURL resolves a relative URL against a base URL.
func resolveURL(baseURL string, ref string) string {
	if ref == "" {
		return baseURL
	}
	// Already absolute
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return ref
	}
	refURL, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return base.ResolveReference(refURL).String()
}

func isJSON(s string) bool {
	s = strings.TrimSpace(s)
	return (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) ||
		(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]"))
}

func isXML(s string) bool {
	return strings.Contains(s, "<?xml") || strings.Contains(s, "<rss")
}
