package raceinsights

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"speed-news/apps/api/internal/domain"
	"speed-news/apps/api/internal/storage/sqlite"
)

var errSourceDown = errors.New("source down")

type fakeResultsProvider struct {
	lastRaceCalls    int
	failLastRace     bool
	failWinners      bool
	requestedSeasons []int
	circuitWinners   []domain.CircuitWinner
}

func (resultsProvider *fakeResultsProvider) FetchLastRaceResult(context.Context) (*domain.RaceResultSummary, error) {
	resultsProvider.lastRaceCalls++
	if resultsProvider.failLastRace {
		return nil, errSourceDown
	}
	return &domain.RaceResultSummary{EventName: "Las Vegas Grand Prix", Season: 2026, Round: 21}, nil
}

func (resultsProvider *fakeResultsProvider) FetchRaceResultAtCircuit(_ context.Context, season int, circuitID string) (*domain.RaceResultSummary, error) {
	resultsProvider.requestedSeasons = append(resultsProvider.requestedSeasons, season)
	return &domain.RaceResultSummary{Season: season, CircuitID: circuitID}, nil
}

func (resultsProvider *fakeResultsProvider) FetchCircuitWinners(context.Context, string) ([]domain.CircuitWinner, error) {
	if resultsProvider.failWinners {
		return nil, errSourceDown
	}
	return resultsProvider.circuitWinners, nil
}

type fakeCircuitFactsProvider struct{}

func (fakeCircuitFactsProvider) FetchCircuitFacts(context.Context, domain.RaceEvent) (domain.CircuitFacts, error) {
	return domain.CircuitFacts{LengthMeters: 5380, FirstGrandPrixYear: 2021}, nil
}

type fakeForecastProvider struct{ calls int }

func (forecastProvider *fakeForecastProvider) FetchSessionForecasts(_ context.Context, _ domain.GeoCoordinates, raceSessions []domain.RaceSession) ([]domain.SessionForecast, error) {
	forecastProvider.calls++
	return []domain.SessionForecast{{SessionKind: raceSessions[0].SessionKind, TemperatureCelsius: 24}}, nil
}

type fixedEventFinder struct{ upcomingEvent *domain.RaceEvent }

func (eventFinder fixedEventFinder) UpcomingEvent(context.Context) (*domain.RaceEvent, error) {
	return eventFinder.upcomingEvent, nil
}

var qatarEvent = &domain.RaceEvent{
	ID: "f1-2026-22", Season: 2026, CircuitID: "losail", CircuitName: "Lusail International Circuit",
	Sessions: []domain.RaceSession{
		{SessionKind: domain.SessionPractice1, StartsAt: time.Date(2026, 11, 27, 13, 30, 0, 0, time.UTC)},
		{SessionKind: domain.SessionRace, StartsAt: time.Date(2026, 11, 29, 16, 0, 0, 0, time.UTC)},
	},
}

type testFixture struct {
	service          *Service
	resultsProvider  *fakeResultsProvider
	forecastProvider *fakeForecastProvider
	currentTime      time.Time
}

func newTestFixture(test *testing.T) *testFixture {
	test.Helper()
	store, err := sqlite.OpenInMemory()
	if err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() { _ = store.Close() })

	fixture := &testFixture{
		resultsProvider:  &fakeResultsProvider{},
		forecastProvider: &fakeForecastProvider{},
		currentTime:      time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC),
	}
	fixture.service = NewService(store, fixture.resultsProvider, fakeCircuitFactsProvider{}, fixture.forecastProvider,
		fixedEventFinder{upcomingEvent: qatarEvent}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	fixture.service.currentTime = func() time.Time { return fixture.currentTime }
	return fixture
}

func TestLastRaceIsCachedThenServedStaleWhenTheSourceFails(test *testing.T) {
	ctx := context.Background()
	fixture := newTestFixture(test)

	if _, err := fixture.service.LastRace(ctx); err != nil {
		test.Fatal(err)
	}
	if _, err := fixture.service.LastRace(ctx); err != nil {
		test.Fatal(err)
	}
	if fixture.resultsProvider.lastRaceCalls != 1 {
		test.Errorf("second call within the TTL must come from the cache, got %d source calls", fixture.resultsProvider.lastRaceCalls)
	}

	fixture.currentTime = fixture.currentTime.Add(lastRaceCacheTTL + time.Minute)
	fixture.resultsProvider.failLastRace = true
	lastRace, err := fixture.service.LastRace(ctx)
	if err != nil || lastRace == nil || lastRace.EventName != "Las Vegas Grand Prix" {
		test.Errorf("an expired entry must still be served when the source is down, got %+v (%v)", lastRace, err)
	}
}

func TestCircuitProfileKeepsOnlyPastWinnersAndLoadsTheLatestEdition(test *testing.T) {
	fixture := newTestFixture(test)
	fixture.resultsProvider.circuitWinners = []domain.CircuitWinner{
		{Season: 2026, DriverName: "Not raced yet?"}, // must be ignored: same season as the upcoming race
		{Season: 2025, DriverName: "Max Verstappen"}, {Season: 2024, DriverName: "Max Verstappen"},
		{Season: 2023, DriverName: "Max Verstappen"}, {Season: 2021, DriverName: "Lewis Hamilton"},
		{Season: 2010, DriverName: "Old 1"}, {Season: 2009, DriverName: "Old 2"},
	}

	circuitProfile, err := fixture.service.CircuitProfile(context.Background())
	if err != nil || circuitProfile == nil {
		test.Fatalf("expected a profile, got %+v (%v)", circuitProfile, err)
	}
	if len(circuitProfile.RecentWinners) != recentWinnersLimit || circuitProfile.RecentWinners[0].Season != 2025 {
		test.Errorf("expected the %d most recent past winners starting in 2025, got %+v", recentWinnersLimit, circuitProfile.RecentWinners)
	}
	if circuitProfile.PreviousEdition == nil || circuitProfile.PreviousEdition.Season != 2025 {
		test.Errorf("expected the 2025 race as previous edition, got %+v", circuitProfile.PreviousEdition)
	}
	if circuitProfile.Facts == nil || circuitProfile.Facts.LengthMeters != 5380 {
		test.Errorf("expected circuit facts, got %+v", circuitProfile.Facts)
	}
}

func TestPartialCircuitProfileIsShownButNotCached(test *testing.T) {
	ctx := context.Background()
	fixture := newTestFixture(test)
	fixture.resultsProvider.failWinners = true

	circuitProfile, err := fixture.service.CircuitProfile(ctx)
	if err != nil || circuitProfile == nil || circuitProfile.Facts == nil {
		test.Fatalf("facts alone must still be served, got %+v (%v)", circuitProfile, err)
	}

	fixture.resultsProvider.failWinners = false
	fixture.resultsProvider.circuitWinners = []domain.CircuitWinner{{Season: 2025, DriverName: "Max Verstappen"}}
	circuitProfile, _ = fixture.service.CircuitProfile(ctx)
	if circuitProfile == nil || len(circuitProfile.RecentWinners) != 1 {
		test.Errorf("the partial answer must not have been cached, got %+v", circuitProfile)
	}
}

func TestForecastOnlyWithinTheHorizon(test *testing.T) {
	ctx := context.Background()
	fixture := newTestFixture(test)

	fixture.currentTime = qatarEvent.FirstSessionStartsAt().Add(-forecastHorizon - time.Hour)
	weekendForecast, err := fixture.service.WeekendForecast(ctx)
	if err != nil || weekendForecast != nil || fixture.forecastProvider.calls != 0 {
		test.Errorf("no forecast more than a week ahead, got %+v (%v), %d calls", weekendForecast, err, fixture.forecastProvider.calls)
	}

	fixture.currentTime = qatarEvent.FirstSessionStartsAt().Add(-72 * time.Hour)
	weekendForecast, err = fixture.service.WeekendForecast(ctx)
	if err != nil || weekendForecast == nil || len(weekendForecast.Sessions) != 1 || weekendForecast.AttributionURL == "" {
		test.Errorf("expected a forecast with attribution three days ahead, got %+v (%v)", weekendForecast, err)
	}
}
