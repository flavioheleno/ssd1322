// Package image4bit provides a 4-bit grayscale image format optimized for the SSD1322 display.
//
// The SSD1322 stores pixels in horizontal nibble packing where each byte contains 2 pixels.
// High nibble represents the left pixel, low nibble represents the right pixel.
// This package provides the Gray4 color type and HorizontalNibble image implementation.
package image4bit

import (
	"image"
	"image/color"
)

// Gray4 represents a 4-bit grayscale color (0-15 intensity levels).
// Only the lower 4 bits of Y are used.
type Gray4 struct {
	Y uint8
}

// RGBA converts the Gray4 color to standard RGBA.
// The 4-bit gray value (0-15) is scaled to 16-bit (0-65535).
func (c Gray4) RGBA() (r, g, b, a uint32) {
	// Scale 4-bit value (0-15) to 16-bit (0-65535)
	// 0xF * 0x1111 = 0xFFFF, 0x5 * 0x1111 = 0x5555, etc.
	y := uint32(c.Y&0x0F) * 0x1111
	return y, y, y, 0xFFFF
}

// toGray4 converts any color.Color to Gray4.
func toGray4(c color.Color) color.Color {
	if g, ok := c.(Gray4); ok {
		return g
	}
	r, g, b, _ := c.RGBA()
	// Standard grayscale conversion: 0.299R + 0.587G + 0.114B
	// RGBA returns 16-bit values, scale down result to 4-bit
	y := (299*r + 587*g + 114*b + 500) / 1000
	// Convert 16-bit (0-65535) to 4-bit (0-15)
	return Gray4{Y: uint8(y >> 12)}
}

// Gray4Model converts colors to Gray4.
var Gray4Model = color.ModelFunc(toGray4)

// HorizontalNibble is a 4-bit grayscale image where pixels are stored in horizontal nibble packing.
// Each byte contains 2 pixels: high nibble = left pixel, low nibble = right pixel.
type HorizontalNibble struct {
	Pix    []byte          // Pixel data (2 pixels per byte)
	Stride int             // Bytes per row
	Rect   image.Rectangle // Image bounds
}

// NewHorizontalNibble creates a new HorizontalNibble image with the specified bounds.
// The width must be even (since 2 pixels per byte).
func NewHorizontalNibble(r image.Rectangle) *HorizontalNibble {
	w, h := r.Dx(), r.Dy()
	if w < 0 || h < 0 {
		return &HorizontalNibble{Rect: r}
	}
	if w%2 != 0 {
		panic("image4bit: width must be even")
	}

	stride := w / 2
	pixelCount := stride * h
	return &HorizontalNibble{
		Pix:    make([]byte, pixelCount),
		Stride: stride,
		Rect:   r,
	}
}

// ColorModel returns the color model of the image.
func (p *HorizontalNibble) ColorModel() color.Model {
	return Gray4Model
}

// Bounds returns the image bounds.
func (p *HorizontalNibble) Bounds() image.Rectangle {
	return p.Rect
}

// At returns the color of the pixel at (x, y).
// It implements the image.Image interface.
func (p *HorizontalNibble) At(x, y int) color.Color {
	return p.Gray4At(x, y)
}

// Gray4At returns the Gray4 color of the pixel at (x, y).
func (p *HorizontalNibble) Gray4At(x, y int) Gray4 {
	if !(image.Point{X: x, Y: y}.In(p.Rect)) {
		return Gray4{}
	}
	offset, shift := p.pixOffset(x, y)
	return Gray4{Y: (p.Pix[offset] >> shift) & 0x0F}
}

// Set sets the color of the pixel at (x, y).
func (p *HorizontalNibble) Set(x, y int, c color.Color) {
	if !(image.Point{X: x, Y: y}.In(p.Rect)) {
		return
	}
	offset, shift := p.pixOffset(x, y)
	gray4 := Gray4Model.Convert(c).(Gray4)
	// Clear the nibble and set the new value
	p.Pix[offset] = (p.Pix[offset] &^ (0x0F << shift)) | ((gray4.Y & 0x0F) << shift)
}

// SetGray4 sets the Gray4 color of the pixel at (x, y).
// This is faster than Set() as it doesn't require color conversion.
func (p *HorizontalNibble) SetGray4(x, y int, c Gray4) {
	if !(image.Point{X: x, Y: y}.In(p.Rect)) {
		return
	}
	offset, shift := p.pixOffset(x, y)
	// Clear the nibble and set the new value
	p.Pix[offset] = (p.Pix[offset] &^ (0x0F << shift)) | ((c.Y & 0x0F) << shift)
}

// pixOffset returns the byte offset and bit shift for the pixel at (x, y).
// Memory layout: each byte contains 2 pixels horizontally.
// High nibble (shift 4) = even x (left pixel)
// Low nibble (shift 0) = odd x (right pixel)
func (p *HorizontalNibble) pixOffset(x, y int) (offset int, shift uint) {
	offset = (y-p.Rect.Min.Y)*p.Stride + (x-p.Rect.Min.X)/2
	// Even x (0, 2, 4...) uses high nibble (shift 4)
	// Odd x (1, 3, 5...) uses low nibble (shift 0)
	shift = uint(4 * (1 - (x & 1)))
	return
}
