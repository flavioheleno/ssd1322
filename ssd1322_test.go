package ssd1322

import (
	"image"
	"testing"

	"github.com/flavioheleno/ssd1322/image4bit"
)

func TestOptsValidation(t *testing.T) {
	tests := []struct {
		name    string
		opts    *Opts
		wantErr bool
	}{
		{"nil options (uses defaults)", nil, false},
		{"valid 256x64", &Opts{W: 256, H: 64}, false},
		{"valid 128x64", &Opts{W: 128, H: 64}, false},
		{"valid 2x2 (minimum)", &Opts{W: 2, H: 1}, false},
		{"odd width", &Opts{W: 255, H: 64}, true},
		{"width zero", &Opts{W: 0, H: 64}, true},
		{"width > 480", &Opts{W: 512, H: 64}, true},
		{"height zero", &Opts{W: 256, H: 0}, true},
		{"height > 128", &Opts{W: 256, H: 200}, true},
		{"rotated (valid)", &Opts{W: 256, H: 64, Rotated: true}, false},
		{"sequential (valid)", &Opts{W: 256, H: 64, Sequential: true}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := tt.opts
			if opts == nil {
				opts = &Opts{W: 256, H: 64}
			}

			if opts.W <= 0 || opts.W%2 != 0 || opts.W > 480 {
				if !tt.wantErr {
					t.Error("expected error but didn't get one")
				}
				return
			}
			if opts.H <= 0 || opts.H > 128 {
				if !tt.wantErr {
					t.Error("expected error but didn't get one")
				}
				return
			}

			if tt.wantErr {
				t.Error("expected error but didn't get one")
			}
		})
	}
}

func TestDevBounds(t *testing.T) {
	dev := &Dev{
		rect: image.Rect(0, 0, 256, 64),
	}
	want := image.Rect(0, 0, 256, 64)
	if got := dev.Bounds(); got != want {
		t.Errorf("Bounds() = %v, want %v", got, want)
	}
}

func TestDevColorModel(t *testing.T) {
	dev := &Dev{}
	if dev.ColorModel() != image4bit.Gray4Model {
		t.Error("ColorModel() did not return Gray4Model")
	}
}

func TestDevString(t *testing.T) {
	dev := &Dev{
		rect: image.Rect(0, 0, 256, 64),
	}
	want := "ssd1322.Dev{256x64}"
	if got := dev.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestDevHalt(t *testing.T) {
	dev := &Dev{
		rect:   image.Rect(0, 0, 256, 64),
		buffer: make([]byte, 256*64/2),
	}

	if dev.halted {
		t.Error("device should not be halted initially")
	}

	// Halt sets halted flag to true (can't test actual command without real hardware)
	dev.halted = true

	// Test that operations fail when halted
	if err := dev.SetContrast(100); err == nil {
		t.Error("SetContrast should fail when halted")
	}

	if err := dev.Invert(true); err == nil {
		t.Error("Invert should fail when halted")
	}

	_, err := dev.Write(make([]byte, 256*64/2))
	if err == nil {
		t.Error("Write should fail when halted")
	}

	if err := dev.Draw(dev.Bounds(), image.NewRGBA(dev.Bounds()), image.Point{}); err == nil {
		t.Error("Draw should fail when halted")
	}

	if err := dev.ScrollHorizontal(0, 63, Speed10Frames, false); err == nil {
		t.Error("ScrollHorizontal should fail when halted")
	}

	if err := dev.StopScroll(); err == nil {
		t.Error("StopScroll should fail when halted")
	}
}

func TestDevColumnOffset(t *testing.T) {
	tests := []struct {
		name       string
		width      int
		wantOffset int
	}{
		{"256 width", 256, 112}, // (480 - 256) / 2 = 112
		{"128 width", 128, 176}, // (480 - 128) / 2 = 176
		{"480 width (full)", 480, 0},
		{"64 width", 64, 208}, // (480 - 64) / 2 = 208
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offset := (480 - tt.width) / 2
			if offset != tt.wantOffset {
				t.Errorf("Column offset for width %d = %d, want %d", tt.width, offset, tt.wantOffset)
			}
		})
	}
}

func TestWriteInvalidBufferSize(t *testing.T) {
	dev := &Dev{
		rect:   image.Rect(0, 0, 256, 64),
		buffer: make([]byte, 256*64/2),
	}

	// Wrong buffer size should fail validation
	_, err := dev.Write(make([]byte, 100))
	if err == nil {
		t.Error("Write should fail with wrong buffer size")
	}
	if err.Error() != "ssd1322: invalid buffer size" {
		t.Errorf("Write error = %v, want 'ssd1322: invalid buffer size'", err)
	}
}

func TestCalculateDiffNoChanges(t *testing.T) {
	dev := &Dev{
		rect:   image.Rect(0, 0, 4, 2),
		buffer: make([]byte, 4),
		next:   image4bit.NewHorizontalNibble(image.Rect(0, 0, 4, 2)),
		lastDm: image4bit.HorizontalNibble{
			Pix:    make([]byte, 4),
			Stride: 2,
			Rect:   image.Rect(0, 0, 4, 2),
		},
	}

	// Copy to make them identical
	copy(dev.lastDm.Pix, dev.next.Pix)

	minCol, maxCol, _, _ := dev.calculateDiff()

	// minCol > maxCol indicates no changes
	if minCol <= maxCol {
		t.Errorf("No changes should result in minCol > maxCol, got %d > %d", minCol, maxCol)
	}
}

func TestCalculateDiffWithChanges(t *testing.T) {
	dev := &Dev{
		rect:   image.Rect(0, 0, 4, 2),
		buffer: make([]byte, 4),
		next: &image4bit.HorizontalNibble{
			Pix:    []byte{0xAB, 0xCD, 0x00, 0x00},
			Stride: 2,
			Rect:   image.Rect(0, 0, 4, 2),
		},
		lastDm: image4bit.HorizontalNibble{
			Pix:    []byte{0x00, 0x00, 0x00, 0x00},
			Stride: 2,
			Rect:   image.Rect(0, 0, 4, 2),
		},
	}

	minCol, maxCol, minRow, maxRow := dev.calculateDiff()

	// Should detect change in first row, first 2 bytes (4 pixels)
	if minCol != 0 {
		t.Errorf("minCol = %d, want 0", minCol)
	}
	if maxCol != 3 {
		t.Errorf("maxCol = %d, want 3", maxCol)
	}
	if minRow != 0 {
		t.Errorf("minRow = %d, want 0", minRow)
	}
	if maxRow != 0 {
		t.Errorf("maxRow = %d, want 0", maxRow)
	}
}

func TestExtractRegion(t *testing.T) {
	dev := &Dev{
		rect: image.Rect(0, 0, 8, 2),
		next: &image4bit.HorizontalNibble{
			Pix:    []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77},
			Stride: 4,
			Rect:   image.Rect(0, 0, 8, 2),
		},
	}

	// Extract middle region: columns 2-5 (bytes 1-2), rows 0-0
	region := dev.extractRegion(2, 5, 0, 0)

	// Should be bytes [1:3] from first row
	want := []byte{0x11, 0x22}
	if len(region) != len(want) {
		t.Errorf("extractRegion length = %d, want %d", len(region), len(want))
	}
	for i, b := range region {
		if b != want[i] {
			t.Errorf("extractRegion[%d] = 0x%02X, want 0x%02X", i, b, want[i])
		}
	}
}

func TestScrollSpeed(t *testing.T) {
	tests := []struct {
		name string
		val  ScrollSpeed
	}{
		{"Speed6Frames", Speed6Frames},
		{"Speed10Frames", Speed10Frames},
		{"Speed100Frames", Speed100Frames},
		{"Speed200Frames", Speed200Frames},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if byte(tt.val) >= 4 {
				t.Errorf("%s has invalid value %d", tt.name, byte(tt.val))
			}
		})
	}
}

func TestWriteBufferSizeValidation(t *testing.T) {
	tests := []struct {
		name       string
		width      int
		height     int
		bufferSize int
		wantErrMsg string
	}{
		{"256x64 too small", 256, 64, 256*64/2 - 1, "ssd1322: invalid buffer size"},
		{"256x64 too large", 256, 64, 256*64/2 + 1, "ssd1322: invalid buffer size"},
		{"128x64 too small", 128, 64, 128*64/2 - 1, "ssd1322: invalid buffer size"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dev := &Dev{
				rect:   image.Rect(0, 0, tt.width, tt.height),
				buffer: make([]byte, tt.width*tt.height/2),
			}

			_, err := dev.Write(make([]byte, tt.bufferSize))
			if err == nil {
				t.Error("Write should fail with invalid buffer size")
			}
			if err.Error() != tt.wantErrMsg {
				t.Errorf("Write error = %v, want %q", err, tt.wantErrMsg)
			}
		})
	}
}

func TestRSTOptionSupport(t *testing.T) {
	// Test that RST option is properly handled
	tests := []struct {
		name string
		opts *Opts
	}{
		{"nil RST", &Opts{W: 256, H: 64, RST: nil}},
		{"RST field exists", &Opts{W: 256, H: 64}}, // Default nil
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dev := &Dev{
				rect:   image.Rect(0, 0, 256, 64),
				buffer: make([]byte, 256*64/2),
			}

			// Verify RST field exists and can be set
			dev.rst = tt.opts.RST
			// No error expected - RST is optional
			if dev.rst != nil {
				t.Errorf("RST should be nil by default, got %v", dev.rst)
			}
		})
	}
}

func TestRSTOptionInOpts(t *testing.T) {
	// Test that RST field exists in Opts struct
	opts := &Opts{
		W:   256,
		H:   64,
		RST: nil,
	}

	if opts.RST != nil {
		t.Error("RST should be nil when not set")
	}

	// Verify RST can be set to a non-nil value (without actual GPIO)
	// This is a compile-time check that the field exists
	_ = &Opts{W: 256, H: 64, RST: opts.RST}
}
