// Normalization: turns raw feed items into clean domain articles.
package news

import (
	"crypto/sha256"
	"encoding/hex"
	"html"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mmcdole/gofeed"

	"speed-news/apps/api/internal/domain"
)

const maximumSummaryLength = 400

var (
	htmlTagPattern     = regexp.MustCompile(`<[^>]*>`)
	whitespacePattern  = regexp.MustCompile(`\s+`)
	imageSourcePattern = regexp.MustCompile(`<img[^>]+src="([^"]+)"`)
	trackingParameters = []string{"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content"}
)

// NormalizeFeedItems converts feed items into articles, skipping items without title or link.
func NormalizeFeedItems(feedItems []*gofeed.Item, newsSource domain.NewsSource, fetchedAt time.Time) []domain.Article {
	normalizedArticles := make([]domain.Article, 0, len(feedItems))
	for _, feedItem := range feedItems {
		normalizedArticle, isValid := normalizeFeedItem(feedItem, newsSource, fetchedAt)
		if isValid {
			normalizedArticles = append(normalizedArticles, normalizedArticle)
		}
	}
	return normalizedArticles
}

func normalizeFeedItem(feedItem *gofeed.Item, newsSource domain.NewsSource, fetchedAt time.Time) (domain.Article, bool) {
	articleTitle := cleanText(feedItem.Title)
	canonicalLink := CanonicalizeURL(feedItem.Link)
	if articleTitle == "" || canonicalLink == "" {
		return domain.Article{}, false
	}

	publishedAt := fetchedAt
	if feedItem.PublishedParsed != nil {
		publishedAt = feedItem.PublishedParsed.UTC()
	} else if feedItem.UpdatedParsed != nil {
		publishedAt = feedItem.UpdatedParsed.UTC()
	}
	// Some feeds publish future dates by mistake; never show an article "from the future".
	if publishedAt.After(fetchedAt) {
		publishedAt = fetchedAt
	}

	return domain.Article{
		ID:           BuildArticleID(canonicalLink),
		SeriesSlug:   newsSource.SeriesSlug,
		SourceSlug:   newsSource.Slug,
		SourceName:   newsSource.DisplayName,
		LanguageCode: newsSource.LanguageCode,
		Title:        articleTitle,
		Summary:      truncateText(cleanText(feedItem.Description), maximumSummaryLength),
		ArticleURL:   canonicalLink,
		ImageURL:     extractImageURL(feedItem),
		PublishedAt:  publishedAt,
		FetchedAt:    fetchedAt,
	}, true
}

// BuildArticleID is a stable id derived from the canonical link, used for deduplication.
func BuildArticleID(canonicalLink string) string {
	linkHash := sha256.Sum256([]byte(canonicalLink))
	return hex.EncodeToString(linkHash[:])[:24]
}

// CanonicalizeURL removes tracking parameters and fragments so the same article has a single id.
func CanonicalizeURL(rawLink string) string {
	parsedLink, err := url.Parse(strings.TrimSpace(rawLink))
	if err != nil || parsedLink.Host == "" {
		return ""
	}
	parsedLink.Fragment = ""
	parsedLink.Scheme = "https"
	parsedLink.Host = strings.ToLower(parsedLink.Host)

	queryValues := parsedLink.Query()
	for _, trackingParameter := range trackingParameters {
		queryValues.Del(trackingParameter)
	}
	parsedLink.RawQuery = queryValues.Encode()
	return parsedLink.String()
}

func cleanText(rawText string) string {
	withoutTags := htmlTagPattern.ReplaceAllString(rawText, " ")
	unescapedText := html.UnescapeString(withoutTags)
	return strings.TrimSpace(whitespacePattern.ReplaceAllString(unescapedText, " "))
}

func truncateText(fullText string, maximumLength int) string {
	if utf8.RuneCountInString(fullText) <= maximumLength {
		return fullText
	}
	textRunes := []rune(fullText)
	truncatedText := string(textRunes[:maximumLength])
	if lastSpaceIndex := strings.LastIndex(truncatedText, " "); lastSpaceIndex > maximumLength/2 {
		truncatedText = truncatedText[:lastSpaceIndex]
	}
	return truncatedText + "…"
}

func extractImageURL(feedItem *gofeed.Item) string {
	if feedItem.Image != nil && feedItem.Image.URL != "" {
		return feedItem.Image.URL
	}
	for _, enclosure := range feedItem.Enclosures {
		if strings.HasPrefix(enclosure.Type, "image/") && enclosure.URL != "" {
			return enclosure.URL
		}
	}
	if mediaExtensions, hasMedia := feedItem.Extensions["media"]; hasMedia {
		for _, extensionName := range []string{"content", "thumbnail"} {
			for _, mediaElement := range mediaExtensions[extensionName] {
				if mediaURL := mediaElement.Attrs["url"]; mediaURL != "" {
					return mediaURL
				}
			}
		}
	}
	for _, htmlContent := range []string{feedItem.Content, feedItem.Description} {
		if imageMatch := imageSourcePattern.FindStringSubmatch(htmlContent); len(imageMatch) == 2 {
			return html.UnescapeString(imageMatch[1])
		}
	}
	return ""
}
