package webbook

import (
	"log"
	"sync"

	"gopro/analyzer"
	"gopro/analyzeurl"
	"gopro/model"

	"github.com/fastschema/qjs"
)

// SearchBooks searches books across multiple sources concurrently.
// Returns aggregated search results from all enabled sources.
func SearchBooks(sources []model.BookSource, key string, page int, maxConcurrency int, jsPool *qjs.Pool) []model.SearchBook {
	if maxConcurrency <= 0 {
		maxConcurrency = 10
	}

	var (
		mu      sync.Mutex
		results []model.SearchBook
		wg      sync.WaitGroup
		sem     = make(chan struct{}, maxConcurrency)
	)

	for _, source := range sources {
		if !source.Enabled || source.SearchUrl == "" {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(src model.BookSource) {
			defer wg.Done()
			defer func() { <-sem }()
			books, err := searchFromSource(src, key, page, jsPool)
			if err != nil {
				return
			}
			mu.Lock()
			results = append(results, books...)
			mu.Unlock()
		}(source)
	}
	wg.Wait()
	return results
}

// searchFromSource searches books from a single source.
func searchFromSource(source model.BookSource, key string, page int, jsPool *qjs.Pool) ([]model.SearchBook, error) {
	aurl := analyzeurl.New(source.SearchUrl, key, page, source.BookSourceUrl, source.Header, jsPool)
	aurl.SetComment(source.BookSourceComment)
	log.Printf("[%s] request: %s", source.BookSourceName, aurl.FinalUrl)

	body, err := aurl.GetStrResponse(jsPool)
	if err != nil {
		log.Printf("[%s] request failed: %v", source.BookSourceName, err)
		return nil, err
	}
	if body == "" {
		log.Printf("[%s] empty response", source.BookSourceName)
		return nil, nil
	}

	ar := analyzer.NewAnalyzeRule(body, aurl.FinalUrl, jsPool)
	bookListRule := source.RuleSearch.BookList
	if bookListRule == "" {
		return nil, nil
	}

	bookItems := ar.GetElements(bookListRule)
	//log.Printf("[%s] bookItems=%d", source.BookSourceName, len(bookItems))
	if len(bookItems) == 0 {
		log.Printf("[%s] no results matched (body len=%d, bookItems=%d)", source.BookSourceName, len(body), len(bookItems))
		return nil, nil
	}

	var books []model.SearchBook
	for _, item := range bookItems {
		itemAr := analyzer.NewAnalyzeRule(item, aurl.FinalUrl, jsPool)
		book := model.SearchBook{
			Name:        itemAr.GetString(source.RuleSearch.Name),
			Author:      itemAr.GetString(source.RuleSearch.Author),
			CoverUrl:    itemAr.GetString(source.RuleSearch.CoverUrl),
			Intro:       itemAr.GetString(source.RuleSearch.Intro),
			Kind:        itemAr.GetString(source.RuleSearch.Kind),
			LastChapter: itemAr.GetString(source.RuleSearch.LastChapter),
			WordCount:   itemAr.GetString(source.RuleSearch.WordCount),
			BookUrl:     itemAr.GetString(source.RuleSearch.BookUrl),
			Origin:      source.BookSourceUrl,
			OriginName:  source.BookSourceName,
		}
		//log.Printf("[%s] book=%s", source.BookSourceName, book)

		// Resolve relative book URL
		if book.BookUrl != "" {
			book.BookUrl = resolveURL(aurl.FinalUrl, book.BookUrl)
		}
		if book.CoverUrl != "" {
			book.CoverUrl = resolveURL(aurl.FinalUrl, book.CoverUrl)
		}

		if book.Name != "" {
			books = append(books, book)
		}
	}
	log.Printf("[%s] books=%d", source.BookSourceName, len(books))
	return books, nil
}

func resolveURL(baseURL string, ref string) string {
	if ref == "" {
		return baseURL
	}
	if len(ref) > 7 && (ref[:7] == "http://" || ref[:8] == "https://") {
		return ref
	}
	// Simple base URL resolution
	if ref[0] == '/' {
		// Find scheme + host
		idx := len(baseURL)
		if i := indexOf(baseURL, "://"); i != -1 {
			idx = i + 3
			if j := indexOf(baseURL[idx:], "/"); j != -1 {
				idx = idx + j
			}
		}
		return baseURL[:idx] + ref
	}
	// Relative path
	lastSlash := lastIndexOf(baseURL, "/")
	if lastSlash == -1 {
		return baseURL + "/" + ref
	}
	return baseURL[:lastSlash+1] + ref
}

func indexOf(s string, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func lastIndexOf(s string, c string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i:i+len(c)] == c {
			return i
		}
	}
	return -1
}
