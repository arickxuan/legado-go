package server

import (
	"sync"

	"gopro/model"

	"github.com/gin-gonic/gin"
	"github.com/fastschema/qjs"
)

// SourceStore manages BookSource list with concurrent-safe access.
type SourceStore struct {
	mu      sync.RWMutex
	sources []model.BookSource
}

// NewSourceStore creates a store pre-loaded with the given sources.
func NewSourceStore(sources []model.BookSource) *SourceStore {
	return &SourceStore{sources: sources}
}

// All returns a copy of all sources.
func (s *SourceStore) All() []model.BookSource {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.BookSource, len(s.sources))
	copy(out, s.sources)
	return out
}

// Find returns the source matching the given bookSourceUrl.
func (s *SourceStore) Find(url string) (model.BookSource, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, src := range s.sources {
		if src.BookSourceUrl == url {
			return src, true
		}
	}
	return model.BookSource{}, false
}

// Add appends new sources (deduplicates by bookSourceUrl).
func (s *SourceStore) Add(newSources []model.BookSource) []model.BookSource {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing := make(map[string]struct{}, len(s.sources))
	for _, src := range s.sources {
		existing[src.BookSourceUrl] = struct{}{}
	}
	var added []model.BookSource
	for _, src := range newSources {
		if _, ok := existing[src.BookSourceUrl]; !ok {
			s.sources = append(s.sources, src)
			added = append(added, src)
			existing[src.BookSourceUrl] = struct{}{}
		}
	}
	return added
}

// Remove deletes sources matching the given bookSourceUrl. Returns true if found.
func (s *SourceStore) Remove(url string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, src := range s.sources {
		if src.BookSourceUrl == url {
			s.sources = append(s.sources[:i], s.sources[i+1:]...)
			return true
		}
	}
	return false
}

// Server wraps a Gin engine with source store and JS pool.
type Server struct {
	router  *gin.Engine
	store   *SourceStore
	jsPool  *qjs.Pool
}

// NewServer creates a new Server with the given sources.
func NewServer(sources []model.BookSource) *Server {
	pool := qjs.NewPool(5, qjs.Option{}, func(r *qjs.Runtime) error { return nil })
	s := &Server{
		router: gin.Default(),
		store:  NewSourceStore(sources),
		jsPool: pool,
	}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	api := s.router.Group("/api")
	{
		api.GET("/sources", s.handleListSources)
		api.POST("/sources", s.handleAddSources)
		api.DELETE("/sources", s.handleRemoveSource)

		api.POST("/search", s.handleSearch)
		api.POST("/bookinfo", s.handleBookInfo)
		api.POST("/chapters", s.handleChapters)
		api.POST("/content", s.handleContent)
	}
}

// Run starts the HTTP server on the given address (e.g. ":8080").
func (s *Server) Run(addr string) error {
	return s.router.Run(addr)
}
