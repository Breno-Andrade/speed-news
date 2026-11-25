package main

import (
	"bytes"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

const (
	committedIconsDirectory = "../../apps/desktop/src-tauri/icons"
	committedGnomeIconPath  = "../../apps/desktop/gnome-shell-extension/icons/speed-news-flag.svg"
)

// The committed icons must be exactly what the generator draws: editing one by hand (or changing
// the generator without running `make icons`) is how Windows and GNOME ended up with different
// tray icons. Pixels are compared, not bytes, so a Go upgrade changing PNG compression is fine.
func TestCommittedIconsMatchTheGenerator(test *testing.T) {
	for _, trayIcon := range append(trayIconFiles(), generatedFile{relativePath: "app-icon.png", pngImage: drawAppIcon(appIconSize)}) {
		committedImage := readPNG(test, filepath.Join(committedIconsDirectory, trayIcon.relativePath))
		if !samePixels(committedImage, pngRoundTrip(test, trayIcon.pngImage)) {
			test.Errorf("%s differs from the generator output: run `make icons`", trayIcon.relativePath)
		}
	}
	committedSVG, err := os.ReadFile(committedGnomeIconPath)
	if err != nil {
		test.Fatal(err)
	}
	if string(committedSVG) != gnomeSymbolSVG() {
		test.Error("the GNOME panel SVG differs from the generator output: run `make icons`")
	}
}

func TestTrayIconsAreTheSameSymbolInEveryVariant(test *testing.T) {
	lightIcon := drawTrayIcon(lightSymbolColor, false)
	darkIcon := drawTrayIcon(darkSymbolColor, false)
	if !sameAlpha(lightIcon, darkIcon) {
		test.Error("light and dark tray icons must have the same shape, only the color changes")
	}
	// The pole's top-left pixel is opaque; the area right of the flag is transparent.
	if lightIcon.RGBAAt(3, 10).A != 255 || lightIcon.RGBAAt(31, 31).A != 0 {
		test.Error("unexpected symbol geometry")
	}
}

func TestUnreadVariantAddsARedDotWithoutTouchingThePole(test *testing.T) {
	plainIcon := drawTrayIcon(lightSymbolColor, false)
	unreadIcon := drawTrayIcon(lightSymbolColor, true)
	dotCenter := unreadIcon.RGBAAt(int(unreadDotCenterX*2), int(unreadDotCenterY*2))
	if dotCenter != unreadDotColor {
		test.Errorf("the dot center must be the unread red, got %+v", dotCenter)
	}
	for pixelRow := range trayIconSize {
		for pixelColumn := range 6 { // pole columns
			if plainIcon.RGBAAt(pixelColumn, pixelRow) != unreadIcon.RGBAAt(pixelColumn, pixelRow) {
				test.Fatalf("the dot must not change the pole (pixel %d,%d)", pixelColumn, pixelRow)
			}
		}
	}
}

// Every symbol edge must fall on the 16 px grid, so the Windows 100 % downscale stays crisp.
func TestSymbolEdgesAreAlignedToTheSixteenPixelGrid(test *testing.T) {
	for _, symbolPart := range flagSymbol {
		for _, gridValue := range []float64{symbolPart.left, symbolPart.top, symbolPart.width, symbolPart.height} {
			if gridValue != float64(int(gridValue)) {
				test.Errorf("symbol part %+v is not on whole grid units", symbolPart)
			}
		}
	}
}

func TestWriteGeneratedFilesCreatesEveryOutput(test *testing.T) {
	outputDirectory := test.TempDir()
	gnomeIconPath := filepath.Join(outputDirectory, "gnome", "flag.svg")
	if err := writeGeneratedFiles(outputDirectory, gnomeIconPath); err != nil {
		test.Fatal(err)
	}
	for _, trayIcon := range trayIconFiles() {
		if readPNG(test, filepath.Join(outputDirectory, trayIcon.relativePath)).Bounds().Dx() != trayIconSize {
			test.Errorf("%s has the wrong size", trayIcon.relativePath)
		}
	}
	if _, err := os.Stat(gnomeIconPath); err != nil {
		test.Error(err)
	}
}

func readPNG(test *testing.T, filePath string) image.Image {
	test.Helper()
	pngFile, err := os.Open(filePath)
	if err != nil {
		test.Fatal(err)
	}
	defer pngFile.Close()
	decodedImage, err := png.Decode(pngFile)
	if err != nil {
		test.Fatalf("%s: %v", filePath, err)
	}
	return decodedImage
}

// pngRoundTrip encodes and decodes like a file on disk would (PNG stores straight alpha, the
// canvas is premultiplied, so partly transparent pixels change slightly on the way).
func pngRoundTrip(test *testing.T, sourceImage image.Image) image.Image {
	test.Helper()
	var encodedImage bytes.Buffer
	if err := png.Encode(&encodedImage, sourceImage); err != nil {
		test.Fatal(err)
	}
	decodedImage, err := png.Decode(&encodedImage)
	if err != nil {
		test.Fatal(err)
	}
	return decodedImage
}

func toRGBA(sourceImage image.Image) *image.RGBA {
	rgbaImage := image.NewRGBA(sourceImage.Bounds())
	draw.Draw(rgbaImage, rgbaImage.Bounds(), sourceImage, sourceImage.Bounds().Min, draw.Src)
	return rgbaImage
}

func samePixels(firstImage image.Image, secondImage image.Image) bool {
	if firstImage.Bounds() != secondImage.Bounds() {
		return false
	}
	return string(toRGBA(firstImage).Pix) == string(toRGBA(secondImage).Pix)
}

func sameAlpha(firstImage *image.RGBA, secondImage *image.RGBA) bool {
	for pixelIndex := 3; pixelIndex < len(firstImage.Pix); pixelIndex += 4 {
		if firstImage.Pix[pixelIndex] != secondImage.Pix[pixelIndex] {
			return false
		}
	}
	return true
}
