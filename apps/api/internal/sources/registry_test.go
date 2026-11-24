package sources

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultSourcesAreValid(test *testing.T) {
	newsSources, err := LoadDefaultSources()
	if err != nil {
		test.Fatal(err)
	}
	if len(newsSources) < 6 {
		test.Fatalf("expected the default source list, got %d sources", len(newsSources))
	}
	languageCodes := map[string]bool{}
	for _, newsSource := range newsSources {
		languageCodes[newsSource.LanguageCode] = true
		feedURL, err := url.Parse(newsSource.FeedURL)
		if err != nil || feedURL.Scheme != "https" {
			test.Errorf("%s: feeds must be https, got %q", newsSource.Slug, newsSource.FeedURL)
		}
		if newsSource.ImageFromArticlePage {
			homepageURL, err := url.Parse(newsSource.HomepageURL)
			if err != nil || homepageURL.Hostname() == "" {
				test.Errorf("%s: imageFromArticlePage needs a homepageUrl to restrict page lookups", newsSource.Slug)
			}
		}
	}
	if !languageCodes["pt"] || !languageCodes["en"] {
		test.Errorf("the default list must offer Portuguese and English, got %v", languageCodes)
	}
}

func TestParseSourcesRejectsInvalidLists(test *testing.T) {
	scenarios := map[string]string{
		"missing feedUrl": `sources: [{slug: a, seriesSlug: f1, feedKind: rss}]`,
		"unknown kind":    `sources: [{slug: a, seriesSlug: f1, feedKind: atom-ish, feedUrl: "https://x"}]`,
		"duplicate slug": `sources: [{slug: a, seriesSlug: f1, feedKind: rss, feedUrl: "https://x"},
			{slug: a, seriesSlug: f1, feedKind: rss, feedUrl: "https://y"}]`,
		"not yaml": `sources: [this is: [broken`,
	}
	for scenarioName, sourcesYAML := range scenarios {
		if _, err := ParseSources([]byte(sourcesYAML)); err == nil {
			test.Errorf("%s: expected an error", scenarioName)
		}
	}
}

func TestLoadSourcesFile(test *testing.T) {
	sourcesFile := filepath.Join(test.TempDir(), "sources.yaml")
	if err := os.WriteFile(sourcesFile, []byte(`sources: [{slug: local, seriesSlug: f1, languageCode: pt, feedKind: rss, feedUrl: "http://127.0.0.1/rss"}]`), 0o644); err != nil {
		test.Fatal(err)
	}
	newsSources, err := LoadSourcesFile(sourcesFile)
	if err != nil || len(newsSources) != 1 || newsSources[0].Slug != "local" {
		test.Errorf("expected the file's single source, got %+v (%v)", newsSources, err)
	}
	if _, err := LoadSourcesFile(filepath.Join(test.TempDir(), "missing.yaml")); err == nil || !strings.Contains(err.Error(), "read sources file") {
		test.Errorf("a missing file must be reported clearly, got %v", err)
	}
}
