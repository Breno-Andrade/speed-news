// Command news-api is the Speed News backend. In production it runs as a Tauri sidecar.
//
// Startup contract with the desktop shell:
//   - the API token comes from the SPEED_NEWS_API_TOKEN environment variable;
//   - once listening, it prints "SPEED_NEWS_API_READY port=<port>" on stdout;
//   - with --exit-on-stdin-close it shuts down when the parent process goes away;
//   - with --report-tray-state it prints "SPEED_NEWS_TRAY {json}" lines for the tray (see shellbridge).
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"

	_ "time/tzdata" // embedded time zone database: validation works even where the OS has none (Windows)

	"speed-news/apps/api/internal/app"
)

type commandOptions struct {
	listenHost       string
	listenPort       int
	dataDirectory    string
	allowedOrigins   string
	exitOnStdinClose bool
	reportTrayState  bool
	verboseLogging   bool
	sourcesFile      string
	jolpicaBaseURL   string
	circuitsBaseURL  string
	openMeteoBaseURL string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "news-api:", err)
		os.Exit(1)
	}
}

func run() error {
	options := parseCommandOptions()
	configureRuntimeForBackgroundProcess()

	logLevel := slog.LevelInfo
	if options.verboseLogging {
		logLevel = slog.LevelDebug
	}
	// Logs go to stderr; stdout is reserved for the startup contract.
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))

	apiToken := os.Getenv("SPEED_NEWS_API_TOKEN")
	if apiToken == "" {
		logger.Warn("SPEED_NEWS_API_TOKEN is empty: authentication disabled (development only)")
	}

	application, err := app.New(app.Config{
		ListenAddress:    net.JoinHostPort(options.listenHost, fmt.Sprint(options.listenPort)),
		DataDirectory:    options.dataDirectory,
		APIToken:         apiToken,
		AllowedOrigins:   splitNonEmpty(options.allowedOrigins),
		SourcesFile:      options.sourcesFile,
		JolpicaBaseURL:   options.jolpicaBaseURL,
		CircuitsBaseURL:  options.circuitsBaseURL,
		OpenMeteoBaseURL: options.openMeteoBaseURL,
		Logger:           logger,
		TrayStateOutput:  trayStateOutput(options.reportTrayState),
	})
	if err != nil {
		return err
	}

	shutdownContext, stopListeningSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopListeningSignals()
	if options.exitOnStdinClose {
		shutdownContext = cancelWhenStdinCloses(shutdownContext)
	}

	fmt.Printf("SPEED_NEWS_API_READY port=%d\n", application.Port())
	logger.Info("news-api listening", "port", application.Port(), "dataDirectory", options.dataDirectory, "sources", application.SourceCount())
	return application.Run(shutdownContext)
}

func parseCommandOptions() commandOptions {
	var options commandOptions
	flag.StringVar(&options.listenHost, "host", "127.0.0.1", "listen address (keep it on loopback)")
	flag.IntVar(&options.listenPort, "port", 0, "listen port; 0 picks a free one")
	flag.StringVar(&options.dataDirectory, "data-dir", defaultDataDirectory(), "directory for the SQLite database")
	flag.StringVar(&options.allowedOrigins, "allowed-origins", "", "comma-separated CORS origins")
	flag.BoolVar(&options.exitOnStdinClose, "exit-on-stdin-close", false, "exit when stdin closes (sidecar mode)")
	flag.BoolVar(&options.reportTrayState, "report-tray-state", false, "print tray state lines on stdout (sidecar mode)")
	flag.BoolVar(&options.verboseLogging, "verbose", false, "debug logging")
	// Test/E2E only: point every external source at a fixture server.
	flag.StringVar(&options.sourcesFile, "sources-file", "", "news sources YAML (default: embedded list)")
	flag.StringVar(&options.jolpicaBaseURL, "jolpica-base-url", "", "Jolpica API base URL (default: real API)")
	flag.StringVar(&options.circuitsBaseURL, "circuits-base-url", "", "f1-circuits base URL (default: GitHub)")
	flag.StringVar(&options.openMeteoBaseURL, "openmeteo-base-url", "", "Open-Meteo forecast URL (default: real API)")
	flag.Parse()
	return options
}

func trayStateOutput(isEnabled bool) io.Writer {
	if !isEnabled {
		return nil
	}
	return os.Stdout
}

// Runtime limits for a process that lives in the tray for months and works in short bursts.
const (
	backgroundMemoryLimitBytes = 48 << 20
	backgroundGCPercent        = 50
)

// configureRuntimeForBackgroundProcess makes the GC collect earlier than the defaults, which are
// tuned for throughput. GOMEMLIMIT / GOGC set in the environment still win (profiling, tests).
// The limit is soft: past it the GC works harder, the process is never killed.
func configureRuntimeForBackgroundProcess() {
	if os.Getenv("GOMEMLIMIT") == "" {
		debug.SetMemoryLimit(backgroundMemoryLimitBytes)
	}
	if os.Getenv("GOGC") == "" {
		debug.SetGCPercent(backgroundGCPercent)
	}
}

func defaultDataDirectory() string {
	userConfigDirectory, err := os.UserConfigDir()
	if err != nil {
		return "speed-news-data"
	}
	return filepath.Join(userConfigDirectory, "speed-news")
}

func cancelWhenStdinCloses(parentContext context.Context) context.Context {
	stdinContext, cancelStdinContext := context.WithCancel(parentContext)
	go func() {
		_, _ = io.Copy(io.Discard, os.Stdin)
		cancelStdinContext()
	}()
	return stdinContext
}

func splitNonEmpty(commaSeparatedValues string) []string {
	splitValues := []string{}
	for _, rawValue := range strings.Split(commaSeparatedValues, ",") {
		if trimmedValue := strings.TrimSpace(rawValue); trimmedValue != "" {
			splitValues = append(splitValues, trimmedValue)
		}
	}
	return splitValues
}
