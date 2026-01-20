# SSD1322 Go Driver

A high-performance Go driver for the SSD1322 4-bit grayscale OLED display controller, compatible with the [periph.io](https://periph.io) ecosystem.

## Features

- **4-bit Grayscale Support**: 16 intensity levels (0-15) with automatic color conversion
- **Configurable Resolution**: Support for 256×64, 128×64, and other resolutions up to 480×128
- **SPI Interface**: 4-wire SPI (CPOL=0, CPHA=0) communication at up to 20 MHz
- **Differential Updates**: Automatic detection of changed regions for minimal bandwidth usage
- **Hardware Scrolling**: Hardware-accelerated horizontal scrolling
- **Display Control**: Contrast adjustment, inversion, power management
- **periph.io Compatible**: Implements the `display.Drawer` interface

## Installation

### Add to your project:

```bash
go get github.com/flavioheleno/ssd1322
go get github.com/flavioheleno/ssd1322/image4bit
go get periph.io/x/host/v3
```

## Quick Start

### Basic Usage

```go
package main

import (
	"image"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/spi/spireg"
	"github.com/flavioheleno/ssd1322"
	"github.com/flavioheleno/ssd1322/image4bit"
	"periph.io/x/host/v3"
)

func main() {
	// Initialize periph.io
	host.Init()

	// Open SPI bus
	spiBus, _ := spireg.Open("")
	dcPin := gpioreg.ByName("GPIO25")

	// Create display
	dev, _ := ssd1322.NewSPI(spiBus, dcPin, &ssd1322.Opts{
		W: 256,
		H: 64,
	})
	defer dev.Halt()

	// Create an image
	img := image4bit.NewHorizontalNibble(dev.Bounds())

	// Draw a gradient
	for y := 0; y < 64; y++ {
		for x := 0; x < 256; x++ {
			gray := byte(x / 16) // 0-15
			img.SetGray4(x, y, image4bit.Gray4{Y: gray})
		}
	}

	// Display the image
	dev.Draw(dev.Bounds(), img, image.Point{})
}
```

### Using Hardware Reset Pin (Optional)

If your display has a reset (RST) pin connected to GPIO, you can provide it for clean hardware initialization:

```go
// Get reset pin
rstPin := gpioreg.ByName("GPIO25")

// Create display with reset pin
dev, _ := ssd1322.NewSPI(spiBus, dcPin, &ssd1322.Opts{
	W:   256,
	H:   64,
	RST: rstPin,  // Optional reset pin
})
defer dev.Halt()
```

The driver automatically performs a hardware reset sequence (pull RST low for 200ms, then high for 200ms) during initialization. If RST is not provided, the driver skips hardware reset and relies on power-on reset.

## Hardware Connection

### Wiring (Raspberry Pi Example)

```
Display Pin    Raspberry Pi GPIO
GND            GND
VCC            3.3V (or 5V depending on display)
SCL/CLK        GPIO11 (SPI0 CLK)
SDA/MOSI       GPIO10 (SPI0 MOSI)
DC             GPIO25 (configurable)
CS             GPIO8 (SPI0 CE0) or GND
RES (optional) Any GPIO or VCC (use GPIO for controlled reset, see example above)
```

## Display Resolutions

The driver supports configurable resolutions:

```go
// 256×64 (most common)
dev, _ := ssd1322.NewSPI(b, dc, &ssd1322.Opts{W: 256, H: 64})

// 128×64 (smaller displays)
dev, _ := ssd1322.NewSPI(b, dc, &ssd1322.Opts{W: 128, H: 64})

// Custom sizes (must have even width, ≤480)
dev, _ := ssd1322.NewSPI(b, dc, &ssd1322.Opts{W: 320, H: 96})
```

## Image Format

### Using Grayscale Colors

```go
// Create image
img := image4bit.NewHorizontalNibble(dev.Bounds())

// Set pixels with grayscale values (0-15)
img.SetGray4(10, 20, image4bit.Gray4{Y: 8}) // Mid-gray

// Or use standard Go colors (auto-converted to grayscale)
white := image4bit.Gray4{Y: 15}
black := image4bit.Gray4{Y: 0}
img.Set(x, y, white)
```

### Direct Pixel Buffer

For maximum performance, you can write raw pixel data directly:

```go
// Create pixel buffer (format: 2 pixels per byte, horizontal nibble packing)
pixels := make([]byte, 256*64/2)

// Fill with pattern (high nibble = left pixel, low nibble = right pixel)
for i := 0; i < len(pixels); i++ {
	pixels[i] = 0xAA // Pattern: pixel 0 = 0xA, pixel 1 = 0xA
}

// Write directly (fastest method)
dev.Write(pixels)
```

## Drawing with Standard Go Images

The driver implements the `display.Drawer` interface, so you can use standard Go drawing operations:

```go
import (
	"image"
	"image/color"
	"image/draw"
)

// Create target image
img := image4bit.NewHorizontalNibble(dev.Bounds())

// Fill with color
fillColor := image.NewUniform(image4bit.Gray4{Y: 8})
draw.Draw(img, img.Bounds(), fillColor, image.Point{}, draw.Src)

// Draw rectangle
rect := image.Rect(50, 30, 150, 50)
draw.Draw(img, rect, image.NewUniform(image4bit.Gray4{Y: 15}), image.Point{}, draw.Src)

// Display
dev.Draw(dev.Bounds(), img, image.Point{})
```

## Grayscale Levels

The SSD1322 supports 16 grayscale levels:

```go
const (
	Black  = 0  // Minimum brightness
	Gray1  = 1
	Gray2  = 2
	// ...
	Gray14 = 14
	White  = 15 // Maximum brightness
)

img.SetGray4(x, y, image4bit.Gray4{Y: 8}) // Mid-gray
```

## Performance

### Update Performance (256×64 display on 10MHz SPI)

- **Full frame update**: ~200 microseconds
- **Typical partial update**: 50-500 microseconds (depends on changed area)
- **Hardware scrolling**: Smooth (handled entirely by display)

The driver uses **differential updates** by default, automatically detecting changed regions and transmitting only the minimum necessary data.

### Example: Partial Update

```go
// First display
img := image4bit.NewHorizontalNibble(dev.Bounds())
// ... draw content ...
dev.Draw(dev.Bounds(), img, image.Point{})

// Update only a region (20×20 pixels)
// Only those pixels are transmitted to the display!
rect := image.Rect(100, 100, 120, 120)
draw.Draw(img, rect, image.NewUniform(image4bit.Gray4{Y: 15}), image.Point{}, draw.Src)
dev.Draw(dev.Bounds(), img, image.Point{})
```

## Hardware Scrolling

```go
// Start scrolling left at 10 frames per cycle
dev.ScrollHorizontal(0, 63, ssd1322.Speed10Frames, false)
time.Sleep(5 * time.Second)

// Stop scrolling
dev.StopScroll()
```

### Scroll Speeds

```go
ssd1322.Speed6Frames    // Fastest
ssd1322.Speed10Frames   // Recommended
ssd1322.Speed100Frames  // Slower
ssd1322.Speed200Frames  // Slowest
```

## Display Control

### Contrast

```go
// Set contrast (0-255)
dev.SetContrast(128) // 50% brightness
dev.SetContrast(255) // Maximum brightness
```

### Inversion

```go
// Invert display colors
dev.Invert(true)  // Black becomes white, white becomes black

// Restore normal mode
dev.Invert(false)
```

### Power Management

```go
// Turn off display
dev.Halt()

// Note: Once halted, the device needs to be re-initialized
// to resume operation (not currently supported by Write)
```

## Examples

Run the demo program to see all features:

```bash
cd examples/ssd1322_demo
go run main.go -demo=all
```

### Demo Modes

- `all` - Run all demonstrations sequentially
- `gradient` - Display a horizontal grayscale gradient
- `patterns` - Show test patterns and grayscale levels
- `scroll` - Demonstrate hardware scrolling
- `contrast` - Cycle through contrast levels

### Command Line Options

```
  -demo string
        Demo to run: all, gradient, patterns, scroll, contrast (default "all")
  -dc string
        Data/Command pin name (default "GPIO25")
  -height int
        Display height in pixels (default 64)
  -hz int
        SPI frequency in Hz (default 10000000)
  -spi string
        SPI bus name (empty for default)
  -width int
        Display width in pixels (default 256)
```

## Memory Layout

The SSD1322 uses **horizontal nibble packing** where each byte contains 2 pixels:

```
Byte Layout: [High Nibble][Low Nibble]
             [Left Pixel] [Right Pixel]

Example: 0x5A represents:
  High nibble (0x5): left pixel, gray level 5
  Low nibble (0xA): right pixel, gray level 10
```

The `HorizontalNibble` image format handles this packing automatically.

## Driver Implementation Details

### Initialization Sequence

The driver sends a complete initialization sequence to the display during `NewSPI()`:
- Unlock command codes
- Display configuration (clock divider, MUX ratio, etc.)
- Grayscale table selection
- Contrast and timing parameters
- Enable internal VDD regulator
- Clear display RAM
- Turn display ON

### Differential Updates

The `Draw()` method automatically:
1. Renders the source image into an internal buffer
2. Compares with the previous frame
3. Detects the minimal bounding rectangle of changes
4. Transmits only changed pixels to the display

This significantly reduces SPI bandwidth usage for typical workloads.

### Column Offset

The SSD1322 has 480 columns of internal RAM. For smaller displays:
- **256×64**: Uses columns 112-367 (centered)
- **128×64**: Uses columns 176-303 (centered)

The driver automatically calculates and applies this offset.

## Testing

Run the test suite:

```bash
go test -v ./...
```

Test coverage includes:
- Grayscale color conversion
- Pixel packing and memory layout
- Image format validation
- Device initialization
- Options validation
- Differential update algorithm
- Buffer size validation

## Datasheet

Full documentation of the SSD1322 controller:
https://www.displayfuture.com/Display/datasheet/controller/SSD1322.pdf

## License

This driver is provided as-is for use with the periph.io ecosystem.

## Credits

Based on the architectural patterns of periph.io's SSD1306 driver and inspired by:
- [luma.oled](https://luma-oled.readthedocs.io/) (Python)
- [ssd1322 driver](https://github.com/mtrudel/ssd1322) (Elixir)

## Issues and Contributions

For issues or questions:
1. Check the [plan document](./PLAN.md) for design decisions
2. Review test cases for usage examples
3. Consult the example program for integration patterns
