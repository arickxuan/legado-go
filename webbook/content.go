package webbook

import (
	"gopro/analyzer"
	"gopro/analyzeurl"
	"gopro/model"
	"regexp"
	"strings"

	"github.com/fastschema/qjs"
)

// GetContent fetches the content of a chapter.
// Handles pagination via nextContentUrl.
func GetContent(source model.BookSource, book *model.Book, chapter *model.BookChapter, jsPool *qjs.Pool) (string, error) {
	if chapter.Url == "" {
		return "", nil
	}

	var contentParts []string
	visited := make(map[string]bool)
	maxPages := 10 // Safety limit for content pagination

	currentUrl := chapter.Url
	for i := 0; i < maxPages; i++ {
		if currentUrl == "" || visited[currentUrl] {
			break
		}
		visited[currentUrl] = true

		content, nextUrl, err := parseContentPage(source, book, chapter, currentUrl, jsPool)
		if err != nil {
			if i == 0 {
				return "", err
			}
			break
		}
		if content != "" {
			contentParts = append(contentParts, content)
		}
		currentUrl = nextUrl
	}

	return strings.Join(contentParts, "\n"), nil
}

// parseContentPage parses a single content page.
// Returns content text and the next page URL.
func parseContentPage(source model.BookSource, book *model.Book, chapter *model.BookChapter, pageUrl string, jsPool *qjs.Pool) (string, string, error) {
	aurl := analyzeurl.New(pageUrl, "", 0, source.BookSourceUrl, source.Header, jsPool)

	body, err := aurl.GetStrResponse(jsPool)
	if err != nil {
		return "", "", err
	}
	if body == "" {
		return "", "", nil
	}

	ar := analyzer.NewAnalyzeRule(body, aurl.FinalUrl, jsPool)
	contentRule := source.RuleContent

	// Get content
	content := ""
	if contentRule.Content != "" {
		content = ar.GetString(contentRule.Content)
	}

	// Clean up content
	if content != "" {
		content = strings.TrimSpace(content)
		if contentRule.ReplaceRegex != "" {
			content = applyReplaceRegex(content, contentRule.ReplaceRegex)
		}
		// Remove common ad patterns
		content = cleanContent(content)
	}

	// Get next content URL
	nextUrl := ""
	if contentRule.NextContentUrl != "" {
		nextUrl = ar.GetString(contentRule.NextContentUrl)
		if nextUrl != "" {
			nextUrl = resolveURL(aurl.FinalUrl, nextUrl)
		}
	}

	return content, nextUrl, nil
}

func applyReplaceRegex(content string, rule string) string {
	parts := strings.SplitN(rule, "##", 3)
	pattern := rule
	replacement := ""
	if len(parts) >= 2 {
		pattern = parts[0]
		replacement = parts[1]
	}
	if len(parts) == 3 {
		replacement = parts[2]
	}
	if strings.TrimSpace(pattern) == "" {
		return content
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return content
	}
	return re.ReplaceAllString(content, replacement)
}

// cleanContent removes common ad patterns and cleans up the content.
func cleanContent(content string) string {
	// Remove common ad phrases
	adPatterns := []string{
		"请到最新章节",
		"手机阅读：",
		"一秒记住",
		"请记住本书首发域名",
		"天才一秒记住",
		"最新章节！",
	}

	for _, pattern := range adPatterns {
		if idx := strings.Index(content, pattern); idx != -1 {
			// Find the end of the line
			endIdx := strings.Index(content[idx:], "\n")
			if endIdx != -1 {
				content = content[:idx] + content[idx+endIdx+1:]
			} else {
				content = content[:idx]
			}
		}
	}

	// Clean up whitespace
	lines := strings.Split(content, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return strings.Join(cleaned, "\n")
}
