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
			// Check if JS result is a JSON array of strings
			if strings.HasPrefix(jsResult, "[") {
				var arr []string
				if err := jsonUnmarshal([]byte(jsResult), &arr); err == nil && len(arr) > 0 {
					return arr
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
		switch r.Mode {
		case ModeJs:
			result = a.evalJS(r.Rule, result)
		case ModeRegex:
			result = applyRegex(r.Rule, result, true)
		case ModeJson:
			result = jsonPathGetString(r.Rule, result)
		case ModeXPath:
			result = xpathGetString(r.Rule, result)
		default:
			result = cssGetString(r.Rule, result, a.baseUrl)
		}
	}
	return result
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
			// Check if JS result is a JSON array of strings
			if strings.HasPrefix(jsResult, "[") {
				var arr []string
				if err := jsonUnmarshal([]byte(jsResult), &arr); err == nil && len(arr) > 0 {
					return arr
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
