package analyzer

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// cssGetElementList parses content with CSS/JSoup rules and returns outer HTML of matches.
// Used by GetElements so that child rules can re-parse each result.
func cssGetElementList(rule string, content string, baseUrl string) []string {
	if rule == "" {
		return []string{content}
	}
	if strings.HasPrefix(rule, "@css:") || strings.HasPrefix(rule, "@CSS:") {
		return cssSelectHTML(rule[5:], content)
	}
	if strings.HasPrefix(rule, "@") {
		return jsoupParseElements(rule[1:], content)
	}
	return jsoupParseElements(rule, content)
}

// cssSelectHTML returns outer HTML of elements matching a pure CSS selector.
func cssSelectHTML(selector string, content string) []string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		return nil
	}
	selection := doc.Find(selector)
	if selection.Length() == 0 {
		return nil
	}
	var results []string
	selection.Each(func(i int, s *goquery.Selection) {
		html, err := goquery.OuterHtml(s)
		if err == nil && html != "" {
			results = append(results, html)
		}
	})
	return results
}

// cssGetString parses content with CSS/JSoup default rules and returns first match.
func cssGetString(rule string, content string, baseUrl string) string {
	list := cssGetStringList(rule, content, baseUrl)
	if len(list) == 0 {
		return ""
	}
	return list[0]
}

// cssGetStringList parses content with CSS/JSoup default rules and returns all matches.
func cssGetStringList(rule string, content string, baseUrl string) []string {
	if rule == "" {
		return []string{content}
	}

	// Check if it's pure CSS selector (starts with @css:)
	if strings.HasPrefix(rule, "@css:") || strings.HasPrefix(rule, "@CSS:") {
		return cssSelect(rule[5:], content)
	}

	// Check if it starts with @ - JSoup default syntax
	if strings.HasPrefix(rule, "@") {
		return jsoupParse(rule[1:], content)
	}

	// Default: try as JSoup @ syntax
	return jsoupParse(rule, content)
}

// cssSelect applies a pure CSS selector.
func cssSelect(selector string, content string) []string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		return nil
	}
	selection := doc.Find(selector)
	if selection.Length() == 0 {
		return nil
	}
	var results []string
	selection.Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text != "" {
			results = append(results, text)
		}
	})
	return results
}

// jsoupParseElements is like jsoupParse but returns outer HTML of matched elements,
// so that results can be re-parsed by child rules in GetElements.
func jsoupParseElements(rule string, content string) []string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		return nil
	}
	parts := strings.Split(rule, "@")
	selections := []*goquery.Selection{doc.Selection}

	lastIdx := len(parts) - 1
	end := lastIdx
	if lastIdx > 0 {
		lastPart := strings.TrimSpace(parts[lastIdx])
		if isExtractionInstruction(lastPart) {
			end = lastIdx - 1
		}
	}

	for i := 0; i <= end; i++ {
		part := strings.TrimSpace(parts[i])
		if part == "" {
			continue
		}
		newSelections := []*goquery.Selection{}
		for _, sel := range selections {
			children := applyJsoupStep(sel, part)
			newSelections = append(newSelections, children...)
		}
		selections = newSelections
	}

	// If extraction instruction present (e.g. @href), use it
	if lastIdx > 0 && end == lastIdx-1 {
		return extractContent(selections, strings.TrimSpace(parts[lastIdx]))
	}
	// Default: return outer HTML for element list (needed by GetElements)
	return extractHTML(selections)
}

// extractHTML returns the outer HTML of each selection.
func extractHTML(selections []*goquery.Selection) []string {
	var results []string
	for _, sel := range selections {
		html, err := goquery.OuterHtml(sel)
		if err == nil && html != "" {
			results = append(results, html)
		}
	}
	return results
}

// jsoupParse handles JSoup default syntax with @ separator.
// Format: class.odd.0@tag.a.0@href
// The last @-part may be an extraction instruction (text, href, src, html, owntext, @attr).
func jsoupParse(rule string, content string) []string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		return nil
	}
	parts := strings.Split(rule, "@")
	selections := []*goquery.Selection{doc.Selection}

	// Separate element-finding steps from the final extraction instruction.
	lastIdx := len(parts) - 1
	end := lastIdx
	if lastIdx > 0 {
		lastPart := strings.TrimSpace(parts[lastIdx])
		if isExtractionInstruction(lastPart) {
			end = lastIdx - 1
		}
	}

	for i := 0; i <= end; i++ {
		part := strings.TrimSpace(parts[i])
		if part == "" {
			continue
		}
		newSelections := []*goquery.Selection{}
		for _, sel := range selections {
			children := applyJsoupStep(sel, part)
			newSelections = append(newSelections, children...)
		}
		selections = newSelections
	}

	// Apply extraction instruction if present, otherwise return outer HTML.
	// HTML is needed so that GetElements results can be re-parsed by child rules.
	if lastIdx > 0 && end == lastIdx-1 {
		return extractContent(selections, strings.TrimSpace(parts[lastIdx]))
	}
	return extractContent(selections, "text")
}

// isExtractionInstruction returns true if the step is a known extraction type
// (text, href, src, etc.) rather than an element-finding step (tag.li, class.foo, id.x).
func isExtractionInstruction(step string) bool {
	pieces := strings.Split(step, ".")
	first := strings.ToLower(pieces[0])
	switch first {
	// Pure extraction types
	case "text", "textnodes", "owntext", "html", "all",
		"href", "src":
		return true
	// Element-finding types — always selectors, never extraction
	case "tag", "class", "id", "children":
		return false
	}
	// Pure numeric index (e.g. "0", "-1") is not an extraction instruction
	if _, err := strconv.Atoi(first); err == nil {
		return false
	}
	// Multi-part step without known type prefix is a CSS selector (e.g. v-list-item)
	if len(pieces) > 1 {
		return false
	}
	// Single-word non-numeric: treat as HTML attribute name (e.g. onclick, data-id, title)
	return true
}

var negativeRegex = regexp.MustCompile(`^-`)

// applyJsoupStep applies one step of JSoup selector.
// Legado JSoup format: type.name.index (e.g. class.txt-list.0, tag.a.0, id.foo)
func applyJsoupStep(sel *goquery.Selection, step string) []*goquery.Selection {
	// Handle negative index for list reversal
	if strings.HasPrefix(step, "-") && !strings.Contains(step, ".") {
		step = step[1:]
	}

	pieces := strings.Split(step, ".")
	if len(pieces) < 2 {
		// Simple tag selector or CSS selector (e.g. "a", "#id", ".class")
		return []*goquery.Selection{sel.Find(step)}
	}

	selectorType := pieces[0]
	selectorName := ""
	if len(pieces) > 1 {
		selectorName = pieces[1]
	}
	index := -1
	if len(pieces) > 2 {
		if idx, err := strconv.Atoi(pieces[2]); err == nil {
			index = idx
		}
	}

	var found *goquery.Selection
	switch strings.ToLower(selectorType) {
	case "tag":
		if selectorName != "" {
			found = sel.Find(selectorName)
		} else {
			found = sel.Children()
		}
	case "class":
		found = sel.Find("." + selectorName)
	case "id":
		found = sel.Find("#" + selectorName)
	case "children":
		found = sel.Children()
	case "text":
		// text selector - handled in extractContent
		return []*goquery.Selection{sel}
	default:
		// Try as CSS selector directly
		found = sel.Find(step)
	}

	if found == nil || found.Length() == 0 {
		return nil
	}

	if index >= 0 {
		if index >= found.Length() {
			return nil
		}
		return []*goquery.Selection{found.Eq(index)}
	}
	if index < 0 && index != -1 {
		// Negative index: count from end
		idx := found.Length() + index
		if idx < 0 {
			return nil
		}
		return []*goquery.Selection{found.Eq(idx)}
	}
	// Return all
	var results []*goquery.Selection
	found.Each(func(i int, s *goquery.Selection) {
		results = append(results, s)
	})
	return results
}

// extractContent extracts the final content from selections.
// lastStep may be a content type (text, href, src, html) or an attribute name (onclick, data-id).
func extractContent(selections []*goquery.Selection, lastStep string) []string {
	var results []string
	pieces := strings.Split(lastStep, ".")
	contentType := "text"
	if len(pieces) >= 2 {
		firstPiece := strings.ToLower(pieces[0])
		switch firstPiece {
		case "tag", "class", "id", "children":
			// Selector step (e.g. tag.li): elements already selected, extract text
			contentType = "text"
		default:
			contentType = strings.ToLower(pieces[1])
		}
	} else {
		// Single-word last step: determine extraction type
		lower := strings.ToLower(strings.TrimSpace(lastStep))
		switch lower {
		case "text", "textnodes", "owntext":
			contentType = "text"
		case "html", "all":
			contentType = "html"
		case "href", "src":
			contentType = lower
		default:
			// Treat as HTML attribute name (e.g. onclick, data-id, title)
			contentType = lower
		}
	}

	for _, sel := range selections {
		var text string
		switch contentType {
		case "text":
			text = strings.TrimSpace(sel.Text())
		case "html", "all":
			html, _ := sel.Html()
			text = strings.TrimSpace(html)
		case "href", "src":
			text, _ = sel.Attr(contentType)
			text = strings.TrimSpace(text)
		default:
			// Try as HTML attribute; fall back to text content
			text, _ = sel.Attr(contentType)
			if text == "" {
				text = strings.TrimSpace(sel.Text())
			} else {
				text = strings.TrimSpace(text)
			}
		}
		if text != "" {
			results = append(results, text)
		}
	}
	return results
}
