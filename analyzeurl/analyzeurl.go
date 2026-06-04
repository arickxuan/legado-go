package analyzeurl

import (
	"crypto/tls"
	"encoding/hex"
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
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

var (
	pagePattern             = regexp.MustCompile(`\{\{page(?:\|(\d+(?:,\d+)*))?\}\}`)
	keyPattern              = regexp.MustCompile(`\{\{key\}\}`)
	jsPattern               = regexp.MustCompile(`\{\{(@js:|<js>)(.*?)(?:</js>)?\}\}`)
	exprPattern             = regexp.MustCompile(`\{\{(.+?)\}\}`)
	angleBracketPagePattern = regexp.MustCompile(`<(.*?)>`)
)

// URLOption represents the JSON option appended after a URL (comma-separated).
type URLOption struct {
	Method           string            `json:"method"`
	Headers          map[string]string `json:"headers"`
	Body             any               `json:"body"`
	Charset          string            `json:"charset"`
	Type             string            `json:"type"`
	JS               string            `json:"js"`
	Retry            any               `json:"retry"`
	WebView          any               `json:"webView"`
	WebJs            string            `json:"webJs"`
	ServerID         any               `json:"serverID"`
	WebViewDelayTime any               `json:"webViewDelayTime"`
	Proxy            string            `json:"proxy"`
}

// AnalyzeUrl handles URL template expansion and HTTP requests.
type AnalyzeUrl struct {
	RuleUrl       string            // original rule URL template
	FinalUrl      string            // expanded URL
	Method        string            // GET or POST
	Body          string            // POST body
	HeaderMap     map[string]string // request headers
	Charset       string
	SourceUrl     string    // base URL of the book source
	SourceHeader  string    // header JSON from book source
	jsCode        string    // bare @js: code from searchUrl (executed in GetStrResponse)
	jsPool        *qjs.Pool // JS pool for inline expression evaluation
	encodedKey    string    // URL-encoded search keyword (for re-replacement after inline expr)
	rawKey        string    // raw search keyword (for POST body encoding)
	page          int       // page number (for re-replacement after inline expr)
	SourceComment string    // bookSourceComment for eval in inline expressions
	Type          string
	Retry         int
	UseWebView    bool
	WebJs         string
	ServerID      string
	WebViewDelay  int
	Proxy         string
}

// New creates a new AnalyzeUrl with the given parameters.
func New(ruleUrl string, key string, page int, sourceUrl string, sourceHeader string, jsPools ...*qjs.Pool) *AnalyzeUrl {
	a := &AnalyzeUrl{
		RuleUrl:      ruleUrl,
		Method:       "GET",
		HeaderMap:    make(map[string]string),
		SourceUrl:    sourceUrl,
		SourceHeader: sourceHeader,
	}
	if len(jsPools) > 0 {
		a.jsPool = jsPools[0]
	}
	a.initUrl(key, page)
	return a
}

// SetComment stores the bookSourceComment for eval() in inline expressions.
// Re-derives the URL from the original template because some searchUrl expressions
// like {{eval(source.bookSourceComment)}} require the comment to be set first.
func (a *AnalyzeUrl) SetComment(comment string) {
	a.SourceComment = comment
	if a.SourceComment == "" {
		return
	}
	// Re-derive URL from original template with key/page already substituted
	ruleUrl := a.RuleUrl
	ruleUrl = keyPattern.ReplaceAllString(ruleUrl, a.encodedKey)
	if a.page > 0 {
		ruleUrl = pagePattern.ReplaceAllStringFunc(ruleUrl, func(match string) string {
			parts := pagePattern.FindStringSubmatch(match)
			if len(parts) > 1 && parts[1] != "" {
				pageStrs := strings.Split(parts[1], ",")
				idx := a.page - 1
				if idx < len(pageStrs) {
					return strings.TrimSpace(pageStrs[idx])
				}
				return strings.TrimSpace(pageStrs[len(pageStrs)-1])
			}
			return strconv.Itoa(a.page)
		})
	}
	// If bare @js: prefix was used, JS runs in GetStrResponse; don't re-derive here
	if a.jsCode != "" {
		return
	}
	// Re-resolve inline expressions with the comment now available
	if strings.Contains(ruleUrl, "{{") {
		a.FinalUrl = ruleUrl
		if a.jsPool != nil {
			a.resolveInlineExpressions()
		}
	} else {
		ruleUrl = a.resolveAngleBracketPages(ruleUrl)
		a.FinalUrl = a.processUrl(ruleUrl)
	}
}

// initUrl processes the URL template: replace key, page, execute JS.
// JS code is executed with nil pool (no JS support); pool is used in GetStrResponse for @js: searchUrl.
func (a *AnalyzeUrl) initUrl(key string, page int) {
	ruleUrl := a.RuleUrl

	// Replace {{key}}
	a.encodedKey = url.QueryEscape(key)
	a.rawKey = key
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

	// If URL still contains {{expression}}, resolve now or defer to SetComment
	if strings.Contains(ruleUrl, "{{") {
		a.FinalUrl = ruleUrl
		if a.jsPool != nil {
			a.resolveInlineExpressions()
		}
	} else {
		ruleUrl = a.resolveAngleBracketPages(ruleUrl)
		a.FinalUrl = a.processUrl(ruleUrl)
	}

	// Parse source header. Many real sources store either a full JSON object or
	// just the inner object fragment: `"User-Agent":"..."`.
	for k, v := range parseStringMap(a.SourceHeader, a.SourceUrl) {
		if strings.EqualFold(k, "proxy") {
			a.Proxy = v
			continue
		}
		if _, exists := a.HeaderMap[k]; !exists {
			a.HeaderMap[k] = v
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
	jsonStr := normalizeJSONLike(optionStr)
	if err := json.Unmarshal([]byte(jsonStr), &opt); err != nil {
		return
	}
	if opt.Method != "" {
		a.Method = strings.ToUpper(opt.Method)
	}
	if opt.Body != nil {
		a.Body = stringifyOptionValue(opt.Body)
		// Replace URL-encoded key with raw key in body for proper charset encoding
		if a.encodedKey != a.rawKey {
			a.Body = strings.ReplaceAll(a.Body, a.encodedKey, a.rawKey)
		}
	}
	if opt.Charset != "" {
		a.Charset = opt.Charset
	}
	if opt.Type != "" {
		a.Type = opt.Type
	}
	if opt.JS != "" {
		a.evalOptionJS(opt.JS)
	}
	if opt.Retry != nil {
		a.Retry = intFromAny(opt.Retry)
	}
	if opt.WebView != nil {
		a.UseWebView = truthyOption(opt.WebView)
	}
	if opt.WebJs != "" {
		a.WebJs = opt.WebJs
	}
	if opt.ServerID != nil {
		a.ServerID = stringifyOptionValue(opt.ServerID)
	}
	if opt.WebViewDelayTime != nil {
		a.WebViewDelay = intFromAny(opt.WebViewDelayTime)
	}
	if opt.Proxy != "" {
		a.Proxy = opt.Proxy
	}
	for k, v := range opt.Headers {
		if strings.EqualFold(k, "proxy") {
			a.Proxy = v
		} else {
			a.HeaderMap[k] = v
		}
	}
}

func parseStringMap(raw string, baseUrl string) map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	raw = strings.ReplaceAll(raw, "{{baseUrl}}", baseUrl)
	jsonStr := normalizeJSONLike(raw)
	if !strings.HasPrefix(strings.TrimSpace(jsonStr), "{") {
		jsonStr = "{" + jsonStr + "}"
	}
	var anyMap map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &anyMap); err != nil {
		return nil
	}
	out := make(map[string]string, len(anyMap))
	for k, v := range anyMap {
		out[k] = stringifyOptionValue(v)
	}
	return out
}

func normalizeJSONLike(s string) string {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "'") {
		s = singleToDoubleQuote(s)
	}
	return s
}

func stringifyOptionValue(v any) string {
	switch value := v.(type) {
	case nil:
		return ""
	case string:
		return value
	case float64:
		if value == float64(int64(value)) {
			return strconv.FormatInt(int64(value), 10)
		}
		return strconv.FormatFloat(value, 'f', -1, 64)
	case bool:
		if value {
			return "true"
		}
		return "false"
	default:
		b, err := json.Marshal(value)
		if err != nil {
			return fmt.Sprint(value)
		}
		return string(b)
	}
}

func intFromAny(v any) int {
	switch value := v.(type) {
	case float64:
		return int(value)
	case string:
		i, _ := strconv.Atoi(strings.TrimSpace(value))
		return i
	case int:
		return value
	case json.Number:
		i, _ := value.Int64()
		return int(i)
	default:
		return 0
	}
}

func truthyOption(v any) bool {
	switch value := v.(type) {
	case nil:
		return false
	case bool:
		return value
	case string:
		value = strings.TrimSpace(strings.ToLower(value))
		return value != "" && value != "false" && value != "0" && value != "null"
	default:
		return true
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
//
//	/book/${key}<,/${page}> with page=2 -> /book/斗破,/2
func (a *AnalyzeUrl) resolveAngleBracketPages(urlStr string) string {
	matches := angleBracketPagePattern.FindStringSubmatch(urlStr)
	if len(matches) < 2 {
		return urlStr
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
	result := strings.Replace(urlStr, fullMatch, replacement, 1)
	// Replace ${page} and ${key} patterns that may appear in angle bracket values
	if a.page > 0 {
		result = strings.ReplaceAll(result, "${page}", strconv.Itoa(a.page))
	}
	result = strings.ReplaceAll(result, "${key}", a.encodedKey)
	return result
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
	// Also handle ${page} and ${key} Kotlin-style template patterns
	if a.page > 0 {
		rawUrl = strings.ReplaceAll(rawUrl, "${page}", strconv.Itoa(a.page))
	}
	rawUrl = strings.ReplaceAll(rawUrl, "${key}", a.encodedKey)
	urlOption := ""
	commaIdx := findOptionSeparator(rawUrl)
	if commaIdx != -1 {
		urlOption = rawUrl[commaIdx+1:]
		rawUrl = rawUrl[:commaIdx]
	}
	result := resolveURL(a.SourceUrl, rawUrl)
	a.FinalUrl = result
	if urlOption != "" {
		a.parseUrlOption(urlOption)
		result = a.FinalUrl
	}
	return result
}

// findTemplateExpr finds the first {{...}} expression in the URL using brace-depth counting.
// This correctly handles expressions containing }} (e.g. JS with URLs containing "://...").
func findTemplateExpr(s string) (start int, end int, expr string) {
	start = strings.Index(s, "{{")
	if start < 0 {
		return -1, -1, ""
	}
	depth := 0
	for i := start; i < len(s)-1; i++ {
		if s[i] == '{' && s[i+1] == '{' {
			depth++
			i++ // skip second '{'
			continue
		}
		if s[i] == '}' && s[i+1] == '}' {
			depth--
			if depth == 0 {
				return start, i + 2, s[start+2 : i]
			}
			i++ // skip second '}'
		}
	}
	return -1, -1, ""
}

// resolveInlineExpressions evaluates {{expression}} JS templates in the URL.
// If the result contains more {{key}}/{{page}}, those are replaced too.
func (a *AnalyzeUrl) resolveInlineExpressions() {
	if a.jsPool == nil || !strings.Contains(a.FinalUrl, "{{") {
		return
	}
	for i := 0; i < 5; i++ { // max 5 iterations to avoid infinite loops
		matchStart, matchEnd, expr := findTemplateExpr(a.FinalUrl)
		if expr == "" {
			break
		}
		_ = matchEnd
		// Skip simple variable names that weren't replaced (e.g. unknown {{foo}})
		if !looksLikeJSExpression(expr) {
			break
		}
		rt, err := a.jsPool.Get()
		if err != nil {
			break
		}
		ctx := rt.Context()
		a.injectJSContext(ctx, "")
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
		a.FinalUrl = a.FinalUrl[:matchStart] + resultStr + a.FinalUrl[matchEnd:]
		// Re-replace {{key}} and {{page}} if result contains them
		a.FinalUrl = keyPattern.ReplaceAllString(a.FinalUrl, a.encodedKey)
		a.FinalUrl = pagePattern.ReplaceAllString(a.FinalUrl, strconv.Itoa(a.page))
		if a.page > 0 {
			a.FinalUrl = strings.ReplaceAll(a.FinalUrl, "${page}", strconv.Itoa(a.page))
		}
		a.FinalUrl = strings.ReplaceAll(a.FinalUrl, "${key}", a.encodedKey)
	}
	a.FinalUrl = strings.TrimSpace(a.FinalUrl)
	// If still has unresolved {{}}, don't URL-encode it
	if strings.Contains(a.FinalUrl, "{{") {
		return
	}
	a.FinalUrl = a.processUrl(a.FinalUrl)
}

func looksLikeJSExpression(expr string) bool {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return false
	}
	if expr == "key" || expr == "page" {
		return true
	}
	return strings.ContainsAny(expr, "().+-*/?:=;[]{}\"'`") ||
		strings.Contains(expr, "java") ||
		strings.Contains(expr, "source") ||
		strings.Contains(expr, "cookie") ||
		strings.Contains(expr, "cache")
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
	a.injectJSContext(ctx, "")

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

func (a *AnalyzeUrl) injectJSContext(ctx *qjs.Context, result string) {
	analyzer.InjectLegadoStubs(ctx)
	decodedKey, _ := url.QueryUnescape(a.encodedKey)
	ctx.Global().SetPropertyStr("baseUrl", ctx.NewString(a.SourceUrl))
	ctx.Global().SetPropertyStr("key", ctx.NewString(decodedKey))
	ctx.Global().SetPropertyStr("page", ctx.NewInt32(int32(a.page)))
	ctx.Global().SetPropertyStr("result", ctx.NewString(result))
	ctx.Global().SetPropertyStr("src", ctx.NewString(result))
	_, _ = ctx.Eval("set_source.js", qjs.Code(
		"source.key = "+quoteJS(a.SourceUrl)+";"+
			"source.bookSourceUrl = "+quoteJS(a.SourceUrl)+";"+
			"source.bookSourceComment = "+quoteJS(a.SourceComment)+";"+
			"source.getKey = function(){return "+quoteJS(a.SourceUrl)+"};"))
}

func (a *AnalyzeUrl) evalOptionJS(jsCode string) {
	if a.jsPool == nil || strings.TrimSpace(jsCode) == "" {
		return
	}
	rt, err := a.jsPool.Get()
	if err != nil {
		return
	}
	defer a.jsPool.Put(rt)
	ctx := rt.Context()
	a.injectJSContext(ctx, a.FinalUrl)
	_, _ = ctx.Eval("set_url.js", qjs.Code(
		"var url = "+quoteJS(a.FinalUrl)+";"+
			"java.url = url;"+
			"java.headerMap = {put:function(k,v){java._headers[String(k)] = String(v)}};"+
			"java._headers = {};"))
	res, err := ctx.Eval("url_option.js", qjs.Code(jsCode))
	if err == nil && res != nil {
		if !res.IsUndefined() && !res.IsNull() && res.String() != "" {
			a.FinalUrl = resolveURL(a.SourceUrl, res.String())
		}
		res.Free()
	}
	if v := ctx.Global().GetPropertyStr("url"); v != nil {
		if !v.IsUndefined() && !v.IsNull() && v.String() != "" {
			a.FinalUrl = resolveURL(a.SourceUrl, v.String())
		}
		v.Free()
	}
	if v := ctx.Global().GetPropertyStr("java"); v != nil {
		h := v.GetPropertyStr("_headers")
		if h != nil && h.IsObject() {
			h.ForEach(func(key *qjs.Value, value *qjs.Value) {
				a.HeaderMap[key.String()] = value.String()
			})
			h.Free()
		}
		v.Free()
	}
}

// GetStrResponse sends the HTTP request and returns the response body as string.
func (a *AnalyzeUrl) GetStrResponse(jsPool *qjs.Pool) (string, error) {
	// Execute bare @js: code from searchUrl (e.g. qidian.com)
	if a.jsCode != "" {
		a.resolveJS(jsPool)
	}
	if strings.TrimSpace(a.FinalUrl) == "" {
		return "", fmt.Errorf("empty final url after analyzing rule %q", a.RuleUrl)
	}

	client := a.httpClient()

	var req *http.Request
	var err error

	switch a.Method {
	case "POST":
		requestBody := a.encodedRequestBody()
		req, err = http.NewRequest("POST", a.FinalUrl, strings.NewReader(requestBody))
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

	var resp *http.Response
	attempts := a.Retry + 1
	if attempts < 1 {
		attempts = 1
	}
	for i := 0; i < attempts; i++ {
		if i > 0 {
			req = req.Clone(req.Context())
			if a.Method == "POST" {
				body := a.encodedRequestBody()
				req.Body = io.NopCloser(strings.NewReader(body))
				req.ContentLength = int64(len(body))
			}
		}
		resp, err = client.Do(req)
		if err == nil && resp.StatusCode < 500 {
			break
		}
		if resp != nil && i < attempts-1 {
			resp.Body.Close()
		}
	}
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
	if a.Type != "" {
		return hex.EncodeToString(body), nil
	}

	// Decode response body from the specified charset to UTF-8
	if a.Charset != "" {
		charset := strings.ToLower(a.Charset)
		if charset == "gbk" || charset == "gb2312" || charset == "gb18030" {
			reader := transform.NewReader(strings.NewReader(string(body)), simplifiedchinese.GBK.NewDecoder())
			if decoded, err := io.ReadAll(reader); err == nil {
				body = decoded
			}
		}
	}

	return string(body), nil
}

func (a *AnalyzeUrl) httpClient() *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	if a.Proxy != "" {
		proxyURL := strings.TrimSpace(a.Proxy)
		if idx := strings.LastIndex(proxyURL, "@"); idx > strings.Index(proxyURL, "://")+2 {
			// Legado also allows proxy://host@user@pass. net/http only accepts
			// standard userinfo syntax; ignore credentials for now rather than failing.
			proxyURL = proxyURL[:idx]
		}
		if u, err := url.Parse(proxyURL); err == nil {
			transport.Proxy = http.ProxyURL(u)
		}
	}
	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}

func (a *AnalyzeUrl) encodedRequestBody() string {
	if a.Body == "" {
		return ""
	}
	contentType := strings.ToLower(a.HeaderMap["Content-Type"])
	if isJSON(a.Body) || isXML(a.Body) || (contentType != "" && !strings.Contains(contentType, "x-www-form-urlencoded")) {
		return encodeWholeBody(a.Body, a.Charset)
	}
	return encodeFormBody(a.Body, a.Charset)
}

func encodeWholeBody(body string, charset string) string {
	if isGBK(charset) {
		if encoded, err := io.ReadAll(transform.NewReader(strings.NewReader(body), simplifiedchinese.GBK.NewEncoder())); err == nil {
			return string(encoded)
		}
	}
	return body
}

func encodeFormBody(body string, charset string) string {
	var out strings.Builder
	parts := strings.Split(body, "&")
	for i, part := range parts {
		if i > 0 {
			out.WriteByte('&')
		}
		key, val, ok := strings.Cut(part, "=")
		out.WriteString(encodeFormComponent(key, charset))
		if ok {
			out.WriteByte('=')
			out.WriteString(encodeFormComponent(val, charset))
		}
	}
	return out.String()
}

func encodeFormComponent(s string, charset string) string {
	if s == "" || alreadyEncoded(s) {
		return s
	}
	if isGBK(charset) {
		encoded, err := io.ReadAll(transform.NewReader(strings.NewReader(s), simplifiedchinese.GBK.NewEncoder()))
		if err == nil {
			return percentEncodeBytes(encoded)
		}
	}
	return url.QueryEscape(s)
}

func percentEncodeBytes(b []byte) string {
	const hexChars = "0123456789ABCDEF"
	var out strings.Builder
	for _, c := range b {
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9',
			c == '-', c == '_', c == '.', c == '*':
			out.WriteByte(c)
		case c == ' ':
			out.WriteByte('+')
		default:
			out.WriteByte('%')
			out.WriteByte(hexChars[c>>4])
			out.WriteByte(hexChars[c&0x0f])
		}
	}
	return out.String()
}

func alreadyEncoded(s string) bool {
	for i := 0; i < len(s)-2; i++ {
		if s[i] == '%' && isHex(s[i+1]) && isHex(s[i+2]) {
			return true
		}
	}
	return false
}

func isHex(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F'
}

func isGBK(charset string) bool {
	charset = strings.ToLower(strings.TrimSpace(charset))
	return charset == "gbk" || charset == "gb2312" || charset == "gb18030"
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
