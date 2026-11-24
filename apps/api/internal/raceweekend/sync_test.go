package raceweekend

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

type fakeCalendarProvider struct {
	eventsBySeason map[int][]domain.RaceEvent
	failingSeason  int
	requestedYears []int
}

func (calendarProvider *fakeCalendarProvider) FetchSeason(_ context.Context, season int) ([]domain.RaceEvent, error) {
	calendarProvider.requestedYears = append(calendarProvider.requestedYears, season)
	if season == calendarProvider.failingSeason {
		return nil, errors.New("jolpica down")
	}
	return calendarProvider.eventsBySeason[season], nil
}

type fakeLayoutProvider struct {
	calls      int
	shouldFail bool
}

func (layoutProvider *fakeLayoutProvider) FetchLayoutForCircuit(_ context.Context, raceEvent domain.RaceEvent) (domain.CircuitLayout, error) {
	layoutProvider.calls++
	if layoutProvider.shouldFail {
		return domain.CircuitLayout{}, errors.New("github down")
	}
	return domain.CircuitLayout{CircuitID: raceEvent.CircuitID, LayoutID: "sg-2008", SVGPath: "M0 0 Z", SVGViewBox: "0 0 1000 1000"}, nil
}

func buildRaceEvent(season int, round int, raceStartsAt time.Time) domain.RaceEvent {
	return domain.RaceEvent{
		ID: string(domain.SeriesFormula1) + "-test-" + raceStartsAt.Format("20060102"), SeriesSlug: domain.SeriesFormula1,
		Season: season, Round: round, EventName: "Test Grand Prix", CircuitID: "marina_bay",
		Sessions: []domain.RaceSession{
			{SessionKind: domain.SessionPractice1, StartsAt: raceStartsAt.Add(-48 * time.Hour), StartTimeConfirmed: true},
			{SessionKind: domain.SessionRace, StartsAt: raceStartsAt, StartTimeConfirmed: true},
		},
	}
}

func newTestService(test *testing.T, calendarProvider SeasonCalendarProvider, layoutProvider CircuitLayoutProvider, currentTime time.Time) *Service {
	test.Helper()
	store, err := sqlite.OpenInMemory()
	if err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() { _ = store.Close() })
	raceWeekendService := NewService(store, calendarProvider, layoutProvider, slog.New(slog.NewTextHandler(io.Discard, nil)))
	raceWeekendService.currentTime = func() time.Time { return currentTime }
	return raceWeekendService
}

func TestSyncCalendarStoresCurrentAndNextSeason(test *testing.T) {
	currentTime := time.Date(2026, 12, 10, 12, 0, 0, 0, time.UTC)
	calendarProvider := &fakeCalendarProvider{eventsBySeason: map[int][]domain.RaceEvent{
		2026: {buildRaceEvent(2026, 24, time.Date(2026, 12, 6, 13, 0, 0, 0, time.UTC))},
		2027: {buildRaceEvent(2027, 1, time.Date(2027, 3, 7, 4, 0, 0, 0, time.UTC))},
	}}
	raceWeekendService := newTestService(test, calendarProvider, &fakeLayoutProvider{}, currentTime)

	raceWeekendService.SyncCalendar(context.Background())

	if len(calendarProvider.requestedYears) != 2 || calendarProvider.requestedYears[0] != 2026 || calendarProvider.requestedYears[1] != 2027 {
		test.Errorf("expected the current and the next season, got %v", calendarProvider.requestedYears)
	}
	// December: this season is over, the next race is the 2027 opener.
	upcomingEvent, err := raceWeekendService.UpcomingEvent(context.Background())
	if err != nil || upcomingEvent == nil || upcomingEvent.Season != 2027 {
		test.Errorf("expected the 2027 opener, got %+v (%v)", upcomingEvent, err)
	}
}

func TestSyncCalendarKeepsStoredDataWhenTheSourceFailsOrIsEmpty(test *testing.T) {
	currentTime := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	singaporeRace := buildRaceEvent(2026, 17, time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC))
	calendarProvider := &fakeCalendarProvider{eventsBySeason: map[int][]domain.RaceEvent{2026: {singaporeRace}}}
	raceWeekendService := newTestService(test, calendarProvider, &fakeLayoutProvider{}, currentTime)
	raceWeekendService.SyncCalendar(context.Background())

	calendarProvider.failingSeason = 2026
	calendarProvider.eventsBySeason = map[int][]domain.RaceEvent{} // and 2027 not published yet
	raceWeekendService.SyncCalendar(context.Background())

	upcomingEvent, _ := raceWeekendService.UpcomingEvent(context.Background())
	if upcomingEvent == nil || upcomingEvent.Round != 17 {
		test.Errorf("a failing or empty source must not wipe the stored calendar, got %+v", upcomingEvent)
	}
}

func TestCurrentOverviewCachesTheCircuitLayout(test *testing.T) {
	currentTime := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	calendarProvider := &fakeCalendarProvider{eventsBySeason: map[int][]domain.RaceEvent{
		2026: {buildRaceEvent(2026, 17, time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC))},
	}}
	layoutProvider := &fakeLayoutProvider{}
	raceWeekendService := newTestService(test, calendarProvider, layoutProvider, currentTime)
	raceWeekendService.SyncCalendar(context.Background())

	for requestIndex := 0; requestIndex < 3; requestIndex++ {
		raceWeekendOverview, err := raceWeekendService.CurrentOverview(context.Background())
		if err != nil || raceWeekendOverview == nil || raceWeekendOverview.CircuitLayout == nil || !raceWeekendOverview.IsRaceWeekendNow {
			test.Fatalf("expected a race weekend overview with layout, got %+v (%v)", raceWeekendOverview, err)
		}
	}
	if layoutProvider.calls != 1 {
		test.Errorf("the circuit drawing must be fetched once and cached, got %d fetches", layoutProvider.calls)
	}
	if !raceWeekendService.IsRaceWeekendNow(context.Background()) {
		test.Error("three days before the race must be race weekend mode")
	}
}

func TestCurrentOverviewWithoutLayoutWhenTheDrawingIsUnavailable(test *testing.T) {
	currentTime := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	calendarProvider := &fakeCalendarProvider{eventsBySeason: map[int][]domain.RaceEvent{
		2026: {buildRaceEvent(2026, 17, time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC))},
	}}
	raceWeekendService := newTestService(test, calendarProvider, &fakeLayoutProvider{shouldFail: true}, currentTime)
	raceWeekendService.SyncCalendar(context.Background())

	raceWeekendOverview, err := raceWeekendService.CurrentOverview(context.Background())
	if err != nil || raceWeekendOverview == nil || raceWeekendOverview.CircuitLayout != nil {
		test.Errorf("the card must still show, just without the drawing; got %+v (%v)", raceWeekendOverview, err)
	}
}

func TestEmptyCalendar(test *testing.T) {
	raceWeekendService := newTestService(test, &fakeCalendarProvider{}, &fakeLayoutProvider{}, time.Now())
	raceWeekendOverview, err := raceWeekendService.CurrentOverview(context.Background())
	if err != nil || raceWeekendOverview != nil || raceWeekendService.IsRaceWeekendNow(context.Background()) {
		test.Errorf("no calendar means no card and no race weekend mode, got %+v (%v)", raceWeekendOverview, err)
	}
}
