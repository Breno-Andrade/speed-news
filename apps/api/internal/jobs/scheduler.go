// Package jobs runs the periodic jobs: calendar sync and news refresh.
package jobs

import (
	"context"
	"log/slog"
	"time"
)

const (
	calendarSyncInterval    = 12 * time.Hour
	regularNewsInterval     = 30 * time.Minute
	raceWeekendNewsInterval = 5 * time.Minute
)

type NewsRefresher interface {
	RefreshAllSources(ctx context.Context)
}

type CalendarSynchronizer interface {
	SyncCalendar(ctx context.Context)
	IsRaceWeekendNow(ctx context.Context) bool
}

// InsightsPrefetcher warms the race card panels (results, circuit, weather) after each calendar sync.
type InsightsPrefetcher interface {
	Prefetch(ctx context.Context)
}

type Scheduler struct {
	newsRefresher        NewsRefresher
	calendarSynchronizer CalendarSynchronizer
	insightsPrefetcher   InsightsPrefetcher
	logger               *slog.Logger
	refreshNowRequests   chan struct{}
	// afterNewsRefresh runs after every news refresh (manual or scheduled), see WithAfterNewsRefresh.
	afterNewsRefresh func(ctx context.Context)
}

func NewScheduler(newsRefresher NewsRefresher, calendarSynchronizer CalendarSynchronizer,
	insightsPrefetcher InsightsPrefetcher, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		newsRefresher:        newsRefresher,
		calendarSynchronizer: calendarSynchronizer,
		insightsPrefetcher:   insightsPrefetcher,
		logger:               logger,
		refreshNowRequests:   make(chan struct{}, 1),
	}
}

// WithAfterNewsRefresh registers work to run right after each news refresh. The backend then
// sits idle for 5 to 30 minutes, the right moment to give back what the refresh burst used.
func (scheduler *Scheduler) WithAfterNewsRefresh(afterNewsRefresh func(ctx context.Context)) *Scheduler {
	scheduler.afterNewsRefresh = afterNewsRefresh
	return scheduler
}

func (scheduler *Scheduler) refreshNews(ctx context.Context) {
	scheduler.newsRefresher.RefreshAllSources(ctx)
	if scheduler.afterNewsRefresh != nil && ctx.Err() == nil {
		scheduler.afterNewsRefresh(ctx)
	}
}

func (scheduler *Scheduler) syncCalendarAndPrefetch(ctx context.Context) {
	scheduler.calendarSynchronizer.SyncCalendar(ctx)
	if scheduler.insightsPrefetcher != nil {
		scheduler.insightsPrefetcher.Prefetch(ctx)
	}
}

// RequestRefreshNow asks for an immediate news refresh (manual "refresh" button). Extra requests are merged.
func (scheduler *Scheduler) RequestRefreshNow() {
	select {
	case scheduler.refreshNowRequests <- struct{}{}:
	default:
	}
}

// Run blocks until ctx is cancelled. Calendar and news run on independent loops: a slow or
// unreachable calendar API must never delay the news (it did: Jolpica down = 60 s without news).
func (scheduler *Scheduler) Run(ctx context.Context) {
	go scheduler.runCalendarLoop(ctx)
	scheduler.runNewsLoop(ctx)
}

func (scheduler *Scheduler) runCalendarLoop(ctx context.Context) {
	scheduler.syncCalendarAndPrefetch(ctx)

	calendarTicker := time.NewTicker(calendarSyncInterval)
	defer calendarTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-calendarTicker.C:
			scheduler.syncCalendarAndPrefetch(ctx)
		}
	}
}

func (scheduler *Scheduler) runNewsLoop(ctx context.Context) {
	scheduler.refreshNews(ctx)

	newsTimer := time.NewTimer(scheduler.nextNewsInterval(ctx))
	defer newsTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-newsTimer.C:
			scheduler.refreshNews(ctx)
			newsTimer.Reset(scheduler.nextNewsInterval(ctx))
		case <-scheduler.refreshNowRequests:
			scheduler.refreshNews(ctx)
			if !newsTimer.Stop() {
				select {
				case <-newsTimer.C:
				default:
				}
			}
			newsTimer.Reset(scheduler.nextNewsInterval(ctx))
		}
	}
}

func (scheduler *Scheduler) nextNewsInterval(ctx context.Context) time.Duration {
	if scheduler.calendarSynchronizer.IsRaceWeekendNow(ctx) {
		scheduler.logger.Debug("race weekend mode", "interval", raceWeekendNewsInterval)
		return raceWeekendNewsInterval
	}
	return regularNewsInterval
}
