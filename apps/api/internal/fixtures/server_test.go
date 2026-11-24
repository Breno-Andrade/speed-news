package fixtures

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Every route the app calls must answer 200; this also catches ServeMux pattern conflicts,
// which panic when the handler is built.
func TestEveryFixtureRouteAnswers(test *testing.T) {
	fixedNow := time.Date(2026, 11, 24, 15, 0, 0, 0, time.UTC)
	fixtureServer := httptest.NewServer(NewServer(func() time.Time { return fixedNow }).Handler())
	defer fixtureServer.Close()

	routeExpectations := map[string]string{
		"/rss/english.xml":                                     "Fixture English headline one",
		"/rss/portuguese.xml":                                  "Manchete de teste em português um",
		"/jolpica/2026/races/?limit=100":                       UpcomingEventName,
		"/jolpica/current/last/results/?limit=100":             LastRaceName,
		"/jolpica/circuits/marina_bay/results/1/":              "Norris",
		"/jolpica/2025/circuits/marina_bay/results/":           "Piastri",
		"/circuits/f1-locations.json":                          "sg-2008",
		"/circuits/circuits/sg-2008.geojson":                   "LineString",
		"/openmeteo?start_date=2026-11-26&end_date=2026-11-28": "temperature_2m",
	}
	for routePath, expectedFragment := range routeExpectations {
		httpResponse, err := http.Get(fixtureServer.URL + routePath)
		if err != nil {
			test.Fatalf("%s: %v", routePath, err)
		}
		responseBody, _ := io.ReadAll(httpResponse.Body)
		httpResponse.Body.Close()
		if httpResponse.StatusCode != http.StatusOK || !strings.Contains(string(responseBody), expectedFragment) {
			test.Errorf("%s: expected 200 containing %q, got %d: %.200s", routePath, expectedFragment, httpResponse.StatusCode, responseBody)
		}
	}
}

func TestUpcomingRaceIsAlwaysTwoDaysAhead(test *testing.T) {
	// The season of the upcoming race follows the clock, so the suite never expires.
	for _, fixedNow := range []time.Time{
		time.Date(2026, 11, 24, 15, 0, 0, 0, time.UTC),
		time.Date(2031, 3, 2, 8, 0, 0, 0, time.UTC),
	} {
		fixtureServer := NewServer(func() time.Time { return fixedNow })
		upcomingEvent := fixtureServer.events()[1]
		daysUntilFirstPractice := upcomingEvent.sessions[0].startsAt.Sub(fixedNow).Hours() / 24
		if upcomingEvent.raceName != UpcomingEventName || daysUntilFirstPractice < 1.5 || daysUntilFirstPractice > 3 {
			test.Errorf("at %s: expected %s about two days ahead, got %.1f days", fixedNow, UpcomingEventName, daysUntilFirstPractice)
		}
	}
}

func TestControlEndpoints(test *testing.T) {
	fixtureServer := httptest.NewServer(NewServer(time.Now).Handler())
	defer fixtureServer.Close()

	postControl := func(controlPath string) {
		test.Helper()
		httpResponse, err := http.Post(fixtureServer.URL+controlPath, "text/plain", nil)
		if err != nil || httpResponse.StatusCode != http.StatusNoContent {
			test.Fatalf("%s: unexpected answer %v %v", controlPath, httpResponse, err)
		}
		httpResponse.Body.Close()
	}
	readFeed := func() (int, string) {
		test.Helper()
		httpResponse, err := http.Get(fixtureServer.URL + "/rss/english.xml")
		if err != nil {
			test.Fatal(err)
		}
		defer httpResponse.Body.Close()
		responseBody, _ := io.ReadAll(httpResponse.Body)
		return httpResponse.StatusCode, string(responseBody)
	}

	postControl("/control/publish?language=en&title=Brand+new+story")
	if _, feedBody := readFeed(); !strings.Contains(feedBody, "Brand new story") {
		test.Error("a published article must appear in its feed")
	}

	postControl("/control/outage?source=rss&down=true")
	if statusCode, _ := readFeed(); statusCode != http.StatusServiceUnavailable {
		test.Errorf("expected 503 during an outage, got %d", statusCode)
	}

	postControl("/control/reset")
	statusCode, feedBody := readFeed()
	if statusCode != http.StatusOK || strings.Contains(feedBody, "Brand new story") {
		test.Errorf("reset must restore the initial world, got %d", statusCode)
	}
}
