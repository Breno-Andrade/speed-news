// Package raceinsights builds the extra race card panels: last Grand Prix, circuit profile and
// the weekend weather forecast. Every answer is cached in SQLite with a time-to-live; when the
// source is down, the last cached answer is served instead of an error.
package raceinsights

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"speed-news/apps/api/internal/domain"
	"speed-news/apps/api/internal/forecast/openmeteo"
	"speed-news/apps/api/internal/storage/sqlite"
)

const (
	lastRaceCacheTTL       = 3 * time.Hour
	circuitProfileCacheTTL = 24 * time.Hour
	forecastCacheTTL       = time.Hour
	// Forecasts further ahead are too unreliable to show.
	forecastHorizon    = 7 * 24 * time.Hour
	recentWinnersLimit = 5
)

// ResultsProvider is implemented by the Jolpica client.
type ResultsProvider interface {
	FetchLastRaceResult(ctx context.Context) (*domain.RaceResultSummary, error)
	FetchRaceResultAtCircuit(ctx context.Context, season int, circuitID string) (*domain.RaceResultSummary, error)
	FetchCircuitWinners(ctx context.Context, circuitID string) ([]domain.CircuitWinner, error)
}

// CircuitFactsProvider is implemented by the f1-circuits client.
type CircuitFactsProvider interface {
	FetchCircuitFacts(ctx context.Context, raceEvent domain.RaceEvent) (domain.CircuitFacts, error)
}

// ForecastProvider is implemented by the Open-Meteo client.
type ForecastProvider interface {
	FetchSessionForecasts(ctx context.Context, coordinates domain.GeoCoordinates, raceSessions []domain.RaceSession) ([]domain.SessionForecast, error)
}

// UpcomingEventFinder is implemented by the race weekend service.
type UpcomingEventFinder interface {
	UpcomingEvent(ctx context.Context) (*domain.RaceEvent, error)
}

type Service struct {
	store                *sqlite.Store
	resultsProvider      ResultsProvider
	circuitFactsProvider CircuitFactsProvider
	forecastProvider     ForecastProvider
	upcomingEventFinder  UpcomingEventFinder
	logger               *slog.Logger
	currentTime          func() time.Time
	// One lock per cache key: concurrent requests wait for a single fetch instead of
	// hitting a slow API several times.
	cacheKeyLocks sync.Map
}

func NewService(store *sqlite.Store, resultsProvider ResultsProvider, circuitFactsProvider CircuitFactsProvider,
	forecastProvider ForecastProvider, upcomingEventFinder UpcomingEventFinder, logger *slog.Logger) *Service {
	return &Service{
		store:                store,
		resultsProvider:      resultsProvider,
		circuitFactsProvider: circuitFactsProvider,
		forecastProvider:     forecastProvider,
		upcomingEventFinder:  upcomingEventFinder,
		logger:               logger,
		currentTime:          time.Now,
	}
}

// LastRace is the most recent race with published results.
func (service *Service) LastRace(ctx context.Context) (*domain.RaceResultSummary, error) {
	return cachedFetch(ctx, service, "last-race", lastRaceCacheTTL, service.resultsProvider.FetchLastRaceResult)
}

// CircuitProfile describes the circuit of the upcoming race and what happened there before.
func (service *Service) CircuitProfile(ctx context.Context) (*domain.CircuitProfile, error) {
	upcomingEvent, err := service.upcomingEventFinder.UpcomingEvent(ctx)
	if err != nil || upcomingEvent == nil {
		return nil, err
	}
	cacheKey := fmt.Sprintf("circuit-profile:%s:%d", upcomingEvent.CircuitID, upcomingEvent.Season)
	return cachedFetch(ctx, service, cacheKey, circuitProfileCacheTTL, func(fetchContext context.Context) (*domain.CircuitProfile, error) {
		return service.buildCircuitProfile(fetchContext, *upcomingEvent)
	})
}

func (service *Service) buildCircuitProfile(ctx context.Context, upcomingEvent domain.RaceEvent) (*domain.CircuitProfile, error) {
	circuitProfile := &domain.CircuitProfile{
		CircuitID:     upcomingEvent.CircuitID,
		CircuitName:   upcomingEvent.CircuitName,
		RecentWinners: []domain.CircuitWinner{},
	}

	circuitFacts, factsErr := service.circuitFactsProvider.FetchCircuitFacts(ctx, upcomingEvent)
	if factsErr == nil {
		circuitProfile.Facts = &circuitFacts
	}

	circuitWinners, winnersErr := service.resultsProvider.FetchCircuitWinners(ctx, upcomingEvent.CircuitID)
	if winnersErr != nil {
		if factsErr != nil {
			return nil, fmt.Errorf("circuit profile unavailable: %w; %w", factsErr, winnersErr)
		}
		// Facts alone are still useful to show, but must not be cached as the full profile.
		return circuitProfile, partialResultError{cause: winnersErr}
	}
	for _, circuitWinner := range circuitWinners {
		if circuitWinner.Season < upcomingEvent.Season && len(circuitProfile.RecentWinners) < recentWinnersLimit {
			circuitProfile.RecentWinners = append(circuitProfile.RecentWinners, circuitWinner)
		}
	}

	if len(circuitProfile.RecentWinners) > 0 {
		previousSeason := circuitProfile.RecentWinners[0].Season
		previousEdition, err := service.resultsProvider.FetchRaceResultAtCircuit(ctx, previousSeason, upcomingEvent.CircuitID)
		if err != nil {
			service.logger.Warn("previous edition unavailable", "circuit", upcomingEvent.CircuitID, "season", previousSeason, "error", err)
		} else {
			circuitProfile.PreviousEdition = previousEdition
		}
	}
	return circuitProfile, nil
}

// WeekendForecast returns nil until the race weekend is within forecastHorizon.
func (service *Service) WeekendForecast(ctx context.Context) (*domain.WeekendForecast, error) {
	upcomingEvent, err := service.upcomingEventFinder.UpcomingEvent(ctx)
	if err != nil || upcomingEvent == nil {
		return nil, err
	}
	if upcomingEvent.FirstSessionStartsAt().Sub(service.currentTime()) > forecastHorizon {
		return nil, nil
	}
	return cachedFetch(ctx, service, "forecast:"+upcomingEvent.ID, forecastCacheTTL, func(fetchContext context.Context) (*domain.WeekendForecast, error) {
		sessionForecasts, err := service.forecastProvider.FetchSessionForecasts(fetchContext, upcomingEvent.Coordinates, upcomingEvent.Sessions)
		if err != nil {
			return nil, err
		}
		return &domain.WeekendForecast{
			Sessions:        sessionForecasts,
			AttributionText: openmeteo.AttributionText,
			AttributionURL:  openmeteo.AttributionURL,
		}, nil
	})
}

// Prefetch warms the cache in the background so the panels open instantly.
func (service *Service) Prefetch(ctx context.Context) {
	if _, err := service.LastRace(ctx); err != nil {
		service.logger.Warn("prefetch last race failed", "error", err)
	}
	if _, err := service.CircuitProfile(ctx); err != nil {
		service.logger.Warn("prefetch circuit profile failed", "error", err)
	}
	if _, err := service.WeekendForecast(ctx); err != nil {
		service.logger.Warn("prefetch forecast failed", "error", err)
	}
}

// partialResultError marks an answer that is worth showing but not worth caching.
type partialResultError struct {
	cause error
}

func (partialResult partialResultError) Error() string {
	return "partial result: " + partialResult.cause.Error()
}

func (partialResult partialResultError) Unwrap() error {
	return partialResult.cause
}

// cachedFetch serves a fresh cache entry, otherwise fetches and stores a new one. When fetching
// fails, a stale entry is better than nothing.
func cachedFetch[Payload any](ctx context.Context, service *Service, cacheKey string, timeToLive time.Duration,
	fetchFresh func(context.Context) (Payload, error)) (Payload, error) {
	keyLock, _ := service.cacheKeyLocks.LoadOrStore(cacheKey, &sync.Mutex{})
	keyLock.(*sync.Mutex).Lock()
	defer keyLock.(*sync.Mutex).Unlock()

	var cachedValue Payload
	cachedJSON, fetchedAt, isCached, err := service.store.LoadCachedPayload(ctx, cacheKey)
	hasUsableCache := err == nil && isCached && json.Unmarshal([]byte(cachedJSON), &cachedValue) == nil
	if hasUsableCache && service.currentTime().Sub(fetchedAt) < timeToLive {
		return cachedValue, nil
	}

	freshValue, fetchErr := fetchFresh(ctx)
	var partialResult partialResultError
	if errors.As(fetchErr, &partialResult) {
		// A complete stale answer beats a fresh incomplete one; neither replaces the cache.
		if hasUsableCache {
			return cachedValue, nil
		}
		service.logger.Warn("serving partial data", "cacheKey", cacheKey, "error", partialResult.cause)
		return freshValue, nil
	}
	if fetchErr != nil {
		if hasUsableCache {
			service.logger.Warn("serving stale data", "cacheKey", cacheKey, "fetchedAt", fetchedAt, "error", fetchErr)
			return cachedValue, nil
		}
		return freshValue, fetchErr
	}

	if encodedValue, err := json.Marshal(freshValue); err == nil {
		if err := service.store.SaveCachedPayload(ctx, cacheKey, string(encodedValue), service.currentTime().UTC()); err != nil {
			service.logger.Warn("cache save failed", "cacheKey", cacheKey, "error", err)
		}
	}
	return freshValue, nil
}
