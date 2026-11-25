// Package httpapi exposes the backend to the desktop UI over loopback HTTP.
package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"speed-news/apps/api/internal/news"
	"speed-news/apps/api/internal/notifier"
	"speed-news/apps/api/internal/raceinsights"
	"speed-news/apps/api/internal/raceweekend"
	"speed-news/apps/api/internal/storage/sqlite"
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
