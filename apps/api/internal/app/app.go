// Package app wires the whole backend together. cmd/news-api only parses flags and calls it;
// tests and the E2E suite use it with external sources pointed at a fixture server.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"speed-news/apps/api/internal/calendar/jolpica"
	"speed-news/apps/api/internal/circuits"
	"speed-news/apps/api/internal/domain"
	"speed-news/apps/api/internal/forecast/openmeteo"
	"speed-news/apps/api/internal/httpapi"
	"speed-news/apps/api/internal/ingestion/enrichment/shareimage"
	"speed-news/apps/api/internal/ingestion/fetchers/rss"
	"speed-news/apps/api/internal/ingestion/scheduler"
	"speed-news/apps/api/internal/news"
	"speed-news/apps/api/internal/notifier"
	"speed-news/apps/api/internal/raceinsights"
	"speed-news/apps/api/internal/raceweekend"
	"speed-news/apps/api/internal/sources"
	"speed-news/apps/api/internal/storage/sqlite"
)

// Config holds everything that varies between production, tests and E2E.
type Config struct {
	// ListenAddress is host:port; port 0 picks a free one. Keep the host on loopback.
	ListenAddress  string
	DataDirectory  string
	APIToken       string
	AllowedOrigins []string
	// SourcesFile replaces the embedded news sources when set (tests point it at fixture feeds).
	SourcesFile string
	// Base URLs of external APIs; empty means the real service.
	JolpicaBaseURL   string
	CircuitsBaseURL  string
	OpenMeteoBaseURL string
	HTTPTimeout      time.Duration
	Logger           *slog.Logger
}

// App is a fully wired backend, already listening.
type App struct {
	logger      *slog.Logger
	store       *sqlite.Store
	listener    net.Listener
	httpServer  *http.Server
	scheduler   *scheduler.Scheduler
	newsSources []domain.NewsSource
}

// New opens the database, wires every service and binds the listener (so Port is known before Run).
func New(config Config) (*App, error) {
	applyDefaults(&config)

	newsSources, err := loadNewsSources(config.SourcesFile)
	if err != nil {
		return nil, err
	}
	store, err := sqlite.Open(config.DataDirectory)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{Timeout: config.HTTPTimeout}
	broadcaster := notifier.NewBroadcaster()
	newsService := news.NewService(store, rss.NewFetcher(httpClient), newsSources, broadcaster, config.Logger).
		WithShareImageFinder(shareimage.NewFinder(httpClient, rss.UserAgent))
	jolpicaClient := jolpica.NewClient(httpClient, config.JolpicaBaseURL, rss.UserAgent)
	circuitsClient := circuits.NewClient(httpClient, config.CircuitsBaseURL, rss.UserAgent)
	raceWeekendService := raceweekend.NewService(store, jolpicaClient, circuitsClient, config.Logger)
	raceInsightsService := raceinsights.NewService(store, jolpicaClient, circuitsClient,
		openmeteo.NewClient(httpClient, config.OpenMeteoBaseURL, rss.UserAgent), raceWeekendService, config.Logger)
	jobScheduler := scheduler.NewScheduler(newsService, raceWeekendService, raceInsightsService, config.Logger)

	apiHandler := httpapi.NewHandler(httpapi.Dependencies{
		NewsService:        newsService,
		RaceWeekendService: raceWeekendService,
		RaceInsights:       raceInsightsService,
		Store:              store,
		Broadcaster:        broadcaster,
		RefreshRequester:   jobScheduler,
		Logger:             config.Logger,
		APIToken:           config.APIToken,
		AllowedOrigins:     config.AllowedOrigins,
	})

	listener, err := net.Listen("tcp", config.ListenAddress)
	if err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("listen: %w", err)
	}

	return &App{
		logger:      config.Logger,
		store:       store,
		listener:    listener,
		httpServer:  &http.Server{Handler: apiHandler, ReadHeaderTimeout: 10 * time.Second},
		scheduler:   jobScheduler,
		newsSources: newsSources,
	}, nil
}

func applyDefaults(config *Config) {
	if config.ListenAddress == "" {
		config.ListenAddress = "127.0.0.1:0"
	}
	if config.JolpicaBaseURL == "" {
		config.JolpicaBaseURL = jolpica.DefaultBaseURL
	}
	if config.CircuitsBaseURL == "" {
		config.CircuitsBaseURL = circuits.DefaultBaseURL
	}
	if config.OpenMeteoBaseURL == "" {
		config.OpenMeteoBaseURL = openmeteo.DefaultBaseURL
	}
	if config.HTTPTimeout == 0 {
		config.HTTPTimeout = 30 * time.Second
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
}

func loadNewsSources(sourcesFile string) ([]domain.NewsSource, error) {
	if sourcesFile == "" {
		return sources.LoadDefaultSources()
	}
	return sources.LoadSourcesFile(sourcesFile)
}

// Port is the TCP port actually bound (useful when the config asked for port 0).
func (application *App) Port() int {
	return application.listener.Addr().(*net.TCPAddr).Port
}

// SourceCount is logged at startup.
func (application *App) SourceCount() int {
	return len(application.newsSources)
}

// Run serves HTTP and runs the background jobs until ctx is cancelled, then shuts down gracefully.
func (application *App) Run(ctx context.Context) error {
	defer application.store.Close()

	serverErrors := make(chan error, 1)
	go func() { serverErrors <- application.httpServer.Serve(application.listener) }()
	go application.scheduler.Run(ctx)

	select {
	case <-ctx.Done():
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}

	application.logger.Info("shutting down")
	gracefulContext, cancelGraceful := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelGraceful()
	return application.httpServer.Shutdown(gracefulContext)
}
