package analyzer

import (
	"strings"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

// xpathGetString applies an XPath rule and returns the first match.
func xpathGetString(rule string, content string) string {
	list := xpathGetStringList(rule, content)
	if len(list) == 0 {
		return ""
	}
	return list[0]
}

// xpathGetStringList applies an XPath rule and returns all matches.
func xpathGetStringList(rule string, content string) []string {
	if rule == "" || content == "" {
		return nil
	}
	// Strip @XPath: prefix if present
	rule = strings.TrimPrefix(rule, "@XPath:")
	rule = strings.TrimPrefix(rule, "@xpath:")

	doc, err := htmlquery.Parse(strings.NewReader(content))
	if err != nil {
		return nil
	}

	// Find matching nodes
	nodes, err := htmlquery.QueryAll(doc, rule)
	if err != nil {
		return nil
	}

	var results []string
	for _, n := range nodes {
		val := extractXNodeValue(n)
		if val != "" {
			results = append(results, val)
		}
	}
	return results
}

// extractXNodeValue extracts the text value from an HTML node.
func extractXNodeValue(n *html.Node) string {
	if n == nil {
		return ""
	}
	switch n.Type {
	case html.TextNode:
		return strings.TrimSpace(n.Data)
	case html.ElementNode, html.DocumentNode:
		// Check for attribute value (e.g. /@content, /@href)
		if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
			return strings.TrimSpace(n.FirstChild.Data)
		}
		// For attribute nodes, return the value
		return strings.TrimSpace(htmlquery.InnerText(n))
	default:
		return strings.TrimSpace(htmlquery.InnerText(n))
	}
}
