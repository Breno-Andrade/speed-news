// Package jolpica reads the F1 season calendar from the Jolpica API (Ergast successor).
package jolpica

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"speed-news/apps/api/internal/domain"
)

const DefaultBaseURL = "https://api.jolpi.ca/ergast/f1"

// Client fetches F1 seasons.
type Client struct {
	httpClient *http.Client
	baseURL    string
	userAgent  string
}

func NewClient(httpClient *http.Client, baseURL string, userAgent string) *Client {
	return &Client{httpClient: httpClient, baseURL: baseURL, userAgent: userAgent}
}

type sessionSchedule struct {
	Date string `json:"date"`
	Time string `json:"time"`
}

type raceResponse struct {
	Season   string `json:"season"`
	Round    string `json:"round"`
	RaceName string `json:"raceName"`
	Circuit  struct {
		CircuitID   string `json:"circuitId"`
		CircuitName string `json:"circuitName"`
		Location    struct {
			Latitude  string `json:"lat"`
			Longitude string `json:"long"`
			Locality  string `json:"locality"`
			Country   string `json:"country"`
		} `json:"Location"`
	} `json:"Circuit"`
	Date             string           `json:"date"`
	Time             string           `json:"time"`
	FirstPractice    *sessionSchedule `json:"FirstPractice"`
	SecondPractice   *sessionSchedule `json:"SecondPractice"`
	ThirdPractice    *sessionSchedule `json:"ThirdPractice"`
	SprintQualifying *sessionSchedule `json:"SprintQualifying"`
	SprintShootout   *sessionSchedule `json:"SprintShootout"`
	Sprint           *sessionSchedule `json:"Sprint"`
	Qualifying       *sessionSchedule `json:"Qualifying"`
}

type seasonResponse struct {
	MRData struct {
		RaceTable struct {
			Races []raceResponse `json:"Races"`
		} `json:"RaceTable"`
	} `json:"MRData"`
}

// FetchSeason returns every race event of a season. An unpublished season returns an empty slice.
func (client *Client) FetchSeason(ctx context.Context, season int) ([]domain.RaceEvent, error) {
	seasonURL := fmt.Sprintf("%s/%d/races/?limit=100", client.baseURL, season)
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, seasonURL, nil)
	if err != nil {
		return nil, err
	}
	httpRequest.Header.Set("User-Agent", client.userAgent)
	httpRequest.Header.Set("Accept", "application/json")

	httpResponse, err := client.httpClient.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("fetch season %d: %w", season, err)
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch season %d: unexpected status %d", season, httpResponse.StatusCode)
	}

	var decodedSeason seasonResponse
	if err := json.NewDecoder(httpResponse.Body).Decode(&decodedSeason); err != nil {
		return nil, fmt.Errorf("decode season %d: %w", season, err)
	}
	return convertRaces(decodedSeason.MRData.RaceTable.Races)
}

// ParseSeasonJSON is exposed for tests and offline fixtures.
func ParseSeasonJSON(seasonJSON []byte) ([]domain.RaceEvent, error) {
	var decodedSeason seasonResponse
	if err := json.Unmarshal(seasonJSON, &decodedSeason); err != nil {
		return nil, err
	}
	return convertRaces(decodedSeason.MRData.RaceTable.Races)
}

func convertRaces(races []raceResponse) ([]domain.RaceEvent, error) {
	raceEvents := make([]domain.RaceEvent, 0, len(races))
	for _, race := range races {
		raceEvent, err := convertRace(race)
		if err != nil {
			return nil, err
		}
		raceEvents = append(raceEvents, raceEvent)
	}
	return raceEvents, nil
}

func convertRace(race raceResponse) (domain.RaceEvent, error) {
	season, err := strconv.Atoi(race.Season)
	if err != nil {
		return domain.RaceEvent{}, fmt.Errorf("invalid season %q", race.Season)
	}
	round, err := strconv.Atoi(race.Round)
	if err != nil {
		return domain.RaceEvent{}, fmt.Errorf("invalid round %q", race.Round)
	}
	latitude, _ := strconv.ParseFloat(race.Circuit.Location.Latitude, 64)
	longitude, _ := strconv.ParseFloat(race.Circuit.Location.Longitude, 64)

	scheduledSessions := []struct {
		sessionKind domain.SessionKind
		schedule    *sessionSchedule
	}{
		{domain.SessionPractice1, race.FirstPractice},
		{domain.SessionPractice2, race.SecondPractice},
		{domain.SessionPractice3, race.ThirdPractice},
		{domain.SessionSprintQualifying, race.SprintQualifying},
		{domain.SessionSprintQualifying, race.SprintShootout},
		{domain.SessionSprint, race.Sprint},
		{domain.SessionQualifying, race.Qualifying},
		{domain.SessionRace, &sessionSchedule{Date: race.Date, Time: race.Time}},
	}

	raceSessions := make([]domain.RaceSession, 0, len(scheduledSessions))
	for _, scheduledSession := range scheduledSessions {
		if scheduledSession.schedule == nil || scheduledSession.schedule.Date == "" {
			continue
		}
		startsAt, startTimeConfirmed, err := parseSessionStart(*scheduledSession.schedule)
		if err != nil {
			return domain.RaceEvent{}, fmt.Errorf("%s %s: %w", race.RaceName, scheduledSession.sessionKind, err)
		}
		raceSessions = append(raceSessions, domain.RaceSession{
			SessionKind:        scheduledSession.sessionKind,
			StartsAt:           startsAt,
			StartTimeConfirmed: startTimeConfirmed,
		})
	}

	return domain.RaceEvent{
		ID:          fmt.Sprintf("%s-%d-%02d", domain.SeriesFormula1, season, round),
		SeriesSlug:  domain.SeriesFormula1,
		Season:      season,
		Round:       round,
		EventName:   race.RaceName,
		CircuitID:   race.Circuit.CircuitID,
		CircuitName: race.Circuit.CircuitName,
		Locality:    race.Circuit.Location.Locality,
		Country:     race.Circuit.Location.Country,
		Coordinates: domain.GeoCoordinates{Latitude: latitude, Longitude: longitude},
		Sessions:    raceSessions,
	}, nil
}

// parseSessionStart combines date and time (UTC). When the time is still unknown it returns midnight UTC, unconfirmed.
func parseSessionStart(schedule sessionSchedule) (time.Time, bool, error) {
	if schedule.Time == "" {
		sessionDate, err := time.Parse(time.DateOnly, schedule.Date)
		return sessionDate.UTC(), false, err
	}
	startsAt, err := time.Parse(time.RFC3339, schedule.Date+"T"+schedule.Time)
	return startsAt.UTC(), true, err
}
