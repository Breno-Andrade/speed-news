// Package openmeteo reads hourly weather forecasts from Open-Meteo (free, no key, CC BY 4.0).
package openmeteo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"speed-news/apps/api/internal/domain"
)

const (
	DefaultBaseURL  = "https://api.open-meteo.com/v1/forecast"
	AttributionText = "Previsão: Open-Meteo (CC BY 4.0)"
	AttributionURL  = "https://open-meteo.com"
	hourLayout      = "2006-01-02T15:04"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	userAgent  string
}

func NewClient(httpClient *http.Client, baseURL string, userAgent string) *Client {
	return &Client{httpClient: httpClient, baseURL: baseURL, userAgent: userAgent}
}

type hourlyForecastResponse struct {
	Hourly struct {
		Time                     []string  `json:"time"`
		TemperatureCelsius       []float64 `json:"temperature_2m"`
		PrecipitationProbability []int     `json:"precipitation_probability"`
		WindSpeedKmh             []float64 `json:"wind_speed_10m"`
		WeatherCode              []int     `json:"weather_code"`
	} `json:"hourly"`
}

// FetchSessionForecasts returns the forecast for the start hour of each session (UTC).
func (client *Client) FetchSessionForecasts(ctx context.Context, coordinates domain.GeoCoordinates, raceSessions []domain.RaceSession) ([]domain.SessionForecast, error) {
	if len(raceSessions) == 0 {
		return []domain.SessionForecast{}, nil
	}
	firstStart, lastStart := raceSessions[0].StartsAt, raceSessions[0].StartsAt
	for _, raceSession := range raceSessions {
		if raceSession.StartsAt.Before(firstStart) {
			firstStart = raceSession.StartsAt
		}
		if raceSession.StartsAt.After(lastStart) {
			lastStart = raceSession.StartsAt
		}
	}

	queryValues := url.Values{}
	queryValues.Set("latitude", fmt.Sprintf("%.4f", coordinates.Latitude))
	queryValues.Set("longitude", fmt.Sprintf("%.4f", coordinates.Longitude))
	queryValues.Set("hourly", "temperature_2m,precipitation_probability,wind_speed_10m,weather_code")
	queryValues.Set("start_date", firstStart.UTC().Format(time.DateOnly))
	queryValues.Set("end_date", lastStart.UTC().Format(time.DateOnly))
	queryValues.Set("timezone", "UTC")

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"?"+queryValues.Encode(), nil)
	if err != nil {
		return nil, err
	}
	httpRequest.Header.Set("User-Agent", client.userAgent)
	httpResponse, err := client.httpClient.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("fetch forecast: %w", err)
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch forecast: unexpected status %d", httpResponse.StatusCode)
	}

	var forecastResponse hourlyForecastResponse
	if err := json.NewDecoder(httpResponse.Body).Decode(&forecastResponse); err != nil {
		return nil, fmt.Errorf("decode forecast: %w", err)
	}
	return MatchSessionsToForecast(forecastResponse, raceSessions), nil
}

// ParseForecastJSON is exposed for tests and offline fixtures.
func ParseForecastJSON(forecastJSON []byte) (hourlyForecastResponse, error) {
	var forecastResponse hourlyForecastResponse
	err := json.Unmarshal(forecastJSON, &forecastResponse)
	return forecastResponse, err
}

// MatchSessionsToForecast picks the hour each session starts in. Sessions outside the forecast
// window are skipped (Open-Meteo forecasts about 16 days ahead).
func MatchSessionsToForecast(forecastResponse hourlyForecastResponse, raceSessions []domain.RaceSession) []domain.SessionForecast {
	hourlyForecast := forecastResponse.Hourly
	hourIndexByTime := make(map[string]int, len(hourlyForecast.Time))
	for hourIndex, hourText := range hourlyForecast.Time {
		hourIndexByTime[hourText] = hourIndex
	}

	sessionForecasts := []domain.SessionForecast{}
	for _, raceSession := range raceSessions {
		hourIndex, isForecasted := hourIndexByTime[raceSession.StartsAt.UTC().Truncate(time.Hour).Format(hourLayout)]
		if !isForecasted || hourIndex >= len(hourlyForecast.TemperatureCelsius) ||
			hourIndex >= len(hourlyForecast.PrecipitationProbability) || hourIndex >= len(hourlyForecast.WindSpeedKmh) ||
			hourIndex >= len(hourlyForecast.WeatherCode) {
			continue
		}
		sessionForecasts = append(sessionForecasts, domain.SessionForecast{
			SessionKind:              raceSession.SessionKind,
			StartsAt:                 raceSession.StartsAt,
			TemperatureCelsius:       hourlyForecast.TemperatureCelsius[hourIndex],
			PrecipitationProbability: hourlyForecast.PrecipitationProbability[hourIndex],
			WindSpeedKmh:             hourlyForecast.WindSpeedKmh[hourIndex],
			WeatherCode:              hourlyForecast.WeatherCode[hourIndex],
		})
	}
	return sessionForecasts
}
