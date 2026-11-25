package shellbridge

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"speed-news/apps/api/internal/domain"
	"speed-news/apps/api/internal/notifier"
)

// lockedBuffer is written by the reporter goroutine and read by the test.
type lockedBuffer struct {
	mutex  sync.Mutex
	buffer bytes.Buffer
}

func (lockedOutput *lockedBuffer) Write(writtenBytes []byte) (int, error) {
	lockedOutput.mutex.Lock()
	defer lockedOutput.mutex.Unlock()
	return lockedOutput.buffer.Write(writtenBytes)
}

func (lockedOutput *lockedBuffer) lines() []string {
	lockedOutput.mutex.Lock()
	defer lockedOutput.mutex.Unlock()
	return strings.Split(strings.TrimSuffix(lockedOutput.buffer.String(), "\n"), "\n")
}

type fakeSettings struct {
	mutex        sync.Mutex
	userSettings domain.UserSettings
	loadError    error
}

func (settings *fakeSettings) LoadUserSettings(context.Context) (domain.UserSettings, error) {
	settings.mutex.Lock()
	defer settings.mutex.Unlock()
	return settings.userSettings, settings.loadError
}

func (settings *fakeSettings) change(applyChange func(*domain.UserSettings)) {
	settings.mutex.Lock()
	defer settings.mutex.Unlock()
	applyChange(&settings.userSettings)
}

func startReporter(test *testing.T, settings SettingsLoader) (*notifier.Broadcaster, *lockedBuffer) {
	test.Helper()
	broadcaster := notifier.NewBroadcaster()
	output := &lockedBuffer{}
	ctx, cancelReporter := context.WithCancel(context.Background())
	reporterFinished := make(chan struct{})
	go func() {
		NewTrayStateReporter(broadcaster, settings, output, slog.New(slog.NewTextHandler(io.Discard, nil))).Run(ctx)
		close(reporterFinished)
	}()
	test.Cleanup(func() {
		cancelReporter()
		<-reporterFinished
	})
	return broadcaster, output
}

func waitForLines(test *testing.T, output *lockedBuffer, expectedLines []string) {
	test.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Join(output.lines(), "\n") == strings.Join(expectedLines, "\n") {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	test.Fatalf("expected lines:\n%s\ngot:\n%s", strings.Join(expectedLines, "\n"), strings.Join(output.lines(), "\n"))
}

func TestReporterWritesTheCountWithTheTraySettings(test *testing.T) {
	settings := &fakeSettings{userSettings: domain.DefaultUserSettings()}
	broadcaster, output := startReporter(test, settings)

	broadcaster.Publish(notifier.UnreadCountChanged{UnreadCount: 4})
	waitForLines(test, output, []string{
		`SPEED_NEWS_TRAY {"unreadCount":4,"showUnreadIndicator":true,"interfaceLanguage":"system","closeOnFocusLoss":true}`,
	})
}

// Saving settings republishes the same count: the line must change with the settings, and only then.
func TestReporterFollowsSettingsAndSkipsRepeatedStates(test *testing.T) {
	settings := &fakeSettings{userSettings: domain.DefaultUserSettings()}
	broadcaster, output := startReporter(test, settings)

	broadcaster.Publish(notifier.UnreadCountChanged{UnreadCount: 2})
	broadcaster.Publish(notifier.UnreadCountChanged{UnreadCount: 2})
	firstLine := `SPEED_NEWS_TRAY {"unreadCount":2,"showUnreadIndicator":true,"interfaceLanguage":"system","closeOnFocusLoss":true}`
	waitForLines(test, output, []string{firstLine})

	settings.change(func(userSettings *domain.UserSettings) {
		userSettings.ShowUnreadIndicator = false
		userSettings.InterfaceLanguage = domain.InterfaceLanguageEnglish
		userSettings.CloseOnFocusLoss = false
	})
	broadcaster.Publish(notifier.UnreadCountChanged{UnreadCount: 2})
	waitForLines(test, output, []string{
		firstLine,
		`SPEED_NEWS_TRAY {"unreadCount":2,"showUnreadIndicator":false,"interfaceLanguage":"en","closeOnFocusLoss":false}`,
	})
}

// A count published before the reporter subscribed (startup) must still reach the tray.
func TestReporterStartsWithTheLatestPublishedCount(test *testing.T) {
	settings := &fakeSettings{userSettings: domain.DefaultUserSettings()}
	broadcaster := notifier.NewBroadcaster()
	broadcaster.Publish(notifier.UnreadCountChanged{UnreadCount: 9})

	output := &lockedBuffer{}
	ctx, cancelReporter := context.WithCancel(context.Background())
	defer cancelReporter()
	go NewTrayStateReporter(broadcaster, settings, output, slog.New(slog.NewTextHandler(io.Discard, nil))).Run(ctx)

	waitForLines(test, output, []string{
		`SPEED_NEWS_TRAY {"unreadCount":9,"showUnreadIndicator":true,"interfaceLanguage":"system","closeOnFocusLoss":true}`,
	})
}

func TestReporterFallsBackToDefaultSettingsWhenTheyCannotBeRead(test *testing.T) {
	settings := &fakeSettings{loadError: errors.New("database is locked")}
	broadcaster, output := startReporter(test, settings)

	broadcaster.Publish(notifier.UnreadCountChanged{UnreadCount: 1})
	waitForLines(test, output, []string{
		`SPEED_NEWS_TRAY {"unreadCount":1,"showUnreadIndicator":true,"interfaceLanguage":"system","closeOnFocusLoss":true}`,
	})
}
