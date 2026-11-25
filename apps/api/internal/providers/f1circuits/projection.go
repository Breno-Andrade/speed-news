package f1circuits

import (
	"fmt"
	"math"
	"strings"

	"speed-news/apps/api/internal/domain"
)

const (
	svgCanvasSize    = 1000.0
	svgCanvasPadding = 40.0
	earthRadiusKm    = 6371.0
)

// ProjectToSVGPath turns [longitude, latitude] points into an SVG path that fits a square canvas.
// It uses an equirectangular projection corrected by the cosine of the latitude, which is
// precise enough for an area the size of a race track.
func ProjectToSVGPath(trackPoints [][2]float64) (svgPath string, svgViewBox string, err error) {
	if len(trackPoints) < 2 {
		return "", "", fmt.Errorf("track needs at least two points, got %d", len(trackPoints))
	}

	averageLatitude := 0.0
	for _, trackPoint := range trackPoints {
		averageLatitude += trackPoint[1]
	}
	averageLatitude /= float64(len(trackPoints))
	longitudeScale := math.Cos(averageLatitude * math.Pi / 180)

	projectedPoints := make([][2]float64, len(trackPoints))
	minimumHorizontal, minimumVertical := math.Inf(1), math.Inf(1)
	maximumHorizontal, maximumVertical := math.Inf(-1), math.Inf(-1)
	for pointIndex, trackPoint := range trackPoints {
		horizontalPosition := trackPoint[0] * longitudeScale
		verticalPosition := -trackPoint[1] // SVG grows downwards, latitude grows upwards
		projectedPoints[pointIndex] = [2]float64{horizontalPosition, verticalPosition}
		minimumHorizontal = math.Min(minimumHorizontal, horizontalPosition)
		maximumHorizontal = math.Max(maximumHorizontal, horizontalPosition)
		minimumVertical = math.Min(minimumVertical, verticalPosition)
		maximumVertical = math.Max(maximumVertical, verticalPosition)
	}

	trackWidth := maximumHorizontal - minimumHorizontal
	trackHeight := maximumVertical - minimumVertical
	largestDimension := math.Max(trackWidth, trackHeight)
	if largestDimension == 0 {
		return "", "", fmt.Errorf("track has no extent")
	}
	drawableSize := svgCanvasSize - 2*svgCanvasPadding
	scaleFactor := drawableSize / largestDimension
	// Center the smaller dimension inside the square canvas.
	horizontalOffset := svgCanvasPadding + (drawableSize-trackWidth*scaleFactor)/2
	verticalOffset := svgCanvasPadding + (drawableSize-trackHeight*scaleFactor)/2

	var pathBuilder strings.Builder
	for pointIndex, projectedPoint := range projectedPoints {
		canvasHorizontal := (projectedPoint[0]-minimumHorizontal)*scaleFactor + horizontalOffset
		canvasVertical := (projectedPoint[1]-minimumVertical)*scaleFactor + verticalOffset
		pathCommand := "L"
		if pointIndex == 0 {
			pathCommand = "M"
		}
		fmt.Fprintf(&pathBuilder, "%s%.1f %.1f ", pathCommand, canvasHorizontal, canvasVertical)
	}
	pathBuilder.WriteString("Z")

	return pathBuilder.String(), fmt.Sprintf("0 0 %.0f %.0f", svgCanvasSize, svgCanvasSize), nil
}

// HaversineDistanceKm is the great-circle distance between two coordinates.
func HaversineDistanceKm(origin domain.GeoCoordinates, destination domain.GeoCoordinates) float64 {
	toRadians := func(degrees float64) float64 { return degrees * math.Pi / 180 }
	latitudeDelta := toRadians(destination.Latitude - origin.Latitude)
	longitudeDelta := toRadians(destination.Longitude - origin.Longitude)
	haversineTerm := math.Sin(latitudeDelta/2)*math.Sin(latitudeDelta/2) +
		math.Cos(toRadians(origin.Latitude))*math.Cos(toRadians(destination.Latitude))*
			math.Sin(longitudeDelta/2)*math.Sin(longitudeDelta/2)
	return 2 * earthRadiusKm * math.Atan2(math.Sqrt(haversineTerm), math.Sqrt(1-haversineTerm))
}

// LayoutLocation is one entry of the f1-circuits location index.
type LayoutLocation struct {
	LayoutID    string
	Name        string
	Coordinates domain.GeoCoordinates
}

// FindNearestLayout links a calendar circuit to a track drawing by geographic proximity.
// It refuses matches farther than maximumDistanceKm to avoid showing the wrong track.
func FindNearestLayout(circuitCoordinates domain.GeoCoordinates, layoutLocations []LayoutLocation, maximumDistanceKm float64) (LayoutLocation, bool) {
	var nearestLayout LayoutLocation
	shortestDistanceKm := math.Inf(1)
	for _, candidateLayout := range layoutLocations {
		distanceKm := HaversineDistanceKm(circuitCoordinates, candidateLayout.Coordinates)
		if distanceKm < shortestDistanceKm {
			shortestDistanceKm = distanceKm
			nearestLayout = candidateLayout
		}
	}
	return nearestLayout, shortestDistanceKm <= maximumDistanceKm
}
