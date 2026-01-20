// Package ssd1322 controls a SSD1322 OLED display via SPI.
//
// The SSD1322 is a 4-bit grayscale OLED controller supporting up to 480x128 pixels.
// Common display resolutions are 256x64 and 128x64.
//
// See the examples for how to use this package.
package ssd1322

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"time"

	"github.com/flavioheleno/ssd1322/image4bit"
	"periph.io/x/conn/v3"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/spi"
)

// Opts is the configuration for the SSD1322 display.
type Opts struct {
	// Display dimensions in pixels
	W int // Width (default: 256, must be even and ≤480)
	H int // Height (default: 64, must be ≤128)

	// Rotation and mirroring
	Rotated       bool // 180° rotation
	Sequential    bool // Sequential COM pin configuration
	SwapTopBottom bool // Swap top/bottom display halves

	// Optional hardware reset pin
	RST gpio.PinIO // Reset pin (optional, nil if not used)
}

// Dev is the device handle for the SSD1322 display.
type Dev struct {
	// Communication
	c   conn.Conn   // SPI connection
	dc  gpio.PinOut // Data/Command pin
	rst gpio.PinIO  // Reset pin (optional)

	// Display geometry
	rect         image.Rectangle
	columnOffset int // For centering on 480-column RAM

	// Pixel buffers
	buffer []byte                      // Current frame
	next   *image4bit.HorizontalNibble // For lazy double buffering
	lastDm image4bit.HorizontalNibble  // Last displayed frame for differential updates

	// Change tracking
	minCol, maxCol int
	minRow, maxRow int

	// State
	halted bool
}

// NewSPI creates a new SSD1322 device connected via SPI.
//
// The SPI port is configured for 10MHz, Mode0 (CPOL=0, CPHA=0), 8-bit transfers.
// The dc (Data/Command) GPIO pin must be provided and configured as an output.
//
// opts can be nil to use defaults (256x64 display).
func NewSPI(p spi.Port, dc gpio.PinOut, opts *Opts) (*Dev, error) {
	// Apply defaults and validate options
	if opts == nil {
		opts = &Opts{W: 256, H: 64}
	}

	if opts.W <= 0 || opts.W%2 != 0 || opts.W > 480 {
		return nil, errors.New("ssd1322: width must be even and between 2 and 480")
	}
	if opts.H <= 0 || opts.H > 128 {
		return nil, errors.New("ssd1322: height must be between 1 and 128")
	}

	// Establish SPI connection
	// SSD1322 supports Mode0 (CPOL=0, CPHA=0) or Mode3 (CPOL=1, CPHA=1)
	// Using Mode0 and 10MHz (conservative, up to 20MHz supported)
	c, err := p.Connect(10*1000000, spi.Mode0, 8)
	if err != nil {
		return nil, err
	}

	// Create device
	d := &Dev{
		c:            c,
		dc:           dc,
		rst:          opts.RST,
		rect:         image.Rect(0, 0, opts.W, opts.H),
		columnOffset: (480 - opts.W) / 2,
		buffer:       make([]byte, opts.W*opts.H/2),
		minCol:       0,
		maxCol:       opts.W - 1,
		minRow:       0,
		maxRow:       opts.H - 1,
	}

	// Initialize the display
	if err := d.init(opts); err != nil {
		return nil, err
	}

	return d, nil
}

// init sends the initialization sequence to the display.
func (d *Dev) init(opts *Opts) error {
	// Hardware reset sequence (if RST pin is provided)
	if d.rst != nil {
		if err := d.rst.Out(gpio.Low); err != nil {
			return fmt.Errorf("ssd1322: failed to pull RST low: %w", err)
		}
		time.Sleep(200 * time.Millisecond)

		if err := d.rst.Out(gpio.High); err != nil {
			return fmt.Errorf("ssd1322: failed to pull RST high: %w", err)
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Build initialization command sequence
	cmds := []byte{
		0xFD, 0x12, // Unlock command codes
		0xAE,       // Display OFF
		0xB3, 0xF2, // Clock divider and oscillator frequency
		0xCA, byte(opts.H - 1), // MUX ratio
		0xA2, 0x00, // Display offset
		0xA1, 0x00, // Start line
	}

	// Remap settings: adjust for rotation and mirroring
	remap1, remap2 := byte(0x14), byte(0x11)
	if opts.Rotated {
		remap1 = 0x06
		remap2 = 0x11
	}
	if opts.Sequential {
		remap2 |= 0x01
	}
	if opts.SwapTopBottom {
		remap2 |= 0x02
	}

	cmds = append(cmds,
		0xA0, remap1, remap2, // Remap and dual COM mode
		0xAB, 0x01, // Function selection (enable internal VDD)
		0xB4, 0xA0, 0xFD, // VSL (display enhancement)
		0xC1, 0xFF, // Contrast (max)
		0xC7, 0x0F, // Master contrast
		0xB9,       // Use default grayscale table
		0xB1, 0xE2, // Phase length
		0xD1, 0x82, 0x20, // Display enhancements
		0xBB, 0x1F, // Pre-charge voltage
		0xB6, 0x08, // Second pre-charge period
		0xBE, 0x07, // VCOMH voltage
		0xA6, // Normal display mode
		0xA9, // Exit partial display mode
	)

	if err := d.sendCommands(cmds); err != nil {
		return err
	}

	// Clear display RAM
	if err := d.clearRAM(); err != nil {
		return err
	}

	// Turn display ON
	return d.sendCommand(0xAF)
}

// clearRAM clears all pixels in the display RAM.
func (d *Dev) clearRAM() error {
	// Set column address window
	colStart := byte(d.columnOffset / 2)
	colEnd := byte((d.columnOffset + d.rect.Dx() - 1) / 2)

	// Set row address window
	commands := []byte{
		0x15, colStart, colEnd, // Column address
		0x75, 0, byte(d.rect.Dy() - 1), // Row address
		0x5C, // Enable write to RAM
	}

	if err := d.sendCommands(commands); err != nil {
		return err
	}

	// Send zero pixels
	zeros := make([]byte, d.rect.Dx()*d.rect.Dy()/2)
	return d.sendData(zeros)
}

// sendCommand sends a single command byte.
func (d *Dev) sendCommand(cmd byte) error {
	return d.sendCommands([]byte{cmd})
}

// sendCommands sends a slice of command bytes.
func (d *Dev) sendCommands(cmds []byte) error {
	if err := d.dc.Out(gpio.Low); err != nil {
		return err
	}
	return d.c.Tx(cmds, nil)
}

// sendData sends a slice of data bytes.
func (d *Dev) sendData(data []byte) error {
	if err := d.dc.Out(gpio.High); err != nil {
		return err
	}
	return d.c.Tx(data, nil)
}

// writeRect writes pixel data to a rectangular region of the display.
func (d *Dev) writeRect(x, y, width, height int, pixels []byte) error {
	// Calculate column addresses (in nibbles)
	colStart := byte((x + d.columnOffset) / 2)
	colEnd := byte((x + width - 1 + d.columnOffset) / 2)

	// Set addressing window and enable RAM write
	commands := []byte{
		0x15, colStart, colEnd, // Column address
		0x75, byte(y), byte(y + height - 1), // Row address
		0x5C, // Enable write to RAM
	}

	if err := d.sendCommands(commands); err != nil {
		return err
	}

	// Send pixel data
	return d.sendData(pixels)
}

// ColorModel returns the color model of the display.
func (d *Dev) ColorModel() color.Model {
	return image4bit.Gray4Model
}

// Bounds returns the image bounds of the display.
func (d *Dev) Bounds() image.Rectangle {
	return d.rect
}

// Write writes raw pixel data to the display in HorizontalNibble format.
// The data must be exactly d.rect.Dx() * d.rect.Dy() / 2 bytes.
func (d *Dev) Write(pixels []byte) (int, error) {
	if d.halted {
		return 0, errors.New("ssd1322: halted")
	}
	if len(pixels) != len(d.buffer) {
		return 0, errors.New("ssd1322: invalid buffer size")
	}
	if err := d.writeFullFrame(pixels); err != nil {
		return 0, err
	}
	copy(d.buffer, pixels)
	return len(pixels), nil
}

// Draw draws an image onto the display with differential update optimization.
// The dst rectangle specifies the destination region on the display.
// The src image is positioned at src point sp within the destination.
func (d *Dev) Draw(dst image.Rectangle, src image.Image, sp image.Point) error {
	if d.halted {
		return errors.New("ssd1322: halted")
	}

	// Clip to display bounds
	dst = dst.Intersect(d.rect)
	if dst.Empty() {
		return nil
	}

	// Fast path: if source is already HorizontalNibble at full size
	if srcImg, ok := src.(*image4bit.HorizontalNibble); ok {
		zeroPoint := image.Point{}
		if dst == d.rect && sp == zeroPoint && srcImg.Rect == d.rect {
			return d.writeFullFrame(srcImg.Pix)
		}
	}

	// Slow path: render to buffer with differential updates
	// Lazy-initialize double buffer
	if d.next == nil {
		d.next = image4bit.NewHorizontalNibble(d.rect)
		// Initialize last frame buffer
		d.lastDm = image4bit.HorizontalNibble{
			Pix:    make([]byte, len(d.buffer)),
			Stride: d.next.Stride,
			Rect:   d.rect,
		}
		copy(d.lastDm.Pix, d.buffer)
	}

	// Draw source into our buffer
	draw.Draw(d.next, dst, src, sp, draw.Src)

	// Calculate minimal bounding box of changed pixels
	minCol, maxCol, minRow, maxRow := d.calculateDiff()
	if minCol > maxCol {
		// No changes
		return nil
	}

	// Extract changed region
	changedData := d.extractRegion(minCol, maxCol, minRow, maxRow)

	// Write to display
	if err := d.writeRect(minCol, minRow, maxCol-minCol+1, maxRow-minRow+1, changedData); err != nil {
		return err
	}

	// Update stored buffers
	copy(d.buffer, d.next.Pix)
	copy(d.lastDm.Pix, d.next.Pix)

	return nil
}

// calculateDiff compares the current and next buffers to find the minimal
// changed region. Returns (minCol, maxCol, minRow, maxRow) or (1, 0, 0, 0) if no changes.
func (d *Dev) calculateDiff() (minCol, maxCol, minRow, maxRow int) {
	width := d.rect.Dx()
	height := d.rect.Dy()
	stride := width / 2

	minRow = height
	maxRow = -1
	minCol = width
	maxCol = -1

	// Scan row by row to find differences
	for y := 0; y < height; y++ {
		rowStart := y * stride
		rowEnd := rowStart + stride

		if !bytes.Equal(d.lastDm.Pix[rowStart:rowEnd], d.next.Pix[rowStart:rowEnd]) {
			if y < minRow {
				minRow = y
			}
			if y > maxRow {
				maxRow = y
			}

			// Scan columns within this row for precise boundaries
			for x := 0; x < stride; x++ {
				if d.lastDm.Pix[rowStart+x] != d.next.Pix[rowStart+x] {
					// Each byte represents 2 pixels
					colStart := x * 2
					colEnd := colStart + 1
					if colStart < minCol {
						minCol = colStart
					}
					if colEnd > maxCol {
						maxCol = colEnd
					}
				}
			}
		}
	}

	// Align to even pixel boundaries
	if minCol%2 != 0 {
		minCol--
	}
	if maxCol%2 == 0 && maxCol < width-1 {
		maxCol++
	}

	return
}

// extractRegion extracts the pixel data for a rectangular region.
func (d *Dev) extractRegion(minCol, maxCol, minRow, maxRow int) []byte {
	width := maxCol - minCol + 1
	height := maxRow - minRow + 1
	stride := d.rect.Dx() / 2
	byteWidth := width / 2

	result := make([]byte, byteWidth*height)
	dstIdx := 0

	for y := minRow; y <= maxRow; y++ {
		srcStart := y*stride + minCol/2
		copy(result[dstIdx:], d.next.Pix[srcStart:srcStart+byteWidth])
		dstIdx += byteWidth
	}

	return result
}

// writeFullFrame writes the entire frame buffer to the display.
func (d *Dev) writeFullFrame(pixels []byte) error {
	return d.writeRect(0, 0, d.rect.Dx(), d.rect.Dy(), pixels)
}

// SetContrast sets the display contrast (0-255).
func (d *Dev) SetContrast(contrast byte) error {
	if d.halted {
		return errors.New("ssd1322: halted")
	}
	return d.sendCommands([]byte{0xC1, contrast})
}

// Invert inverts the display colors (black becomes white and vice versa).
func (d *Dev) Invert(invert bool) error {
	if d.halted {
		return errors.New("ssd1322: halted")
	}
	mode := byte(0xA6) // Normal display
	if invert {
		mode = 0xA7 // Inverted display
	}
	return d.sendCommand(mode)
}

// Halt powers off the display.
// After calling Halt, the display will not respond to further commands
// until the device is re-initialized.
func (d *Dev) Halt() error {
	d.halted = true
	return d.sendCommand(0xAE) // Display OFF
}

// String returns a string representation of the device.
func (d *Dev) String() string {
	return fmt.Sprintf("ssd1322.Dev{%dx%d}", d.rect.Dx(), d.rect.Dy())
}

// ScrollSpeed defines the horizontal scroll frame rate.
type ScrollSpeed byte

const (
	// Scroll frame rates (in display refresh cycles)
	Speed6Frames   ScrollSpeed = 0x00
	Speed10Frames  ScrollSpeed = 0x01
	Speed100Frames ScrollSpeed = 0x02
	Speed200Frames ScrollSpeed = 0x03
)

// ScrollHorizontal starts horizontal scrolling on the display.
// startRow and endRow specify the scroll region (must be >= 0 and < height).
// If right is true, scrolls right; otherwise scrolls left.
func (d *Dev) ScrollHorizontal(startRow, endRow byte, speed ScrollSpeed, right bool) error {
	if d.halted {
		return errors.New("ssd1322: halted")
	}

	if int(startRow) >= d.rect.Dy() || int(endRow) >= d.rect.Dy() {
		return errors.New("ssd1322: scroll row out of range")
	}

	// Select scroll direction command
	scrollCmd := byte(0x26) // Left
	if right {
		scrollCmd = 0x27 // Right
	}

	// Send scroll setup command
	return d.sendCommands([]byte{
		scrollCmd,
		0x00,        // Dummy byte (always 0x00)
		startRow,    // Start row
		byte(speed), // Scroll speed
		endRow,      // End row
		0x00, 0x00,  // Dummy bytes
		0x2F, // Activate scroll
	})
}

// StopScroll stops all scrolling and resets the display to normal operation.
func (d *Dev) StopScroll() error {
	if d.halted {
		return errors.New("ssd1322: halted")
	}
	return d.sendCommand(0x2E) // Deactivate scroll
}
