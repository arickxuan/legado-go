package server

import (
	"net/http"

	"gopro/model"
	"gopro/webbook"

	"github.com/gin-gonic/gin"
)

// --- Request types ---

type SearchReq struct {
	Keyword     string   `json:"keyword" binding:"required"`
	Page        int      `json:"page"`
	SourceUrls  []string `json:"sourceUrls"` // empty = search all
	Diagnostics bool     `json:"diagnostics"`
}

type BookInfoReq struct {
	SourceUrl string `json:"sourceUrl" binding:"required"`
	BookUrl   string `json:"bookUrl" binding:"required"`
}

type ChaptersReq struct {
	SourceUrl string     `json:"sourceUrl" binding:"required"`
	Book      model.Book `json:"book" binding:"required"`
}

type ContentReq struct {
	SourceUrl string            `json:"sourceUrl" binding:"required"`
	Book      model.Book        `json:"book" binding:"required"`
	Chapter   model.BookChapter `json:"chapter" binding:"required"`
}

// --- Handlers ---

func (s *Server) handleSearch(c *gin.Context) {
	var req SearchReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Page < 1 {
		req.Page = 1
	}

	sources := s.resolveSources(req.SourceUrls)
	if len(sources) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no matching sources found"})
		return
	}

	if req.Diagnostics {
		report := webbook.SearchBooksDetailed(sources, req.Keyword, req.Page, 10, s.jsPool)
		c.JSON(http.StatusOK, report)
		return
	}
	results := webbook.SearchBooks(sources, req.Keyword, req.Page, 10, s.jsPool)
	c.JSON(http.StatusOK, results)
}

func (s *Server) handleBookInfo(c *gin.Context) {
	var req BookInfoReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	src, ok := s.store.Find(req.SourceUrl)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "source not found"})
		return
	}

	book, err := webbook.GetBookInfo(src, req.BookUrl, s.jsPool)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, book)
}

func (s *Server) handleChapters(c *gin.Context) {
	var req ChaptersReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	src, ok := s.store.Find(req.SourceUrl)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "source not found"})
		return
	}

	chapters, err := webbook.GetChapterList(src, &req.Book, s.jsPool)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, chapters)
}

func (s *Server) handleContent(c *gin.Context) {
	var req ContentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	src, ok := s.store.Find(req.SourceUrl)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "source not found"})
		return
	}

	content, err := webbook.GetContent(src, &req.Book, &req.Chapter, s.jsPool)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"content": content})
}

func (s *Server) handleListSources(c *gin.Context) {
	c.JSON(http.StatusOK, s.store.All())
}

func (s *Server) handleAddSources(c *gin.Context) {
	var sources []model.BookSource
	if err := c.ShouldBindJSON(&sources); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	added := s.store.Add(sources)
	c.JSON(http.StatusOK, gin.H{"added": len(added)})
}

func (s *Server) handleRemoveSource(c *gin.Context) {
	url := c.Query("url")
	if url == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query param 'url' is required"})
		return
	}
	if s.store.Remove(url) {
		c.JSON(http.StatusOK, gin.H{"removed": url})
	} else {
		c.JSON(http.StatusNotFound, gin.H{"error": "source not found"})
	}
}

// resolveSources returns sources matching the given URLs, or all sources if urls is empty.
func (s *Server) resolveSources(urls []string) []model.BookSource {
	all := s.store.All()
	if len(urls) == 0 {
		return all
	}
	urlSet := make(map[string]struct{}, len(urls))
	for _, u := range urls {
		urlSet[u] = struct{}{}
	}
	var result []model.BookSource
	for _, src := range all {
		if _, ok := urlSet[src.BookSourceUrl]; ok {
			result = append(result, src)
		}
	}
	return result
}
