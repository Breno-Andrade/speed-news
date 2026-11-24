package openmeteo

import (
	"os"
	"testing"
	"time"

	"speed-news/apps/api/internal/domain"
)

func TestMatchSessionsToForecast(test *testing.T) {
	forecastJSON, err := os.ReadFile("testdata/lusail_forecast.json")
	if err != nil {
		test.Fatal(err)
	}
	forecastResponse, err := ParseForecastJSON(forecastJSON)
	if err != nil {
		test.Fatal(err)
	}

	qatarSessions := []domain.RaceSession{
		{SessionKind: domain.SessionPractice1, StartsAt: time.Date(2026, 11, 27, 13, 30, 0, 0, time.UTC)},
		{SessionKind: domain.SessionRace, StartsAt: time.Date(2026, 11, 29, 16, 0, 0, 0, time.UTC)},
		// Outside the downloaded window: must be skipped, not crash.
		{SessionKind: domain.SessionQualifying, StartsAt: time.Date(2026, 12, 15, 16, 0, 0, 0, time.UTC)},
	}
	sessionForecasts := MatchSessionsToForecast(forecastResponse, qatarSessions)
	if len(sessionForecasts) != 2 {
		test.Fatalf("expected 2 forecasted sessions, got %d", len(sessionForecasts))
	}

	// FP1 at 13:30 uses the 13:00 hour.
	practiceHourIndex := indexOfHour(test, forecastResponse, "2026-11-27T13:00")
	if sessionForecasts[0].TemperatureCelsius != forecastResponse.Hourly.TemperatureCelsius[practiceHourIndex] {
		test.Errorf("FP1 must use the 13:00 hour, got %+v", sessionForecasts[0])
	}
	raceHourIndex := indexOfHour(test, forecastResponse, "2026-11-29T16:00")
	if sessionForecasts[1].PrecipitationProbability != forecastResponse.Hourly.PrecipitationProbability[raceHourIndex] ||
		sessionForecasts[1].WeatherCode != forecastResponse.Hourly.WeatherCode[raceHourIndex] {
		test.Errorf("race must use the 16:00 hour, got %+v", sessionForecasts[1])
	}
}

func indexOfHour(test *testing.T, forecastResponse hourlyForecastResponse, hourText string) int {
	test.Helper()
	for hourIndex, forecastHour := range forecastResponse.Hourly.Time {
		if forecastHour == hourText {
			return hourIndex
		}
	}
	test.Fatalf("hour %s not in fixture", hourText)
	return -1
}
