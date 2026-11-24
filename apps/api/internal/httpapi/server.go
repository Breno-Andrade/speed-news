// Package httpapi exposes the backend to the desktop UI over loopback HTTP.
package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"speed-news/apps/api/internal/domain"
	"speed-news/apps/api/internal/news"
	"speed-news/apps/api/internal/notifier"
	"speed-news/apps/api/internal/raceinsights"
	"speed-news/apps/api/internal/raceweekend"
	"speed-news/apps/api/internal/storage/sqlite"
)

const (
	defaultArticleLimit  = 50
	maximumArticleLimit  = 200
	sseKeepAliveInterval = 25 * time.Second
)

// RefreshRequester is implemented by the scheduler.
type RefreshRequester interface {
	RequestRefreshNow()
}

type Dependencies struct {
	NewsService        *news.Service
	RaceWeekendService *raceweekend.Service
	RaceInsights       *raceinsights.Service
	Store              *sqlite.Store
	Broadcaster        *notifier.Broadcaster
	RefreshRequester   RefreshRequester
	Logger             *slog.Logger
	// APIToken protects every route except /health. Empty disables the check (development only).
	APIToken string
	// AllowedOrigins lists browser origins allowed by CORS (Vite dev server, Tauri webview).
	AllowedOrigins []string
}

type server struct {
	Dependencies
}

// NewHandler builds the router with all middlewares.
func NewHandler(dependencies Dependencies) http.Handler {
	apiServer := &server{Dependencies: dependencies}
	router := http.NewServeMux()

	router.HandleFunc("GET /health", apiServer.handleHealth)
	router.HandleFunc("GET /articles", apiServer.handleListArticles)
	router.HandleFunc("GET /articles/unread-count", apiServer.handleUnreadCount)
	router.HandleFunc("POST /articles/{articleId}/read", apiServer.handleMarkArticleRead)
	router.HandleFunc("POST /articles/{articleId}/unread", apiServer.handleMarkArticleUnread)
	router.HandleFunc("POST /articles/read-all", apiServer.handleMarkAllArticlesRead)
	router.HandleFunc("POST /articles/refresh", apiServer.handleRefreshArticles)
	router.HandleFunc("GET /sources", apiServer.handleListSources)
	router.HandleFunc("GET /race-weekend/current", apiServer.handleCurrentRaceWeekend)
	router.HandleFunc("GET /race-weekend/last-race", apiServer.handleLastRace)
	router.HandleFunc("GET /race-weekend/circuit", apiServer.handleCircuitProfile)
	router.HandleFunc("GET /race-weekend/forecast", apiServer.handleWeekendForecast)
	router.HandleFunc("GET /settings", apiServer.handleGetSettings)
	router.HandleFunc("PUT /settings", apiServer.handlePutSettings)
	router.HandleFunc("GET /events", apiServer.handleEvents)

	return apiServer.withCORS(apiServer.withAuthentication(router))
}

// ---------- middlewares ----------

func (apiServer *server) withAuthentication(nextHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if apiServer.APIToken == "" || request.URL.Path == "/health" || request.Method == http.MethodOptions {
			nextHandler.ServeHTTP(responseWriter, request)
			return
		}
		providedToken := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
		if providedToken == "" {
			// EventSource cannot send headers, so /events accepts the token as a query parameter.
			providedToken = request.URL.Query().Get("token")
		}
		if subtle.ConstantTimeCompare([]byte(providedToken), []byte(apiServer.APIToken)) != 1 {
			writeError(responseWriter, http.StatusUnauthorized, "invalid or missing token")
			return
		}
		nextHandler.ServeHTTP(responseWriter, request)
	})
}

func (apiServer *server) withCORS(nextHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		requestOrigin := request.Header.Get("Origin")
		for _, allowedOrigin := range apiServer.AllowedOrigins {
			if requestOrigin == allowedOrigin {
				responseWriter.Header().Set("Access-Control-Allow-Origin", requestOrigin)
				responseWriter.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				responseWriter.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
				responseWriter.Header().Add("Vary", "Origin")
				break
			}
		}
		if request.Method == http.MethodOptions {
			responseWriter.WriteHeader(http.StatusNoContent)
			return
		}
		nextHandler.ServeHTTP(responseWriter, request)
	})
}

// ---------- handlers ----------

func (apiServer *server) handleHealth(responseWriter http.ResponseWriter, _ *http.Request) {
	writeJSON(responseWriter, http.StatusOK, map[string]string{"status": "ok"})
}

func (apiServer *server) handleListArticles(responseWriter http.ResponseWriter, request *http.Request) {
	queryValues := request.URL.Query()
	onlyUnread := queryValues.Get("status") == "unread"

	articleLimit := defaultArticleLimit
	if rawLimit := queryValues.Get("limit"); rawLimit != "" {
		parsedLimit, err := strconv.Atoi(rawLimit)
		if err != nil || parsedLimit < 1 || parsedLimit > maximumArticleLimit {
			writeError(responseWriter, http.StatusBadRequest, fmt.Sprintf("limit must be between 1 and %d", maximumArticleLimit))
			return
		}
		articleLimit = parsedLimit
	}

	articles, err := apiServer.NewsService.ListArticles(request.Context(), onlyUnread, articleLimit)
	if err != nil {
		apiServer.writeInternalError(responseWriter, "list articles", err)
		return
	}
	writeJSON(responseWriter, http.StatusOK, map[string]any{"articles": articles})
}

func (apiServer *server) handleUnreadCount(responseWriter http.ResponseWriter, request *http.Request) {
	unreadCount, err := apiServer.NewsService.CountUnreadArticles(request.Context())
	if err != nil {
		apiServer.writeInternalError(responseWriter, "count unread", err)
		return
	}
	writeJSON(responseWriter, http.StatusOK, notifier.UnreadCountChanged{UnreadCount: unreadCount})
}

func (apiServer *server) handleMarkArticleRead(responseWriter http.ResponseWriter, request *http.Request) {
	wasFound, err := apiServer.NewsService.MarkArticleRead(request.Context(), request.PathValue("articleId"))
	if err != nil {
		apiServer.writeInternalError(responseWriter, "mark article read", err)
		return
	}
	if !wasFound {
		writeError(responseWriter, http.StatusNotFound, "article not found")
		return
	}
	responseWriter.WriteHeader(http.StatusNoContent)
}

func (apiServer *server) handleMarkArticleUnread(responseWriter http.ResponseWriter, request *http.Request) {
	wasFound, err := apiServer.NewsService.MarkArticleUnread(request.Context(), request.PathValue("articleId"))
	if err != nil {
		apiServer.writeInternalError(responseWriter, "mark article unread", err)
		return
	}
	if !wasFound {
		writeError(responseWriter, http.StatusNotFound, "article not found")
		return
	}
	responseWriter.WriteHeader(http.StatusNoContent)
}

func (apiServer *server) handleMarkAllArticlesRead(responseWriter http.ResponseWriter, request *http.Request) {
	if err := apiServer.NewsService.MarkAllArticlesRead(request.Context()); err != nil {
		apiServer.writeInternalError(responseWriter, "mark all read", err)
		return
	}
	responseWriter.WriteHeader(http.StatusNoContent)
}

func (apiServer *server) handleRefreshArticles(responseWriter http.ResponseWriter, _ *http.Request) {
	apiServer.RefreshRequester.RequestRefreshNow()
	responseWriter.WriteHeader(http.StatusAccepted)
}

func (apiServer *server) handleListSources(responseWriter http.ResponseWriter, _ *http.Request) {
	writeJSON(responseWriter, http.StatusOK, map[string]any{"sources": apiServer.NewsService.NewsSources()})
}

// The three race card panels answer 200 with a null payload when there is nothing to show
// (no race yet, forecast still too far ahead) and 502 when the source failed with no cache.
func (apiServer *server) handleLastRace(responseWriter http.ResponseWriter, request *http.Request) {
	lastRace, err := apiServer.RaceInsights.LastRace(request.Context())
	apiServer.writeInsight(responseWriter, "lastRace", lastRace, err)
}

func (apiServer *server) handleCircuitProfile(responseWriter http.ResponseWriter, request *http.Request) {
	circuitProfile, err := apiServer.RaceInsights.CircuitProfile(request.Context())
	apiServer.writeInsight(responseWriter, "circuit", circuitProfile, err)
}

func (apiServer *server) handleWeekendForecast(responseWriter http.ResponseWriter, request *http.Request) {
	weekendForecast, err := apiServer.RaceInsights.WeekendForecast(request.Context())
	apiServer.writeInsight(responseWriter, "forecast", weekendForecast, err)
}

func (apiServer *server) writeInsight(responseWriter http.ResponseWriter, responseField string, insightPayload any, err error) {
	if err != nil {
		apiServer.Logger.Warn(responseField+" unavailable", "error", err)
		writeError(responseWriter, http.StatusBadGateway, "source unavailable, try again later")
		return
	}
	writeJSON(responseWriter, http.StatusOK, map[string]any{responseField: insightPayload})
}

func (apiServer *server) handleCurrentRaceWeekend(responseWriter http.ResponseWriter, request *http.Request) {
	raceWeekendOverview, err := apiServer.RaceWeekendService.CurrentOverview(request.Context())
	if err != nil {
		apiServer.writeInternalError(responseWriter, "current race weekend", err)
		return
	}
	writeJSON(responseWriter, http.StatusOK, map[string]any{"raceWeekend": raceWeekendOverview})
}

func (apiServer *server) handleGetSettings(responseWriter http.ResponseWriter, request *http.Request) {
	userSettings, err := apiServer.Store.LoadUserSettings(request.Context())
	if err != nil {
		apiServer.writeInternalError(responseWriter, "load settings", err)
		return
	}
	writeJSON(responseWriter, http.StatusOK, userSettings)
}

func (apiServer *server) handlePutSettings(responseWriter http.ResponseWriter, request *http.Request) {
	// Start from the defaults so a client that omits a field never wipes it with a zero value.
	userSettings := domain.DefaultUserSettings()
	request.Body = http.MaxBytesReader(responseWriter, request.Body, 16*1024)
	if err := json.NewDecoder(request.Body).Decode(&userSettings); err != nil {
		writeError(responseWriter, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := ValidateUserSettings(userSettings, apiServer.availableLanguageCodes()); err != nil {
		writeError(responseWriter, http.StatusBadRequest, err.Error())
		return
	}
	if userSettings.NewsLanguageCodes == nil {
		userSettings.NewsLanguageCodes = []string{}
	}
	if err := apiServer.Store.SaveUserSettings(request.Context(), userSettings); err != nil {
		apiServer.writeInternalError(responseWriter, "save settings", err)
		return
	}
	// The news languages change what the tray counts.
	apiServer.NewsService.PublishUnreadCount(request.Context())
	writeJSON(responseWriter, http.StatusOK, userSettings)
}

// handleEvents streams unread-count changes with Server-Sent Events.
func (apiServer *server) handleEvents(responseWriter http.ResponseWriter, request *http.Request) {
	responseFlusher, canFlush := responseWriter.(http.Flusher)
	if !canFlush {
		writeError(responseWriter, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	responseWriter.Header().Set("Content-Type", "text/event-stream")
	responseWriter.Header().Set("Cache-Control", "no-cache")
	responseWriter.Header().Set("Connection", "keep-alive")

	// Make sure a brand-new client gets the current count right away.
	apiServer.NewsService.PublishUnreadCount(request.Context())
	unreadCountChannel, unsubscribe := apiServer.Broadcaster.Subscribe()
	defer unsubscribe()

	keepAliveTicker := time.NewTicker(sseKeepAliveInterval)
	defer keepAliveTicker.Stop()

	for {
		select {
		case <-request.Context().Done():
			return
		case <-keepAliveTicker.C:
			// A failed write means the client is gone: stop streaming.
			if _, err := fmt.Fprint(responseWriter, ": keep-alive\n\n"); err != nil {
				return
			}
			responseFlusher.Flush()
		case unreadCountChanged, isOpen := <-unreadCountChannel:
			if !isOpen {
				return
			}
			encodedPayload, _ := json.Marshal(unreadCountChanged)
			if _, err := fmt.Fprintf(responseWriter, "event: unread-count\ndata: %s\n\n", encodedPayload); err != nil {
				return
			}
			responseFlusher.Flush()
		}
	}
}

// ---------- helpers ----------

func (apiServer *server) availableLanguageCodes() map[string]bool {
	languageCodes := map[string]bool{}
	for _, newsSource := range apiServer.NewsService.NewsSources() {
		languageCodes[newsSource.LanguageCode] = true
	}
	return languageCodes
}

// ValidateUserSettings checks the time zone, the interface language and that every news
// language exists among the configured sources.
func ValidateUserSettings(userSettings domain.UserSettings, availableLanguageCodes map[string]bool) error {
	if err := validateTimeZonePreference(userSettings.TimeZonePreference); err != nil {
		return err
	}

	switch userSettings.InterfaceLanguage {
	case domain.InterfaceLanguageSystem, domain.InterfaceLanguagePortuguese, domain.InterfaceLanguageEnglish:
	default:
		return fmt.Errorf("interfaceLanguage must be %q, %q or %q",
			domain.InterfaceLanguageSystem, domain.InterfaceLanguagePortuguese, domain.InterfaceLanguageEnglish)
	}

	for _, languageCode := range userSettings.NewsLanguageCodes {
		if !availableLanguageCodes[languageCode] {
			return fmt.Errorf("no news source in language %q", languageCode)
		}
	}
	return nil
}

// validateTimeZonePreference accepts mode "system" or mode "manual" with a valid IANA time zone.
func validateTimeZonePreference(timeZonePreference domain.TimeZonePreference) error {
	switch timeZonePreference.Mode {
	case "system":
		return nil
	case "manual":
		if timeZonePreference.TimeZoneID == "" {
			return errors.New("timeZoneId is required when mode is manual")
		}
		if _, err := time.LoadLocation(timeZonePreference.TimeZoneID); err != nil {
			return fmt.Errorf("unknown time zone %q", timeZonePreference.TimeZoneID)
		}
		return nil
	default:
		return fmt.Errorf("timeZonePreference.mode must be \"system\" or \"manual\"")
	}
}

func (apiServer *server) writeInternalError(responseWriter http.ResponseWriter, operationName string, err error) {
	if errors.Is(err, context.Canceled) {
		return
	}
	apiServer.Logger.Error(operationName+" failed", "error", err)
	writeError(responseWriter, http.StatusInternalServerError, "internal error")
}

func writeJSON(responseWriter http.ResponseWriter, statusCode int, responseBody any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	_ = json.NewEncoder(responseWriter).Encode(responseBody)
}

func writeError(responseWriter http.ResponseWriter, statusCode int, errorMessage string) {
	writeJSON(responseWriter, statusCode, map[string]string{"error": errorMessage})
}
