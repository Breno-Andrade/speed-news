// Package news orchestrates fetching, storing and reading articles.
package news

import (
	"context"
	"log/slog"
	"net/url"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"

	"speed-news/apps/api/internal/domain"
	"speed-news/apps/api/internal/notifier"
	"speed-news/apps/api/internal/storage/sqlite"
)

const (
	// Articles older than this on the first sight are stored as already read.
	autoReadAge = 48 * time.Hour
	// Articles older than this are deleted.
	retentionPeriod = 30 * 24 * time.Hour
)

// Share image lookups per source and refresh: the first run of a source may find a whole feed
// without pictures; the rest is picked up by the next refreshes instead of one burst of requests.
const maximumImageLookupsPerRefresh = 12

// FeedFetcher is the port implemented by the RSS fetcher.
type FeedFetcher interface {
	FetchFeedItems(ctx context.Context, newsSource domain.NewsSource) ([]*gofeed.Item, error)
}

// ShareImageFinder is the port implemented by the og:image reader.
type ShareImageFinder interface {
	FindShareImage(ctx context.Context, articleURL string, allowedHost string) (string, error)
}

// Service is the entry point for everything related to articles.
type Service struct {
	store            *sqlite.Store
	feedFetcher      FeedFetcher
	shareImageFinder ShareImageFinder
	newsSources      []domain.NewsSource
	broadcaster      *notifier.Broadcaster
	logger           *slog.Logger
	currentTime      func() time.Time
}

// WithShareImageFinder enables the og:image lookup for sources flagged imageFromArticlePage.
func (service *Service) WithShareImageFinder(shareImageFinder ShareImageFinder) *Service {
	service.shareImageFinder = shareImageFinder
	return service
}

func NewService(store *sqlite.Store, feedFetcher FeedFetcher, newsSources []domain.NewsSource,
	broadcaster *notifier.Broadcaster, logger *slog.Logger) *Service {
	return &Service{
		store:       store,
		feedFetcher: feedFetcher,
		newsSources: newsSources,
		broadcaster: broadcaster,
		logger:      logger,
		currentTime: time.Now,
	}
}

// RefreshAllSources fetches every source in parallel. A failing source never blocks the others.
func (service *Service) RefreshAllSources(ctx context.Context) {
	fetchedAt := service.currentTime().UTC()
	var waitGroup sync.WaitGroup
	for _, newsSource := range service.newsSources {
		waitGroup.Add(1)
		go func(newsSource domain.NewsSource) {
			defer waitGroup.Done()
			service.refreshSource(ctx, newsSource, fetchedAt)
		}(newsSource)
	}
	waitGroup.Wait()

	if err := service.store.DeleteArticlesPublishedBefore(ctx, fetchedAt.Add(-retentionPeriod)); err != nil {
		service.logger.Warn("delete old articles failed", "error", err)
	}
	service.PublishUnreadCount(ctx)
}

func (service *Service) refreshSource(ctx context.Context, newsSource domain.NewsSource, fetchedAt time.Time) {
	feedItems, err := service.feedFetcher.FetchFeedItems(ctx, newsSource)
	if err != nil {
		service.logger.Warn("fetch source failed", "source", newsSource.Slug, "error", err)
		return
	}
	normalizedArticles := NormalizeFeedItems(feedItems, newsSource, fetchedAt)
	insertedCount, err := service.store.InsertArticles(ctx, normalizedArticles, fetchedAt.Add(-autoReadAge))
	if err != nil {
		service.logger.Warn("store articles failed", "source", newsSource.Slug, "error", err)
		return
	}
	service.logger.Info("source refreshed", "source", newsSource.Slug, "items", len(feedItems), "new", insertedCount)

	if newsSource.ImageFromArticlePage && service.shareImageFinder != nil {
		service.fillShareImages(ctx, newsSource)
	}
}

// fillShareImages reads og:image for articles of the source that still have no picture.
// Each article page is checked once, found or not.
func (service *Service) fillShareImages(ctx context.Context, newsSource domain.NewsSource) {
	homepageURL, err := url.Parse(newsSource.HomepageURL)
	if err != nil || homepageURL.Hostname() == "" {
		service.logger.Warn("image lookup skipped: source has no valid homepageUrl", "source", newsSource.Slug)
		return
	}
	articlePages, err := service.store.ListArticlesAwaitingImageLookup(ctx, newsSource.Slug, maximumImageLookupsPerRefresh)
	if err != nil {
		service.logger.Warn("list articles awaiting image failed", "source", newsSource.Slug, "error", err)
		return
	}

	foundCount := 0
	for _, articlePage := range articlePages {
		if ctx.Err() != nil {
			return
		}
		shareImage, err := service.shareImageFinder.FindShareImage(ctx, articlePage.ArticleURL, homepageURL.Hostname())
		if err != nil {
			// Transient failures (timeout, 5xx) leave the article in the queue for the next refresh.
			service.logger.Debug("share image lookup failed", "article", articlePage.ArticleURL, "error", err)
			continue
		}
		if err := service.store.SaveImageLookupResult(ctx, articlePage.ArticleID, shareImage); err != nil {
			service.logger.Warn("save share image failed", "error", err)
			continue
		}
		if shareImage != "" {
			foundCount++
		}
	}
	if len(articlePages) > 0 {
		service.logger.Info("share images looked up", "source", newsSource.Slug, "checked", len(articlePages), "found", foundCount)
	}
}

// NewsSources lists the configured sources (the UI derives the available news languages from it).
func (service *Service) NewsSources() []domain.NewsSource {
	return service.newsSources
}

// selectedLanguageCodes reads the user's news languages; on failure it falls back to all languages.
func (service *Service) selectedLanguageCodes(ctx context.Context) []string {
	userSettings, err := service.store.LoadUserSettings(ctx)
	if err != nil {
		service.logger.Warn("load settings failed, showing all languages", "error", err)
		return nil
	}
	return userSettings.NewsLanguageCodes
}

func (service *Service) ListArticles(ctx context.Context, onlyUnread bool, maximumResults int) ([]domain.Article, error) {
	articleFilter := sqlite.ArticleFilter{OnlyUnread: onlyUnread, LanguageCodes: service.selectedLanguageCodes(ctx)}
	return service.store.ListArticles(ctx, articleFilter, maximumResults)
}

func (service *Service) CountUnreadArticles(ctx context.Context) (int, error) {
	return service.store.CountUnreadArticles(ctx, service.selectedLanguageCodes(ctx))
}

func (service *Service) MarkArticleRead(ctx context.Context, articleID string) (bool, error) {
	wasFound, err := service.store.MarkArticleRead(ctx, articleID, service.currentTime().UTC())
	if err == nil && wasFound {
		service.PublishUnreadCount(ctx)
	}
	return wasFound, err
}

// MarkArticleUnread puts an article back in the unread list (and the tray count).
func (service *Service) MarkArticleUnread(ctx context.Context, articleID string) (bool, error) {
	wasFound, err := service.store.MarkArticleUnread(ctx, articleID)
	if err == nil && wasFound {
		service.PublishUnreadCount(ctx)
	}
	return wasFound, err
}

func (service *Service) MarkAllArticlesRead(ctx context.Context) error {
	err := service.store.MarkAllArticlesRead(ctx, service.currentTime().UTC(), service.selectedLanguageCodes(ctx))
	if err == nil {
		service.PublishUnreadCount(ctx)
	}
	return err
}

// PublishUnreadCount notifies listeners (tray badge) about the unread count in the selected languages.
func (service *Service) PublishUnreadCount(ctx context.Context) {
	unreadCount, err := service.CountUnreadArticles(ctx)
	if err != nil {
		service.logger.Warn("count unread failed", "error", err)
		return
	}
	service.broadcaster.Publish(notifier.UnreadCountChanged{UnreadCount: unreadCount})
}
