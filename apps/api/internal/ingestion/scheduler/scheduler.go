// Package scheduler runs the periodic jobs: calendar sync and news refresh.
package scheduler

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
	scheduler.newsRefresher.RefreshAllSources(ctx)

	newsTimer := time.NewTimer(scheduler.nextNewsInterval(ctx))
	defer newsTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-newsTimer.C:
			scheduler.newsRefresher.RefreshAllSources(ctx)
			newsTimer.Reset(scheduler.nextNewsInterval(ctx))
		case <-scheduler.refreshNowRequests:
			scheduler.newsRefresher.RefreshAllSources(ctx)
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
