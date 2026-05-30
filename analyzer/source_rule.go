package analyzer

import "strings"

// SourceRule represents a single parsed rule with its mode.
type SourceRule struct {
	Rule string
	Mode Mode
}

// IsJs returns true if this rule should be executed as JavaScript.
func (s SourceRule) IsJs() bool {
	return s.Mode == ModeJs
}

// DetectMode detects the mode of a rule string based on prefix.
func DetectMode(ruleStr string, isJSON bool) Mode {
	upper := strings.ToUpper(ruleStr)
	switch {
	case strings.HasPrefix(upper, "@CSS:"):
		return ModeDefault
	case strings.HasPrefix(ruleStr, "@@"):
		return ModeDefault
	case strings.HasPrefix(upper, "@XPATH:"):
		return ModeXPath
	case strings.HasPrefix(upper, "@JSON:"):
		return ModeJson
	case isJSON,
		strings.HasPrefix(ruleStr, "$."),
		strings.HasPrefix(ruleStr, "$["):
		return ModeJson
	case strings.HasPrefix(ruleStr, "//"):
		return ModeXPath
	case strings.HasPrefix(ruleStr, ":"):
		return ModeRegex
	default:
		return ModeDefault
	}
}

// SplitSourceRule splits a rule string into a list of SourceRules,
// handling <js>...</js> and @js: markers.
func SplitSourceRule(ruleStr string, isJSON bool, allInOne bool) []SourceRule {
	if ruleStr == "" {
		return nil
	}
	var result []SourceRule
	mode := ModeDefault
	start := 0

	// AllInOne regex mode
	if allInOne && strings.HasPrefix(ruleStr, ":") {
		mode = ModeRegex
		start = 1
	}

	matches := JS_PATTERN.FindAllStringSubmatchIndex(ruleStr, -1)
	if len(matches) == 0 {
		// No JS, entire rule is one piece
		r := ruleStr[start:]
		if strings.Contains(r, "##") && mode != ModeJs {
			mode = ModeRegex
		}
		result = append(result, SourceRule{Rule: cleanRule(r, mode, isJSON), Mode: mode})
		return result
	}

	for _, m := range matches {
		if m[0] > start {
			before := strings.TrimSpace(ruleStr[start:m[0]])
			if before != "" {
				r := before
				if strings.Contains(r, "##") && mode != ModeJs {
					mode = ModeRegex
				}
				result = append(result, SourceRule{Rule: cleanRule(r, mode, isJSON), Mode: mode})
			}
		}
		// JS part: group 1 = @js:..., group 2 = <js>...</js>
		jsCode := ""
		if m[2] != -1 && m[3] != -1 {
			jsCode = ruleStr[m[2]:m[3]]
		}
		if m[4] != -1 && m[5] != -1 {
			jsCode = ruleStr[m[4]:m[5]]
		}
		if jsCode != "" {
			result = append(result, SourceRule{Rule: jsCode, Mode: ModeJs})
		}
		start = m[1]
	}

	if start < len(ruleStr) {
		after := strings.TrimSpace(ruleStr[start:])
		if after != "" {
			result = append(result, SourceRule{Rule: cleanRule(after, mode, isJSON), Mode: mode})
		}
	}
	return result
}

// cleanRule strips the prefix and detects mode for a rule fragment.
func cleanRule(rule string, fallback Mode, isJSON bool) string {
	upper := strings.ToUpper(rule)
	switch {
	case strings.HasPrefix(upper, "@CSS:"):
		return rule[5:]
	case strings.HasPrefix(rule, "@@"):
		return rule[2:]
	case strings.HasPrefix(upper, "@XPATH:"):
		return rule[7:]
	case strings.HasPrefix(upper, "@JSON:"):
		return rule[6:]
	default:
		return rule
	}
}
