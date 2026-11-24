package rss

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"speed-news/apps/api/internal/domain"
)

// Shape of the Autoracing feed (2026-11): the picture sits outside every RSS field.
const malformedImageFeed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/">
<channel><title>Autoracing</title><link>https://www.autoracing.com.br</link><description>x</description>
<item>
  <title>Wolff cobra reação da Mercedes</title>
  <link>https://www.autoracing.com.br/wolff-cobra/</link>
  <description><![CDATA[<p>Resumo sem imagem</p>]]></description>
  <a href="https://www.autoracing.com.br/wolff-cobra/"><img class="size-full" src="https://www.autoracing.com.br/uploads/wolff.jpg" alt="" /></a>
  <content:encoded><![CDATA[Texto sem imagem]]></content:encoded>
</item>
<item>
  <title>Item com imagem oficial</title>
  <link>https://www.autoracing.com.br/oficial/</link>
  <description>sem imagem no texto</description>
  <enclosure url="https://www.autoracing.com.br/uploads/oficial.jpg" type="image/jpeg" length="0"/>
  <img src="https://www.autoracing.com.br/uploads/nao-usar.jpg" />
</item>
<item>
  <title>Item sem imagem nenhuma</title>
  <link>https://www.autoracing.com.br/sem-imagem/</link>
  <description>Só texto, nenhuma imagem em lugar nenhum.</description>
</item>
</channel></rss>`

func TestFetchFeedItemsRecoversImagesOutsideRSSFields(test *testing.T) {
	feedServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.Header.Get("User-Agent") != UserAgent {
			test.Errorf("expected the generic app user agent, got %q", request.Header.Get("User-Agent"))
		}
		_, _ = responseWriter.Write([]byte(malformedImageFeed))
	}))
	defer feedServer.Close()

	feedItems, err := NewFetcher(feedServer.Client()).FetchFeedItems(context.Background(), domain.NewsSource{Slug: "autoracing", FeedURL: feedServer.URL})
	if err != nil {
		test.Fatal(err)
	}
	if len(feedItems) != 3 {
		test.Fatalf("expected 3 items, got %d", len(feedItems))
	}
	if feedItems[0].Image == nil || feedItems[0].Image.URL != "https://www.autoracing.com.br/uploads/wolff.jpg" {
		test.Errorf("the injected <img> must be recovered, got %+v", feedItems[0].Image)
	}
	// An item with an official image keeps it; the raw fallback is only for items without one.
	if feedItems[1].Image != nil && feedItems[1].Image.URL == "https://www.autoracing.com.br/uploads/nao-usar.jpg" {
		test.Error("the raw fallback must not override an item that already has an image")
	}
	if feedItems[2].Image != nil {
		test.Errorf("an item without any picture must stay without image; got %+v", feedItems[2].Image)
	}
}

func TestFetchFeedItemsReportsHTTPErrors(test *testing.T) {
	feedServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		responseWriter.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer feedServer.Close()

	if _, err := NewFetcher(feedServer.Client()).FetchFeedItems(context.Background(), domain.NewsSource{Slug: "down", FeedURL: feedServer.URL}); err == nil {
		test.Error("a 503 must be reported as an error")
	}
}
