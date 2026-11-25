package app

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"speed-news/apps/api/internal/fixtures"
)

// Months of refreshes compressed into a test: goroutines, the Go heap and the database file must
// settle instead of growing with every cycle. Catches the leaks that would only show after weeks.
func TestRepeatedRefreshCyclesReachASteadyState(test *testing.T) {
	if testing.Short() {
		test.Skip("soak test")
	}
	ctx := context.Background()
	fixtureServer := httptest.NewServer(fixtures.NewServer(time.Now).Handler())
	defer fixtureServer.Close()

	dataDirectory := test.TempDir()
	sourcesFile := filepath.Join(dataDirectory, "sources.yaml")
	if err := os.WriteFile(sourcesFile, []byte(fixtures.SourcesYAML(fixtureServer.URL)), 0o644); err != nil {
		test.Fatal(err)
	}
	application, err := New(Config{
		DataDirectory:    dataDirectory,
		SourcesFile:      sourcesFile,
		JolpicaBaseURL:   fixtureServer.URL + "/jolpica",
		CircuitsBaseURL:  fixtureServer.URL + "/circuits",
		OpenMeteoBaseURL: fixtureServer.URL + "/openmeteo",
		HTTPTimeout:      5 * time.Second,
		Logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		test.Fatal(err)
	}
	defer application.store.Close()
	releaser := newResourceReleaser(application.store, newHTTPClient(5*time.Second), application.logger)

	runCycles := func(cycleCount int) {
		for range cycleCount {
			application.newsService.RefreshAllSources(ctx)
			releaser.releaseAfterRefresh(ctx)
		}
	}
	measure := func() (goroutineCount int, heapBytes uint64, fileBytes int64) {
		runtime.GC()
		var memoryStats runtime.MemStats
		runtime.ReadMemStats(&memoryStats)
		fileInfo, err := os.Stat(filepath.Join(dataDirectory, "speed-news.db"))
		if err != nil {
			test.Fatal(err)
		}
		return runtime.NumGoroutine(), memoryStats.HeapAlloc, fileInfo.Size()
	}

	runCycles(5) // warm up: first inserts, connection pools, regexps
	baselineGoroutines, baselineHeap, baselineFile := measure()
	runCycles(40)
	finalGoroutines, finalHeap, finalFile := measure()

	if finalGoroutines > baselineGoroutines+2 {
		test.Errorf("goroutines grew from %d to %d over 40 refreshes", baselineGoroutines, finalGoroutines)
	}
	if finalHeap > baselineHeap*2+(1<<20) {
		test.Errorf("heap grew from %d to %d bytes over 40 refreshes", baselineHeap, finalHeap)
	}
	if finalFile > baselineFile+(256<<10) {
		test.Errorf("database file grew from %d to %d bytes refreshing the same feeds", baselineFile, finalFile)
	}
}
