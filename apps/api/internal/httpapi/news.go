package httpapi

import (
	"fmt"
	"net/http"
	"strconv"

	"speed-news/apps/api/internal/notifier"
)

const (
	defaultArticleLimit = 50
	maximumArticleLimit = 200
)

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
