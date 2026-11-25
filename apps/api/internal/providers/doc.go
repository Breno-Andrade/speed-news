// Package providers groups every client that talks to the internet. Nothing outside this tree
// opens outbound connections; each subpackage has a fake in internal/fixtures.
//
//   - rss: news feeds (gofeed)
//   - ogimage: og:image of article pages, for feeds without pictures
//   - jolpica: race calendar and results (Jolpica F1 API)
//   - f1circuits: circuit drawings (f1-circuits GeoJSON on GitHub)
//   - openmeteo: weather forecast
package providers
