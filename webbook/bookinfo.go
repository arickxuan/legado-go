package webbook

import (
	"gopro/analyzer"
	"gopro/analyzeurl"
	"gopro/model"

	"github.com/fastschema/qjs"
)

// GetBookInfo fetches book information from the book detail page.
// Returns a Book with details populated and the table of contents URL.
func GetBookInfo(source model.BookSource, bookUrl string, jsPool *qjs.Pool) (*model.Book, error) {
	// Build URL to fetch book detail page
	aurl := analyzeurl.New(bookUrl, "", 0, source.BookSourceUrl, source.Header, jsPool)

	body, err := aurl.GetStrResponse(jsPool)
	if err != nil {
		return nil, err
	}
	if body == "" {
		return nil, nil
	}

	ar := analyzer.NewAnalyzeRule(body, aurl.FinalUrl, jsPool)
	infoRule := source.RuleBookInfo

	// Execute init rule if present
	if infoRule.Init != "" {
		initResult := ar.GetElement(infoRule.Init)
		if initResult != "" {
			ar.SetContent(initResult, aurl.FinalUrl)
		}
	}

	book := &model.Book{
		BookUrl:    bookUrl,
		Origin:     source.BookSourceUrl,
		OriginName: source.BookSourceName,
	}

	if infoRule.Name != "" {
		book.Name = ar.GetString(infoRule.Name)
	}
	if infoRule.Author != "" {
		book.Author = ar.GetString(infoRule.Author)
	}
	if infoRule.CoverUrl != "" {
		coverUrl := ar.GetString(infoRule.CoverUrl)
		if coverUrl != "" {
			book.CoverUrl = resolveURL(aurl.FinalUrl, coverUrl)
		}
	}
	if infoRule.Intro != "" {
		book.Intro = ar.GetString(infoRule.Intro)
	}
	if infoRule.Kind != "" {
		book.Kind = ar.GetString(infoRule.Kind)
	}
	if infoRule.LastChapter != "" {
		book.LastChapter = ar.GetString(infoRule.LastChapter)
	}
	if infoRule.WordCount != "" {
		book.WordCount = ar.GetString(infoRule.WordCount)
	}
	if infoRule.TocUrl != "" {
		tocUrl := ar.GetString(infoRule.TocUrl)
		if tocUrl != "" {
			book.TocUrl = resolveURL(aurl.FinalUrl, tocUrl)
		}
	}

	// If no tocUrl found, use the current page as toc
	if book.TocUrl == "" {
		book.TocUrl = aurl.FinalUrl
	}

	return book, nil
}
