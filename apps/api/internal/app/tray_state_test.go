package app

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"speed-news/apps/api/internal/fixtures"
	"speed-news/apps/api/internal/shellbridge"
)

type lockedBuffer struct {
	mutex  sync.Mutex
	buffer bytes.Buffer
}

func (lockedOutput *lockedBuffer) Write(writtenBytes []byte) (int, error) {
	lockedOutput.mutex.Lock()
	defer lockedOutput.mutex.Unlock()
	return lockedOutput.buffer.Write(writtenBytes)
}

func (lockedOutput *lockedBuffer) String() string {
	lockedOutput.mutex.Lock()
	defer lockedOutput.mutex.Unlock()
	return lockedOutput.buffer.String()
}

// The desktop shell draws the tray from these lines while the popover webview does not exist:
// the count, the "show on icon" setting and the interface language must all reach it.
func TestTrayStateLinesFollowTheCountAndTheSettings(test *testing.T) {
	fixtureServer := httptest.NewServer(fixtures.NewServer(time.Now).Handler())
	defer fixtureServer.Close()
	trayOutput := &lockedBuffer{}
	backend := startBackendWithTrayOutput(test, fixtureServer.URL, test.TempDir(), trayOutput)

	waitUntil(test, "the startup count reaches the tray", func() bool {
		return strings.Contains(trayOutput.String(), shellbridge.TrayStateLinePrefix+`{"unreadCount":0,`)
	})
	waitUntil(test, "the fetched articles reach the tray", func() bool {
		return backend.unreadCount() > 0 && strings.Contains(trayOutput.String(), shellbridge.FormatTrayStateLine(shellbridge.TrayState{
			UnreadCount: backend.unreadCount(), ShowUnreadIndicator: true, InterfaceLanguage: "system", CloseOnFocusLoss: true,
		}))
	})

	statusCode, responseBody := backend.request(http.MethodPut, "/settings",
		`{"timeZonePreference":{"mode":"system"},"interfaceLanguage":"en","newsLanguageCodes":[],"closeOnFocusLoss":false,"showUnreadIndicator":false}`)
	if statusCode != http.StatusOK {
		test.Fatalf("saving settings failed: %d %s", statusCode, responseBody)
	}
	waitUntil(test, "the new settings reach the tray", func() bool {
		return strings.Contains(trayOutput.String(), `"showUnreadIndicator":false,"interfaceLanguage":"en","closeOnFocusLoss":false}`)
	})
	for _, trayLine := range strings.Split(strings.TrimSuffix(trayOutput.String(), "\n"), "\n") {
		if !strings.HasPrefix(trayLine, shellbridge.TrayStateLinePrefix) {
			test.Errorf("only tray state lines may be written, got %q", trayLine)
		}
	}
}
