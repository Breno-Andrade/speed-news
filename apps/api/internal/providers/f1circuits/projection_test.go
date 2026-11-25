package f1circuits

import (
	"strings"
	"testing"

	"speed-news/apps/api/internal/domain"
)

func TestProjectToSVGPathFitsCanvas(test *testing.T) {
	// Twice as wide as tall: the width fills the canvas and the height is centered vertically.
	wideTrack := [][2]float64{{103.84, 1.28}, {103.86, 1.28}, {103.86, 1.29}, {103.84, 1.29}}
	svgPath, svgViewBox, err := ProjectToSVGPath(wideTrack)
	if err != nil {
		test.Fatal(err)
	}
	if svgViewBox != "0 0 1000 1000" {
		test.Errorf("unexpected view box %q", svgViewBox)
	}
	if !strings.HasPrefix(svgPath, "M") || !strings.HasSuffix(svgPath, "Z") {
		test.Errorf("path must start with M and be closed: %q", svgPath)
	}
	// The first point is the south-west corner: left edge (after padding) and below the vertical center.
	// 730 = 40 padding + (920 - 460) / 2 centering + 460 height; the decimal varies with the latitude correction.
	if !strings.HasPrefix(svgPath, "M40.0 730.") {
		test.Errorf("expected first point at the bottom-left corner, got %q", svgPath[:20])
	}
}

func TestProjectToSVGPathRejectsTooFewPoints(test *testing.T) {
	if _, _, err := ProjectToSVGPath([][2]float64{{1, 1}}); err == nil {
		test.Error("expected error for a single point")
	}
}

func TestFindNearestLayoutMatchesMarinaBay(test *testing.T) {
	jolpicaMarinaBay := domain.GeoCoordinates{Latitude: 1.2914, Longitude: 103.864}
	layoutLocations := []LayoutLocation{
		{LayoutID: "bh-2002", Coordinates: domain.GeoCoordinates{Latitude: 26.032, Longitude: 50.511}},
		{LayoutID: "sg-2008", Coordinates: domain.GeoCoordinates{Latitude: 1.291, Longitude: 103.859}},
		{LayoutID: "br-1940", Coordinates: domain.GeoCoordinates{Latitude: -23.701, Longitude: -46.697}},
	}

	nearestLayout, isCloseEnough := FindNearestLayout(jolpicaMarinaBay, layoutLocations, 5)
	if !isCloseEnough || nearestLayout.LayoutID != "sg-2008" {
		test.Errorf("expected sg-2008 within 5 km, got %s (close enough: %v)", nearestLayout.LayoutID, isCloseEnough)
	}

	_, isCloseEnough = FindNearestLayout(domain.GeoCoordinates{Latitude: 40, Longitude: -3}, layoutLocations, 5)
	if isCloseEnough {
		test.Error("a circuit far from every layout must not match")
	}
}
