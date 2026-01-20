// Package ssd1322 controls a SSD1322 OLED display via SPI.
//
// The SSD1322 is a 4-bit grayscale OLED controller supporting up to 480×128 pixels.
// This driver implements the display.Drawer interface from periph.io.
//
// # Display Characteristics
//
// - 4-bit grayscale with 16 intensity levels (0-15)
// - Support for various resolutions (typically 256×64 or 128×64)
// - Hardware scrolling support (horizontal only)
// - Adjustable contrast (0-255)
// - Display inversion
// - 480-column internal RAM with automatic centering for smaller displays
//
// # Hardware Connection
//
// Connect the SSD1322 display to your system via SPI:
//
//	Display Pin → System Pin
//	GND         → GND
//	VCC         → 3.3V (or 5V depending on display)
//	SCL/CLK     → SPI Clock (SCLK)
//	SDA/MOSI    → SPI Data (MOSI)
//	DC          → GPIO (any available pin)
//	CS          → SPI Chip Select (or GND if always selected)
//	RES         → Optional: GPIO for hardware reset
//
// # Basic Usage
//
// Example of creating and using the display:
//
//	package main
//
//	import (
//		"image"
//		"periph.io/x/conn/v3/gpio/gpioreg"
//		"periph.io/x/conn/v3/spi/spireg"
//		"github.com/flavioheleno/ssd1322"
//		"github.com/flavioheleno/ssd1322/image4bit"
//		"periph.io/x/host/v3"
//	)
//
//	func main() {
//		// Initialize periph.io
//		host.Init()
//
//		// Open SPI bus
//		spiBus, _ := spireg.Open("")
//
//		// Get Data/Command GPIO pin
//		dcPin := gpioreg.ByName("GPIO25")
//
//		// Create device
//		dev, _ := ssd1322.NewSPI(spiBus, dcPin, &ssd1322.Opts{
//			W: 256,
//			H: 64,
//		})
//		defer dev.Halt()
//
//		// Create an image with grayscale pixels
//		img := image4bit.NewHorizontalNibble(dev.Bounds())
//
//		// Draw a gradient (from black to white)
//		for y := 0; y < 64; y++ {
//			for x := 0; x < 256; x++ {
//				gray := byte(x / 16) // Divide into 16 levels
//				img.SetGray4(x, y, image4bit.Gray4{Y: gray})
//			}
//		}
//
//		// Display the image
//		dev.Draw(dev.Bounds(), img, image.Point{})
//	}
//
// # Using Hardware Reset Pin (Optional)
//
// If your display has a reset (RST) pin connected to a GPIO, you can provide it
// in the Opts struct for clean hardware initialization:
//
//	rstPin := gpioreg.ByName("GPIO25")
//
//	dev, _ := ssd1322.NewSPI(spiBus, dcPin, &ssd1322.Opts{
//		W:   256,
//		H:   64,
//		RST: rstPin,  // Optional reset pin
//	})
//
// The driver will automatically perform a hardware reset sequence (pull RST low,
// wait 200ms, pull high, wait 200ms) during initialization. If RST is nil or not
// provided, the driver skips the hardware reset and relies on power-on reset.
//
// # Drawing Modes
//
// The driver supports two drawing modes:
//
// ## Full-Frame Update
//
// Write raw pixel data directly to the display. Use this for maximum performance
// when updating the entire frame:
//
//	pixels := make([]byte, 256*64/2) // 8192 bytes for 256×64
//	// ... fill pixels ...
//	dev.Write(pixels)
//
// ## Differential Updates
//
// Use the Draw method for automatic differential updates. The driver computes
// the minimal bounding rectangle of changes and only updates that region.
// This is more efficient for partial updates:
//
//	dev.Draw(dev.Bounds(), myImage, image.Point{})
//
// # Grayscale Colors
//
// The display supports 16 grayscale levels (0-15). Use the Gray4 color type:
//
//	// Black (0)
//	black := image4bit.Gray4{Y: 0}
//
//	// Mid-gray (8)
//	gray := image4bit.Gray4{Y: 8}
//
//	// White (15)
//	white := image4bit.Gray4{Y: 15}
//
// Standard Go colors are automatically converted to Gray4 grayscale.
//
// # Hardware Scrolling
//
// The display supports horizontal scrolling:
//
//	// Start scrolling left
//	dev.ScrollHorizontal(0, 63, ssd1322.Speed10Frames, false)
//	time.Sleep(5 * time.Second)
//
//	// Stop scrolling
//	dev.StopScroll()
//
// # Performance
//
// With differential updates, typical performance on 100kHz I2C:
// - Full-frame update: ~500ms (limited by I2C bandwidth)
// - Typical partial update: 10-50ms (depends on changed area)
// - Hardware scrolling: Smooth (handled by display)
//
// SPI communication is significantly faster (typical 10MHz):
// - Full-frame update: ~200μs
// - Typical partial update: 50-500μs
//
// # Display Resolution
//
// This driver supports configurable resolutions. Common options:
//
//	Opts{W: 256, H: 64}  // 256×64 (most common)
//	Opts{W: 128, H: 64}  // 128×64 (smaller displays)
//	Opts{W: 256, H: 128} // 256×128 (extended height, if available)
//
// Width must be even and ≤480. Height must be ≤128.
//
// # Datasheet
//
// For detailed register descriptions and timing information, see:
// https://www.displayfuture.com/Display/datasheet/controller/SSD1322.pdf
//
// # Compatibility with periph.io
//
// This driver implements the display.Drawer interface from periph.io:
// https://pkg.go.dev/periph.io/x/periph/conn/display
//
// It can be used with any periph.io tool or library expecting a display.Drawer.
package ssd1322
