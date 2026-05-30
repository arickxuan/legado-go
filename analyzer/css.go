package analyzer

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// cssGetElementList parses content with CSS/JSoup rules and returns outer HTML of matches.
// Used by GetElements so that child rules can re-parse each result.
// Handles || cascade: tries each alternative, returns first non-empty result.
func cssGetElementList(rule string, content string, baseUrl string) []string {
	if rule == "" {
		return []string{content}
	}
	if findTopLevelOr(rule) >= 0 {
		parts := splitByTopLevelOr(rule)
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			result := cssGetElementListSingle(part, content, baseUrl)
			if len(result) > 0 {
				return result
			}
		}
		return nil
	}
	return cssGetElementListSingle(rule, content, baseUrl)
}

// cssGetElementListSingle parses a single CSS/JSoup rule without || cascade.
func cssGetElementListSingle(rule string, content string, baseUrl string) []string {
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
// Handles || cascade: tries each alternative, returns first non-empty result.
func cssGetStringList(rule string, content string, baseUrl string) []string {
	if rule == "" {
		return []string{content}
	}

	// Handle || cascade (Legado alternatives)
	if idx := findTopLevelOr(rule); idx >= 0 {
		parts := splitByTopLevelOr(rule)
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" || part == `""` || part == `''` {
				continue
			}
			result := cssGetStringListSingle(part, content, baseUrl)
			if len(result) > 0 {
				return result
			}
		}
		return nil
	}

	return cssGetStringListSingle(rule, content, baseUrl)
}

// cssGetStringListSingle parses a single CSS/JSoup rule without || cascade.
func cssGetStringListSingle(rule string, content string, baseUrl string) []string {
	if rule == "" {
		return []string{content}
	}
	if strings.HasPrefix(rule, "@css:") || strings.HasPrefix(rule, "@CSS:") {
		return cssSelect(rule[5:], content)
	}
	if strings.HasPrefix(rule, "@") {
		return jsoupParse(rule[1:], content)
	}
	return jsoupParse(rule, content)
}

// findTopLevelOr returns the index of the first top-level || in the rule, or -1.
func findTopLevelOr(rule string) int {
	inQuote := byte(0)
	depth := 0
	for i := 0; i < len(rule); i++ {
		ch := rule[i]
		if inQuote != 0 {
			if ch == '\\' {
				i++
				continue
			}
			if ch == inQuote {
				inQuote = 0
			}
			continue
		}
		if ch == '\'' || ch == '"' || ch == '`' {
			inQuote = ch
			continue
		}
		if ch == '{' || ch == '[' || ch == '(' {
			depth++
			continue
		}
		if ch == '}' || ch == ']' || ch == ')' {
			if depth > 0 {
				depth--
			}
			continue
		}
		if ch == '|' && i+1 < len(rule) && rule[i+1] == '|' && depth == 0 {
			return i
		}
	}
	return -1
}

// splitByTopLevelOr splits a string by top-level || (not inside quotes or braces).
func splitByTopLevelOr(rule string) []string {
	var parts []string
	inQuote := byte(0)
	depth := 0
	last := 0
	for i := 0; i < len(rule); i++ {
		ch := rule[i]
		if inQuote != 0 {
			if ch == '\\' {
				i++
				continue
			}
			if ch == inQuote {
				inQuote = 0
			}
			continue
		}
		if ch == '\'' || ch == '"' || ch == '`' {
			inQuote = ch
			continue
		}
		if ch == '{' || ch == '[' || ch == '(' {
			depth++
			continue
		}
		if ch == '}' || ch == ']' || ch == ')' {
			if depth > 0 {
				depth--
			}
			continue
		}
		if ch == '|' && i+1 < len(rule) && rule[i+1] == '|' && depth == 0 {
			parts = append(parts, rule[last:i])
			last = i + 2
			i++
		}
	}
	parts = append(parts, rule[last:])
	return parts
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

// extractHTML returns the outer HTML of each individual element in the selections.
// Each *goquery.Selection may contain multiple DOM nodes; we iterate with .Each()
// to return one outer-HTML string per node.
func extractHTML(selections []*goquery.Selection) []string {
	var results []string
	for _, sel := range selections {
		sel.Each(func(i int, s *goquery.Selection) {
			html, err := goquery.OuterHtml(s)
			if err == nil && html != "" {
				results = append(results, html)
			}
		})
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

// htmlTags is a set of common HTML tag names used to distinguish
// tag selectors (e.g. "a", "img", "div") from attribute extraction (e.g. "title", "onclick").
var htmlTags = map[string]bool{
	"a": true, "abbr": true, "address": true, "area": true, "article": true,
	"aside": true, "audio": true, "b": true, "base": true, "bdi": true,
	"bdo": true, "blockquote": true, "body": true, "br": true, "button": true,
	"canvas": true, "caption": true, "cite": true, "code": true, "col": true,
	"colgroup": true, "data": true, "datalist": true, "dd": true, "del": true,
	"details": true, "dfn": true, "dialog": true, "div": true, "dl": true,
	"dt": true, "em": true, "embed": true, "fieldset": true, "figcaption": true,
	"figure": true, "footer": true, "form": true, "h1": true, "h2": true,
	"h3": true, "h4": true, "h5": true, "h6": true, "head": true,
	"header": true, "hr": true, "html": true, "i": true, "iframe": true,
	"img": true, "input": true, "ins": true, "kbd": true, "label": true,
	"legend": true, "li": true, "link": true, "main": true, "map": true,
	"mark": true, "math": true, "menu": true, "meta": true, "meter": true,
	"nav": true, "noscript": true, "object": true, "ol": true, "optgroup": true,
	"option": true, "output": true, "p": true, "param": true, "picture": true,
	"pre": true, "progress": true, "q": true, "rb": true, "rp": true,
	"rt": true, "rtc": true, "ruby": true, "s": true, "samp": true,
	"script": true, "section": true, "select": true, "slot": true, "small": true,
	"source": true, "span": true, "strong": true, "style": true, "sub": true,
	"summary": true, "sup": true, "svg": true, "table": true, "tbody": true,
	"td": true, "template": true, "textarea": true, "tfoot": true, "th": true,
	"thead": true, "time": true, "title": true, "tr": true, "track": true,
	"u": true, "ul": true, "var": true, "video": true, "wbr": true,
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
	// HTML tag names are selectors, not extraction instructions
	if htmlTags[first] {
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
// The ! modifier excludes elements at the given index (e.g. li!0 = all li except first).
func applyJsoupStep(sel *goquery.Selection, step string) []*goquery.Selection {
	// Handle negative index for list reversal
	if strings.HasPrefix(step, "-") && !strings.Contains(step, ".") {
		step = step[1:]
	}

	// Handle ! (exclude) modifier: e.g. "li!0" means "all li except index 0"
	excludeMode := false
	excludeIdx := -1
	if bangIdx := strings.Index(step, "!"); bangIdx > 0 {
		excludeMode = true
		idxStr := step[bangIdx+1:]
		step = step[:bangIdx]
		if idx, err := strconv.Atoi(idxStr); err == nil {
			excludeIdx = idx
		}
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
		// Multi-class: "mod block book-all-list" → ".mod.block.book-all-list"
		found = sel.Find("." + strings.ReplaceAll(selectorName, " ", "."))
	case "id":
		found = sel.Find("#" + selectorName)
	case "children":
		found = sel.Children()
	case "text":
		// text selector - handled in extractContent
		return []*goquery.Selection{sel}
	default:
		// In Legado JSoup format, the first part is always a type keyword (tag/class/id/etc).
		// But many book sources use bare tag names like "a.0" instead of "tag.a.0".
		// Try as CSS selector first; if no results and pieces[1] is numeric, treat as tag+index.
		found = sel.Find(step)
		if (found == nil || found.Length() == 0) && selectorName != "" {
			if _, err := strconv.Atoi(selectorName); err == nil {
				// e.g. "a.0" → find all <a> tags, then index=0
				found = sel.Find(selectorType)
				if found != nil && found.Length() > 0 {
					idx, _ := strconv.Atoi(selectorName)
					if idx >= 0 && idx < found.Length() {
						return []*goquery.Selection{found.Eq(idx)}
					}
					return nil
				}
			}
		}
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
	// Return all (or all-except-excluded)
	var results []*goquery.Selection
	found.Each(func(i int, s *goquery.Selection) {
		if excludeMode && i == excludeIdx {
			return // skip excluded index
		}
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
