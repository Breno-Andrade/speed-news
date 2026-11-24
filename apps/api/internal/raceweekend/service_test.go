package raceweekend

import (
	"testing"
	"time"

	"speed-news/apps/api/internal/domain"
)

func TestIsWithinRaceWeekend(test *testing.T) {
	singaporeEvent := domain.RaceEvent{Sessions: []domain.RaceSession{
		{SessionKind: domain.SessionPractice1, StartsAt: time.Date(2026, 10, 9, 8, 30, 0, 0, time.UTC)},
		{SessionKind: domain.SessionRace, StartsAt: time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)},
	}}

	scenarios := []struct {
		scenarioName   string
		currentTime    time.Time
		expectedResult bool
	}{
		{"one week before", time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC), false},
		{"monday of race week", time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC), true},
		{"during the race", time.Date(2026, 10, 11, 13, 0, 0, 0, time.UTC), true},
		{"five hours after race start", time.Date(2026, 10, 11, 17, 0, 0, 0, time.UTC), true},
		{"day after the race", time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC), false},
	}
	for _, scenario := range scenarios {
		if actualResult := IsWithinRaceWeekend(singaporeEvent, scenario.currentTime); actualResult != scenario.expectedResult {
			test.Errorf("%s: expected %v, got %v", scenario.scenarioName, scenario.expectedResult, actualResult)
		}
	}
}
