package analyzer

import (
	"regexp"
	"strings"
)

// AnalyzeByRegex handles regex-based rule parsing.
type AnalyzeByRegex struct{}

// getElement applies a single regex to content and returns captured groups.
func regexGetElement(content string, pattern string) []string {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}
	matches := re.FindStringSubmatch(content)
	if matches == nil {
		return nil
	}
	return matches
}

// getElements applies a regex to content and returns all matches as list of groups.
func regexGetElements(content string, pattern string) [][]string {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}
	matches := re.FindAllStringSubmatch(content, -1)
	return matches
}

// applyOnlyOne applies the OnlyOne regex format: ##regex##replacement
func applyOnlyOne(content string, rule string) string {
	parts := strings.SplitN(rule, "##", 3)
	if len(parts) < 2 {
		return content
	}
	pattern := parts[0]
	replacement := ""
	if len(parts) >= 3 {
		replacement = parts[2]
	} else {
		replacement = parts[1]
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return content
	}
	return re.ReplaceAllString(content, replacement)
}

// applyAllInOne applies the AllInOne regex format (starts with :).
// Returns list of captured groups from the first match.
func applyAllInOne(content string, rule string) []string {
	// Remove leading ":"
	if strings.HasPrefix(rule, ":") {
		rule = rule[1:]
	}
	return regexGetElement(content, rule)
}
