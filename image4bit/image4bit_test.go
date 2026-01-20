package image4bit

import (
	"image"
	"image/color"
	"testing"
)

func TestGray4RGBA(t *testing.T) {
	tests := []struct {
		name string
		gray Gray4
		want uint32
	}{
		{"black", Gray4{Y: 0}, 0x0000},
		{"dark gray", Gray4{Y: 5}, 0x5555},
		{"mid gray", Gray4{Y: 8}, 0x8888},
		{"light gray", Gray4{Y: 10}, 0xAAAA},
		{"white", Gray4{Y: 15}, 0xFFFF},
		{"mask ignored", Gray4{Y: 0x5F}, 0xFFFF}, // Only lower 4 bits used (0x5F & 0x0F = 0x0F = 15)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, g, b, a := tt.gray.RGBA()
			if r != tt.want || g != tt.want || b != tt.want || a != 0xFFFF {
				t.Errorf("RGBA() = (%x, %x, %x, %x), want (%x, %x, %x, %x)",
					r, g, b, a, tt.want, tt.want, tt.want, uint32(0xFFFF))
			}
		})
	}
}

func TestGray4ModelConvert(t *testing.T) {
	tests := []struct {
		name  string
		input color.Color
		want  uint8
	}{
		{"gray4 passthrough", Gray4{Y: 7}, 7},
		{"black", color.Black, 0},
		{"white", color.White, 15},
		{"gray rgb", color.RGBA{0x88, 0x88, 0x88, 0xFF}, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Gray4Model.Convert(tt.input).(Gray4)
			if result.Y != tt.want {
				t.Errorf("Gray4Model.Convert(%v).Y = %d, want %d", tt.input, result.Y, tt.want)
			}
		})
	}
}

func TestNewHorizontalNibble(t *testing.T) {
	tests := []struct {
		name      string
		rect      image.Rectangle
		wantPanic bool
		wantW     int
		wantH     int
		wantStride int
		wantPixLen int
	}{
		{"256x64", image.Rect(0, 0, 256, 64), false, 256, 64, 128, 8192},
		{"128x64", image.Rect(0, 0, 128, 64), false, 128, 64, 64, 4096},
		{"4x2", image.Rect(0, 0, 4, 2), false, 4, 2, 2, 4},
		{"2x2", image.Rect(0, 0, 2, 2), false, 2, 2, 1, 2},
		{"offset rect", image.Rect(10, 20, 14, 22), false, 4, 2, 2, 4},
		{"odd width panics", image.Rect(0, 0, 5, 2), true, 0, 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if (r != nil) != tt.wantPanic {
					t.Errorf("panic = %v, want panic = %v", r != nil, tt.wantPanic)
				}
			}()

			img := NewHorizontalNibble(tt.rect)
			if !tt.wantPanic {
				if img.Rect != tt.rect {
					t.Errorf("Rect = %v, want %v", img.Rect, tt.rect)
				}
				if w := img.Rect.Dx(); w != tt.wantW {
					t.Errorf("width = %d, want %d", w, tt.wantW)
				}
				if h := img.Rect.Dy(); h != tt.wantH {
					t.Errorf("height = %d, want %d", h, tt.wantH)
				}
				if img.Stride != tt.wantStride {
					t.Errorf("Stride = %d, want %d", img.Stride, tt.wantStride)
				}
				if len(img.Pix) != tt.wantPixLen {
					t.Errorf("len(Pix) = %d, want %d", len(img.Pix), tt.wantPixLen)
				}
			}
		})
	}
}

func TestHorizontalNibbleNibblePacking(t *testing.T) {
	img := NewHorizontalNibble(image.Rect(0, 0, 4, 1))

	// Set four pixels with known values
	img.SetGray4(0, 0, Gray4{Y: 5})
	img.SetGray4(1, 0, Gray4{Y: 10})
	img.SetGray4(2, 0, Gray4{Y: 3})
	img.SetGray4(3, 0, Gray4{Y: 12})

	// Check byte layout: high nibble = even x, low nibble = odd x
	// Byte 0: pixel 0 (5) in high nibble, pixel 1 (10=A) in low nibble = 0x5A
	if img.Pix[0] != 0x5A {
		t.Errorf("Pix[0] = 0x%02X, want 0x5A", img.Pix[0])
	}
	// Byte 1: pixel 2 (3) in high nibble, pixel 3 (12=C) in low nibble = 0x3C
	if img.Pix[1] != 0x3C {
		t.Errorf("Pix[1] = 0x%02X, want 0x3C", img.Pix[1])
	}
}

func TestHorizontalNibbleSetGet(t *testing.T) {
	img := NewHorizontalNibble(image.Rect(0, 0, 4, 2))

	// Set test pattern
	testCases := [][4]uint8{
		{0, 1, 2, 3},
		{15, 14, 13, 12},
	}

	for y, row := range testCases {
		for x, val := range row {
			img.SetGray4(x, y, Gray4{Y: val})
		}
	}

	// Verify round-trip
	for y, row := range testCases {
		for x, wantVal := range row {
			result := img.Gray4At(x, y)
			if result.Y != wantVal {
				t.Errorf("Gray4At(%d, %d).Y = %d, want %d", x, y, result.Y, wantVal)
			}
		}
	}
}

func TestHorizontalNibbleAt(t *testing.T) {
	img := NewHorizontalNibble(image.Rect(0, 0, 2, 2))
	img.SetGray4(0, 0, Gray4{Y: 7})

	// Test At() interface
	c := img.At(0, 0)
	g, ok := c.(Gray4)
	if !ok {
		t.Errorf("At(0, 0) returned %T, want Gray4", c)
	}
	if g.Y != 7 {
		t.Errorf("At(0, 0).Y = %d, want 7", g.Y)
	}
}

func TestHorizontalNibbleSet(t *testing.T) {
	img := NewHorizontalNibble(image.Rect(0, 0, 2, 2))

	// Set with color.Color interface
	img.Set(0, 0, Gray4{Y: 9})
	result := img.Gray4At(0, 0)
	if result.Y != 9 {
		t.Errorf("After Set(0, 0, Gray4{9}), Gray4At(0, 0).Y = %d, want 9", result.Y)
	}

	// Convert from standard color
	img.Set(1, 0, color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}) // White
	result = img.Gray4At(1, 0)
	if result.Y != 15 {
		t.Errorf("After Set(1, 0, color.White), Gray4At(1, 0).Y = %d, want 15", result.Y)
	}
}

func TestHorizontalNibbleColorModel(t *testing.T) {
	img := NewHorizontalNibble(image.Rect(0, 0, 4, 4))
	if img.ColorModel() != Gray4Model {
		t.Error("ColorModel() did not return Gray4Model")
	}
}

func TestHorizontalNibbleBounds(t *testing.T) {
	rect := image.Rect(10, 20, 14, 24)
	img := NewHorizontalNibble(rect)
	if img.Bounds() != rect {
		t.Errorf("Bounds() = %v, want %v", img.Bounds(), rect)
	}
}

func TestHorizontalNibbleOutOfBounds(t *testing.T) {
	img := NewHorizontalNibble(image.Rect(0, 0, 4, 4))

	// Out of bounds reads should return zero
	result := img.Gray4At(-1, 0)
	if result.Y != 0 {
		t.Errorf("Gray4At(-1, 0).Y = %d, want 0 (out of bounds)", result.Y)
	}

	result = img.Gray4At(0, -1)
	if result.Y != 0 {
		t.Errorf("Gray4At(0, -1).Y = %d, want 0 (out of bounds)", result.Y)
	}

	result = img.Gray4At(4, 0)
	if result.Y != 0 {
		t.Errorf("Gray4At(4, 0).Y = %d, want 0 (out of bounds)", result.Y)
	}

	// Out of bounds writes should do nothing
	img.SetGray4(-1, 0, Gray4{Y: 15})
	img.SetGray4(0, -1, Gray4{Y: 15})
	img.SetGray4(4, 0, Gray4{Y: 15})

	result = img.Gray4At(-1, 0)
	if result.Y != 0 {
		t.Errorf("After out-of-bounds Set, Gray4At(-1, 0).Y = %d, want 0", result.Y)
	}
}

func TestHorizontalNibbleOffsetRect(t *testing.T) {
	// Test with offset rectangle (min != 0,0)
	rect := image.Rect(100, 50, 104, 52)
	img := NewHorizontalNibble(rect)

	// Set pixel at absolute coordinates
	img.SetGray4(100, 50, Gray4{Y: 11})

	// Verify read-back
	result := img.Gray4At(100, 50)
	if result.Y != 11 {
		t.Errorf("SetGray4(100, 50, 11) then Gray4At(100, 50).Y = %d, want 11", result.Y)
	}

	// Verify byte layout (0-based offset)
	if img.Pix[0]>>4 != 11 {
		t.Errorf("Pix[0]>>4 = %d, want 11", img.Pix[0]>>4)
	}
}

func TestHorizontalNibblePixOffset(t *testing.T) {
	img := NewHorizontalNibble(image.Rect(0, 0, 8, 2))

	tests := []struct {
		x, y   int
		offset int
		shift  uint
	}{
		// Row 0
		{0, 0, 0, 4}, // High nibble of byte 0
		{1, 0, 0, 0}, // Low nibble of byte 0
		{2, 0, 1, 4}, // High nibble of byte 1
		{3, 0, 1, 0}, // Low nibble of byte 1
		// Row 1
		{0, 1, 4, 4}, // High nibble of byte 4 (4 bytes per row)
		{1, 1, 4, 0}, // Low nibble of byte 4
	}

	for _, tt := range tests {
		offset, shift := img.pixOffset(tt.x, tt.y)
		if offset != tt.offset || shift != tt.shift {
			t.Errorf("pixOffset(%d, %d) = (%d, %d), want (%d, %d)",
				tt.x, tt.y, offset, shift, tt.offset, tt.shift)
		}
	}
}

func TestHorizontalNibbleNibbleMask(t *testing.T) {
	// Verify that only 4 bits are stored
	img := NewHorizontalNibble(image.Rect(0, 0, 2, 1))

	// Set with value that has high bits set
	img.SetGray4(0, 0, Gray4{Y: 0xF5}) // Only 0x5 should be stored
	result := img.Gray4At(0, 0)
	if result.Y != 0x5 {
		t.Errorf("SetGray4(0, 0, 0xF5) then Gray4At(0, 0).Y = 0x%X, want 0x5", result.Y)
	}
}

func TestHorizontalNibbleAllGrayLevels(t *testing.T) {
	// Test all 16 gray levels
	img := NewHorizontalNibble(image.Rect(0, 0, 16, 1))

	for level := uint8(0); level < 16; level++ {
		img.SetGray4(int(level), 0, Gray4{Y: level})
	}

	for level := uint8(0); level < 16; level++ {
		result := img.Gray4At(int(level), 0)
		if result.Y != level {
			t.Errorf("SetGray4(%d, 0, %d) then Gray4At(%d, 0).Y = %d, want %d",
				level, level, level, result.Y, level)
		}
	}
}
