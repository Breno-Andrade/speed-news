package app

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"speed-news/apps/api/internal/fixtures"
)

const integrationToken = "integration-token"

// runningBackend is the real backend wired by New, with every external source pointed at the
// fixture server. Only the network boundary is fake.
type runningBackend struct {
	test          *testing.T
	baseURL       string
	fixtureURL    string
	stopBackend   func()
	dataDirectory string
}

func startBackend(test *testing.T, fixtureURL string, dataDirectory string) *runningBackend {
	test.Helper()
	sourcesFile := filepath.Join(dataDirectory, "sources.yaml")
	if err := os.WriteFile(sourcesFile, []byte(fixtures.SourcesYAML(fixtureURL)), 0o644); err != nil {
		test.Fatal(err)
	}
	application, err := New(Config{
		DataDirectory:    dataDirectory,
		APIToken:         integrationToken,
		SourcesFile:      sourcesFile,
		JolpicaBaseURL:   fixtureURL + "/jolpica",
		CircuitsBaseURL:  fixtureURL + "/circuits",
		OpenMeteoBaseURL: fixtureURL + "/openmeteo",
		HTTPTimeout:      5 * time.Second,
		Logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		test.Fatal(err)
	}

	runContext, cancelRun := context.WithCancel(context.Background())
	runFinished := make(chan error, 1)
	go func() { runFinished <- application.Run(runContext) }()

	backend := &runningBackend{
		test:          test,
		baseURL:       fmt.Sprintf("http://127.0.0.1:%d", application.Port()),
		fixtureURL:    fixtureURL,
		dataDirectory: dataDirectory,
	}
	var isStopped bool
	backend.stopBackend = func() {
		if isStopped {
			return
		}
		isStopped = true
		cancelRun()
		if err := <-runFinished; err != nil {
			test.Errorf("backend stopped with error: %v", err)
		}
	}
	test.Cleanup(backend.stopBackend)
	return backend
}

func (backend *runningBackend) request(method string, path string, requestBody string) (int, []byte) {
	backend.test.Helper()
	httpRequest, err := http.NewRequest(method, backend.baseURL+path, strings.NewReader(requestBody))
	if err != nil {
		backend.test.Fatal(err)
	}
	httpRequest.Header.Set("Authorization", "Bearer "+integrationToken)
	httpResponse, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		backend.test.Fatalf("%s %s: %v", method, path, err)
	}
	defer httpResponse.Body.Close()
	responseBody, _ := io.ReadAll(httpResponse.Body)
	return httpResponse.StatusCode, responseBody
}

func (backend *runningBackend) getJSON(path string, decodedTarget any) {
	backend.test.Helper()
	statusCode, responseBody := backend.request(http.MethodGet, path, "")
	if statusCode != http.StatusOK {
		backend.test.Fatalf("GET %s: status %d: %s", path, statusCode, responseBody)
	}
	if err := json.Unmarshal(responseBody, decodedTarget); err != nil {
		backend.test.Fatalf("GET %s: %v", path, err)
	}
}

func (backend *runningBackend) unreadCount() int {
	backend.test.Helper()
	var unreadResponse struct {
		UnreadCount int `json:"unreadCount"`
	}
	backend.getJSON("/articles/unread-count", &unreadResponse)
	return unreadResponse.UnreadCount
}

func (backend *runningBackend) controlFixtures(controlPath string) {
	backend.test.Helper()
	httpResponse, err := http.Post(backend.fixtureURL+controlPath, "text/plain", nil)
	if err != nil {
		backend.test.Fatal(err)
	}
	httpResponse.Body.Close()
}

func waitUntil(test *testing.T, description string, condition func() bool) {
	test.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	test.Fatalf("timed out waiting until %s", description)
}

// unreadEvents reads the SSE stream and forwards every unread-count payload.
func (backend *runningBackend) unreadEvents(ctx context.Context) <-chan int {
	backend.test.Helper()
	httpRequest, _ := http.NewRequestWithContext(ctx, http.MethodGet, backend.baseURL+"/events?token="+integrationToken, nil)
	httpResponse, err := http.DefaultClient.Do(httpRequest) //nolint:bodyclose // closed by the reader goroutine below
	if err != nil {
		backend.test.Fatal(err)
	}
	if contentType := httpResponse.Header.Get("Content-Type"); contentType != "text/event-stream" {
		backend.test.Fatalf("expected an event stream, got %q", contentType)
	}
	unreadCounts := make(chan int, 16)
	go func() {
		defer httpResponse.Body.Close()
		lineScanner := bufio.NewScanner(httpResponse.Body)
		for lineScanner.Scan() {
			eventLine := lineScanner.Text()
			if payload, isData := strings.CutPrefix(eventLine, "data: "); isData {
				var unreadPayload struct {
					UnreadCount int `json:"unreadCount"`
				}
				if json.Unmarshal([]byte(payload), &unreadPayload) == nil {
					unreadCounts <- unreadPayload.UnreadCount
				}
			}
		}
	}()
	return unreadCounts
}

func expectEvent(test *testing.T, unreadCounts <-chan int, expectedCount int) {
	test.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case receivedCount := <-unreadCounts:
			if receivedCount == expectedCount {
				return
			}
		case <-timeout:
			test.Fatalf("no unread-count event with %d", expectedCount)
		}
	}
}

func TestBackendEndToEndAgainstFixtures(test *testing.T) {
	fixtureServer := httptest.NewServer(fixtures.NewServer(time.Now).Handler())
	defer fixtureServer.Close()
	backend := startBackend(test, fixtureServer.URL, test.TempDir())

	test.Run("rejects requests without the token", func(test *testing.T) {
		httpResponse, err := http.Get(backend.baseURL + "/articles")
		if err != nil {
			test.Fatal(err)
		}
		httpResponse.Body.Close()
		if httpResponse.StatusCode != http.StatusUnauthorized {
			test.Errorf("expected 401, got %d", httpResponse.StatusCode)
		}
	})

	// Fixtures: 4 English (one older than 48 h, stored as read) + 2 Portuguese.
	waitUntil(test, "the first refresh stored the fixture articles", func() bool { return backend.unreadCount() == 5 })

	test.Run("lists articles newest first with normalized fields", func(test *testing.T) {
		var articlesResponse struct {
			Articles []struct {
				Title        string `json:"title"`
				SourceName   string `json:"sourceName"`
				LanguageCode string `json:"languageCode"`
				ImageURL     string `json:"imageUrl"`
				ReadAt       string `json:"readAt"`
			} `json:"articles"`
		}
		backend.getJSON("/articles?status=all&limit=50", &articlesResponse)
		if len(articlesResponse.Articles) != 6 {
			test.Fatalf("expected 6 articles, got %d", len(articlesResponse.Articles))
		}
		newestArticle := articlesResponse.Articles[0]
		if newestArticle.Title != "Fixture English headline one" || newestArticle.ImageURL == "" || newestArticle.SourceName != "Fixture Racing News" {
			test.Errorf("unexpected newest article: %+v", newestArticle)
		}
		if oldestArticle := articlesResponse.Articles[5]; oldestArticle.Title != "Old English story" || oldestArticle.ReadAt == "" {
			test.Errorf("an article older than 48 h must arrive already read: %+v", oldestArticle)
		}
	})

	test.Run("race weekend card and its three panels", func(test *testing.T) {
		var currentResponse struct {
			RaceWeekend struct {
				RaceEvent struct {
					EventName string `json:"eventName"`
					Sessions  []any  `json:"sessions"`
				} `json:"raceEvent"`
				CircuitLayout *struct {
					SVGPath string `json:"svgPath"`
				} `json:"circuitLayout"`
				IsRaceWeekendNow bool `json:"isRaceWeekendNow"`
			} `json:"raceWeekend"`
		}
		waitUntil(test, "the calendar synced", func() bool {
			backend.getJSON("/race-weekend/current", &currentResponse)
			return currentResponse.RaceWeekend.RaceEvent.EventName != ""
		})
		raceWeekend := currentResponse.RaceWeekend
		if raceWeekend.RaceEvent.EventName != fixtures.UpcomingEventName || len(raceWeekend.RaceEvent.Sessions) != 5 || !raceWeekend.IsRaceWeekendNow {
			test.Errorf("unexpected race weekend: %+v", raceWeekend)
		}
		if raceWeekend.CircuitLayout == nil || !strings.HasPrefix(raceWeekend.CircuitLayout.SVGPath, "M") {
			test.Error("the circuit drawing must be projected to an SVG path")
		}

		var lastRaceResponse struct {
			LastRace struct {
				EventName string `json:"eventName"`
				Podium    []struct {
					DriverCode string `json:"driverCode"`
				} `json:"podium"`
				FastestLap struct {
					DriverCode string `json:"driverCode"`
				} `json:"fastestLap"`
			} `json:"lastRace"`
		}
		backend.getJSON("/race-weekend/last-race", &lastRaceResponse)
		if lastRaceResponse.LastRace.EventName != fixtures.LastRaceName || len(lastRaceResponse.LastRace.Podium) != 3 || lastRaceResponse.LastRace.FastestLap.DriverCode != "LAW" {
			test.Errorf("unexpected last race: %+v", lastRaceResponse.LastRace)
		}

		var circuitResponse struct {
			Circuit struct {
				Facts struct {
					LengthMeters int `json:"lengthMeters"`
				} `json:"facts"`
				RecentWinners   []any `json:"recentWinners"`
				PreviousEdition struct {
					PoleSitter struct {
						DriverCode string `json:"driverCode"`
					} `json:"poleSitter"`
				} `json:"previousEdition"`
			} `json:"circuit"`
		}
		backend.getJSON("/race-weekend/circuit", &circuitResponse)
		if circuitResponse.Circuit.Facts.LengthMeters != 4928 || len(circuitResponse.Circuit.RecentWinners) != 3 || circuitResponse.Circuit.PreviousEdition.PoleSitter.DriverCode != "PIA" {
			test.Errorf("unexpected circuit profile: %+v", circuitResponse.Circuit)
		}

		var forecastResponse struct {
			Forecast struct {
				Sessions []struct {
					WeatherCode int `json:"weatherCode"`
				} `json:"sessions"`
			} `json:"forecast"`
		}
		backend.getJSON("/race-weekend/forecast", &forecastResponse)
		if len(forecastResponse.Forecast.Sessions) != 5 || forecastResponse.Forecast.Sessions[0].WeatherCode != 61 {
			test.Errorf("expected a rainy forecast for the 5 sessions, got %+v", forecastResponse.Forecast)
		}
	})

	streamContext, stopStream := context.WithCancel(context.Background())
	defer stopStream()
	unreadCounts := backend.unreadEvents(streamContext)
	expectEvent(test, unreadCounts, 5)

	test.Run("news language filter changes the count and pushes it", func(test *testing.T) {
		statusCode, responseBody := backend.request(http.MethodPut, "/settings", `{"timeZonePreference":{"mode":"system"},"interfaceLanguage":"pt-BR","newsLanguageCodes":["pt"],"closeOnFocusLoss":true}`)
		if statusCode != http.StatusOK {
			test.Fatalf("saving settings failed: %d %s", statusCode, responseBody)
		}
		expectEvent(test, unreadCounts, 2)
		if unreadCount := backend.unreadCount(); unreadCount != 2 {
			test.Errorf("only the 2 Portuguese articles must count, got %d", unreadCount)
		}
	})

	test.Run("a story published while running arrives after a refresh", func(test *testing.T) {
		backend.controlFixtures("/control/publish?language=pt&title=Not%C3%ADcia+urgente")
		if statusCode, _ := backend.request(http.MethodPost, "/articles/refresh", ""); statusCode != http.StatusAccepted {
			test.Fatalf("refresh not accepted: %d", statusCode)
		}
		expectEvent(test, unreadCounts, 3)
	})

	test.Run("read, unread and read-all keep the count in sync", func(test *testing.T) {
		var articlesResponse struct {
			Articles []struct {
				ID string `json:"id"`
			} `json:"articles"`
		}
		backend.getJSON("/articles?status=unread", &articlesResponse)
		firstArticleID := articlesResponse.Articles[0].ID
		backend.request(http.MethodPost, "/articles/"+firstArticleID+"/read", "")
		expectEvent(test, unreadCounts, 2)
		backend.request(http.MethodPost, "/articles/"+firstArticleID+"/unread", "")
		expectEvent(test, unreadCounts, 3)
		backend.request(http.MethodPost, "/articles/read-all", "")
		expectEvent(test, unreadCounts, 0)
	})

	test.Run("a source outage keeps what is already stored", func(test *testing.T) {
		var articlesResponse struct {
			Articles []any `json:"articles"`
		}
		backend.getJSON("/articles?status=all&limit=50", &articlesResponse)
		articleCountBeforeOutage := len(articlesResponse.Articles)

		backend.controlFixtures("/control/outage?source=rss&down=true")
		backend.controlFixtures("/control/outage?source=jolpica&down=true")
		backend.request(http.MethodPost, "/articles/refresh", "")
		time.Sleep(300 * time.Millisecond)

		backend.getJSON("/articles?status=all&limit=50", &articlesResponse)
		if articleCountBeforeOutage == 0 || len(articlesResponse.Articles) != articleCountBeforeOutage {
			test.Errorf("stored articles must survive a feed outage: %d before, %d after", articleCountBeforeOutage, len(articlesResponse.Articles))
		}
		if statusCode, _ := backend.request(http.MethodGet, "/race-weekend/last-race", ""); statusCode != http.StatusOK {
			test.Errorf("cached race results must be served while Jolpica is down, got %d", statusCode)
		}
	})

	test.Run("data survives a restart with every source down", func(test *testing.T) {
		stopStream()
		backend.stopBackend()
		restartedBackend := startBackend(test, fixtureServer.URL, backend.dataDirectory)

		var settingsResponse struct {
			NewsLanguageCodes []string `json:"newsLanguageCodes"`
		}
		restartedBackend.getJSON("/settings", &settingsResponse)
		if len(settingsResponse.NewsLanguageCodes) != 1 || settingsResponse.NewsLanguageCodes[0] != "pt" {
			test.Errorf("settings must persist across restarts, got %+v", settingsResponse)
		}
		var currentResponse struct {
			RaceWeekend *struct {
				RaceEvent struct {
					EventName string `json:"eventName"`
				} `json:"raceEvent"`
			} `json:"raceWeekend"`
		}
		restartedBackend.getJSON("/race-weekend/current", &currentResponse)
		if currentResponse.RaceWeekend == nil || currentResponse.RaceWeekend.RaceEvent.EventName != fixtures.UpcomingEventName {
			test.Error("the stored calendar must be used while Jolpica is down")
		}
	})
}
