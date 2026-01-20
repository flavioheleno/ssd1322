// Package main demonstrates basic usage of the SSD1322 OLED display driver.
//
// This example shows how to:
// - Initialize the display
// - Draw basic shapes and patterns
// - Use grayscale colors
// - Perform full-frame and partial updates
// - Use hardware scrolling
// - Adjust contrast
//
// Hardware Setup:
//
// Connect your SSD1322 display via SPI:
//
//	Display    Raspberry Pi
//	GND        GND
//	VCC        3.3V
//	SCL/CLK    GPIO11 (SPI0 CLK)
//	SDA/MOSI   GPIO10 (SPI0 MOSI)
//	DC         GPIO25 (configurable)
//	CS         GPIO8 (SPI0 CE0) or GND
//
// Install periph.io:
//
//	go get periph.io/x/host/cmd/...
//	go get periph.io/x/conn/v3
//	go get periph.io/x/devices/v3
//
package main

import (
	"flag"
	"fmt"
	"image"
	"image/draw"
	"log"
	"time"

	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/devices/v3/ssd1322"
	"periph.io/x/devices/v3/ssd1322/image4bit"
	"periph.io/x/host/v3"
)

var (
	width      = flag.Int("width", 256, "Display width in pixels")
	height     = flag.Int("height", 64, "Display height in pixels")
	spiBus     = flag.String("spi", "", "SPI bus name (empty for default)")
	dcPin      = flag.String("dc", "GPIO25", "Data/Command pin name")
	demoMode   = flag.String("demo", "all", "Demo to run: all, gradient, patterns, scroll, contrast")
	spiHz      = flag.Int("hz", 10000000, "SPI frequency in Hz")
)

func main() {
	flag.Parse()

	// Initialize periph.io
	if _, err := host.Init(); err != nil {
		log.Fatalf("Failed to initialize periph.io: %v", err)
	}

	// Open SPI bus
	b, err := spireg.Open(*spiBus)
	if err != nil {
		log.Fatalf("Failed to open SPI bus: %v", err)
	}
	defer b.Close()

	// Get DC GPIO pin
	pin := gpioreg.ByName(*dcPin)
	if pin == nil {
		log.Fatalf("GPIO pin %s not found", *dcPin)
	}

	// Create display device
	dev, err := ssd1322.NewSPI(b, pin, &ssd1322.Opts{
		W: *width,
		H: *height,
	})
	if err != nil {
		log.Fatalf("Failed to create display: %v", err)
	}
	defer dev.Halt()

	fmt.Printf("Display initialized: %v\n", dev)
	fmt.Printf("Resolution: %dx%d\n", *width, *height)

	// Run demo based on flag
	switch *demoMode {
	case "all":
		runAllDemos(dev)
	case "gradient":
		runGradientDemo(dev)
	case "patterns":
		runPatternDemo(dev)
	case "scroll":
		runScrollDemo(dev)
	case "contrast":
		runContrastDemo(dev)
	default:
		fmt.Printf("Unknown demo: %s\n", *demoMode)
	}

	fmt.Println("Demo complete")
}

// runAllDemos runs all demonstrations sequentially
func runAllDemos(dev *ssd1322.Dev) {
	fmt.Println("\n=== Running All Demos ===")

	fmt.Println("1. Gradient Demo (5 seconds)")
	runGradientDemo(dev)
	time.Sleep(5 * time.Second)

	fmt.Println("\n2. Pattern Demo (5 seconds)")
	runPatternDemo(dev)
	time.Sleep(5 * time.Second)

	fmt.Println("\n3. Contrast Demo (5 seconds)")
	runContrastDemo(dev)
	time.Sleep(5 * time.Second)

	fmt.Println("\n4. Scroll Demo (5 seconds)")
	runScrollDemo(dev)
	time.Sleep(5 * time.Second)

	// Clear display
	img := image4bit.NewHorizontalNibble(dev.Bounds())
	dev.Draw(dev.Bounds(), img, image.Point{})
}

// runGradientDemo displays a horizontal grayscale gradient
func runGradientDemo(dev *ssd1322.Dev) {
	fmt.Println("  Drawing horizontal gradient...")

	// Create image
	img := image4bit.NewHorizontalNibble(dev.Bounds())

	// Draw gradient from left (black) to right (white)
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Calculate grayscale value based on x position
			gray := byte(x * 15 / width)
			img.SetGray4(x, y, image4bit.Gray4{Y: gray})
		}
	}

	// Display the gradient
	dev.Draw(dev.Bounds(), img, image.Point{})
	fmt.Println("  Gradient displayed")
}

// runPatternDemo displays various test patterns
func runPatternDemo(dev *ssd1322.Dev) {
	fmt.Println("  Drawing test patterns...")

	img := image4bit.NewHorizontalNibble(dev.Bounds())
	bounds := img.Bounds()

	// Draw checkerboard pattern
	checkSize := 4
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			checker := ((x / checkSize) + (y / checkSize)) % 2
			var gray byte
			if checker == 0 {
				gray = 15
			} else {
				gray = 0
			}
			img.SetGray4(x, y, image4bit.Gray4{Y: gray})
		}
	}

	// Draw vertical bars with different gray levels
	barWidth := bounds.Dx() / 16
	for barIdx := 0; barIdx < 16; barIdx++ {
		startX := bounds.Min.X + barIdx*barWidth
		endX := startX + barWidth
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := startX; x < endX && x < bounds.Max.X; x++ {
				img.SetGray4(x, y, image4bit.Gray4{Y: byte(barIdx)})
			}
		}
	}

	dev.Draw(dev.Bounds(), img, image.Point{})
	fmt.Println("  Pattern displayed")
}

// runContrastDemo cycles through different contrast levels
func runContrastDemo(dev *ssd1322.Dev) {
	fmt.Println("  Cycling contrast levels...")

	// Create test image with mid-gray content
	img := image4bit.NewHorizontalNibble(dev.Bounds())
	gray := image.NewUniform(image4bit.Gray4{Y: 8})
	draw.Draw(img, img.Bounds(), gray, image.Point{}, draw.Src)
	dev.Draw(dev.Bounds(), img, image.Point{})

	// Cycle through contrast levels
	for contrast := byte(0); contrast < 255; contrast += 16 {
		if err := dev.SetContrast(contrast); err != nil {
			fmt.Printf("  Error setting contrast %d: %v\n", contrast, err)
			continue
		}
		fmt.Printf("  Contrast: %d\n", contrast)
		time.Sleep(500 * time.Millisecond)
	}

	// Reset to max contrast
	if err := dev.SetContrast(255); err != nil {
		fmt.Printf("  Error resetting contrast: %v\n", err)
	}
}

// runScrollDemo demonstrates hardware scrolling
func runScrollDemo(dev *ssd1322.Dev) {
	fmt.Println("  Testing horizontal scroll...")

	// Create test image with text pattern
	img := image4bit.NewHorizontalNibble(dev.Bounds())

	// Fill with pattern
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			var gray byte
			if (x/2)%8 == 0 || (y/8)%2 == 0 {
				gray = 15
			} else {
				gray = 0
			}
			img.SetGray4(x, y, image4bit.Gray4{Y: gray})
		}
	}

	dev.Draw(dev.Bounds(), img, image.Point{})

	// Scroll left
	fmt.Println("  Scrolling left...")
	if err := dev.ScrollHorizontal(0, byte(bounds.Dy()-1), ssd1322.Speed10Frames, false); err != nil {
		fmt.Printf("  Error starting scroll: %v\n", err)
		return
	}

	time.Sleep(2 * time.Second)

	// Stop scroll
	if err := dev.StopScroll(); err != nil {
		fmt.Printf("  Error stopping scroll: %v\n", err)
	}
	fmt.Println("  Scroll stopped")
}

// Example of efficient partial update using differential updates
func examplePartialUpdate(dev *ssd1322.Dev) {
	// Create initial image
	img := image4bit.NewHorizontalNibble(dev.Bounds())

	// Draw initial content
	black := image.NewUniform(image4bit.Gray4{Y: 0})
	draw.Draw(img, img.Bounds(), black, image.Point{}, draw.Src)
	dev.Draw(dev.Bounds(), img, image.Point{})

	// Update just a small region (differential update)
	updateRect := image.Rect(10, 10, 50, 50)
	white := image.NewUniform(image4bit.Gray4{Y: 15})
	draw.Draw(img, updateRect, white, image.Point{}, draw.Src)

	// Only the changed region (40x40 pixels) is transferred to display
	dev.Draw(dev.Bounds(), img, image.Point{})

	fmt.Println("Partial update example: only the 40x40 pixel region was transmitted")
}

// Example of direct pixel buffer write (maximum performance)
func exampleDirectWrite(dev *ssd1322.Dev) {
	// Create raw pixel data (8192 bytes for 256x64 display)
	pixels := make([]byte, 256*64/2)

	// Fill with alternating pattern
	for i := 0; i < len(pixels); i++ {
		pixels[i] = 0xAA // Alternating 0x0A patterns
	}

	// Write directly (fastest method, but no conversion)
	if n, err := dev.Write(pixels); err != nil {
		fmt.Printf("Error writing pixels: %v\n", err)
	} else {
		fmt.Printf("Wrote %d bytes directly\n", n)
	}
}
