// Package app wires the whole backend together. cmd/news-api only parses flags and calls it;
// tests and the E2E suite use it with external sources pointed at a fixture server.
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"speed-news/apps/api/internal/domain"
	"speed-news/apps/api/internal/httpapi"
	"speed-news/apps/api/internal/jobs"
	"speed-news/apps/api/internal/news"
	"speed-news/apps/api/internal/notifier"
	"speed-news/apps/api/internal/providers/f1circuits"
	"speed-news/apps/api/internal/providers/jolpica"
	"speed-news/apps/api/internal/providers/ogimage"
	"speed-news/apps/api/internal/providers/openmeteo"
	"speed-news/apps/api/internal/providers/rss"
	"speed-news/apps/api/internal/raceinsights"
	"speed-news/apps/api/internal/raceweekend"
	"speed-news/apps/api/internal/shellbridge"
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
	// TrayStateOutput receives the tray state lines for the desktop shell (stdout in sidecar
	// mode, see shellbridge). Nil disables them (browser development, tests, E2E).
	TrayStateOutput io.Writer
}

// App is a fully wired backend, already listening.
type App struct {
	logger      *slog.Logger
	store       *sqlite.Store
	listener    net.Listener
	httpServer  *http.Server
	scheduler   *jobs.Scheduler
	newsSources []domain.NewsSource
	newsService *news.Service
	// trayStateReporter is nil when Config.TrayStateOutput is nil.
	trayStateReporter *shellbridge.TrayStateReporter
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

	httpClient := newHTTPClient(config.HTTPTimeout)
	broadcaster := notifier.NewBroadcaster()
	newsService := news.NewService(store, rss.NewFetcher(httpClient), newsSources, broadcaster, config.Logger).
		WithShareImageFinder(ogimage.NewFinder(httpClient, rss.UserAgent))
	jolpicaClient := jolpica.NewClient(httpClient, config.JolpicaBaseURL, rss.UserAgent)
	circuitsClient := f1circuits.NewClient(httpClient, config.CircuitsBaseURL, rss.UserAgent)
	raceWeekendService := raceweekend.NewService(store, jolpicaClient, circuitsClient, config.Logger)
	raceInsightsService := raceinsights.NewService(store, jolpicaClient, circuitsClient,
		openmeteo.NewClient(httpClient, config.OpenMeteoBaseURL, rss.UserAgent), raceWeekendService, config.Logger)
	jobScheduler := jobs.NewScheduler(newsService, raceWeekendService, raceInsightsService, config.Logger).
		WithAfterNewsRefresh(newResourceReleaser(store, httpClient, config.Logger).releaseAfterRefresh)
	var trayStateReporter *shellbridge.TrayStateReporter
	if config.TrayStateOutput != nil {
		trayStateReporter = shellbridge.NewTrayStateReporter(broadcaster, store, config.TrayStateOutput, config.Logger)
	}

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
		newsService: newsService,

		trayStateReporter: trayStateReporter,
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
		config.CircuitsBaseURL = f1circuits.DefaultBaseURL
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
	if application.trayStateReporter != nil {
		go application.trayStateReporter.Run(ctx)
	}
	// The stored count reaches the tray (and early SSE clients) right away, not only after the
	// first refresh, which can take the whole HTTP timeout when a source is down.
	application.newsService.PublishUnreadCount(ctx)

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
