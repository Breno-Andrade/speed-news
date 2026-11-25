package jolpica

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"

	"speed-news/apps/api/internal/domain"
)

type resultEntryResponse struct {
	Position string `json:"position"`
	Grid     string `json:"grid"`
	Laps     string `json:"laps"`
	Status   string `json:"status"`
	Driver   struct {
		Code       string `json:"code"`
		GivenName  string `json:"givenName"`
		FamilyName string `json:"familyName"`
	} `json:"Driver"`
	Constructor struct {
		Name string `json:"name"`
	} `json:"Constructor"`
	Time *struct {
		Time string `json:"time"`
	} `json:"Time"`
	FastestLap *struct {
		Rank string `json:"rank"`
		Lap  string `json:"lap"`
		Time struct {
			Time string `json:"time"`
		} `json:"Time"`
	} `json:"FastestLap"`
}

type raceResultsResponse struct {
	Season   string `json:"season"`
	Round    string `json:"round"`
	RaceName string `json:"raceName"`
	Date     string `json:"date"`
	Circuit  struct {
		CircuitID   string `json:"circuitId"`
		CircuitName string `json:"circuitName"`
	} `json:"Circuit"`
	Results []resultEntryResponse `json:"Results"`
}

type resultsTableResponse struct {
	MRData struct {
		RaceTable struct {
			Races []raceResultsResponse `json:"Races"`
		} `json:"RaceTable"`
	} `json:"MRData"`
}

// FetchLastRaceResult returns the most recent race with published results.
func (client *Client) FetchLastRaceResult(ctx context.Context) (*domain.RaceResultSummary, error) {
	resultsTable, err := client.fetchResultsTable(ctx, client.baseURL+"/current/last/results/?limit=100")
	if err != nil {
		return nil, err
	}
	return firstRaceSummary(resultsTable)
}

// FetchRaceResultAtCircuit returns the full result of the race held at circuitID in season.
func (client *Client) FetchRaceResultAtCircuit(ctx context.Context, season int, circuitID string) (*domain.RaceResultSummary, error) {
	resultsURL := fmt.Sprintf("%s/%d/circuits/%s/results/?limit=100", client.baseURL, season, circuitID)
	resultsTable, err := client.fetchResultsTable(ctx, resultsURL)
	if err != nil {
		return nil, err
	}
	return firstRaceSummary(resultsTable)
}

// FetchCircuitWinners lists every Grand Prix winner at circuitID, newest first.
func (client *Client) FetchCircuitWinners(ctx context.Context, circuitID string) ([]domain.CircuitWinner, error) {
	resultsTable, err := client.fetchResultsTable(ctx, fmt.Sprintf("%s/circuits/%s/results/1/?limit=100", client.baseURL, circuitID))
	if err != nil {
		return nil, err
	}
	return convertCircuitWinners(resultsTable), nil
}

func (client *Client) fetchResultsTable(ctx context.Context, resultsURL string) (resultsTableResponse, error) {
	var resultsTable resultsTableResponse
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, resultsURL, nil)
	if err != nil {
		return resultsTable, err
	}
	httpRequest.Header.Set("User-Agent", client.userAgent)
	httpRequest.Header.Set("Accept", "application/json")

	httpResponse, err := client.httpClient.Do(httpRequest)
	if err != nil {
		return resultsTable, fmt.Errorf("fetch %s: %w", resultsURL, err)
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode != http.StatusOK {
		return resultsTable, fmt.Errorf("fetch %s: unexpected status %d", resultsURL, httpResponse.StatusCode)
	}
	if err := json.NewDecoder(httpResponse.Body).Decode(&resultsTable); err != nil {
		return resultsTable, fmt.Errorf("decode %s: %w", resultsURL, err)
	}
	return resultsTable, nil
}

// ParseRaceResultJSON and ParseCircuitWinnersJSON are exposed for tests and offline fixtures.
func ParseRaceResultJSON(resultsJSON []byte) (*domain.RaceResultSummary, error) {
	var resultsTable resultsTableResponse
	if err := json.Unmarshal(resultsJSON, &resultsTable); err != nil {
		return nil, err
	}
	return firstRaceSummary(resultsTable)
}

func ParseCircuitWinnersJSON(winnersJSON []byte) ([]domain.CircuitWinner, error) {
	var resultsTable resultsTableResponse
	if err := json.Unmarshal(winnersJSON, &resultsTable); err != nil {
		return nil, err
	}
	return convertCircuitWinners(resultsTable), nil
}

// firstRaceSummary returns nil (no error) when the table has no race, e.g. a circuit not raced that season.
func firstRaceSummary(resultsTable resultsTableResponse) (*domain.RaceResultSummary, error) {
	races := resultsTable.MRData.RaceTable.Races
	if len(races) == 0 {
		return nil, nil
	}
	return convertRaceResult(races[0])
}

func convertRaceResult(race raceResultsResponse) (*domain.RaceResultSummary, error) {
	season, err := strconv.Atoi(race.Season)
	if err != nil {
		return nil, fmt.Errorf("invalid season %q", race.Season)
	}
	round, _ := strconv.Atoi(race.Round)

	raceSummary := &domain.RaceResultSummary{
		Season:      season,
		Round:       round,
		EventName:   race.RaceName,
		CircuitID:   race.Circuit.CircuitID,
		CircuitName: race.Circuit.CircuitName,
		RaceDate:    race.Date,
		Podium:      []domain.DriverResult{},
	}

	for _, resultEntry := range race.Results {
		driverResult := convertDriverResult(resultEntry)
		if driverResult.Position == 1 {
			raceSummary.Laps, _ = strconv.Atoi(resultEntry.Laps)
		}
		if driverResult.Position >= 1 && driverResult.Position <= 3 {
			raceSummary.Podium = append(raceSummary.Podium, driverResult)
		}
		if driverResult.GridPosition == 1 {
			poleSitter := driverResult
			raceSummary.PoleSitter = &poleSitter
		}
		if resultEntry.FastestLap != nil && resultEntry.FastestLap.Rank == "1" {
			lapNumber, _ := strconv.Atoi(resultEntry.FastestLap.Lap)
			raceSummary.FastestLap = &domain.FastestLap{
				DriverCode: driverResult.DriverCode,
				DriverName: driverResult.DriverName,
				TeamName:   driverResult.TeamName,
				LapTime:    resultEntry.FastestLap.Time.Time,
				LapNumber:  lapNumber,
			}
		}
	}
	sort.Slice(raceSummary.Podium, func(firstIndex, secondIndex int) bool {
		return raceSummary.Podium[firstIndex].Position < raceSummary.Podium[secondIndex].Position
	})
	return raceSummary, nil
}

func convertDriverResult(resultEntry resultEntryResponse) domain.DriverResult {
	position, _ := strconv.Atoi(resultEntry.Position)
	gridPosition, _ := strconv.Atoi(resultEntry.Grid)
	raceTime := resultEntry.Status
	if resultEntry.Time != nil && resultEntry.Time.Time != "" {
		raceTime = resultEntry.Time.Time
	}
	return domain.DriverResult{
		Position:     position,
		DriverCode:   resultEntry.Driver.Code,
		DriverName:   resultEntry.Driver.GivenName + " " + resultEntry.Driver.FamilyName,
		TeamName:     resultEntry.Constructor.Name,
		GridPosition: gridPosition,
		RaceTime:     raceTime,
	}
}

func convertCircuitWinners(resultsTable resultsTableResponse) []domain.CircuitWinner {
	circuitWinners := []domain.CircuitWinner{}
	for _, race := range resultsTable.MRData.RaceTable.Races {
		season, err := strconv.Atoi(race.Season)
		if err != nil || len(race.Results) == 0 {
			continue
		}
		winnerResult := convertDriverResult(race.Results[0])
		circuitWinners = append(circuitWinners, domain.CircuitWinner{
			Season:     season,
			DriverName: winnerResult.DriverName,
			TeamName:   winnerResult.TeamName,
		})
	}
	sort.Slice(circuitWinners, func(firstIndex, secondIndex int) bool {
		return circuitWinners[firstIndex].Season > circuitWinners[secondIndex].Season
	})
	return circuitWinners
}
