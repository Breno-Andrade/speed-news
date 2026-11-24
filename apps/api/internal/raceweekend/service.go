// Package raceweekend keeps the calendar in sync and answers "which race is this week?".
package raceweekend

import (
	"context"
	"log/slog"
	"time"

	"speed-news/apps/api/internal/domain"
	"speed-news/apps/api/internal/storage/sqlite"
)

const (
	// An event stays "current" until this long after its last session starts (results, post-race news).
	eventEndGracePeriod = 6 * time.Hour
	// Race weekend mode starts this long before the first session.
	raceWeekendLeadTime = 4 * 24 * time.Hour
)

// SeasonCalendarProvider is the port implemented by the Jolpica client.
type SeasonCalendarProvider interface {
	FetchSeason(ctx context.Context, season int) ([]domain.RaceEvent, error)
}

// CircuitLayoutProvider is the port implemented by the f1-circuits client.
type CircuitLayoutProvider interface {
	FetchLayoutForCircuit(ctx context.Context, raceEvent domain.RaceEvent) (domain.CircuitLayout, error)
}

type Service struct {
	store                 *sqlite.Store
	calendarProvider      SeasonCalendarProvider
	circuitLayoutProvider CircuitLayoutProvider
	logger                *slog.Logger
	currentTime           func() time.Time
}

func NewService(store *sqlite.Store, calendarProvider SeasonCalendarProvider,
	circuitLayoutProvider CircuitLayoutProvider, logger *slog.Logger) *Service {
	return &Service{
		store:                 store,
		calendarProvider:      calendarProvider,
		circuitLayoutProvider: circuitLayoutProvider,
		logger:                logger,
		currentTime:           time.Now,
	}
}

// SyncCalendar refreshes the current and next season (the next one is published around December).
func (service *Service) SyncCalendar(ctx context.Context) {
	currentYear := service.currentTime().UTC().Year()
	for _, season := range []int{currentYear, currentYear + 1} {
		raceEvents, err := service.calendarProvider.FetchSeason(ctx, season)
		if err != nil {
			service.logger.Warn("calendar sync failed", "season", season, "error", err)
			continue
		}
		if len(raceEvents) == 0 {
			continue // season not published yet: keep whatever we have
		}
		if err := service.store.ReplaceSeasonEvents(ctx, domain.SeriesFormula1, season, raceEvents); err != nil {
			service.logger.Warn("calendar store failed", "season", season, "error", err)
			continue
		}
		service.logger.Info("calendar synced", "season", season, "events", len(raceEvents))
	}
}

// CurrentOverview returns the current or next race weekend, or nil when the calendar is empty.
// The circuit drawing is fetched lazily once and cached forever.
func (service *Service) CurrentOverview(ctx context.Context) (*domain.RaceWeekendOverview, error) {
	currentTime := service.currentTime().UTC()
	upcomingEvent, err := service.store.FindUpcomingRaceEvent(ctx, domain.SeriesFormula1, currentTime.Add(-eventEndGracePeriod))
	if err != nil || upcomingEvent == nil {
		return nil, err
	}

	circuitLayout, err := service.store.FindCircuitLayout(ctx, upcomingEvent.CircuitID)
	if err != nil {
		return nil, err
	}
	if circuitLayout == nil {
		fetchedLayout, fetchErr := service.circuitLayoutProvider.FetchLayoutForCircuit(ctx, *upcomingEvent)
		if fetchErr != nil {
			service.logger.Warn("circuit layout unavailable", "circuit", upcomingEvent.CircuitID, "error", fetchErr)
		} else {
			if saveErr := service.store.SaveCircuitLayout(ctx, fetchedLayout, currentTime); saveErr != nil {
				service.logger.Warn("circuit layout cache failed", "error", saveErr)
			}
			circuitLayout = &fetchedLayout
		}
	}

	return &domain.RaceWeekendOverview{
		RaceEvent:        *upcomingEvent,
		CircuitLayout:    circuitLayout,
		IsRaceWeekendNow: IsWithinRaceWeekend(*upcomingEvent, currentTime),
	}, nil
}

// UpcomingEvent is the current or next race weekend (nil when the calendar has nothing ahead).
func (service *Service) UpcomingEvent(ctx context.Context) (*domain.RaceEvent, error) {
	currentTime := service.currentTime().UTC()
	return service.store.FindUpcomingRaceEvent(ctx, domain.SeriesFormula1, currentTime.Add(-eventEndGracePeriod))
}

// IsRaceWeekendNow drives the news refresh frequency.
func (service *Service) IsRaceWeekendNow(ctx context.Context) bool {
	currentTime := service.currentTime().UTC()
	upcomingEvent, err := service.store.FindUpcomingRaceEvent(ctx, domain.SeriesFormula1, currentTime.Add(-eventEndGracePeriod))
	if err != nil || upcomingEvent == nil {
		return false
	}
	return IsWithinRaceWeekend(*upcomingEvent, currentTime)
}

func IsWithinRaceWeekend(raceEvent domain.RaceEvent, currentTime time.Time) bool {
	weekendStartsAt := raceEvent.FirstSessionStartsAt().Add(-raceWeekendLeadTime)
	weekendEndsAt := raceEvent.LastSessionStartsAt().Add(eventEndGracePeriod)
	return !currentTime.Before(weekendStartsAt) && currentTime.Before(weekendEndsAt)
}
