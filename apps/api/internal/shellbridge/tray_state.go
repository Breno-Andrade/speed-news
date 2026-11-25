// Package shellbridge reports to the desktop shell (Tauri) over stdout, next to the startup
// "ready" line. The shell owns the tray icon, so it must know what to show even while the
// popover webview does not exist (it is destroyed when hidden to save memory).
package shellbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	"speed-news/apps/api/internal/domain"
	"speed-news/apps/api/internal/notifier"
)

// TrayStateLinePrefix starts every tray state line; the rest of the line is TrayState as JSON.
// Parsed by apps/desktop/src-tauri/src/sidecar.rs: keep both in sync.
const TrayStateLinePrefix = "SPEED_NEWS_TRAY "

// TrayState is everything the shell needs to draw the tray and decide how the popover behaves.
type TrayState struct {
	UnreadCount         int    `json:"unreadCount"`
	ShowUnreadIndicator bool   `json:"showUnreadIndicator"`
	InterfaceLanguage   string `json:"interfaceLanguage"`
	CloseOnFocusLoss    bool   `json:"closeOnFocusLoss"`
}

// SettingsLoader is implemented by the SQLite store.
type SettingsLoader interface {
	LoadUserSettings(ctx context.Context) (domain.UserSettings, error)
}

// TrayStateReporter writes a line whenever the unread count or a tray-related setting changes.
// Saving settings republishes the unread count (see httpapi), which is what wakes it up.
type TrayStateReporter struct {
	broadcaster    *notifier.Broadcaster
	settingsLoader SettingsLoader
	output         io.Writer
	logger         *slog.Logger
}

func NewTrayStateReporter(broadcaster *notifier.Broadcaster, settingsLoader SettingsLoader, output io.Writer,
	logger *slog.Logger) *TrayStateReporter {
	return &TrayStateReporter{broadcaster: broadcaster, settingsLoader: settingsLoader, output: output, logger: logger}
}

// Run blocks until ctx is cancelled. Identical consecutive states are written once.
func (reporter *TrayStateReporter) Run(ctx context.Context) {
	unreadCountChannel, unsubscribe := reporter.broadcaster.Subscribe()
	defer unsubscribe()

	lastWrittenLine := ""
	for {
		select {
		case <-ctx.Done():
			return
		case unreadCountChanged, isOpen := <-unreadCountChannel:
			if !isOpen {
				return
			}
			trayStateLine := FormatTrayStateLine(reporter.currentTrayState(ctx, unreadCountChanged.UnreadCount))
			if trayStateLine == lastWrittenLine {
				continue
			}
			if _, err := io.WriteString(reporter.output, trayStateLine); err != nil {
				reporter.logger.Warn("write tray state failed", "error", err)
				continue
			}
			lastWrittenLine = trayStateLine
		}
	}
}

func (reporter *TrayStateReporter) currentTrayState(ctx context.Context, unreadCount int) TrayState {
	userSettings, err := reporter.settingsLoader.LoadUserSettings(ctx)
	if err != nil {
		reporter.logger.Warn("load settings for the tray failed, using defaults", "error", err)
		userSettings = domain.DefaultUserSettings()
	}
	return TrayState{
		UnreadCount:         unreadCount,
		ShowUnreadIndicator: userSettings.ShowUnreadIndicator,
		InterfaceLanguage:   userSettings.InterfaceLanguage,
		CloseOnFocusLoss:    userSettings.CloseOnFocusLoss,
	}
}

// FormatTrayStateLine renders one newline-terminated line of the stdout contract.
func FormatTrayStateLine(trayState TrayState) string {
	encodedState, _ := json.Marshal(trayState) // a struct of plain fields always encodes
	return fmt.Sprintf("%s%s\n", TrayStateLinePrefix, encodedState)
}
