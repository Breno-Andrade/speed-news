package news

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"

	"speed-news/apps/api/internal/domain"
	"speed-news/apps/api/internal/notifier"
	"speed-news/apps/api/internal/storage/sqlite"
)

var errFeedDown = errors.New("feed down")

type fakeFeedFetcher struct {
	itemsBySource map[string][]*gofeed.Item
	failingSource string
}

func (feedFetcher fakeFeedFetcher) FetchFeedItems(_ context.Context, newsSource domain.NewsSource) ([]*gofeed.Item, error) {
	if newsSource.Slug == feedFetcher.failingSource {
		return nil, errFeedDown
	}
	return feedFetcher.itemsBySource[newsSource.Slug], nil
}

type fakeShareImageFinder struct {
	mutex          sync.Mutex
	requestedPages []string
	failingPage    string
}

func (shareImageFinder *fakeShareImageFinder) FindShareImage(_ context.Context, articleURL string, allowedHost string) (string, error) {
	shareImageFinder.mutex.Lock()
	defer shareImageFinder.mutex.Unlock()
	shareImageFinder.requestedPages = append(shareImageFinder.requestedPages, articleURL)
	if articleURL == shareImageFinder.failingPage {
		return "", errors.New("timeout")
	}
	if allowedHost != "www.formula1.com" {
		return "", fmt.Errorf("unexpected allowed host %q", allowedHost)
	}
	return articleURL + "/cover.jpg", nil
}

func buildFeedItems(linkPrefix string, itemCount int, publishedAt time.Time) []*gofeed.Item {
	feedItems := make([]*gofeed.Item, 0, itemCount)
	for itemIndex := 0; itemIndex < itemCount; itemIndex++ {
		itemPublishedAt := publishedAt.Add(-time.Duration(itemIndex) * time.Minute)
		feedItems = append(feedItems, &gofeed.Item{
			Title:           fmt.Sprintf("Story %d", itemIndex),
			Link:            fmt.Sprintf("%s/story-%d", linkPrefix, itemIndex),
			PublishedParsed: &itemPublishedAt,
		})
	}
	return feedItems
}

var (
	formulaOneSource = domain.NewsSource{Slug: "formula1-com", DisplayName: "Formula1.com", SeriesSlug: domain.SeriesFormula1,
		LanguageCode: "en", HomepageURL: "https://www.formula1.com", ImageFromArticlePage: true}
	plainSource = domain.NewsSource{Slug: "plain", DisplayName: "Plain", SeriesSlug: domain.SeriesFormula1,
		LanguageCode: "pt", HomepageURL: "https://plain.example"}
	brokenSource = domain.NewsSource{Slug: "broken", DisplayName: "Broken", SeriesSlug: domain.SeriesFormula1, LanguageCode: "en"}
)

func newTestService(test *testing.T, feedFetcher FeedFetcher) (*Service, *sqlite.Store, *notifier.Broadcaster) {
	test.Helper()
	store, err := sqlite.OpenInMemory()
	if err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() { _ = store.Close() })
	broadcaster := notifier.NewBroadcaster()
	newsService := NewService(store, feedFetcher, []domain.NewsSource{formulaOneSource, plainSource, brokenSource}, broadcaster,
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	return newsService, store, broadcaster
}

func TestRefreshStoresArticlesAndPublishesTheCountDespiteABrokenSource(test *testing.T) {
	ctx := context.Background()
	recentTime := time.Now().UTC().Add(-time.Hour)
	newsService, _, broadcaster := newTestService(test, fakeFeedFetcher{
		itemsBySource: map[string][]*gofeed.Item{
			"formula1-com": buildFeedItems("https://www.formula1.com/en/latest", 2, recentTime),
			"plain":        buildFeedItems("https://plain.example/news", 3, recentTime),
		},
		failingSource: "broken",
	})
	unreadCountChannel, unsubscribe := broadcaster.Subscribe()
	defer unsubscribe()

	newsService.RefreshAllSources(ctx)

	select {
	case unreadCountChanged := <-unreadCountChannel:
		if unreadCountChanged.UnreadCount != 5 {
			test.Errorf("expected 5 unread published to the tray, got %d", unreadCountChanged.UnreadCount)
		}
	case <-time.After(time.Second):
		test.Fatal("the unread count was not published after the refresh")
	}

	listedArticles, err := newsService.ListArticles(ctx, true, 50)
	if err != nil || len(listedArticles) != 5 {
		test.Errorf("a broken source must not prevent the others from being stored: %d articles (%v)", len(listedArticles), err)
	}
}

func TestShareImagesAreLookedUpOnlyForFlaggedSourcesAndRetriedAfterFailures(test *testing.T) {
	ctx := context.Background()
	recentTime := time.Now().UTC().Add(-time.Hour)
	formulaOneItems := buildFeedItems("https://www.formula1.com/en/latest", maximumImageLookupsPerRefresh+3, recentTime)
	newsService, store, _ := newTestService(test, fakeFeedFetcher{itemsBySource: map[string][]*gofeed.Item{
		"formula1-com": formulaOneItems,
		"plain":        buildFeedItems("https://plain.example/news", 2, recentTime),
	}})
	shareImageFinder := &fakeShareImageFinder{failingPage: "https://www.formula1.com/en/latest/story-0"}
	newsService.WithShareImageFinder(shareImageFinder)

	newsService.RefreshAllSources(ctx)

	if len(shareImageFinder.requestedPages) != maximumImageLookupsPerRefresh {
		test.Fatalf("expected %d lookups in one refresh (rate limit), got %d", maximumImageLookupsPerRefresh, len(shareImageFinder.requestedPages))
	}
	for _, requestedPage := range shareImageFinder.requestedPages {
		if requestedPage[:len("https://www.formula1.com")] != "https://www.formula1.com" {
			test.Errorf("only the flagged source may be looked up, got %s", requestedPage)
		}
	}

	storedArticles, _ := store.ListArticles(ctx, sqlite.ArticleFilter{}, 50)
	imagesByLink := map[string]string{}
	for _, storedArticle := range storedArticles {
		imagesByLink[storedArticle.ArticleURL] = storedArticle.ImageURL
	}
	if imagesByLink["https://www.formula1.com/en/latest/story-1"] != "https://www.formula1.com/en/latest/story-1/cover.jpg" {
		test.Errorf("a found share image must be stored, got %q", imagesByLink["https://www.formula1.com/en/latest/story-1"])
	}
	if imagesByLink["https://plain.example/news/story-0"] != "" {
		test.Error("an unflagged source must keep its feed images only")
	}

	// Next refresh: the failed page is retried, the remaining ones are checked, found ones are not.
	shareImageFinder.requestedPages = nil
	shareImageFinder.failingPage = ""
	newsService.RefreshAllSources(ctx)
	if len(shareImageFinder.requestedPages) != 4 {
		test.Errorf("expected the failed page + 3 remaining pages on the next refresh, got %v", shareImageFinder.requestedPages)
	}
}
