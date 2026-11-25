package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"speed-news/apps/api/internal/domain"
)

func TestMaintainDropsOnlyOldCacheEntries(test *testing.T) {
	ctx := context.Background()
	store := openTestStore(test)
	currentTime := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	if err := store.SaveCachedPayload(ctx, "forecast:2025-qatar", `{}`, currentTime.AddDate(-1, 0, 0)); err != nil {
		test.Fatal(err)
	}
	if err := store.SaveCachedPayload(ctx, "last-race", `{}`, currentTime.Add(-time.Hour)); err != nil {
		test.Fatal(err)
	}

	if err := store.Maintain(ctx, currentTime.AddDate(0, 0, -30)); err != nil {
		test.Fatal(err)
	}
	if _, _, isCached, _ := store.LoadCachedPayload(ctx, "forecast:2025-qatar"); isCached {
		test.Error("an entry older than the cutoff must be deleted")
	}
	if _, _, isCached, _ := store.LoadCachedPayload(ctx, "last-race"); !isCached {
		test.Error("a recent entry must survive maintenance")
	}
}

func TestOpenSwitchesALegacyDatabaseToIncrementalAutoVacuumKeepingItsRows(test *testing.T) {
	dataDirectory := test.TempDir()
	// A database created by v0.1.x: no auto_vacuum, one stored setting.
	legacyDatabase, err := sql.Open("sqlite", "file:"+filepath.Join(dataDirectory, "speed-news.db"))
	if err != nil {
		test.Fatal(err)
	}
	for _, legacyStatement := range []string{
		`CREATE TABLE settings (setting_key TEXT PRIMARY KEY, setting_value TEXT NOT NULL)`,
		`INSERT INTO settings VALUES ('user_settings', '{"interfaceLanguage":"en"}')`,
	} {
		if _, err := legacyDatabase.Exec(legacyStatement); err != nil {
			test.Fatal(err)
		}
	}
	_ = legacyDatabase.Close()

	store, err := Open(dataDirectory)
	if err != nil {
		test.Fatalf("opening a legacy database must succeed: %v", err)
	}
	defer store.Close()

	var autoVacuumMode int
	if err := store.database.QueryRow(`PRAGMA auto_vacuum`).Scan(&autoVacuumMode); err != nil || autoVacuumMode != autoVacuumIncremental {
		test.Fatalf("auto_vacuum must become INCREMENTAL, got %d (%v)", autoVacuumMode, err)
	}
	userSettings, err := store.LoadUserSettings(context.Background())
	if err != nil || userSettings.InterfaceLanguage != "en" {
		test.Fatalf("saved settings must survive the switch, got %+v (%v)", userSettings, err)
	}
}

// Years of use: articles keep arriving and expiring. The file must shrink back after a purge
// instead of staying at its peak size forever.
func TestDatabaseFileShrinksAfterOldArticlesAreDeleted(test *testing.T) {
	ctx := context.Background()
	dataDirectory := test.TempDir()
	store, err := Open(dataDirectory)
	if err != nil {
		test.Fatal(err)
	}
	defer store.Close()

	publishedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	articles := make([]domain.Article, 0, 1500)
	for articleIndex := range 1500 {
		article := buildArticle(fmt.Sprintf("article-%04d", articleIndex), publishedAt, publishedAt)
		article.Summary = strings.Repeat("summary text ", 30)
		articles = append(articles, article)
	}
	if _, err := store.InsertArticles(ctx, articles, publishedAt); err != nil {
		test.Fatal(err)
	}
	if err := store.CheckpointWAL(ctx); err != nil {
		test.Fatal(err)
	}
	peakSize := databaseFileSize(test, dataDirectory)

	if err := store.DeleteArticlesPublishedBefore(ctx, publishedAt.Add(time.Hour)); err != nil {
		test.Fatal(err)
	}
	if err := store.Maintain(ctx, publishedAt); err != nil {
		test.Fatal(err)
	}
	if err := store.CheckpointWAL(ctx); err != nil {
		test.Fatal(err)
	}
	if shrunkSize := databaseFileSize(test, dataDirectory); shrunkSize >= peakSize/2 {
		test.Fatalf("file must give the space back: %d bytes at peak, %d after maintenance", peakSize, shrunkSize)
	}
	if walInfo, err := os.Stat(filepath.Join(dataDirectory, "speed-news.db-wal")); err == nil && walInfo.Size() != 0 {
		test.Errorf("a TRUNCATE checkpoint must empty the WAL file, it has %d bytes", walInfo.Size())
	}
}

func databaseFileSize(test *testing.T, dataDirectory string) int64 {
	test.Helper()
	fileInfo, err := os.Stat(filepath.Join(dataDirectory, "speed-news.db"))
	if err != nil {
		test.Fatal(err)
	}
	return fileInfo.Size()
}
