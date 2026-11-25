package jolpica

import (
	"os"
	"testing"
	"time"

	"speed-news/apps/api/internal/domain"
)

func TestParseSeasonJSONReadsSingaporeSprintWeekend(test *testing.T) {
	seasonJSON, err := os.ReadFile("testdata/season_2026.json")
	if err != nil {
		test.Fatal(err)
	}
	raceEvents, err := ParseSeasonJSON(seasonJSON)
	if err != nil {
		test.Fatal(err)
	}

	var singaporeEvent *domain.RaceEvent
	for eventIndex := range raceEvents {
		if raceEvents[eventIndex].CircuitID == "marina_bay" {
			singaporeEvent = &raceEvents[eventIndex]
		}
	}
	if singaporeEvent == nil {
		test.Fatal("Singapore Grand Prix not found in fixture")
	}
	if singaporeEvent.ID != "f1-2026-17" || singaporeEvent.CircuitName != "Marina Bay Street Circuit" {
		test.Fatalf("unexpected event identity: %+v", singaporeEvent)
	}

	expectedStartsBySession := map[domain.SessionKind]time.Time{
		domain.SessionPractice1:        time.Date(2026, 10, 9, 8, 30, 0, 0, time.UTC),
		domain.SessionSprintQualifying: time.Date(2026, 10, 9, 12, 30, 0, 0, time.UTC),
		domain.SessionSprint:           time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC),
		domain.SessionQualifying:       time.Date(2026, 10, 10, 13, 0, 0, 0, time.UTC),
		domain.SessionRace:             time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC),
	}
	if len(singaporeEvent.Sessions) != len(expectedStartsBySession) {
		test.Fatalf("expected %d sessions, got %d", len(expectedStartsBySession), len(singaporeEvent.Sessions))
	}
	for _, raceSession := range singaporeEvent.Sessions {
		expectedStart, isExpected := expectedStartsBySession[raceSession.SessionKind]
		if !isExpected {
			test.Errorf("unexpected session %s", raceSession.SessionKind)
			continue
		}
		if !raceSession.StartsAt.Equal(expectedStart) || !raceSession.StartTimeConfirmed {
			test.Errorf("%s: expected %s confirmed, got %s confirmed=%v",
				raceSession.SessionKind, expectedStart, raceSession.StartsAt, raceSession.StartTimeConfirmed)
		}
	}

	// The same instant shown in Brasília must match the official schedule (FP1 at 05:30).
	brasiliaLocation, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		test.Fatal(err)
	}
	firstPracticeInBrasilia := expectedStartsBySession[domain.SessionPractice1].In(brasiliaLocation)
	if firstPracticeInBrasilia.Format("15:04") != "05:30" {
		test.Errorf("expected FP1 at 05:30 in Brasília, got %s", firstPracticeInBrasilia.Format("15:04"))
	}
}

func TestParseSessionStartWithoutTimeIsUnconfirmed(test *testing.T) {
	startsAt, startTimeConfirmed, err := parseSessionStart(sessionSchedule{Date: "2027-03-14"})
	if err != nil {
		test.Fatal(err)
	}
	if startTimeConfirmed || !startsAt.Equal(time.Date(2027, 3, 14, 0, 0, 0, 0, time.UTC)) {
		test.Errorf("expected unconfirmed midnight UTC, got %s confirmed=%v", startsAt, startTimeConfirmed)
	}
}
