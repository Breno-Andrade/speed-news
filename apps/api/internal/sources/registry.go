// Package sources loads the list of news sources the app reads.
package sources

import (
	_ "embed"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"speed-news/apps/api/internal/domain"
)

//go:embed default_sources.yaml
var defaultSourcesYAML []byte

type sourcesFile struct {
	Sources []domain.NewsSource `yaml:"sources"`
}

// LoadDefaultSources parses the embedded source list.
func LoadDefaultSources() ([]domain.NewsSource, error) {
	return ParseSources(defaultSourcesYAML)
}

// LoadSourcesFile reads a source list from disk (used by tests and the E2E suite).
func LoadSourcesFile(sourcesFilePath string) ([]domain.NewsSource, error) {
	sourcesYAML, err := os.ReadFile(sourcesFilePath)
	if err != nil {
		return nil, fmt.Errorf("read sources file: %w", err)
	}
	return ParseSources(sourcesYAML)
}

// ParseSources parses and validates a YAML source list.
func ParseSources(sourcesYAML []byte) ([]domain.NewsSource, error) {
	var parsedFile sourcesFile
	if err := yaml.Unmarshal(sourcesYAML, &parsedFile); err != nil {
		return nil, fmt.Errorf("parse sources yaml: %w", err)
	}

	seenSlugs := make(map[string]bool, len(parsedFile.Sources))
	for _, newsSource := range parsedFile.Sources {
		if newsSource.Slug == "" || newsSource.FeedURL == "" || newsSource.SeriesSlug == "" {
			return nil, fmt.Errorf("source %q is missing slug, feedUrl or seriesSlug", newsSource.Slug)
		}
		if newsSource.FeedKind != "rss" {
			return nil, fmt.Errorf("source %q has unsupported feedKind %q", newsSource.Slug, newsSource.FeedKind)
		}
		if seenSlugs[newsSource.Slug] {
			return nil, fmt.Errorf("duplicated source slug %q", newsSource.Slug)
		}
		seenSlugs[newsSource.Slug] = true
	}
	return parsedFile.Sources, nil
}
