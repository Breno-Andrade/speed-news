// Package ogimage finds the picture a site publishes for link previews (og:image /
// twitter:image) on an article page. Used only for sources whose feed carries no image
// (imageFromArticlePage in the sources YAML). It reads the page <head>, never the article text.
package ogimage

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Metadata lives in <head>; pages are big, so stop reading early.
const maximumHeadBytes = 256 << 10

var shareImagePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)<meta[^>]+(?:property|name)\s*=\s*["']og:image(?::url|:secure_url)?["'][^>]*content\s*=\s*["']([^"']+)["']`),
	regexp.MustCompile(`(?i)<meta[^>]+content\s*=\s*["']([^"']+)["'][^>]*(?:property|name)\s*=\s*["']og:image(?::url|:secure_url)?["']`),
	regexp.MustCompile(`(?i)<meta[^>]+(?:property|name)\s*=\s*["']twitter:image(?::src)?["'][^>]*content\s*=\s*["']([^"']+)["']`),
	regexp.MustCompile(`(?i)<meta[^>]+content\s*=\s*["']([^"']+)["'][^>]*(?:property|name)\s*=\s*["']twitter:image(?::src)?["']`),
}

type Finder struct {
	httpClient *http.Client
	userAgent  string
}

func NewFinder(httpClient *http.Client, userAgent string) *Finder {
	return &Finder{httpClient: httpClient, userAgent: userAgent}
}

// FindShareImage returns "" (no error) when the page has no share image.
// articleURL must belong to the source's own site (allowedHost or one of its subdomains):
// a feed must never make the app request arbitrary hosts, local network included.
func (finder *Finder) FindShareImage(ctx context.Context, articleURL string, allowedHost string) (string, error) {
	parsedArticleURL, err := url.Parse(articleURL)
	if err != nil || !IsHostAllowed(parsedArticleURL, allowedHost) {
		return "", fmt.Errorf("article %q is outside the source site %q", articleURL, allowedHost)
	}

	requestContext, cancelRequest := context.WithTimeout(ctx, 15*time.Second)
	defer cancelRequest()
	httpRequest, err := http.NewRequestWithContext(requestContext, http.MethodGet, articleURL, nil)
	if err != nil {
		return "", err
	}
	httpRequest.Header.Set("User-Agent", finder.userAgent)
	httpRequest.Header.Set("Accept", "text/html")

	httpResponse, err := finder.httpClient.Do(httpRequest)
	if err != nil {
		return "", fmt.Errorf("fetch article page: %w", err)
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch article page: unexpected status %d", httpResponse.StatusCode)
	}
	pageHead, err := io.ReadAll(io.LimitReader(httpResponse.Body, maximumHeadBytes))
	if err != nil {
		return "", err
	}
	return ExtractShareImage(string(pageHead), parsedArticleURL), nil
}

// ExtractShareImage reads og:image (preferred) or twitter:image, resolving relative URLs.
func ExtractShareImage(pageHTML string, pageURL *url.URL) string {
	if headEnd := strings.Index(strings.ToLower(pageHTML), "</head>"); headEnd >= 0 {
		pageHTML = pageHTML[:headEnd]
	}
	for _, shareImagePattern := range shareImagePatterns {
		imageMatch := shareImagePattern.FindStringSubmatch(pageHTML)
		if imageMatch == nil {
			continue
		}
		imageURL, err := pageURL.Parse(strings.TrimSpace(html.UnescapeString(imageMatch[1])))
		if err == nil && (imageURL.Scheme == "https" || imageURL.Scheme == "http") {
			return imageURL.String()
		}
	}
	return ""
}

// IsHostAllowed accepts allowedHost itself and its subdomains, over http(s) only.
func IsHostAllowed(candidateURL *url.URL, allowedHost string) bool {
	if candidateURL.Scheme != "https" && candidateURL.Scheme != "http" {
		return false
	}
	candidateHost := strings.ToLower(candidateURL.Hostname())
	allowedHost = strings.ToLower(strings.TrimPrefix(allowedHost, "www."))
	candidateHost = strings.TrimPrefix(candidateHost, "www.")
	return allowedHost != "" && (candidateHost == allowedHost || strings.HasSuffix(candidateHost, "."+allowedHost))
}
