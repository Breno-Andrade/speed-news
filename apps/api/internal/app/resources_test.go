package app

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"speed-news/apps/api/internal/storage/sqlite"
)

func TestResourceReleaserMaintainsTheDatabaseOncePerDay(test *testing.T) {
	ctx := context.Background()
	store, err := sqlite.OpenInMemory()
	if err != nil {
		test.Fatal(err)
	}
	defer store.Close()

	currentTime := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	freedCount := 0
	releaser := newResourceReleaser(store, &http.Client{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	releaser.currentTime = func() time.Time { return currentTime }
	releaser.freeOSMemory = func() { freedCount++ }

	saveCacheEntry := func(cacheKey string, fetchedAt time.Time) {
		if err := store.SaveCachedPayload(ctx, cacheKey, `{}`, fetchedAt); err != nil {
			test.Fatal(err)
		}
	}
	isCached := func(cacheKey string) bool {
		_, _, found, _ := store.LoadCachedPayload(ctx, cacheKey)
		return found
	}

	saveCacheEntry("forecast:2025-qatar", currentTime.AddDate(-1, 0, 0))
	releaser.releaseAfterRefresh(ctx)
	if isCached("forecast:2025-qatar") {
		test.Fatal("the first refresh must run the maintenance and prune old cache entries")
	}

	// An hour later (race weekend refreshes every 5 min): no maintenance yet.
	currentTime = currentTime.Add(time.Hour)
	saveCacheEntry("forecast:2025-brazil", currentTime.AddDate(-1, 0, 0))
	releaser.releaseAfterRefresh(ctx)
	if !isCached("forecast:2025-brazil") {
		test.Fatal("maintenance must not run more than once a day")
	}

	currentTime = currentTime.Add(databaseMaintenanceInterval)
	releaser.releaseAfterRefresh(ctx)
	if isCached("forecast:2025-brazil") {
		test.Fatal("maintenance must run again after a day")
	}
	if freedCount != 3 {
		test.Errorf("memory must be returned to the OS after every refresh, got %d of 3", freedCount)
	}
}

func TestHTTPClientDropsIdleConnectionsQuickly(test *testing.T) {
	httpClient := newHTTPClient(5 * time.Second)
	transport, isTransport := httpClient.Transport.(*http.Transport)
	if !isTransport {
		test.Fatalf("unexpected transport %T", httpClient.Transport)
	}
	if transport.IdleConnTimeout > time.Minute || transport.MaxIdleConnsPerHost > 2 || httpClient.Timeout != 5*time.Second {
		test.Errorf("idle pool too generous: timeout %v, per host %d", transport.IdleConnTimeout, transport.MaxIdleConnsPerHost)
	}
}
