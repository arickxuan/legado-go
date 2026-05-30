package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"gopro/model"
	"gopro/server"
	"gopro/webbook"

	"github.com/fastschema/qjs"
)

var (
	jsPoolOnce sync.Once
	jsPool     *qjs.Pool
)

// getJSPool returns a singleton JS pool.
func getJSPool() *qjs.Pool {
	jsPoolOnce.Do(func() {
		poolSize := 5
		jsPool = qjs.NewPool(poolSize, qjs.Option{}, func(r *qjs.Runtime) error {
			return nil
		})
	})
	return jsPool
}

// SearchBooks searches books across multiple sources.
func SearchBooks(sources []model.BookSource, key string, page int) []model.SearchBook {
	pool := getJSPool()
	return webbook.SearchBooks(sources, key, page, 10, pool)
}

// GetBookInfo gets book information from a source.
func GetBookInfo(source model.BookSource, bookUrl string) (*model.Book, error) {
	pool := getJSPool()
	return webbook.GetBookInfo(source, bookUrl, pool)
}

// GetChapterList gets the table of contents for a book.
func GetChapterList(source model.BookSource, book *model.Book) ([]model.BookChapter, error) {
	pool := getJSPool()
	return webbook.GetChapterList(source, book, pool)
}

// GetContent gets the content of a chapter.
func GetContent(source model.BookSource, book *model.Book, chapter *model.BookChapter) (string, error) {
	pool := getJSPool()
	return webbook.GetContent(source, book, chapter, pool)
}

// isHTTPMode checks if -mode=http was passed before full flag parsing.
func isHTTPMode() bool {
	for _, arg := range os.Args[1:] {
		if arg == "-mode=http" || arg == "--mode=http" {
			return true
		}
	}
	return false
}

func main() {
	if isHTTPMode() {
		fs := flag.NewFlagSet("http", flag.ExitOnError)
		port := fs.String("port", defaultPort(), "HTTP server port")
		sourcesPath := fs.String("sources", "doc/example.json", "path to book sources JSON file")
		mode := fs.String("mode", "http", "running mode")
		_ = mode
		fs.Parse(os.Args[1:])
		runHTTP(*sourcesPath, *port)
	} else {
		runCLI()
	}
}

func defaultPort() string {
	if p := os.Getenv("PORT"); p != "" {
		return p
	}
	return "8080"
}

func runHTTP(sourcesPath string, port string) {
	sources, err := model.LoadSources(sourcesPath)
	if err != nil {
		log.Fatalf("Failed to load sources: %v", err)
	}
	log.Printf("Loaded %d enabled sources from %s", len(sources), sourcesPath)

	srv := server.NewServer(sources)
	addr := ":" + port
	log.Printf("HTTP server starting on %s", addr)
	if err := srv.Run(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func runCLI() {
	if len(os.Args) < 3 {
		printUsage()
		return
	}

	jsonFile := os.Args[1]
	sources, err := model.LoadSources(jsonFile)
	if err != nil {
		log.Fatalf("Failed to load sources: %v", err)
	}
	log.Printf("Loaded %d enabled sources", len(sources))

	command := os.Args[2]
	switch command {
	case "search":
		if len(os.Args) < 4 {
			log.Fatal("Usage: gopro <json> search <keyword>")
		}
		key := strings.Join(os.Args[3:], " ")
		searchAndDisplay(sources, key)
	default:
		printUsage()
	}
}

func searchAndDisplay(sources []model.BookSource, key string) {
	log.Printf("Searching for: %s", key)
	results := SearchBooks(sources, key, 1)
	log.Printf("Found %d results", len(results))

	data, _ := json.MarshalIndent(results, "", "  ")
	fmt.Println(string(data))
}

func printUsage() {
	fmt.Println(`GoPro - Legado Book Source Reader (Go Implementation)

Usage:
  # HTTP mode
  gopro -mode=http [-port=8080] [-sources=doc/example.json]

  # CLI mode (default)
  gopro <json_file> search <keyword>

Example:
  gopro -mode=http -port=9090 -sources=doc/example.json
  gopro doc/example.json search "斗破苍穹"`)
}
