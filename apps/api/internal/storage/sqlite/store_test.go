package sqlite

import (
	"context"
	"database/sql"
	"reflect"
	"testing"
	"time"

	"speed-news/apps/api/internal/domain"
)

func openTestStore(test *testing.T) *Store {
	test.Helper()
	store, err := OpenInMemory()
	if err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() { store.Close() })
	return store
}

func buildArticle(articleID string, publishedAt time.Time, fetchedAt time.Time) domain.Article {
	return domain.Article{
		ID: articleID, SeriesSlug: domain.SeriesFormula1, SourceSlug: "source", SourceName: "Source",
		LanguageCode: "en", Title: "Title " + articleID, ArticleURL: "https://example.com/" + articleID,
		PublishedAt: publishedAt, FetchedAt: fetchedAt,
	}
}

func TestArticlesLifecycle(test *testing.T) {
	ctx := context.Background()
	store := openTestStore(test)
	fetchedAt := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	autoReadBefore := fetchedAt.Add(-48 * time.Hour)

	freshArticle := buildArticle("fresh", fetchedAt.Add(-time.Hour), fetchedAt)
	oldArticle := buildArticle("old", fetchedAt.Add(-72*time.Hour), fetchedAt)

	insertedCount, err := store.InsertArticles(ctx, []domain.Article{freshArticle, oldArticle}, autoReadBefore)
	if err != nil || insertedCount != 2 {
		test.Fatalf("expected 2 inserted, got %d (%v)", insertedCount, err)
	}
	insertedCount, _ = store.InsertArticles(ctx, []domain.Article{freshArticle}, autoReadBefore)
	if insertedCount != 0 {
		test.Errorf("duplicates must be ignored, inserted %d", insertedCount)
	}

	unreadCount, _ := store.CountUnreadArticles(ctx, nil)
	if unreadCount != 1 {
		test.Errorf("old articles must be auto-read: expected 1 unread, got %d", unreadCount)
	}

	unreadArticles, _ := store.ListArticles(ctx, ArticleFilter{OnlyUnread: true}, 10)
	if len(unreadArticles) != 1 || unreadArticles[0].ID != "fresh" {
		test.Errorf("unexpected unread list %+v", unreadArticles)
	}

	wasFound, _ := store.MarkArticleRead(ctx, "fresh", fetchedAt)
	if !wasFound {
		test.Error("expected article to be found")
	}
	wasFound, _ = store.MarkArticleRead(ctx, "missing", fetchedAt)
	if wasFound {
		test.Error("missing article must report not found")
	}
	unreadCount, _ = store.CountUnreadArticles(ctx, nil)
	if unreadCount != 0 {
		test.Errorf("expected 0 unread, got %d", unreadCount)
	}

	wasFound, _ = store.MarkArticleUnread(ctx, "fresh")
	unreadCount, _ = store.CountUnreadArticles(ctx, nil)
	if !wasFound || unreadCount != 1 {
		test.Errorf("marking unread must bring the article back: found=%v unread=%d", wasFound, unreadCount)
	}
	if wasFound, _ = store.MarkArticleUnread(ctx, "missing"); wasFound {
		test.Error("missing article must report not found when marked unread")
	}
}

func TestKnownArticleOnlyGainsAMissingImage(test *testing.T) {
	ctx := context.Background()
	store := openTestStore(test)
	fetchedAt := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)

	articleWithoutImage := buildArticle("story", fetchedAt.Add(-time.Hour), fetchedAt)
	if _, err := store.InsertArticles(ctx, []domain.Article{articleWithoutImage}, time.Time{}); err != nil {
		test.Fatal(err)
	}
	if _, err := store.MarkArticleRead(ctx, "story", fetchedAt); err != nil {
		test.Fatal(err)
	}

	articleWithImage := articleWithoutImage
	articleWithImage.ImageURL = "https://example.com/first.jpg"
	articleWithImage.Title = "A changed title must not overwrite the stored one"
	if _, err := store.InsertArticles(ctx, []domain.Article{articleWithImage}, time.Time{}); err != nil {
		test.Fatal(err)
	}
	articleWithOtherImage := articleWithoutImage
	articleWithOtherImage.ImageURL = "https://example.com/second.jpg"
	if _, err := store.InsertArticles(ctx, []domain.Article{articleWithOtherImage}, time.Time{}); err != nil {
		test.Fatal(err)
	}

	storedArticles, _ := store.ListArticles(ctx, ArticleFilter{}, 10)
	storedArticle := storedArticles[0]
	if storedArticle.ImageURL != "https://example.com/first.jpg" {
		test.Errorf("the missing image must be filled once and never replaced, got %q", storedArticle.ImageURL)
	}
	if storedArticle.Title != articleWithoutImage.Title || storedArticle.ReadAt == nil {
		test.Errorf("everything else (title, read state) must be kept, got %+v", storedArticle)
	}
}

func TestImageLookupQueue(test *testing.T) {
	ctx := context.Background()
	store := openTestStore(test)
	fetchedAt := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)

	withoutImage := buildArticle("without-image", fetchedAt.Add(-time.Hour), fetchedAt)
	withImage := buildArticle("with-image", fetchedAt.Add(-2*time.Hour), fetchedAt)
	withImage.ImageURL = "https://example.com/has.jpg"
	otherSource := buildArticle("other-source", fetchedAt.Add(-3*time.Hour), fetchedAt)
	otherSource.SourceSlug = "another"
	if _, err := store.InsertArticles(ctx, []domain.Article{withoutImage, withImage, otherSource}, time.Time{}); err != nil {
		test.Fatal(err)
	}

	awaitingPages, _ := store.ListArticlesAwaitingImageLookup(ctx, "source", 10)
	if len(awaitingPages) != 1 || awaitingPages[0].ArticleID != "without-image" {
		test.Fatalf("only the imageless article of that source must wait, got %+v", awaitingPages)
	}

	// Page without share image: checked once, never queued again.
	if err := store.SaveImageLookupResult(ctx, "without-image", ""); err != nil {
		test.Fatal(err)
	}
	if awaitingPages, _ = store.ListArticlesAwaitingImageLookup(ctx, "source", 10); len(awaitingPages) != 0 {
		test.Errorf("a checked article must leave the queue, got %+v", awaitingPages)
	}

	if err := store.SaveImageLookupResult(ctx, "with-image", "https://example.com/replacement.jpg"); err != nil {
		test.Fatal(err)
	}
	storedArticles, _ := store.ListArticles(ctx, ArticleFilter{}, 10)
	for _, storedArticle := range storedArticles {
		if storedArticle.ID == "with-image" && storedArticle.ImageURL != "https://example.com/has.jpg" {
			test.Errorf("a lookup must never replace an existing image, got %q", storedArticle.ImageURL)
		}
	}
}

func TestMigrationAddsColumnToAnExistingDatabase(test *testing.T) {
	dataDirectory := test.TempDir()
	// A database created before image_lookup_done existed.
	legacyDatabase, err := sql.Open("sqlite", "file:"+dataDirectory+"/speed-news.db")
	if err != nil {
		test.Fatal(err)
	}
	if _, err := legacyDatabase.Exec(`CREATE TABLE articles (id TEXT PRIMARY KEY, series_slug TEXT NOT NULL, source_slug TEXT NOT NULL,
		source_name TEXT NOT NULL, language_code TEXT NOT NULL, title TEXT NOT NULL, summary TEXT NOT NULL, article_url TEXT NOT NULL,
		image_url TEXT NOT NULL DEFAULT '', published_at INTEGER NOT NULL, fetched_at INTEGER NOT NULL, read_at INTEGER)`); err != nil {
		test.Fatal(err)
	}
	_ = legacyDatabase.Close()

	store, err := Open(dataDirectory)
	if err != nil {
		test.Fatalf("opening a legacy database must migrate it: %v", err)
	}
	defer store.Close()
	if _, err := store.ListArticlesAwaitingImageLookup(context.Background(), "source", 1); err != nil {
		test.Errorf("the new column must exist after migration: %v", err)
	}
	// Opening again must be a no-op, not a "duplicate column" error.
	_ = store.Close()
	reopenedStore, err := Open(dataDirectory)
	if err != nil {
		test.Fatalf("reopening must not fail: %v", err)
	}
	_ = reopenedStore.Close()
}

func TestLanguageFilter(test *testing.T) {
	ctx := context.Background()
	store := openTestStore(test)
	fetchedAt := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)

	portugueseArticle := buildArticle("portuguese", fetchedAt.Add(-time.Hour), fetchedAt)
	portugueseArticle.LanguageCode = "pt"
	englishArticle := buildArticle("english", fetchedAt.Add(-2*time.Hour), fetchedAt)
	englishArticle.LanguageCode = "en"
	if _, err := store.InsertArticles(ctx, []domain.Article{portugueseArticle, englishArticle}, time.Time{}); err != nil {
		test.Fatal(err)
	}

	portugueseOnly := []string{"pt"}
	if unreadCount, _ := store.CountUnreadArticles(ctx, portugueseOnly); unreadCount != 1 {
		test.Errorf("expected 1 unread in Portuguese, got %d", unreadCount)
	}
	if unreadCount, _ := store.CountUnreadArticles(ctx, []string{}); unreadCount != 2 {
		test.Errorf("an empty language list means all languages: expected 2, got %d", unreadCount)
	}

	listedArticles, _ := store.ListArticles(ctx, ArticleFilter{LanguageCodes: portugueseOnly}, 10)
	if len(listedArticles) != 1 || listedArticles[0].ID != "portuguese" {
		test.Errorf("expected only the Portuguese article, got %+v", listedArticles)
	}

	// "Mark all as read" while seeing only Portuguese must not touch the hidden English article.
	if err := store.MarkAllArticlesRead(ctx, fetchedAt, portugueseOnly); err != nil {
		test.Fatal(err)
	}
	if unreadCount, _ := store.CountUnreadArticles(ctx, []string{"en"}); unreadCount != 1 {
		test.Errorf("hidden languages must keep their unread state, got %d unread in English", unreadCount)
	}
}

func TestFindUpcomingRaceEventSkipsFinishedEvents(test *testing.T) {
	ctx := context.Background()
	store := openTestStore(test)
	buildEvent := func(round int, raceStartsAt time.Time) domain.RaceEvent {
		return domain.RaceEvent{
			ID: "f1-2026-" + string(rune('a'+round)), SeriesSlug: domain.SeriesFormula1, Season: 2026, Round: round,
			EventName: "Event", CircuitID: "circuit", Sessions: []domain.RaceSession{
				{SessionKind: domain.SessionPractice1, StartsAt: raceStartsAt.Add(-48 * time.Hour), StartTimeConfirmed: true},
				{SessionKind: domain.SessionRace, StartsAt: raceStartsAt, StartTimeConfirmed: true},
			},
		}
	}
	finishedEvent := buildEvent(16, time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC))
	singaporeEvent := buildEvent(17, time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC))
	if err := store.ReplaceSeasonEvents(ctx, domain.SeriesFormula1, 2026, []domain.RaceEvent{singaporeEvent, finishedEvent}); err != nil {
		test.Fatal(err)
	}

	upcomingEvent, err := store.FindUpcomingRaceEvent(ctx, domain.SeriesFormula1, time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC))
	if err != nil || upcomingEvent == nil {
		test.Fatalf("expected an upcoming event, got %v (%v)", upcomingEvent, err)
	}
	if upcomingEvent.Round != 17 || len(upcomingEvent.Sessions) != 2 {
		test.Errorf("expected round 17 with 2 sessions, got %+v", upcomingEvent)
	}

	upcomingEvent, _ = store.FindUpcomingRaceEvent(ctx, domain.SeriesFormula1, time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC))
	if upcomingEvent != nil {
		test.Error("expected nil after the last event")
	}
}

func TestUserSettingsDefaultAndRoundTrip(test *testing.T) {
	ctx := context.Background()
	store := openTestStore(test)

	userSettings, err := store.LoadUserSettings(ctx)
	if err != nil || !reflect.DeepEqual(userSettings, domain.DefaultUserSettings()) {
		test.Fatalf("expected defaults, got %+v (%v)", userSettings, err)
	}

	customSettings := domain.UserSettings{
		TimeZonePreference: domain.TimeZonePreference{Mode: "manual", TimeZoneID: "Europe/Lisbon"},
		InterfaceLanguage:  domain.InterfaceLanguageEnglish,
		NewsLanguageCodes:  []string{"pt"},
		// The default is true: a saved false must survive loading over the defaults.
		CloseOnFocusLoss:    false,
		ShowUnreadIndicator: false,
	}
	if err := store.SaveUserSettings(ctx, customSettings); err != nil {
		test.Fatal(err)
	}
	userSettings, _ = store.LoadUserSettings(ctx)
	if !reflect.DeepEqual(userSettings, customSettings) {
		test.Errorf("expected %+v, got %+v", customSettings, userSettings)
	}
}

func TestSettingsSavedBeforeLanguagesExistedGetDefaults(test *testing.T) {
	ctx := context.Background()
	store := openTestStore(test)
	// Exactly what version 0.1 stored: only the time zone.
	if _, err := store.database.ExecContext(ctx, `INSERT INTO settings (setting_key, setting_value) VALUES (?, ?)`,
		userSettingsKey, `{"timeZonePreference":{"mode":"manual","timeZoneId":"Europe/Lisbon"}}`); err != nil {
		test.Fatal(err)
	}
	userSettings, err := store.LoadUserSettings(ctx)
	if err != nil {
		test.Fatal(err)
	}
	if userSettings.InterfaceLanguage != domain.InterfaceLanguageSystem || userSettings.NewsLanguageCodes == nil || !userSettings.CloseOnFocusLoss ||
		!userSettings.ShowUnreadIndicator ||
		userSettings.TimeZonePreference.TimeZoneID != "Europe/Lisbon" {
		test.Errorf("old settings must keep the time zone and get language defaults, got %+v", userSettings)
	}
}
