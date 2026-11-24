package scheduler

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

type countingNewsRefresher struct {
	refreshedSignal chan struct{}
}

func (newsRefresher *countingNewsRefresher) RefreshAllSources(context.Context) {
	select {
	case newsRefresher.refreshedSignal <- struct{}{}:
	default:
	}
}

// hangingCalendar simulates the calendar API being down: every sync blocks until shutdown.
type hangingCalendar struct{}

func (hangingCalendar) SyncCalendar(ctx context.Context) { <-ctx.Done() }

func (hangingCalendar) IsRaceWeekendNow(context.Context) bool { return false }

func TestNewsRefreshDoesNotWaitForAHangingCalendar(test *testing.T) {
	ctx, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()

	newsRefresher := &countingNewsRefresher{refreshedSignal: make(chan struct{}, 1)}
	silentLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	go NewScheduler(newsRefresher, hangingCalendar{}, nil, silentLogger).Run(ctx)

	select {
	case <-newsRefresher.refreshedSignal:
	case <-time.After(2 * time.Second):
		test.Fatal("news were not refreshed while the calendar sync was hanging")
	}
}
