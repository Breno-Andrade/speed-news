package fixtures_test

// The HTTP clients of every external source, exercised against the fixture server: real
// requests, URL building and decoding; only the remote site is fake. Error paths use a
// server that answers 503.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"speed-news/apps/api/internal/calendar/jolpica"
	"speed-news/apps/api/internal/circuits"
	"speed-news/apps/api/internal/domain"
	"speed-news/apps/api/internal/fixtures"
	"speed-news/apps/api/internal/forecast/openmeteo"
)

const clientUserAgent = "SpeedNews/test"

func startFixtureServer(test *testing.T) (*httptest.Server, time.Time) {
	test.Helper()
	fixedNow := time.Date(2026, 11, 24, 15, 0, 0, 0, time.UTC)
	fixtureServer := httptest.NewServer(fixtures.NewServer(func() time.Time { return fixedNow }).Handler())
	test.Cleanup(fixtureServer.Close)
	return fixtureServer, fixedNow
}

func startUnavailableServer(test *testing.T) *httptest.Server {
	test.Helper()
	unavailableServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		responseWriter.WriteHeader(http.StatusServiceUnavailable)
	}))
	test.Cleanup(unavailableServer.Close)
	return unavailableServer
}

func TestJolpicaClient(test *testing.T) {
	fixtureServer, fixedNow := startFixtureServer(test)
	jolpicaClient := jolpica.NewClient(fixtureServer.Client(), fixtureServer.URL+"/jolpica", clientUserAgent)
	ctx := context.Background()

	raceEvents, err := jolpicaClient.FetchSeason(ctx, fixedNow.Year())
	if err != nil || len(raceEvents) != 2 || raceEvents[1].EventName != fixtures.UpcomingEventName || len(raceEvents[1].Sessions) != 5 {
		test.Errorf("unexpected season: %+v (%v)", raceEvents, err)
	}
	if nextSeason, err := jolpicaClient.FetchSeason(ctx, fixedNow.Year()+1); err != nil || len(nextSeason) != 0 {
		test.Errorf("an unpublished season is empty, not an error: %+v (%v)", nextSeason, err)
	}

	lastRace, err := jolpicaClient.FetchLastRaceResult(ctx)
	if err != nil || lastRace == nil || lastRace.EventName != fixtures.LastRaceName || lastRace.FastestLap == nil || lastRace.FastestLap.DriverCode != "LAW" {
		test.Errorf("unexpected last race: %+v (%v)", lastRace, err)
	}
	if lastRace != nil && lastRace.Podium[2].DriverCode != "PIA" {
		test.Errorf("podium must stop at P3 and keep order, got %+v", lastRace.Podium)
	}

	circuitWinners, err := jolpicaClient.FetchCircuitWinners(ctx, "marina_bay")
	if err != nil || len(circuitWinners) != 3 || circuitWinners[0].Season != fixedNow.Year()-1 {
		test.Errorf("unexpected winners: %+v (%v)", circuitWinners, err)
	}
	previousEdition, err := jolpicaClient.FetchRaceResultAtCircuit(ctx, fixedNow.Year()-1, "marina_bay")
	if err != nil || previousEdition == nil || previousEdition.PoleSitter == nil || previousEdition.PoleSitter.DriverCode != "PIA" {
		test.Errorf("unexpected previous edition: %+v (%v)", previousEdition, err)
	}

	unavailableClient := jolpica.NewClient(http.DefaultClient, startUnavailableServer(test).URL, clientUserAgent)
	if _, err := unavailableClient.FetchSeason(ctx, 2026); err == nil {
		test.Error("a 503 must be an error for the season")
	}
	if _, err := unavailableClient.FetchLastRaceResult(ctx); err == nil {
		test.Error("a 503 must be an error for the last race")
	}
}

func TestCircuitsClient(test *testing.T) {
	fixtureServer, _ := startFixtureServer(test)
	circuitsClient := circuits.NewClient(fixtureServer.Client(), fixtureServer.URL+"/circuits", clientUserAgent)
	singaporeEvent := domain.RaceEvent{CircuitID: "marina_bay", Coordinates: domain.GeoCoordinates{Latitude: 1.2914, Longitude: 103.864}}

	circuitLayout, err := circuitsClient.FetchLayoutForCircuit(context.Background(), singaporeEvent)
	if err != nil || circuitLayout.LayoutID != "sg-2008" || circuitLayout.SVGViewBox != "0 0 1000 1000" || circuitLayout.AttributionURL == "" {
		test.Errorf("unexpected layout: %+v (%v)", circuitLayout, err)
	}
	circuitFacts, err := circuitsClient.FetchCircuitFacts(context.Background(), singaporeEvent)
	if err != nil || circuitFacts.LengthMeters != 4928 || circuitFacts.FirstGrandPrixYear != 2008 {
		test.Errorf("unexpected facts: %+v (%v)", circuitFacts, err)
	}

	farAwayEvent := domain.RaceEvent{CircuitID: "nowhere", Coordinates: domain.GeoCoordinates{Latitude: -40, Longitude: 0}}
	if _, err := circuitsClient.FetchLayoutForCircuit(context.Background(), farAwayEvent); err == nil {
		test.Error("a circuit far from every drawing must not get the wrong track")
	}
	unavailableClient := circuits.NewClient(http.DefaultClient, startUnavailableServer(test).URL, clientUserAgent)
	if _, err := unavailableClient.FetchCircuitFacts(context.Background(), singaporeEvent); err == nil {
		test.Error("a 503 must be an error")
	}
}

func TestOpenMeteoClient(test *testing.T) {
	fixtureServer, fixedNow := startFixtureServer(test)
	forecastClient := openmeteo.NewClient(fixtureServer.Client(), fixtureServer.URL+"/openmeteo", clientUserAgent)
	raceSessions := []domain.RaceSession{
		{SessionKind: domain.SessionQualifying, StartsAt: fixedNow.Add(48*time.Hour + 30*time.Minute)},
		{SessionKind: domain.SessionRace, StartsAt: fixedNow.Add(72 * time.Hour)},
	}

	sessionForecasts, err := forecastClient.FetchSessionForecasts(context.Background(), domain.GeoCoordinates{Latitude: 1.29, Longitude: 103.86}, raceSessions)
	if err != nil || len(sessionForecasts) != 2 || sessionForecasts[1].WeatherCode != 61 || sessionForecasts[1].PrecipitationProbability != 40 {
		test.Errorf("unexpected forecast: %+v (%v)", sessionForecasts, err)
	}
	if emptyForecast, err := forecastClient.FetchSessionForecasts(context.Background(), domain.GeoCoordinates{}, nil); err != nil || len(emptyForecast) != 0 {
		test.Errorf("no sessions means no request and no forecast, got %+v (%v)", emptyForecast, err)
	}

	unavailableClient := openmeteo.NewClient(http.DefaultClient, startUnavailableServer(test).URL, clientUserAgent)
	if _, err := unavailableClient.FetchSessionForecasts(context.Background(), domain.GeoCoordinates{}, raceSessions); err == nil {
		test.Error("a 503 must be an error")
	}
}
