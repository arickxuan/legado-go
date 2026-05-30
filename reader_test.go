package main

import (
	"encoding/json"
	"fmt"
	"log"
	"testing"

	"gopro/model"
	"gopro/webbook"

	"github.com/fastschema/qjs"
)

func TestLoadSources(t *testing.T) {
	sources, err := model.LoadSources("doc/example.json")
	if err != nil {
		t.Fatalf("Failed to load sources: %v", err)
	}

	if len(sources) == 0 {
		t.Fatal("No sources loaded")
	}

	log.Printf("Loaded %d enabled sources", len(sources))
	for i, s := range sources {
		log.Printf("  [%d] %s - %s", i+1, s.BookSourceName, s.BookSourceUrl)
	}
}

func TestSearchBooks(t *testing.T) {
	sources, err := model.LoadSources("doc/example.json")
	if err != nil {
		t.Fatalf("Failed to load sources: %v", err)
	}

	// Test with sources that have simple search rules (not heavy JS)
	var testSources []model.BookSource
	for _, s := range sources {
		// Skip sources with complex JS in search URL
		if len(s.SearchUrl) < 100 && s.RuleSearch.BookList != "" {
			testSources = append(testSources, s)
		}
		if len(testSources) >= 3 {
			break
		}
	}

	if len(testSources) == 0 {
		t.Skip("No suitable sources found for testing")
	}

	t.Logf("Testing with %d sources", len(testSources))
	for _, s := range testSources {
		t.Logf("  - %s: %s", s.BookSourceName, s.SearchUrl)
	}

	pool := qjs.NewPool(3, qjs.Option{}, func(r *qjs.Runtime) error {
		return nil
	})
	results := webbook.SearchBooks(testSources, "斗破苍穹", 1, 3, pool)
	t.Logf("Found %d results", len(results))

	if len(results) > 0 {
		data, _ := json.MarshalIndent(results[0], "", "  ")
		fmt.Printf("\nFirst result:\n%s\n", string(data))
	}
}
