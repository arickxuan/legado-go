package analyzer

import (
	"encoding/json"
	"strings"

	"github.com/ohler55/ojg/jp"
	"github.com/ohler55/ojg/oj"
)

// jsonPathGetString applies a JSONPath rule and returns the first match.
func jsonPathGetString(rule string, content string) string {
	list := jsonPathGetStringList(rule, content)
	if len(list) == 0 {
		return ""
	}
	return list[0]
}

// jsonPathGetStringList applies a JSONPath rule and returns all matches.
func jsonPathGetStringList(rule string, content string) []string {
	if rule == "" || content == "" {
		return nil
	}
	// Strip @json: prefix if present
	rule = strings.TrimPrefix(rule, "@json:")
	rule = strings.TrimPrefix(rule, "@Json:")

	// Parse JSON
	obj, err := oj.ParseString(content)
	if err != nil {
		return nil
	}

	// Parse JSONPath expression
	expr, err := jp.ParseString(rule)
	if err != nil {
		return nil
	}

	// Evaluate
	results := expr.Get(obj)
	if len(results) == 0 {
		return nil
	}

	var out []string
	for _, r := range results {
		switch v := r.(type) {
		case string:
			out = append(out, v)
		case nil:
			// skip
		default:
			b, err := json.Marshal(v)
			if err == nil {
				out = append(out, string(b))
			}
		}
	}
	return out
}
