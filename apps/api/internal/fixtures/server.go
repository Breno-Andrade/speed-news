// Package fixtures is a fake of every external source (RSS feeds, Jolpica, f1-circuits,
// Open-Meteo) used by the integration tests and the E2E suite.
//
// Dates are relative to when the server starts, so the suite never expires: the upcoming
// "Singapore Grand Prix" is always two days away, and the last race always happened 10 days ago.
//
// Control endpoints let tests change the world while the app runs:
//
//	POST /control/publish?language=en&title=...   adds a brand new article to a feed
//	POST /control/outage?source=jolpica&down=true  makes a source answer 503
//	POST /control/reset                            back to the initial state
package fixtures

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	UpcomingEventName = "Singapore Grand Prix"
	LastRaceName      = "Las Vegas Grand Prix"
	upcomingCircuitID = "marina_bay"
	upcomingLayoutID  = "sg-2008"
)

type fixtureArticle struct {
	title        string
	summary      string
	articleURL   string
	imageURL     string
	languageCode string
	publishedAt  time.Time
}

type fixtureSession struct {
	jolpicaField string // "FirstPractice", "Qualifying"... empty for the race itself
	startsAt     time.Time
}

type fixtureEvent struct {
	round       int
	raceName    string
	circuitID   string
	circuitName string
	locality    string
	country     string
	latitude    float64
	longitude   float64
	sessions    []fixtureSession
}

// Server holds the fake world. It is safe for concurrent use.
type Server struct {
	mutex            sync.Mutex
	startedAt        time.Time
	currentTime      func() time.Time
	extraArticles    []fixtureArticle
	sourcesDown      map[string]bool
	publishedCounter int
}

func NewServer(currentTime func() time.Time) *Server {
	return &Server{startedAt: currentTime().UTC().Truncate(time.Minute), currentTime: currentTime, sourcesDown: map[string]bool{}}
}

// Handler routes every fake source. Mount it at the root of an httptest.Server or a real listener.
func (server *Server) Handler() http.Handler {
	router := http.NewServeMux()
	router.HandleFunc("GET /rss/{feedName}", server.guard("rss", server.handleFeed))
	// One subtree route: patterns ending in "/" match everything below them, so several Jolpica
	// patterns would conflict in ServeMux. routeJolpica dispatches on the path segments instead.
	router.HandleFunc("GET /jolpica/", server.guard("jolpica", server.routeJolpica))
	router.HandleFunc("GET /circuits/f1-locations.json", server.guard("circuits", server.handleCircuitLocations))
	router.HandleFunc("GET /circuits/circuits/{layoutFile}", server.guard("circuits", server.handleCircuitLayout))
	router.HandleFunc("GET /openmeteo", server.guard("openmeteo", server.handleForecast))
	router.HandleFunc("POST /control/publish", server.handlePublish)
	router.HandleFunc("POST /control/outage", server.handleOutage)
	router.HandleFunc("POST /control/reset", server.handleReset)
	return router
}

// SourcesYAML is a news source list pointing at this server's feeds (one English, one Portuguese).
func SourcesYAML(fixtureBaseURL string) string {
	return fmt.Sprintf(`sources:
  - slug: fixture-english
    displayName: Fixture Racing News
    seriesSlug: f1
    languageCode: en
    feedKind: rss
    feedUrl: %[1]s/rss/english.xml
    homepageUrl: %[1]s
  - slug: fixture-portuguese
    displayName: Notícias de Teste
    seriesSlug: f1
    languageCode: pt
    feedKind: rss
    feedUrl: %[1]s/rss/portuguese.xml
    homepageUrl: %[1]s
`, fixtureBaseURL)
}

// ---------- control ----------

func (server *Server) guard(sourceName string, handler http.HandlerFunc) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		server.mutex.Lock()
		isDown := server.sourcesDown[sourceName]
		server.mutex.Unlock()
		if isDown {
			http.Error(responseWriter, sourceName+" is down (fixture outage)", http.StatusServiceUnavailable)
			return
		}
		handler(responseWriter, request)
	}
}

func (server *Server) handlePublish(responseWriter http.ResponseWriter, request *http.Request) {
	languageCode := request.URL.Query().Get("language")
	if languageCode != "pt" {
		languageCode = "en"
	}
	server.mutex.Lock()
	server.publishedCounter++
	articleTitle := request.URL.Query().Get("title")
	if articleTitle == "" {
		articleTitle = fmt.Sprintf("Breaking fixture story %d", server.publishedCounter)
	}
	server.extraArticles = append(server.extraArticles, fixtureArticle{
		title:        articleTitle,
		summary:      "Published by a test while the app was running.",
		articleURL:   fmt.Sprintf("https://fixtures.speed-news.test/published/%d", server.publishedCounter),
		languageCode: languageCode,
		publishedAt:  server.currentTime().UTC(),
	})
	server.mutex.Unlock()
	responseWriter.WriteHeader(http.StatusNoContent)
}

func (server *Server) handleOutage(responseWriter http.ResponseWriter, request *http.Request) {
	sourceName := request.URL.Query().Get("source")
	isDown, _ := strconv.ParseBool(request.URL.Query().Get("down"))
	server.mutex.Lock()
	server.sourcesDown[sourceName] = isDown
	server.mutex.Unlock()
	responseWriter.WriteHeader(http.StatusNoContent)
}

func (server *Server) handleReset(responseWriter http.ResponseWriter, _ *http.Request) {
	server.mutex.Lock()
	server.extraArticles = nil
	server.sourcesDown = map[string]bool{}
	server.mutex.Unlock()
	responseWriter.WriteHeader(http.StatusNoContent)
}

// ---------- RSS ----------

func (server *Server) baseArticles() []fixtureArticle {
	return []fixtureArticle{
		{title: "Fixture English headline one", summary: "First English summary.", articleURL: "https://fixtures.speed-news.test/en/1",
			imageURL: "https://fixtures.speed-news.test/images/1.jpg", languageCode: "en", publishedAt: server.startedAt.Add(-1 * time.Hour)},
		{title: "Fixture English headline two", summary: "Second English summary.", articleURL: "https://fixtures.speed-news.test/en/2",
			languageCode: "en", publishedAt: server.startedAt.Add(-2 * time.Hour)},
		{title: "Fixture English headline three", summary: "Third English summary.", articleURL: "https://fixtures.speed-news.test/en/3",
			languageCode: "en", publishedAt: server.startedAt.Add(-3 * time.Hour)},
		{title: "Manchete de teste em português um", summary: "Primeiro resumo em português.", articleURL: "https://fixtures.speed-news.test/pt/1",
			languageCode: "pt", publishedAt: server.startedAt.Add(-90 * time.Minute)},
		{title: "Manchete de teste em português dois", summary: "Segundo resumo em português.", articleURL: "https://fixtures.speed-news.test/pt/2",
			languageCode: "pt", publishedAt: server.startedAt.Add(-150 * time.Minute)},
		// Older than 48 h: the app must store it as already read.
		{title: "Old English story", summary: "Too old to count as unread.", articleURL: "https://fixtures.speed-news.test/en/old",
			languageCode: "en", publishedAt: server.startedAt.Add(-72 * time.Hour)},
	}
}

func (server *Server) handleFeed(responseWriter http.ResponseWriter, request *http.Request) {
	languageCode := map[string]string{"english.xml": "en", "portuguese.xml": "pt"}[request.PathValue("feedName")]
	if languageCode == "" {
		http.NotFound(responseWriter, request)
		return
	}

	server.mutex.Lock()
	allArticles := append(server.baseArticles(), server.extraArticles...)
	server.mutex.Unlock()

	var feedItems strings.Builder
	for _, article := range allArticles {
		if article.languageCode != languageCode {
			continue
		}
		enclosure := ""
		if article.imageURL != "" {
			enclosure = fmt.Sprintf(`<enclosure url="%s" type="image/jpeg" length="0"/>`, article.imageURL)
		}
		fmt.Fprintf(&feedItems, "<item><title>%s</title><link>%s</link><description>%s</description><pubDate>%s</pubDate>%s</item>\n",
			html.EscapeString(article.title), article.articleURL, html.EscapeString(article.summary),
			article.publishedAt.Format(time.RFC1123Z), enclosure)
	}
	responseWriter.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	fmt.Fprintf(responseWriter, `<?xml version="1.0" encoding="UTF-8"?><rss version="2.0"><channel><title>Fixture %s</title><link>https://fixtures.speed-news.test</link><description>fixture</description>%s</channel></rss>`,
		languageCode, feedItems.String())
}

// ---------- Jolpica ----------

// routeJolpica serves the four Jolpica endpoints the app uses:
//
//	/jolpica/{season}/races/
//	/jolpica/current/last/results/
//	/jolpica/circuits/{circuitId}/results/1/
//	/jolpica/{season}/circuits/{circuitId}/results/
func (server *Server) routeJolpica(responseWriter http.ResponseWriter, request *http.Request) {
	pathSegments := strings.Split(strings.Trim(strings.TrimPrefix(request.URL.Path, "/jolpica/"), "/"), "/")
	switch {
	case len(pathSegments) == 3 && pathSegments[0] == "current" && pathSegments[1] == "last" && pathSegments[2] == "results":
		server.handleLastRace(responseWriter, request)
	case len(pathSegments) == 4 && pathSegments[0] == "circuits" && pathSegments[2] == "results" && pathSegments[3] == "1":
		request.SetPathValue("circuitId", pathSegments[1])
		server.handleCircuitWinners(responseWriter, request)
	case len(pathSegments) == 2 && pathSegments[1] == "races":
		request.SetPathValue("season", pathSegments[0])
		server.handleSeason(responseWriter, request)
	case len(pathSegments) == 4 && pathSegments[1] == "circuits" && pathSegments[3] == "results":
		request.SetPathValue("season", pathSegments[0])
		request.SetPathValue("circuitId", pathSegments[2])
		server.handlePreviousEdition(responseWriter, request)
	default:
		http.NotFound(responseWriter, request)
	}
}

func (server *Server) events() []fixtureEvent {
	upcomingFirstPractice := time.Date(server.startedAt.Year(), server.startedAt.Month(), server.startedAt.Day(), 10, 30, 0, 0, time.UTC).AddDate(0, 0, 2)
	lastRaceStart := server.startedAt.AddDate(0, 0, -10).Truncate(time.Hour)
	return []fixtureEvent{
		{
			round: 1, raceName: LastRaceName, circuitID: "vegas", circuitName: "Las Vegas Strip Street Circuit",
			locality: "Las Vegas", country: "USA", latitude: 36.1147, longitude: -115.173,
			sessions: []fixtureSession{{jolpicaField: "Qualifying", startsAt: lastRaceStart.Add(-24 * time.Hour)}, {startsAt: lastRaceStart}},
		},
		{
			round: 2, raceName: UpcomingEventName, circuitID: upcomingCircuitID, circuitName: "Marina Bay Street Circuit",
			locality: "Marina Bay", country: "Singapore", latitude: 1.2914, longitude: 103.864,
			sessions: []fixtureSession{
				{jolpicaField: "FirstPractice", startsAt: upcomingFirstPractice},
				{jolpicaField: "SprintQualifying", startsAt: upcomingFirstPractice.Add(4 * time.Hour)},
				{jolpicaField: "Sprint", startsAt: upcomingFirstPractice.Add(24*time.Hour - 90*time.Minute)},
				{jolpicaField: "Qualifying", startsAt: upcomingFirstPractice.Add(24*time.Hour + 150*time.Minute)},
				{startsAt: upcomingFirstPractice.Add(48*time.Hour + 90*time.Minute)},
			},
		},
	}
}

func (server *Server) handleSeason(responseWriter http.ResponseWriter, request *http.Request) {
	season, err := strconv.Atoi(request.PathValue("season"))
	if err != nil {
		http.NotFound(responseWriter, request)
		return
	}
	races := []map[string]any{}
	for _, event := range server.events() {
		raceSession := event.sessions[len(event.sessions)-1]
		if raceSession.startsAt.Year() != season {
			continue
		}
		raceEntry := map[string]any{
			"season": strconv.Itoa(season), "round": strconv.Itoa(event.round), "raceName": event.raceName,
			"Circuit": map[string]any{
				"circuitId": event.circuitID, "circuitName": event.circuitName,
				"Location": map[string]string{
					"lat": fmt.Sprint(event.latitude), "long": fmt.Sprint(event.longitude),
					"locality": event.locality, "country": event.country,
				},
			},
			"date": raceSession.startsAt.Format(time.DateOnly), "time": raceSession.startsAt.Format("15:04:05Z"),
		}
		for _, eventSession := range event.sessions[:len(event.sessions)-1] {
			raceEntry[eventSession.jolpicaField] = map[string]string{
				"date": eventSession.startsAt.Format(time.DateOnly), "time": eventSession.startsAt.Format("15:04:05Z"),
			}
		}
		races = append(races, raceEntry)
	}
	writeRaceTable(responseWriter, races)
}

func fixtureResult(position int, driverCode, givenName, familyName, teamName string, gridPosition int, raceTime string, fastestRank int, fastestLapTime string) map[string]any {
	resultEntry := map[string]any{
		"position": strconv.Itoa(position), "grid": strconv.Itoa(gridPosition), "laps": "50", "status": "Finished",
		"Driver":      map[string]string{"code": driverCode, "givenName": givenName, "familyName": familyName},
		"Constructor": map[string]string{"name": teamName},
		"FastestLap":  map[string]any{"rank": strconv.Itoa(fastestRank), "lap": "48", "Time": map[string]string{"time": fastestLapTime}},
	}
	if raceTime != "" {
		resultEntry["Time"] = map[string]string{"time": raceTime}
	}
	return resultEntry
}

func (server *Server) handleLastRace(responseWriter http.ResponseWriter, _ *http.Request) {
	lastEvent := server.events()[0]
	raceStart := lastEvent.sessions[len(lastEvent.sessions)-1].startsAt
	writeRaceTable(responseWriter, []map[string]any{{
		"season": strconv.Itoa(raceStart.Year()), "round": "1", "raceName": lastEvent.raceName, "date": raceStart.Format(time.DateOnly),
		"Circuit": map[string]string{"circuitId": lastEvent.circuitID, "circuitName": lastEvent.circuitName},
		"Results": []map[string]any{
			fixtureResult(1, "VER", "Max", "Verstappen", "Red Bull", 1, "1:27:39.624", 3, "1:36.100"),
			fixtureResult(2, "NOR", "Lando", "Norris", "McLaren", 3, "+0.639", 2, "1:36.000"),
			fixtureResult(3, "PIA", "Oscar", "Piastri", "McLaren", 17, "+1.529", 4, "1:36.300"),
			fixtureResult(4, "LAW", "Liam", "Lawson", "Racing Bulls", 9, "+1 Lap", 1, "1:35.591"),
		},
	}})
}

func (server *Server) handleCircuitWinners(responseWriter http.ResponseWriter, request *http.Request) {
	if request.PathValue("circuitId") != upcomingCircuitID {
		writeRaceTable(responseWriter, []map[string]any{})
		return
	}
	currentSeason := server.startedAt.Year()
	races := []map[string]any{}
	for yearsAgo, winner := range []struct{ code, given, family, team string }{
		{"NOR", "Lando", "Norris", "McLaren"}, {"RUS", "George", "Russell", "Mercedes"}, {"SAI", "Carlos", "Sainz", "Ferrari"},
	} {
		races = append(races, map[string]any{
			"season": strconv.Itoa(currentSeason - 1 - yearsAgo), "round": "17", "raceName": UpcomingEventName,
			"Circuit": map[string]string{"circuitId": upcomingCircuitID, "circuitName": "Marina Bay Street Circuit"},
			"Results": []map[string]any{fixtureResult(1, winner.code, winner.given, winner.family, winner.team, 2, "1:40:52.571", 2, "1:34.000")},
		})
	}
	writeRaceTable(responseWriter, races)
}

func (server *Server) handlePreviousEdition(responseWriter http.ResponseWriter, request *http.Request) {
	writeRaceTable(responseWriter, []map[string]any{{
		"season": request.PathValue("season"), "round": "17", "raceName": UpcomingEventName, "date": "2000-10-05",
		"Circuit": map[string]string{"circuitId": request.PathValue("circuitId"), "circuitName": "Marina Bay Street Circuit"},
		"Results": []map[string]any{
			fixtureResult(1, "NOR", "Lando", "Norris", "McLaren", 2, "1:40:52.571", 2, "1:34.300"),
			fixtureResult(2, "VER", "Max", "Verstappen", "Red Bull", 3, "+20.945", 3, "1:34.500"),
			fixtureResult(3, "PIA", "Oscar", "Piastri", "McLaren", 1, "+41.823", 1, "1:33.808"),
		},
	}})
}

func writeRaceTable(responseWriter http.ResponseWriter, races []map[string]any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(responseWriter).Encode(map[string]any{"MRData": map[string]any{"RaceTable": map[string]any{"Races": races}}})
}

// ---------- f1-circuits ----------

func (server *Server) handleCircuitLocations(responseWriter http.ResponseWriter, _ *http.Request) {
	responseWriter.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(responseWriter).Encode([]map[string]any{
		{"id": upcomingLayoutID, "name": "Marina Bay Street Circuit", "location": "Singapore", "lat": 1.291, "lon": 103.859},
		{"id": "us-2023", "name": "Las Vegas Strip Circuit", "location": "Las Vegas", "lat": 36.11, "lon": -115.17},
	})
}

func (server *Server) handleCircuitLayout(responseWriter http.ResponseWriter, request *http.Request) {
	if request.PathValue("layoutFile") != upcomingLayoutID+".geojson" {
		http.NotFound(responseWriter, request)
		return
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(responseWriter).Encode(map[string]any{
		"type": "FeatureCollection",
		"features": []map[string]any{{
			"type":       "Feature",
			"properties": map[string]any{"id": upcomingLayoutID, "length": 4928, "altitude": 18, "firstgp": 2008},
			"geometry": map[string]any{"type": "LineString", "coordinates": [][2]float64{
				{103.850, 1.286}, {103.864, 1.286}, {103.864, 1.295}, {103.857, 1.293}, {103.850, 1.295}, {103.850, 1.286},
			}},
		}},
	})
}

// ---------- Open-Meteo ----------

func (server *Server) handleForecast(responseWriter http.ResponseWriter, request *http.Request) {
	startDate, startErr := time.Parse(time.DateOnly, request.URL.Query().Get("start_date"))
	endDate, endErr := time.Parse(time.DateOnly, request.URL.Query().Get("end_date"))
	if startErr != nil || endErr != nil || endDate.Before(startDate) {
		http.Error(responseWriter, "invalid date range", http.StatusBadRequest)
		return
	}
	hourlyTimes := []string{}
	temperatures, precipitationProbabilities, windSpeeds, weatherCodes := []float64{}, []int{}, []float64{}, []int{}
	for forecastHour := startDate; forecastHour.Before(endDate.AddDate(0, 0, 1)); forecastHour = forecastHour.Add(time.Hour) {
		hourlyTimes = append(hourlyTimes, forecastHour.Format("2006-01-02T15:04"))
		temperatures = append(temperatures, 28.4)
		precipitationProbabilities = append(precipitationProbabilities, 40)
		windSpeeds = append(windSpeeds, 11.6)
		weatherCodes = append(weatherCodes, 61) // rain: easy to assert in the UI
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(responseWriter).Encode(map[string]any{"hourly": map[string]any{
		"time": hourlyTimes, "temperature_2m": temperatures, "precipitation_probability": precipitationProbabilities,
		"wind_speed_10m": windSpeeds, "weather_code": weatherCodes,
	}})
}
