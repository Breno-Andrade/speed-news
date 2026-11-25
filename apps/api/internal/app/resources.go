package app

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"speed-news/apps/api/internal/storage/sqlite"
)

const (
	// Daily housekeeping of the database (cache pruning, vacuum, planner statistics).
	databaseMaintenanceInterval = 24 * time.Hour
	// Cached API responses older than this belong to past races or seasons: nobody asks again.
	apiCacheRetention = 30 * 24 * time.Hour
)

// resourceReleaser runs after every news refresh. Between refreshes the backend is idle for 5 to
// 30 minutes, so the memory, sockets and WAL pages used by the burst are given back right away
// instead of being held until the next one.
type resourceReleaser struct {
	store             *sqlite.Store
	httpClient        *http.Client
	logger            *slog.Logger
	currentTime       func() time.Time
	freeOSMemory      func()
	lastMaintenanceAt time.Time
}

func newResourceReleaser(store *sqlite.Store, httpClient *http.Client, logger *slog.Logger) *resourceReleaser {
	return &resourceReleaser{
		store:        store,
		httpClient:   httpClient,
		logger:       logger,
		currentTime:  time.Now,
		freeOSMemory: debug.FreeOSMemory,
	}
}

// releaseAfterRefresh is only called from the news loop, never concurrently.
func (releaser *resourceReleaser) releaseAfterRefresh(ctx context.Context) {
	if err := releaser.store.CheckpointWAL(ctx); err != nil {
		releaser.logger.Warn("wal checkpoint failed", "error", err)
	}
	currentTime := releaser.currentTime()
	if currentTime.Sub(releaser.lastMaintenanceAt) >= databaseMaintenanceInterval {
		if err := releaser.store.Maintain(ctx, currentTime.Add(-apiCacheRetention)); err != nil {
			releaser.logger.Warn("database maintenance failed", "error", err)
		} else {
			releaser.lastMaintenanceAt = currentTime
		}
	}
	releaser.httpClient.CloseIdleConnections()
	releaser.freeOSMemory()
}

// newHTTPClient keeps few idle connections and drops them quickly: requests come in bursts every
// few minutes, so pooled sockets (and their TLS buffers) would only sit there in between.
func newHTTPClient(requestTimeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConnsPerHost = 2
	transport.IdleConnTimeout = 30 * time.Second
	return &http.Client{Timeout: requestTimeout, Transport: transport}
}
