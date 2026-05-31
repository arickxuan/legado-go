package webbook

import (
	"strings"

	"gopro/analyzer"
	"gopro/analyzeurl"
	"gopro/model"

	"github.com/fastschema/qjs"
)

// GetChapterList fetches the table of contents for a book.
// Handles pagination via nextTocUrl.
func GetChapterList(source model.BookSource, book *model.Book, jsPool *qjs.Pool) ([]model.BookChapter, error) {
	tocUrl := book.TocUrl
	if tocUrl == "" {
		tocUrl = book.BookUrl
	}

	var allChapters []model.BookChapter
	visited := make(map[string]bool)
	maxPages := 20 // Safety limit

	for i := 0; i < maxPages; i++ {
		if tocUrl == "" || visited[tocUrl] {
			break
		}
		visited[tocUrl] = true

		chapters, nextUrl, err := parseChapterPage(source, book, tocUrl, jsPool)
		if err != nil {
			if i == 0 {
				return nil, err
			}
			break
		}
		allChapters = append(allChapters, chapters...)
		tocUrl = nextUrl
	}

	// Assign indices
	for i := range allChapters {
		allChapters[i].Index = i
		allChapters[i].BookUrl = book.BookUrl
	}

	return allChapters, nil
}

// parseChapterPage parses a single chapter list page.
// Returns chapters and the next page URL.
func parseChapterPage(source model.BookSource, book *model.Book, pageUrl string, jsPool *qjs.Pool) ([]model.BookChapter, string, error) {
	aurl := analyzeurl.New(pageUrl, "", 0, source.BookSourceUrl, source.Header, jsPool)

	body, err := aurl.GetStrResponse(jsPool)
	if err != nil {
		return nil, "", err
	}
	if body == "" {
		return nil, "", nil
	}

	ar := analyzer.NewAnalyzeRule(body, aurl.FinalUrl, jsPool)
	tocRule := source.RuleToc

	// Get chapter list
	listRule := tocRule.ChapterList
	if listRule == "" {
		return nil, "", nil
	}

	// Check for reverse flag
	reverse := false
	if strings.HasPrefix(listRule, "-") {
		reverse = true
		listRule = listRule[1:]
	}

	// Get chapter elements
	chapterElements := ar.GetElements(listRule)
	if len(chapterElements) == 0 {
		return nil, "", nil
	}

	var chapters []model.BookChapter
	for _, elem := range chapterElements {
		elemAr := analyzer.NewAnalyzeRule(elem, aurl.FinalUrl, jsPool)

		chapterName := ""
		chapterUrl := ""

		if tocRule.ChapterName != "" {
			chapterName = elemAr.GetString(tocRule.ChapterName)
		}
		if tocRule.ChapterUrl != "" {
			chapterUrl = elemAr.GetString(tocRule.ChapterUrl)
		}

		// Resolve relative URL
		if chapterUrl != "" {
			chapterUrl = resolveURL(aurl.FinalUrl, chapterUrl)
		}

		if chapterName != "" || chapterUrl != "" {
			chapters = append(chapters, model.BookChapter{
				Title: chapterName,
				Url:   chapterUrl,
			})
		}
	}

	// Reverse if needed
	if reverse {
		for i, j := 0, len(chapters)-1; i < j; i, j = i+1, j-1 {
			chapters[i], chapters[j] = chapters[j], chapters[i]
		}
	}

	// Get next page URL
	nextUrl := ""
	if tocRule.NextTocUrl != "" {
		nextUrl = ar.GetString(tocRule.NextTocUrl)
		if nextUrl != "" {
			nextUrl = resolveURL(aurl.FinalUrl, nextUrl)
		}
	}

	return chapters, nextUrl, nil
}
