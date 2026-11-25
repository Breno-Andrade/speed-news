package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Comments sent on idle streams so proxies and the webview do not drop the connection.
const sseKeepAliveInterval = 25 * time.Second

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
