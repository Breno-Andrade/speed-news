// Package circuits fetches track drawings from github.com/bacinger/f1-circuits (MIT license)
// and converts them to SVG paths.
package circuits

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"speed-news/apps/api/internal/domain"
)

const (
	DefaultBaseURL         = "https://raw.githubusercontent.com/bacinger/f1-circuits/master"
	attributionText        = "f1-circuits (MIT)"
	attributionURL         = "https://github.com/bacinger/f1-circuits"
	maximumMatchDistanceKm = 5.0
)

// Client downloads the circuit index and individual layouts.
type Client struct {
	httpClient *http.Client
	baseURL    string
	userAgent  string
}

func NewClient(httpClient *http.Client, baseURL string, userAgent string) *Client {
	return &Client{httpClient: httpClient, baseURL: baseURL, userAgent: userAgent}
}

type locationEntry struct {
	LayoutID  string  `json:"id"`
	Name      string  `json:"name"`
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lon"`
}

type layoutFeature struct {
	Properties struct {
		LengthMeters       int `json:"length"`
		AltitudeMeters     int `json:"altitude"`
		FirstGrandPrixYear int `json:"firstgp"`
	} `json:"properties"`
	Geometry struct {
		GeometryType string       `json:"type"`
		Coordinates  [][2]float64 `json:"coordinates"`
	} `json:"geometry"`
}

type layoutFeatureCollection struct {
	Features []layoutFeature `json:"features"`
}

// fetchNearestLayoutFeature finds the dataset entry closest to the circuit and downloads it.
func (client *Client) fetchNearestLayoutFeature(ctx context.Context, raceEvent domain.RaceEvent) (string, layoutFeature, error) {
	var locationEntries []locationEntry
	if err := client.fetchJSON(ctx, client.baseURL+"/f1-locations.json", &locationEntries); err != nil {
		return "", layoutFeature{}, err
	}

	layoutLocations := make([]LayoutLocation, 0, len(locationEntries))
	for _, locationItem := range locationEntries {
		layoutLocations = append(layoutLocations, LayoutLocation{
			LayoutID:    locationItem.LayoutID,
			Name:        locationItem.Name,
			Coordinates: domain.GeoCoordinates{Latitude: locationItem.Latitude, Longitude: locationItem.Longitude},
		})
	}

	nearestLayout, isCloseEnough := FindNearestLayout(raceEvent.Coordinates, layoutLocations, maximumMatchDistanceKm)
	if !isCloseEnough {
		return "", layoutFeature{}, fmt.Errorf("no track drawing near circuit %s", raceEvent.CircuitID)
	}

	var featureCollection layoutFeatureCollection
	layoutURL := fmt.Sprintf("%s/circuits/%s.geojson", client.baseURL, nearestLayout.LayoutID)
	if err := client.fetchJSON(ctx, layoutURL, &featureCollection); err != nil {
		return "", layoutFeature{}, err
	}
	if len(featureCollection.Features) == 0 {
		return "", layoutFeature{}, fmt.Errorf("layout %s has no feature", nearestLayout.LayoutID)
	}
	return nearestLayout.LayoutID, featureCollection.Features[0], nil
}

// FetchCircuitFacts returns length, altitude and first Grand Prix year of the circuit.
func (client *Client) FetchCircuitFacts(ctx context.Context, raceEvent domain.RaceEvent) (domain.CircuitFacts, error) {
	_, nearestFeature, err := client.fetchNearestLayoutFeature(ctx, raceEvent)
	if err != nil {
		return domain.CircuitFacts{}, err
	}
	return domain.CircuitFacts{
		LengthMeters:       nearestFeature.Properties.LengthMeters,
		AltitudeMeters:     nearestFeature.Properties.AltitudeMeters,
		FirstGrandPrixYear: nearestFeature.Properties.FirstGrandPrixYear,
	}, nil
}

// FetchLayoutForCircuit finds the drawing closest to the circuit and returns it as SVG.
func (client *Client) FetchLayoutForCircuit(ctx context.Context, raceEvent domain.RaceEvent) (domain.CircuitLayout, error) {
	layoutID, nearestFeature, err := client.fetchNearestLayoutFeature(ctx, raceEvent)
	if err != nil {
		return domain.CircuitLayout{}, err
	}
	if nearestFeature.Geometry.GeometryType != "LineString" {
		return domain.CircuitLayout{}, fmt.Errorf("layout %s has no LineString geometry", layoutID)
	}

	svgPath, svgViewBox, err := ProjectToSVGPath(nearestFeature.Geometry.Coordinates)
	if err != nil {
		return domain.CircuitLayout{}, fmt.Errorf("project layout %s: %w", layoutID, err)
	}

	return domain.CircuitLayout{
		CircuitID:       raceEvent.CircuitID,
		LayoutID:        layoutID,
		SVGPath:         svgPath,
		SVGViewBox:      svgViewBox,
		AttributionText: attributionText,
		AttributionURL:  attributionURL,
	}, nil
}

func (client *Client) fetchJSON(ctx context.Context, resourceURL string, decodedTarget any) error {
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, resourceURL, nil)
	if err != nil {
		return err
	}
	httpRequest.Header.Set("User-Agent", client.userAgent)

	httpResponse, err := client.httpClient.Do(httpRequest)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", resourceURL, err)
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch %s: unexpected status %d", resourceURL, httpResponse.StatusCode)
	}
	return json.NewDecoder(httpResponse.Body).Decode(decodedTarget)
}
