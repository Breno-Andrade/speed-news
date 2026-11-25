// Command icon-generator draws every Speed News icon from one source, so the platforms cannot
// drift apart again (Windows used to show a different tray icon than the GNOME panel):
//
//   - the app icon (rounded square with a checkered flag), fed to `tauri icon` for the bundles;
//   - the tray symbol, a checkered flag on a pole drawn on a 16-unit pixel grid, in a light
//     variant (dark panels and taskbars) and a dark one (light taskbars, macOS template image),
//     each with an "unread" twin carrying a red dot;
//   - the GNOME Shell panel icon (SVG) with the very same symbol.
//
// macOS shows the unread count as menu bar text (TrayIcon::set_title). Linux (ksni backend) and
// Windows cannot show text next to the tray icon, so there the icon switches to the "unread"
// variant and the count is shown in the popover header.
//
// Usage (from apps/desktop): go run ../../tools/icon-generator --output-dir src-tauri/icons \
//
//	--gnome-icon gnome-shell-extension/icons/speed-news-flag.svg
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
	"strings"
)

var (
	appBackgroundColor = color.RGBA{R: 0x15, G: 0x15, B: 0x1e, A: 0xff}
	appCheckerColor    = color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	unreadDotColor     = color.RGBA{R: 0xe1, G: 0x06, B: 0x00, A: 0xff}
	// Symbol colors: white for dark panels (GNOME, KDE, default Windows taskbar), near black for
	// light Windows taskbars. macOS only uses the alpha channel (template image) of the dark one.
	lightSymbolColor = color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	darkSymbolColor  = color.RGBA{R: 0x1c, G: 0x1c, B: 0x22, A: 0xff}
)

const (
	appIconSize = 1024
	// The tray symbol is drawn on a 16-unit grid; 32 px = 2 px per unit, so every edge falls on
	// a pixel boundary at 32 px and at the 16 px Windows downscale (100 % scaling).
	symbolGridUnits = 16
	trayIconSize    = 32
)

// gridRectangle is a rectangle in symbol grid units.
type gridRectangle struct {
	left, top, width, height float64
	cornerRadius             float64
}

// flagSymbol is the checkered flag on a pole. The GNOME SVG and the tray PNGs are both drawn
// from this list.
var flagSymbol = []gridRectangle{
	{left: 1, top: 1, width: 2, height: 14, cornerRadius: 1}, // pole
	{left: 3, top: 2, width: 3, height: 3},
	{left: 9, top: 2, width: 3, height: 3},
	{left: 6, top: 5, width: 3, height: 3},
	{left: 12, top: 5, width: 3, height: 3},
	{left: 3, top: 8, width: 3, height: 3},
	{left: 9, top: 8, width: 3, height: 3},
}

// Unread dot in grid units: top-right corner, separated from the flag by a transparent ring so it
// stays readable on any panel color.
const (
	unreadDotCenterX = 12.5
	unreadDotCenterY = 3.5
	unreadDotRadius  = 3.5
	unreadRingWidth  = 1
)

// generatedFile is one output file with its content.
type generatedFile struct {
	relativePath string
	pngImage     image.Image
}

func main() {
	outputDirectory := flag.String("output-dir", "apps/desktop/src-tauri/icons", "where to write the PNG files")
	gnomeIconPath := flag.String("gnome-icon", "apps/desktop/gnome-shell-extension/icons/speed-news-flag.svg",
		"where to write the GNOME panel SVG")
	flag.Parse()

	if err := writeGeneratedFiles(*outputDirectory, *gnomeIconPath); err != nil {
		fmt.Fprintln(os.Stderr, "icon-generator:", err)
		os.Exit(1)
	}
	fmt.Println("icons written to", *outputDirectory, "and", *gnomeIconPath)
}

// trayIconFiles lists the tray PNGs, relative to the icons directory. Embedded by
// apps/desktop/src-tauri/src/tray.rs: keep the names in sync.
func trayIconFiles() []generatedFile {
	return []generatedFile{
		{relativePath: "tray/tray-light.png", pngImage: drawTrayIcon(lightSymbolColor, false)},
		{relativePath: "tray/tray-light-unread.png", pngImage: drawTrayIcon(lightSymbolColor, true)},
		{relativePath: "tray/tray-dark.png", pngImage: drawTrayIcon(darkSymbolColor, false)},
		{relativePath: "tray/tray-dark-unread.png", pngImage: drawTrayIcon(darkSymbolColor, true)},
	}
}

func writeGeneratedFiles(outputDirectory string, gnomeIconPath string) error {
	pngFiles := append([]generatedFile{{relativePath: "app-icon.png", pngImage: drawAppIcon(appIconSize)}}, trayIconFiles()...)
	for _, pngFile := range pngFiles {
		if err := writePNG(filepath.Join(outputDirectory, pngFile.relativePath), pngFile.pngImage); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(gnomeIconPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(gnomeIconPath, []byte(gnomeSymbolSVG()), 0o644)
}

// drawTrayIcon draws the flag symbol, with the unread dot when hasUnread.
func drawTrayIcon(symbolColor color.RGBA, hasUnread bool) *image.RGBA {
	iconCanvas := image.NewRGBA(image.Rect(0, 0, trayIconSize, trayIconSize))
	pixelsPerUnit := float64(trayIconSize) / symbolGridUnits
	for pixelRow := range trayIconSize {
		for pixelColumn := range trayIconSize {
			coverage := supersampledCoverage(pixelColumn, pixelRow, func(sampleX, sampleY float64) bool {
				gridX, gridY := sampleX/pixelsPerUnit, sampleY/pixelsPerUnit
				if hasUnread && math.Hypot(gridX-unreadDotCenterX, gridY-unreadDotCenterY) < unreadDotRadius+unreadRingWidth {
					return false // carved ring and dot area: the flag does not show through
				}
				return symbolContains(gridX, gridY)
			})
			pixelColor := withAlpha(symbolColor, coverage)
			if hasUnread {
				dotCoverage := supersampledCoverage(pixelColumn, pixelRow, func(sampleX, sampleY float64) bool {
					return math.Hypot(sampleX/pixelsPerUnit-unreadDotCenterX, sampleY/pixelsPerUnit-unreadDotCenterY) < unreadDotRadius
				})
				if dotCoverage > 0 {
					pixelColor = withAlpha(unreadDotColor, dotCoverage)
				}
			}
			iconCanvas.SetRGBA(pixelColumn, pixelRow, pixelColor)
		}
	}
	return iconCanvas
}

// symbolContains reports whether a point (grid units) is inside the flag symbol.
func symbolContains(gridX, gridY float64) bool {
	for _, symbolPart := range flagSymbol {
		if roundedRectangleContains(symbolPart, gridX, gridY) {
			return true
		}
	}
	return false
}

func roundedRectangleContains(rectangle gridRectangle, pointX, pointY float64) bool {
	if pointX < rectangle.left || pointY < rectangle.top || pointX >= rectangle.left+rectangle.width || pointY >= rectangle.top+rectangle.height {
		return false
	}
	if rectangle.cornerRadius == 0 {
		return true
	}
	nearestX := math.Max(rectangle.left+rectangle.cornerRadius, math.Min(rectangle.left+rectangle.width-rectangle.cornerRadius, pointX))
	nearestY := math.Max(rectangle.top+rectangle.cornerRadius, math.Min(rectangle.top+rectangle.height-rectangle.cornerRadius, pointY))
	return math.Hypot(pointX-nearestX, pointY-nearestY) <= rectangle.cornerRadius
}

// supersampledCoverage returns the share (0..1) of a 4x4 sample grid inside the shape.
func supersampledCoverage(pixelColumn, pixelRow int, isInside func(sampleX, sampleY float64) bool) float64 {
	const samplesPerAxis = 4
	insideCount := 0
	for sampleRow := range samplesPerAxis {
		for sampleColumn := range samplesPerAxis {
			sampleX := float64(pixelColumn) + (float64(sampleColumn)+0.5)/samplesPerAxis
			sampleY := float64(pixelRow) + (float64(sampleRow)+0.5)/samplesPerAxis
			if isInside(sampleX, sampleY) {
				insideCount++
			}
		}
	}
	return float64(insideCount) / (samplesPerAxis * samplesPerAxis)
}

// gnomeSymbolSVG is the panel icon of the GNOME Shell extension.
func gnomeSymbolSVG() string {
	var svgBuilder strings.Builder
	svgBuilder.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 16 16">
  <!-- Generated by tools/icon-generator (same symbol as the tray icons): do not edit by hand.
       Checkered flag on a pole, drawn for 16px: every checker cell is exactly 3x3 px on the pixel
       grid so it stays crisp in the panel. Plain (non-symbolic) SVG in the panel text color: GNOME 46 fails to
       recolor "-symbolic" SVGs loaded from a file and draws a blank square instead. -->
  <g fill="#ffffff">
`)
	for _, symbolPart := range flagSymbol {
		roundedCorner := ""
		if symbolPart.cornerRadius > 0 {
			roundedCorner = fmt.Sprintf(` rx="%g"`, symbolPart.cornerRadius)
		}
		fmt.Fprintf(&svgBuilder, "    <rect x=\"%g\" y=\"%g\" width=\"%g\" height=\"%g\"%s/>\n",
			symbolPart.left, symbolPart.top, symbolPart.width, symbolPart.height, roundedCorner)
	}
	svgBuilder.WriteString("  </g>\n</svg>\n")
	return svgBuilder.String()
}

// drawAppIcon draws a rounded square holding a checkered flag (bundles, installers, menus).
func drawAppIcon(iconSize int) *image.RGBA {
	iconCanvas := image.NewRGBA(image.Rect(0, 0, iconSize, iconSize))
	cornerRadius := float64(iconSize) * 0.22
	checkerMargin := float64(iconSize) * 0.2
	checkerCells := 4
	checkerSpan := float64(iconSize) - 2*checkerMargin
	checkerCellSize := checkerSpan / float64(checkerCells)

	for pixelRow := range iconSize {
		for pixelColumn := range iconSize {
			pixelCenterHorizontal := float64(pixelColumn) + 0.5
			pixelCenterVertical := float64(pixelRow) + 0.5
			coverage := roundedSquareCoverage(pixelCenterHorizontal, pixelCenterVertical, float64(iconSize), cornerRadius)
			if coverage <= 0 {
				continue
			}
			pixelColor := appBackgroundColor
			insideCheckerHorizontal := pixelCenterHorizontal - checkerMargin
			insideCheckerVertical := pixelCenterVertical - checkerMargin
			if insideCheckerHorizontal >= 0 && insideCheckerVertical >= 0 && insideCheckerHorizontal < checkerSpan && insideCheckerVertical < checkerSpan {
				cellColumn := int(insideCheckerHorizontal / checkerCellSize)
				cellRow := int(insideCheckerVertical / checkerCellSize)
				if (cellColumn+cellRow)%2 == 0 {
					pixelColor = appCheckerColor
				}
			}
			iconCanvas.SetRGBA(pixelColumn, pixelRow, withAlpha(pixelColor, coverage))
		}
	}
	return iconCanvas
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
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return err
	}
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
