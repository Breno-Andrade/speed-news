package jolpica

import (
	"os"
	"testing"
)

func TestParseLastRaceResult(test *testing.T) {
	resultsJSON, err := os.ReadFile("testdata/last_race_results.json")
	if err != nil {
		test.Fatal(err)
	}
	raceSummary, err := ParseRaceResultJSON(resultsJSON)
	if err != nil || raceSummary == nil {
		test.Fatalf("expected a race summary, got %v (%v)", raceSummary, err)
	}

	if raceSummary.EventName != "Las Vegas Grand Prix" || raceSummary.Season != 2026 || raceSummary.Round != 21 || raceSummary.Laps != 50 {
		test.Errorf("unexpected race identity: %+v", raceSummary)
	}
	if len(raceSummary.Podium) != 3 {
		test.Fatalf("expected a 3-driver podium, got %d", len(raceSummary.Podium))
	}
	expectedPodium := []struct {
		driverCode string
		raceTime   string
	}{{"VER", "1:27:39.624"}, {"NOR", "+0.639"}, {"PIA", "+1.529"}}
	for podiumIndex, expectedDriver := range expectedPodium {
		podiumDriver := raceSummary.Podium[podiumIndex]
		if podiumDriver.DriverCode != expectedDriver.driverCode || podiumDriver.RaceTime != expectedDriver.raceTime {
			test.Errorf("podium %d: expected %+v, got %+v", podiumIndex+1, expectedDriver, podiumDriver)
		}
	}
	if raceSummary.PoleSitter == nil || raceSummary.PoleSitter.DriverCode != "VER" {
		test.Errorf("expected Verstappen on pole, got %+v", raceSummary.PoleSitter)
	}
	if raceSummary.FastestLap == nil || raceSummary.FastestLap.DriverCode != "LAW" || raceSummary.FastestLap.LapTime != "1:35.591" || raceSummary.FastestLap.LapNumber != 50 {
		test.Errorf("expected Lawson's 1:35.591 on lap 50, got %+v", raceSummary.FastestLap)
	}
}

func TestParseRaceResultWithRetirements(test *testing.T) {
	resultsJSON, err := os.ReadFile("testdata/losail_2025_results.json")
	if err != nil {
		test.Fatal(err)
	}
	raceSummary, err := ParseRaceResultJSON(resultsJSON)
	if err != nil || raceSummary == nil {
		test.Fatalf("expected a race summary, got %v (%v)", raceSummary, err)
	}
	if raceSummary.PoleSitter == nil || raceSummary.PoleSitter.DriverName != "Oscar Piastri" {
		test.Errorf("expected Piastri on pole, got %+v", raceSummary.PoleSitter)
	}
	if raceSummary.Laps != 57 {
		test.Errorf("expected 57 laps, got %d", raceSummary.Laps)
	}
}

func TestParseCircuitWinnersNewestFirst(test *testing.T) {
	winnersJSON, err := os.ReadFile("testdata/losail_winners.json")
	if err != nil {
		test.Fatal(err)
	}
	circuitWinners, err := ParseCircuitWinnersJSON(winnersJSON)
	if err != nil {
		test.Fatal(err)
	}
	if len(circuitWinners) != 4 || circuitWinners[0].Season != 2025 || circuitWinners[len(circuitWinners)-1].DriverName != "Lewis Hamilton" {
		test.Errorf("expected 4 winners from 2025 back to Hamilton in 2021, got %+v", circuitWinners)
	}
}

func TestEmptyResultTableIsNotAnError(test *testing.T) {
	raceSummary, err := ParseRaceResultJSON([]byte(`{"MRData":{"RaceTable":{"Races":[]}}}`))
	if err != nil || raceSummary != nil {
		test.Errorf("a season without that race must return nil, nil; got %v, %v", raceSummary, err)
	}
}
