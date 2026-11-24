// Package rss downloads and parses RSS/Atom feeds.
package rss

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"

	"speed-news/apps/api/internal/domain"
)

// UserAgent is generic on purpose: it identifies the app, never the user.
const UserAgent = "SpeedNews/0.1 (+desktop news reader)"

const maximumFeedBytes = 10 << 20

var (
	rawItemPattern  = regexp.MustCompile(`(?s)<item[\s>].*?</item>`)
	rawLinkPattern  = regexp.MustCompile(`(?s)<link>\s*(?:<!\[CDATA\[)?\s*(.*?)\s*(?:\]\]>)?\s*</link>`)
	rawImagePattern = regexp.MustCompile(`<img[^>]+src=["']([^"']+)["']`)
)

// Fetcher reads a feed and returns its raw items.
type Fetcher struct {
	httpClient *http.Client
	feedParser *gofeed.Parser
}

func NewFetcher(httpClient *http.Client) *Fetcher {
	return &Fetcher{httpClient: httpClient, feedParser: gofeed.NewParser()}
}

// FetchFeedItems downloads the feed of a source.
func (fetcher *Fetcher) FetchFeedItems(ctx context.Context, newsSource domain.NewsSource) ([]*gofeed.Item, error) {
	requestContext, cancelRequest := context.WithTimeout(ctx, 20*time.Second)
	defer cancelRequest()

	httpRequest, err := http.NewRequestWithContext(requestContext, http.MethodGet, newsSource.FeedURL, nil)
	if err != nil {
		return nil, err
	}
	httpRequest.Header.Set("User-Agent", UserAgent)
	httpResponse, err := fetcher.httpClient.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("fetch feed %s: %w", newsSource.Slug, err)
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch feed %s: unexpected status %d", newsSource.Slug, httpResponse.StatusCode)
	}
	feedBody, err := io.ReadAll(io.LimitReader(httpResponse.Body, maximumFeedBytes))
	if err != nil {
		return nil, fmt.Errorf("read feed %s: %w", newsSource.Slug, err)
	}

	parsedFeed, err := fetcher.feedParser.ParseString(string(feedBody))
	if err != nil {
		return nil, fmt.Errorf("parse feed %s: %w", newsSource.Slug, err)
	}
	fillImagesFromRawItems(parsedFeed.Items, feedBody)
	return parsedFeed.Items, nil
}

// fillImagesFromRawItems is a fallback for feeds that put the picture outside any RSS field.
// Example (Autoracing, 2026-11): a WordPress plugin injects `<a><img src=...></a>` straight inside
// <item>, between <description> and <content:encoded>; the parser drops it. Items that already
// have an image through a proper field are left untouched.
func fillImagesFromRawItems(feedItems []*gofeed.Item, feedBody []byte) {
	imageURLByLink := ExtractRawItemImages(feedBody)
	if len(imageURLByLink) == 0 {
		return
	}
	for _, feedItem := range feedItems {
		if feedItem.Image != nil && feedItem.Image.URL != "" {
			continue
		}
		if rawImageURL, hasImage := imageURLByLink[strings.TrimSpace(feedItem.Link)]; hasImage {
			feedItem.Image = &gofeed.Image{URL: rawImageURL}
		}
	}
}

// ExtractRawItemImages maps each item's link to the first <img> found anywhere inside the raw
// <item> markup (HTML-escaped content is ignored: only real tags count).
func ExtractRawItemImages(feedBody []byte) map[string]string {
	imageURLByLink := map[string]string{}
	for _, rawItem := range rawItemPattern.FindAll(feedBody, -1) {
		linkMatch := rawLinkPattern.FindSubmatch(rawItem)
		imageMatch := rawImagePattern.FindSubmatch(rawItem)
		if linkMatch == nil || imageMatch == nil {
			continue
		}
		imageURL := html.UnescapeString(string(imageMatch[1]))
		if strings.HasPrefix(imageURL, "https://") || strings.HasPrefix(imageURL, "http://") {
			imageURLByLink[strings.TrimSpace(html.UnescapeString(string(linkMatch[1])))] = imageURL
		}
	}
	return imageURLByLink
}
