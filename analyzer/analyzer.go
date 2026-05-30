package analyzer

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/fastschema/qjs"
)

// Mode represents the parsing mode for a rule.
type Mode int

const (
	ModeDefault Mode = iota // JSoup @ syntax / CSS
	ModeXPath
	ModeJson
	ModeJs
	ModeRegex
)

// JS_PATTERN matches <js>...</js> or @js:...
var JS_PATTERN = regexp.MustCompile(`@js:(.+)|<js>([\s\S]*?)</js>`)

// templateExprPattern matches {{expression}} inline templates.
// Uses non-greedy match to handle nested braces correctly.
var templateExprPattern = regexp.MustCompile(`\{\{[\w\W]*?\}\}`)

// AnalyzeRule is the main rule dispatcher.
type AnalyzeRule struct {
	content string // the HTML/JSON/text to parse
	baseUrl string
	isJSON  bool
	jsPool  *qjs.Pool
}

// NewAnalyzeRule creates a new AnalyzeRule for the given content.
func NewAnalyzeRule(content string, baseUrl string, jsPool *qjs.Pool) *AnalyzeRule {
	isJSON := isJSON(content)
	return &AnalyzeRule{
		content: content,
		baseUrl: baseUrl,
		isJSON:  isJSON,
		jsPool:  jsPool,
	}
}

// GetString applies a rule string and returns the first non-empty result.
func (a *AnalyzeRule) GetString(ruleStr string) string {
	if ruleStr == "" {
		return ""
	}
	rules := SplitSourceRule(ruleStr, a.isJSON, false)
	return a.getStringFromRules(rules, a.content)
}

// GetStringList applies a rule string and returns all matches.
func (a *AnalyzeRule) GetStringList(ruleStr string) []string {
	if ruleStr == "" {
		return nil
	}
	rules := SplitSourceRule(ruleStr, a.isJSON, false)
	return a.getStringListFromRules(rules, a.content)
}

// GetElement returns a single element matching the rule (for init-type rules).
func (a *AnalyzeRule) GetElement(ruleStr string) string {
	if ruleStr == "" {
		return a.content
	}
	rules := SplitSourceRule(ruleStr, a.isJSON, false)
	return a.getStringFromRules(rules, a.content)
}

// GetElements returns all elements matching the rule as HTML strings,
// so that child rules can re-parse each element.
func (a *AnalyzeRule) GetElements(ruleStr string) []string {
	if ruleStr == "" {
		return nil
	}
	rules := SplitSourceRule(ruleStr, a.isJSON, false)
	return a.getElementListFromRules(rules, a.content)
}

// getElementListFromRules is like getStringListFromRules but uses CSS element list mode
// which returns outer HTML of matched elements instead of text content.
func (a *AnalyzeRule) getElementListFromRules(rules []SourceRule, content string) []string {
	var results []string
	current := content
	for _, r := range rules {
		if r.IsJs() {
			current = a.evalJS(r.Rule, current)
			continue
		}
		switch r.Mode {
		case ModeJs:
			jsResult := a.evalJS(r.Rule, current)
			if jsResult == "" {
				return nil
			}
			// Check if JS result is a JSON array
			if strings.HasPrefix(jsResult, "[") {
				var arr []string
				if err := jsonUnmarshal([]byte(jsResult), &arr); err == nil && len(arr) > 0 {
					return arr
				}
				// Array of JSON objects: parse as []interface{} and serialize each as JSON string
				var arrObj []interface{}
				if err := jsonUnmarshal([]byte(jsResult), &arrObj); err == nil && len(arrObj) > 0 {
					var items []string
					for _, item := range arrObj {
						if s, ok := item.(string); ok {
							items = append(items, s)
						} else {
							b, err := json.Marshal(item)
							if err == nil {
								items = append(items, string(b))
							}
						}
					}
					if len(items) > 0 {
						return items
					}
				}
			}
			current = jsResult
		case ModeRegex:
			results = applyRegexList(r.Rule, current)
			return results
		case ModeJson:
			results = jsonPathGetStringList(r.Rule, current)
			if len(results) == 0 {
				return nil
			}
			if len(results) > 0 {
				current = results[0]
			}
		case ModeXPath:
			results = xpathGetStringList(r.Rule, current)
			if len(results) == 0 {
				return nil
			}
			if len(results) > 0 {
				current = results[0]
			}
		default:
			results = cssGetElementList(r.Rule, current, a.baseUrl)
			if len(results) == 0 {
				return nil
			}
			if len(results) > 0 {
				current = results[0]
			}
		}
	}
	if len(results) == 0 {
		if current != "" {
			return []string{current}
		}
		return nil
	}
	return results
}

// getStringFromRules processes a chain of SourceRules, returning the first non-empty string.
func (a *AnalyzeRule) getStringFromRules(rules []SourceRule, content string) string {
	result := content
	for _, r := range rules {
		if r.IsJs() {
			result = a.evalJS(r.Rule, result)
			continue
		}
		rule := r.Rule
		// Resolve {{...}} inline templates before mode dispatch
		if strings.Contains(rule, "{{") {
			resolved := a.resolveInlineTemplates(rule, result)
			result = resolved
			continue
		}
		// Handle && concatenation (e.g. $.tags&&$.category&&$.status)
		if strings.Contains(rule, "&&") {
			result = a.handleAndConcat(rule, result)
			continue
		}
		// Split off ##regex##replacement post-processing before mode dispatch
		mainRule, replaceRegex, replacement := splitHashReplacement(rule)
		// Re-detect mode from the cleaned rule (without ##)
		mode := r.Mode
		if replaceRegex != "" && mode == ModeRegex {
			mode = DetectMode(mainRule, a.isJSON)
		}
		switch mode {
		case ModeJs:
			result = a.evalJS(mainRule, result)
		case ModeRegex:
			result = applyRegex(mainRule, result, true)
		case ModeJson:
			result = jsonPathGetString(mainRule, result)
		case ModeXPath:
			result = xpathGetString(mainRule, result)
		default:
			result = cssGetString(mainRule, result, a.baseUrl)
		}
		// Apply ## post-processing if present
		if replaceRegex != "" {
			result = applyRegexPost(result, replaceRegex, replacement)
		}
	}
	return result
}

// splitHashReplacement splits a rule into the main rule and ##regex##replacement parts.
// "class.info@ownText##作者：" → ("class.info@ownText", "作者：", "")
// "rule##regex##replacement" → ("rule", "regex", "replacement")
func splitHashReplacement(rule string) (string, string, string) {
	if !strings.Contains(rule, "##") {
		return rule, "", ""
	}
	parts := strings.SplitN(rule, "##", 3)
	main := strings.TrimSpace(parts[0])
	regex := ""
	replacement := ""
	if len(parts) >= 2 {
		regex = parts[1]
	}
	if len(parts) >= 3 {
		replacement = parts[2]
	}
	return main, regex, replacement
}

// applyRegexPost applies a regex replacement to a string.
func applyRegexPost(content string, pattern string, replacement string) string {
	if content == "" || pattern == "" {
		return content
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return content
	}
	return strings.TrimSpace(re.ReplaceAllString(content, replacement))
}

// handleAndConcat splits a rule by && and concatenates all non-empty results.
// Each part is evaluated as an individual expression.
func (a *AnalyzeRule) handleAndConcat(rule string, content string) string {
	parts := strings.Split(rule, "&&")
	var results []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		val := a.evalSingleRule(part, content)
		if val != "" {
			results = append(results, val)
		}
	}
	return strings.Join(results, " ")
}

// evalSingleRule evaluates a single rule expression (no templates, no &&, no ||).
func (a *AnalyzeRule) evalSingleRule(rule string, content string) string {
	switch DetectMode(rule, a.isJSON) {
	case ModeJs:
		return a.evalJS(rule, content)
	case ModeRegex:
		return applyRegex(rule, content, true)
	case ModeJson:
		return jsonPathGetString(rule, content)
	case ModeXPath:
		return xpathGetString(rule, content)
	default:
		return cssGetString(rule, content, a.baseUrl)
	}
}

// resolveInlineTemplates resolves {{expression}} templates in a rule string.
// Each {{expression}} may contain alternatives separated by || (try each until non-empty).
// Expressions starting with @ are CSS/JSoup rules, $. or $[ are JSONPath, // are XPath, others are JS.
// After template resolution, ##regex##replacement is applied if present.
func (a *AnalyzeRule) resolveInlineTemplates(rule string, content string) string {
	matches := templateExprPattern.FindAllStringIndex(rule, -1)
	if len(matches) == 0 {
		return rule
	}

	var result strings.Builder
	lastEnd := 0
	for _, m := range matches {
		// Append literal text before this template
		if m[0] > lastEnd {
			result.WriteString(rule[lastEnd:m[0]])
		}
		// Extract expression inside {{...}}
		expr := rule[m[0]+2 : m[1]-2] // strip {{ and }}
		val := a.resolveTemplateExpr(expr, content)
		result.WriteString(val)
		lastEnd = m[1]
	}
	// Append remaining literal text
	if lastEnd < len(rule) {
		result.WriteString(rule[lastEnd:])
	}

	resolved := result.String()

	// Handle ##regex##replacement after the resolved rule
	if idx := strings.Index(resolved, "##"); idx != -1 {
		mainRule := strings.TrimSpace(resolved[:idx])
		regexPart := resolved[idx+2:]
		replacement := ""
		if idx2 := strings.Index(regexPart, "##"); idx2 != -1 {
			replacement = regexPart[idx2+2:]
			regexPart = regexPart[:idx2]
		}
		if regexPart != "" {
			if re, err := regexp.Compile(regexPart); err == nil {
				return strings.TrimSpace(re.ReplaceAllString(mainRule, replacement))
			}
		}
		return mainRule
	}

	return strings.TrimSpace(resolved)
}

// resolveTemplateExpr evaluates a single expression from inside {{...}}.
// If the expression starts with @, it is treated as a CSS/JSoup rule with || cascade.
// Otherwise, || splits into alternatives tried until one returns non-empty.
func (a *AnalyzeRule) resolveTemplateExpr(expr string, content string) string {
	// If the whole expression is a CSS/JSoup rule (starts with @), pass it directly
	// so that || cascade is handled by the CSS parser.
	if strings.HasPrefix(expr, "@") {
		return cssGetString(expr[1:], content, a.baseUrl)
	}

	// For non-CSS expressions, split by || and try each alternative.
	alts := splitTopLevelOr(expr)
	for _, alt := range alts {
		alt = strings.TrimSpace(alt)
		if alt == "" || alt == `""` || alt == `''` {
			continue
		}
		val := a.evalSingleExpr(alt, content)
		if val != "" {
			return val
		}
	}
	return ""
}

// evalSingleExpr evaluates a single expression (no || alternatives).
func (a *AnalyzeRule) evalSingleExpr(expr string, content string) string {
	switch {
	case strings.HasPrefix(expr, "$.") || strings.HasPrefix(expr, "$["):
		// JSONPath rule
		return jsonPathGetString(expr, content)
	case strings.HasPrefix(expr, "//"):
		// XPath rule
		return xpathGetString(expr, content)
	default:
		// JavaScript expression
		return a.evalJS(expr, content)
	}
}

// splitTopLevelOr splits a string by || at the top level (not inside quotes or braces).
func splitTopLevelOr(s string) []string {
	var parts []string
	depth := 0
	inSingle := false
	inDouble := false
	inBacktick := false
	last := 0
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch == '\'' && !inDouble && !inBacktick:
			inSingle = !inSingle
		case ch == '"' && !inSingle && !inBacktick:
			inDouble = !inDouble
		case ch == '`' && !inSingle && !inDouble:
			inBacktick = !inBacktick
		case ch == '\\' && (inSingle || inDouble || inBacktick):
			i++ // skip escaped char
		case ch == '{' && !inSingle && !inDouble && !inBacktick:
			depth++
		case ch == '}' && !inSingle && !inDouble && !inBacktick:
			if depth > 0 {
				depth--
			}
		case ch == '|' && i+1 < len(s) && s[i+1] == '|' && !inSingle && !inDouble && !inBacktick && depth == 0:
			parts = append(parts, s[last:i])
			last = i + 2
			i++ // skip second |
		}
	}
	parts = append(parts, s[last:])
	return parts
}

// getStringListFromRules processes a chain of SourceRules, returning all matches.
func (a *AnalyzeRule) getStringListFromRules(rules []SourceRule, content string) []string {
	var results []string
	current := content
	for _, r := range rules {
		if r.IsJs() {
			current = a.evalJS(r.Rule, current)
			continue
		}
		switch r.Mode {
		case ModeJs:
			jsResult := a.evalJS(r.Rule, current)
			if jsResult == "" {
				return nil
			}
			// Check if JS result is a JSON array
			if strings.HasPrefix(jsResult, "[") {
				var arr []string
				if err := jsonUnmarshal([]byte(jsResult), &arr); err == nil && len(arr) > 0 {
					return arr
				}
				// Array of JSON objects: parse as []interface{} and serialize each as JSON string
				var arrObj []interface{}
				if err := jsonUnmarshal([]byte(jsResult), &arrObj); err == nil && len(arrObj) > 0 {
					var items []string
					for _, item := range arrObj {
						if s, ok := item.(string); ok {
							items = append(items, s)
						} else {
							b, err := json.Marshal(item)
							if err == nil {
								items = append(items, string(b))
							}
						}
					}
					if len(items) > 0 {
						return items
					}
				}
			}
			current = jsResult
		case ModeRegex:
			results = applyRegexList(r.Rule, current)
			return results
		case ModeJson:
			results = jsonPathGetStringList(r.Rule, current)
			if len(results) == 0 {
				return nil
			}
			if len(results) > 0 {
				current = results[0]
			}
		case ModeXPath:
			results = xpathGetStringList(r.Rule, current)
			if len(results) == 0 {
				return nil
			}
			if len(results) > 0 {
				current = results[0]
			}
		default:
			results = cssGetStringList(r.Rule, current, a.baseUrl)
			if len(results) == 0 {
				return nil
			}
			if len(results) > 0 {
				current = results[0]
			}
		}
	}
	if len(results) == 0 {
		if current != "" {
			return []string{current}
		}
		return nil
	}
	return results
}

// jsonUnmarshal is a helper to avoid import cycle issues.
var jsonUnmarshal = json.Unmarshal

// evalJS executes JavaScript code via qjs Pool with injected variables.
func (a *AnalyzeRule) evalJS(jsCode string, result string) string {
	if a.jsPool == nil {
		return result
	}
	rt, err := a.jsPool.Get()
	if err != nil {
		return result
	}
	defer a.jsPool.Put(rt)
	ctx := rt.Context()
	InjectLegadoStubs(ctx)
	ctx.Global().SetPropertyStr("result", ctx.NewString(result))
	ctx.Global().SetPropertyStr("baseUrl", ctx.NewString(a.baseUrl))
	res, err := ctx.Eval("rule.js", qjs.Code(jsCode))
	if err != nil {
		return result
	}
	defer res.Free()
	if res.IsUndefined() || res.IsNull() {
		return ""
	}
	// If result is an array or object, serialize to JSON
	if res.IsObject() || res.IsArray() {
		ctx.Global().SetPropertyStr("__eval_result", res)
		jsonRes, jsonErr := ctx.Eval("jsonify.js", qjs.Code("JSON.stringify(__eval_result)"))
		if jsonErr == nil && !jsonRes.IsUndefined() && !jsonRes.IsNull() {
			s := jsonRes.String()
			jsonRes.Free()
			return s
		}
		if jsonErr == nil {
			jsonRes.Free()
		}
	}
	return res.String()
}

// SetContent updates the content to parse.
func (a *AnalyzeRule) SetContent(content string, baseUrl string) {
	a.content = content
	if baseUrl != "" {
		a.baseUrl = baseUrl
	}
	a.isJSON = isJSON(content)
}

func isJSON(s string) bool {
	s = strings.TrimSpace(s)
	return (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) ||
		(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]"))
}

// applyRegex applies a regex rule and returns the replaced result.
func applyRegex(rule string, content string, first bool) string {
	parts := strings.SplitN(rule, "##", 3)
	if len(parts) < 2 {
		return content
	}
	pattern := parts[0]
	replacement := ""
	if len(parts) >= 3 {
		replacement = parts[2]
	} else if len(parts) == 2 {
		replacement = parts[1]
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return content
	}
	if first {
		return re.ReplaceAllString(content, replacement)
	}
	return re.ReplaceAllString(content, replacement)
}

// applyRegexList applies a regex and returns captured groups as list.
func applyRegexList(rule string, content string) []string {
	re, err := regexp.Compile(rule)
	if err != nil {
		return nil
	}
	matches := re.FindStringSubmatch(content)
	if matches == nil {
		return nil
	}
	return matches
}
