package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"speed-news/apps/api/internal/domain"
	"speed-news/apps/api/internal/news"
	"speed-news/apps/api/internal/notifier"
	"speed-news/apps/api/internal/storage/sqlite"
)

func buildTestHandler(test *testing.T) http.Handler {
	test.Helper()
	store, err := sqlite.OpenInMemory()
	if err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() { store.Close() })
	silentLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	broadcaster := notifier.NewBroadcaster()
	newsSources := []domain.NewsSource{
		{Slug: "portuguese-source", LanguageCode: "pt"},
		{Slug: "english-source", LanguageCode: "en"},
	}
	return NewHandler(Dependencies{
		NewsService:    news.NewService(store, nil, newsSources, broadcaster, silentLogger),
		Store:          store,
		Broadcaster:    broadcaster,
		Logger:         silentLogger,
		APIToken:       "secret-token",
		AllowedOrigins: []string{"http://localhost:1420"},
	})
}

func performRequest(handler http.Handler, method string, path string, requestBody string, authorizationHeader string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(requestBody))
	if authorizationHeader != "" {
		request.Header.Set("Authorization", authorizationHeader)
	}
	responseRecorder := httptest.NewRecorder()
	handler.ServeHTTP(responseRecorder, request)
	return responseRecorder
}

func TestAuthentication(test *testing.T) {
	handler := buildTestHandler(test)

	if response := performRequest(handler, http.MethodGet, "/health", "", ""); response.Code != http.StatusOK {
		test.Errorf("health must be public, got %d", response.Code)
	}
	if response := performRequest(handler, http.MethodGet, "/articles", "", ""); response.Code != http.StatusUnauthorized {
		test.Errorf("missing token must be rejected, got %d", response.Code)
	}
	if response := performRequest(handler, http.MethodGet, "/articles", "", "Bearer wrong"); response.Code != http.StatusUnauthorized {
		test.Errorf("wrong token must be rejected, got %d", response.Code)
	}
	if response := performRequest(handler, http.MethodGet, "/articles", "", "Bearer secret-token"); response.Code != http.StatusOK {
		test.Errorf("valid token must be accepted, got %d", response.Code)
	}
}

func TestSettingsValidation(test *testing.T) {
	handler := buildTestHandler(test)
	validAuthorization := "Bearer secret-token"

	scenarios := []struct {
		requestBody        string
		expectedStatusCode int
	}{
		{`{"timeZonePreference":{"mode":"system"}}`, http.StatusOK},
		{`{"timeZonePreference":{"mode":"manual","timeZoneId":"America/Sao_Paulo"}}`, http.StatusOK},
		{`{"timeZonePreference":{"mode":"manual","timeZoneId":"Mars/Olympus"}}`, http.StatusBadRequest},
		{`{"timeZonePreference":{"mode":"manual"}}`, http.StatusBadRequest},
		{`{"timeZonePreference":{"mode":"sometimes"}}`, http.StatusBadRequest},
		{`not json`, http.StatusBadRequest},
	}
	for _, scenario := range scenarios {
		response := performRequest(handler, http.MethodPut, "/settings", scenario.requestBody, validAuthorization)
		if response.Code != scenario.expectedStatusCode {
			test.Errorf("%s: expected %d, got %d", scenario.requestBody, scenario.expectedStatusCode, response.Code)
		}
	}

	response := performRequest(handler, http.MethodGet, "/settings", "", validAuthorization)
	if !strings.Contains(response.Body.String(), "America/Sao_Paulo") {
		test.Errorf("last valid settings must persist, got %s", response.Body.String())
	}
}

func TestCORSOnlyForAllowedOrigins(test *testing.T) {
	handler := buildTestHandler(test)
	preflight := func(origin string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodOptions, "/articles", nil)
		request.Header.Set("Origin", origin)
		request.Header.Set("Access-Control-Request-Method", "GET")
		responseRecorder := httptest.NewRecorder()
		handler.ServeHTTP(responseRecorder, request)
		return responseRecorder
	}

	allowedResponse := preflight("http://localhost:1420")
	if allowedResponse.Code != http.StatusNoContent || allowedResponse.Header().Get("Access-Control-Allow-Origin") != "http://localhost:1420" {
		test.Errorf("the dev UI origin must be allowed, got %d %q", allowedResponse.Code, allowedResponse.Header().Get("Access-Control-Allow-Origin"))
	}
	if foreignResponse := preflight("https://evil.example"); foreignResponse.Header().Get("Access-Control-Allow-Origin") != "" {
		test.Error("an unknown website must never be allowed to call the local API")
	}
}

func TestListArticlesValidatesTheLimit(test *testing.T) {
	handler := buildTestHandler(test)
	for _, invalidLimit := range []string{"0", "-1", "abc", "201"} {
		if response := performRequest(handler, http.MethodGet, "/articles?limit="+invalidLimit, "", "Bearer secret-token"); response.Code != http.StatusBadRequest {
			test.Errorf("limit=%s: expected 400, got %d", invalidLimit, response.Code)
		}
	}
	if response := performRequest(handler, http.MethodGet, "/articles?limit=200&status=unread", "", "Bearer secret-token"); response.Code != http.StatusOK {
		test.Errorf("the maximum limit must be accepted, got %d", response.Code)
	}
	if response := performRequest(handler, http.MethodGet, "/articles/unread-count", "", "Bearer secret-token"); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"unreadCount":0`) {
		test.Errorf("unexpected unread count answer: %d %s", response.Code, response.Body.String())
	}
}

func TestMarkUnknownArticleReturnsNotFound(test *testing.T) {
	handler := buildTestHandler(test)
	response := performRequest(handler, http.MethodPost, "/articles/unknown/read", "", "Bearer secret-token")
	if response.Code != http.StatusNotFound {
		test.Errorf("expected 404, got %d", response.Code)
	}
}

func TestValidateUserSettingsAcceptsDefaults(test *testing.T) {
	if err := ValidateUserSettings(domain.DefaultUserSettings(), map[string]bool{}); err != nil {
		test.Error(err)
	}
}

func TestLanguageSettings(test *testing.T) {
	handler := buildTestHandler(test)
	validAuthorization := "Bearer secret-token"

	scenarios := []struct {
		requestBody        string
		expectedStatusCode int
	}{
		{`{"timeZonePreference":{"mode":"system"},"interfaceLanguage":"en","newsLanguageCodes":["pt"]}`, http.StatusOK},
		{`{"timeZonePreference":{"mode":"system"},"interfaceLanguage":"pt-BR","newsLanguageCodes":[]}`, http.StatusOK},
		{`{"timeZonePreference":{"mode":"system"},"interfaceLanguage":"klingon"}`, http.StatusBadRequest},
		{`{"timeZonePreference":{"mode":"system"},"newsLanguageCodes":["fr"]}`, http.StatusBadRequest},
	}
	for _, scenario := range scenarios {
		response := performRequest(handler, http.MethodPut, "/settings", scenario.requestBody, validAuthorization)
		if response.Code != scenario.expectedStatusCode {
			test.Errorf("%s: expected %d, got %d (%s)", scenario.requestBody, scenario.expectedStatusCode, response.Code, response.Body.String())
		}
	}

	// A client sending only the time zone (like version 0.1) must not wipe the language fields.
	response := performRequest(handler, http.MethodPut, "/settings", `{"timeZonePreference":{"mode":"system"}}`, validAuthorization)
	if !strings.Contains(response.Body.String(), `"interfaceLanguage":"system"`) || !strings.Contains(response.Body.String(), `"newsLanguageCodes":[]`) {
		test.Errorf("omitted fields must fall back to defaults, got %s", response.Body.String())
	}

	response = performRequest(handler, http.MethodGet, "/sources", "", validAuthorization)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"languageCode":"pt"`) {
		test.Errorf("sources must expose their languages, got %d %s", response.Code, response.Body.String())
	}
}
