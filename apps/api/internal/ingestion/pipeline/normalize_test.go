package pipeline

import (
	"testing"
	"time"

	"github.com/mmcdole/gofeed"

	"speed-news/apps/api/internal/domain"
)

var testSource = domain.NewsSource{Slug: "test-source", DisplayName: "Test Source", SeriesSlug: domain.SeriesFormula1, LanguageCode: "pt"}

func TestCanonicalizeURLRemovesTrackingAndFragment(test *testing.T) {
	canonicalLink := CanonicalizeURL("http://WWW.Example.com/news/1?utm_source=rss&page=2#comments")
	if canonicalLink != "https://www.example.com/news/1?page=2" {
		test.Errorf("unexpected canonical link %q", canonicalLink)
	}
	if CanonicalizeURL("not a link") != "" {
		test.Error("invalid links must canonicalize to empty")
	}
}

func TestNormalizeFeedItemsCleansAndDeduplicatesIds(test *testing.T) {
	fetchedAt := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	futureDate := fetchedAt.Add(2 * time.Hour)
	feedItems := []*gofeed.Item{
		{
			Title:           "  Verstappen &amp; Norris  ",
			Link:            "https://example.com/a?utm_medium=feed",
			Description:     `<p>Resumo <b>com</b> HTML</p><img src="https://example.com/photo.jpg">`,
			PublishedParsed: &futureDate,
		},
		{Title: "", Link: "https://example.com/no-title"},
		{Title: "No link", Link: ""},
	}

	normalizedArticles := NormalizeFeedItems(feedItems, testSource, fetchedAt)
	if len(normalizedArticles) != 1 {
		test.Fatalf("expected 1 valid article, got %d", len(normalizedArticles))
	}
	normalizedArticle := normalizedArticles[0]
	if normalizedArticle.Title != "Verstappen & Norris" {
		test.Errorf("unexpected title %q", normalizedArticle.Title)
	}
	if normalizedArticle.Summary != "Resumo com HTML" {
		test.Errorf("unexpected summary %q", normalizedArticle.Summary)
	}
	if normalizedArticle.ImageURL != "https://example.com/photo.jpg" {
		test.Errorf("unexpected image %q", normalizedArticle.ImageURL)
	}
	if !normalizedArticle.PublishedAt.Equal(fetchedAt) {
		test.Errorf("future publish dates must be clamped to fetch time, got %s", normalizedArticle.PublishedAt)
	}
	if normalizedArticle.ID != BuildArticleID("https://example.com/a") {
		test.Error("id must ignore tracking parameters")
	}
}

func TestTruncateTextCutsOnWordBoundary(test *testing.T) {
	truncatedText := truncateText("uma frase bem longa para cortar", 14)
	if truncatedText != "uma frase bem…" {
		test.Errorf("unexpected truncation %q", truncatedText)
	}
}
