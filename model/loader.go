package model

import (
	"encoding/json"
	"os"
)

// LoadSources reads a JSON file and returns a list of BookSource.
// Disabled sources (enabled=false) are filtered out.
func LoadSources(jsonPath string) ([]BookSource, error) {
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, err
	}
	return ParseSources(data)
}

// ParseSources parses JSON bytes into a list of BookSource.
// Disabled sources (enabled=false) are filtered out.
func ParseSources(data []byte) ([]BookSource, error) {
	var sources []BookSource
	if err := json.Unmarshal(data, &sources); err != nil {
		return nil, err
	}
	result := make([]BookSource, 0, len(sources))
	for _, s := range sources {
		if s.Enabled {
			result = append(result, s)
		}
	}
	return result, nil
}
