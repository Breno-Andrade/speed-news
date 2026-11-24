package shareimage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func mustParseURL(test *testing.T, rawURL string) *url.URL {
	test.Helper()
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		test.Fatal(err)
	}
	return parsedURL
}

func TestExtractShareImage(test *testing.T) {
	pageURL := mustParseURL(test, "https://www.formula1.com/en/latest/article/story")
	scenarios := []struct {
		scenarioName  string
		pageHTML      string
		expectedImage string
	}{
		{"og:image, property first", `<head><meta property="og:image" content="https://media.formula1.com/a.jpg"></head>`, "https://media.formula1.com/a.jpg"},
		{"og:image, content first", `<head><meta content="https://media.formula1.com/b.jpg" property="og:image" /></head>`, "https://media.formula1.com/b.jpg"},
		{"twitter fallback", `<head><meta name="twitter:image" content="https://media.formula1.com/c.jpg"></head>`, "https://media.formula1.com/c.jpg"},
		{"relative URL is resolved", `<head><meta property="og:image" content="/images/d.jpg"></head>`, "https://www.formula1.com/images/d.jpg"},
		{"entities are decoded", `<head><meta property="og:image" content="https://cdn.example/e.jpg?w=1&amp;h=2"></head>`, "https://cdn.example/e.jpg?w=1&h=2"},
		{"og:image wins over twitter", `<head><meta name="twitter:image" content="https://x/t.jpg"><meta property="og:image" content="https://x/o.jpg"></head>`, "https://x/o.jpg"},
		{"only <head> counts", `<head></head><body><meta property="og:image" content="https://x/body.jpg"></body>`, ""},
		{"no metadata", `<head><title>t</title></head>`, ""},
		{"javascript scheme is refused", `<head><meta property="og:image" content="javascript:alert(1)"></head>`, ""},
	}
	for _, scenario := range scenarios {
		if extractedImage := ExtractShareImage(scenario.pageHTML, pageURL); extractedImage != scenario.expectedImage {
			test.Errorf("%s: expected %q, got %q", scenario.scenarioName, scenario.expectedImage, extractedImage)
		}
	}
}

func TestIsHostAllowed(test *testing.T) {
	scenarios := map[string]bool{
		"https://www.formula1.com/en/x":     true,
		"https://formula1.com/en/x":         true,
		"https://media.formula1.com/x":      true,
		"https://evil-formula1.com/x":       false,
		"https://formula1.com.evil.test/x":  false,
		"http://127.0.0.1:8080/admin":       false,
		"file:///etc/passwd":                false,
		"https://www.formula1.com.br/notes": false,
	}
	for candidateURL, expectedResult := range scenarios {
		if actualResult := IsHostAllowed(mustParseURL(test, candidateURL), "www.formula1.com"); actualResult != expectedResult {
			test.Errorf("%s: expected %v, got %v", candidateURL, expectedResult, actualResult)
		}
	}
}

func TestFindShareImageOnlyRequestsTheSourceSite(test *testing.T) {
	requestedPaths := []string{}
	articleServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		requestedPaths = append(requestedPaths, request.URL.Path)
		_, _ = responseWriter.Write([]byte(`<html><head><meta property="og:image" content="/cover.jpg"></head><body>` + strings.Repeat("text ", 100) + `</body></html>`))
	}))
	defer articleServer.Close()
	finder := NewFinder(articleServer.Client(), "SpeedNews/test")
	serverHost := mustParseURL(test, articleServer.URL).Hostname()

	shareImage, err := finder.FindShareImage(context.Background(), articleServer.URL+"/article", serverHost)
	if err != nil || shareImage != articleServer.URL+"/cover.jpg" {
		test.Errorf("expected the resolved cover, got %q (%v)", shareImage, err)
	}

	if _, err := finder.FindShareImage(context.Background(), articleServer.URL+"/article", "another-site.test"); err == nil {
		test.Error("an article outside the source site must be refused")
	}
	if len(requestedPaths) != 1 {
		test.Errorf("the refused article must not be requested at all, got %v", requestedPaths)
	}
}
