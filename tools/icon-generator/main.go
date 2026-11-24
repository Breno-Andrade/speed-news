// Command icon-generator draws the app icon and the two tray icon states.
//
// macOS shows the unread count as menu bar text (TrayIcon::set_title). Linux (ksni backend) and
// Windows cannot show text next to the tray icon, so there the icon switches to the
// "unread" variant (red dot) and the count is shown in the popover header.
//
// Usage: go run ./tools/icon-generator --output-dir apps/desktop/src-tauri/icons
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

var (
	backgroundColor = color.RGBA{R: 0x15, G: 0x15, B: 0x1e, A: 0xff}
	checkerColor    = color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	unreadDotColor  = color.RGBA{R: 0xe1, G: 0x06, B: 0x00, A: 0xff}
)

const (
	trayIconSize = 64
	appIconSize  = 1024
)

func main() {
	outputDirectory := flag.String("output-dir", "apps/desktop/src-tauri/icons", "where to write the PNG files")
	flag.Parse()

	if err := generateIcons(*outputDirectory); err != nil {
		fmt.Fprintln(os.Stderr, "icon-generator:", err)
		os.Exit(1)
	}
}

func generateIcons(outputDirectory string) error {
	trayDirectory := filepath.Join(outputDirectory, "tray")
	if err := os.MkdirAll(trayDirectory, 0o755); err != nil {
		return err
	}

	if err := writePNG(filepath.Join(outputDirectory, "app-icon.png"), drawBaseIcon(appIconSize)); err != nil {
		return err
	}
	if err := writePNG(filepath.Join(trayDirectory, "tray-default.png"), drawBaseIcon(trayIconSize)); err != nil {
		return err
	}
	unreadTrayIcon := drawBaseIcon(trayIconSize)
	drawUnreadDot(unreadTrayIcon)
	if err := writePNG(filepath.Join(trayDirectory, "tray-unread.png"), unreadTrayIcon); err != nil {
		return err
	}

	fmt.Println("icons written to", outputDirectory)
	return nil
}

// drawBaseIcon draws a rounded square holding a checkered flag.
func drawBaseIcon(iconSize int) *image.RGBA {
	iconCanvas := image.NewRGBA(image.Rect(0, 0, iconSize, iconSize))
	cornerRadius := float64(iconSize) * 0.22
	checkerMargin := float64(iconSize) * 0.2
	checkerCells := 4
	checkerSpan := float64(iconSize) - 2*checkerMargin
	checkerCellSize := checkerSpan / float64(checkerCells)

	for pixelRow := 0; pixelRow < iconSize; pixelRow++ {
		for pixelColumn := 0; pixelColumn < iconSize; pixelColumn++ {
			pixelCenterHorizontal := float64(pixelColumn) + 0.5
			pixelCenterVertical := float64(pixelRow) + 0.5
			coverage := roundedSquareCoverage(pixelCenterHorizontal, pixelCenterVertical, float64(iconSize), cornerRadius)
			if coverage <= 0 {
				continue
			}
			pixelColor := backgroundColor
			insideCheckerHorizontal := pixelCenterHorizontal - checkerMargin
			insideCheckerVertical := pixelCenterVertical - checkerMargin
			if insideCheckerHorizontal >= 0 && insideCheckerVertical >= 0 && insideCheckerHorizontal < checkerSpan && insideCheckerVertical < checkerSpan {
				cellColumn := int(insideCheckerHorizontal / checkerCellSize)
				cellRow := int(insideCheckerVertical / checkerCellSize)
				if (cellColumn+cellRow)%2 == 0 {
					pixelColor = checkerColor
				}
			}
			iconCanvas.SetRGBA(pixelColumn, pixelRow, withAlpha(pixelColor, coverage))
		}
	}
	return iconCanvas
}

// drawUnreadDot paints a red dot at the top-right corner, separated from the square by a
// transparent ring so it stays readable on light and dark panels.
func drawUnreadDot(iconCanvas *image.RGBA) {
	iconSize := float64(iconCanvas.Bounds().Dx())
	dotRadius := iconSize * 0.2
	ringRadius := dotRadius + iconSize*0.06
	dotCenterHorizontal := iconSize - ringRadius
	dotCenterVertical := ringRadius

	for pixelRow := 0; pixelRow < int(iconSize); pixelRow++ {
		for pixelColumn := 0; pixelColumn < int(iconSize); pixelColumn++ {
			distanceToCenter := math.Hypot(float64(pixelColumn)+0.5-dotCenterHorizontal, float64(pixelRow)+0.5-dotCenterVertical)
			ringCoverage := clampUnit(ringRadius - distanceToCenter + 0.5)
			if ringCoverage <= 0 {
				continue
			}
			// Carve the ring: fade whatever is underneath.
			existingColor := iconCanvas.RGBAAt(pixelColumn, pixelRow)
			remainingFactor := 1 - ringCoverage
			carvedColor := color.RGBA{
				R: uint8(float64(existingColor.R) * remainingFactor),
				G: uint8(float64(existingColor.G) * remainingFactor),
				B: uint8(float64(existingColor.B) * remainingFactor),
				A: uint8(float64(existingColor.A) * remainingFactor),
			}
			// Paint the dot on top (premultiplied "over" composition).
			dotCoverage := clampUnit(dotRadius - distanceToCenter + 0.5)
			paintedDot := withAlpha(unreadDotColor, dotCoverage)
			backdropFactor := 1 - dotCoverage
			iconCanvas.SetRGBA(pixelColumn, pixelRow, color.RGBA{
				R: paintedDot.R + uint8(float64(carvedColor.R)*backdropFactor),
				G: paintedDot.G + uint8(float64(carvedColor.G)*backdropFactor),
				B: paintedDot.B + uint8(float64(carvedColor.B)*backdropFactor),
				A: paintedDot.A + uint8(float64(carvedColor.A)*backdropFactor),
			})
		}
	}
}

// roundedSquareCoverage returns 0..1 for anti-aliased edges.
func roundedSquareCoverage(pointHorizontal, pointVertical, squareSize, cornerRadius float64) float64 {
	nearestHorizontal := math.Max(cornerRadius, math.Min(squareSize-cornerRadius, pointHorizontal))
	nearestVertical := math.Max(cornerRadius, math.Min(squareSize-cornerRadius, pointVertical))
	distanceToCorner := math.Hypot(pointHorizontal-nearestHorizontal, pointVertical-nearestVertical)
	return clampUnit(cornerRadius - distanceToCorner + 0.5)
}

func clampUnit(unboundedValue float64) float64 {
	return math.Max(0, math.Min(1, unboundedValue))
}

// withAlpha returns a premultiplied color with the given coverage.
func withAlpha(baseColor color.RGBA, coverage float64) color.RGBA {
	scale := func(channel uint8) uint8 { return uint8(math.Round(float64(channel) * coverage)) }
	return color.RGBA{R: scale(baseColor.R), G: scale(baseColor.G), B: scale(baseColor.B), A: uint8(math.Round(255 * coverage))}
}

func writePNG(filePath string, iconImage image.Image) error {
	outputFile, err := os.Create(filePath)
	if err != nil {
		return err
	}
	if err := png.Encode(outputFile, iconImage); err != nil {
		_ = outputFile.Close()
		return err
	}
	// Close flushes the file: its error means the PNG on disk may be truncated.
	return outputFile.Close()
}
