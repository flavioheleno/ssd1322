# SSD1322 Go Driver Implementation Plan

## Overview

Build a Go driver for the SSD1322 4-bit grayscale OLED display controller, modeled after periph.io's SSD1306 driver. The SSD1322 differs from SSD1306 in three key ways: 4 bits per pixel (16 grayscale levels), horizontal nibble packing memory organization, and distinct command set.

## Requirements Summary

- **Protocol**: SPI 4-wire only (with DC pin)
- **Resolution**: Configurable (256x64 default, support 128x64 and others)
- **Features**: Full feature set (basic drawing, 4-bit grayscale, hardware scrolling, contrast/brightness)
- **Target**: General-purpose driver compatible with periph.io ecosystem

## File Structure

```
ssd1322/
├── doc.go                      # Package documentation with examples
├── ssd1322.go                  # Main driver implementation
├── ssd1322_test.go             # Unit tests
├── image4bit/                  # 4-bit grayscale image format subpackage
│   ├── doc.go
│   ├── image4bit.go
│   └── image4bit_test.go
└── examples/
    └── ssd1322_demo/
        └── main.go             # Demo program
```

## Critical Implementation Details

### 1. Image Format (image4bit package)

**Memory Layout**: SSD1322 uses horizontal nibble packing where each byte contains 2 pixels:
- High nibble = left pixel
- Low nibble = right pixel
- Example: byte 0x5A = pixel 0 with gray level 5, pixel 1 with gray level 10

**Color Model**: Create `Gray4` type (4-bit grayscale, 0-15 range) and `Gray4Model` for conversion from standard Go colors.

**Image Type**: `HorizontalNibble` struct implementing `image.Image` interface with:
- `Pix []byte` - raw pixel data (width * height / 2 bytes)
- `Stride int` - bytes per row (width / 2)
- Pixel access methods: `Gray4At()`, `Set()`

### 2. Main Driver (ssd1322.go)

**Dev struct**:
```go
type Dev struct {
    c            conn.Conn      // SPI connection
    dc           gpio.PinOut    // Data/Command pin
    rect         image.Rectangle
    columnOffset int            // (480 - width) / 2 for centering
    buffer       []byte         // Current frame buffer
    next         *image4bit.HorizontalNibble  // For differential updates
}
```

**Constructor**: `NewSPI(p spi.Port, dc gpio.PinOut, opts *Opts) (*Dev, error)`
- Validate opts (width must be even, ≤480; height ≤128)
- Configure SPI: 10MHz, Mode0, 8-bit
- Calculate column offset for centering
- Send initialization sequence
- Clear display RAM

**Opts struct**:
```go
type Opts struct {
    W           int   // Width (default: 256)
    H           int   // Height (default: 64)
    Rotated     bool  // 180° rotation
    Sequential  bool  // COM pin configuration
    SwapTopBottom bool
}
```

### 3. Initialization Sequence

Key commands to send in order:
1. `0xFD, 0x12` - Unlock commands
2. `0xAE` - Display OFF
3. `0xB3, 0xF2` - Clock divider
4. `0xCA, <height-1>` - MUX ratio (critical: depends on display height)
5. `0xA0, 0x14, 0x11` - Remap settings (adjust for rotation)
6. `0xC1, 0xFF` - Max contrast
7. `0xB9` - Default grayscale table
8. Clear RAM with zeros
9. `0xAF` - Display ON

**Column Addressing**: SSD1322 has 480-column RAM but displays are smaller. Use column offset: `(480 - width) / 2`. For 256px display, columns 0x1C-0x5B are used. Note: column addresses are in nibbles (divide pixel x by 2).

### 4. Display Interface Methods

**ColorModel()**: Return `image4bit.Gray4Model`

**Bounds()**: Return `d.rect`

**Draw(r image.Rectangle, src image.Image, sp image.Point)**:
1. Fast path: if src is already HorizontalNibble at full size, write directly
2. Slow path with differential updates:
   - Lazy-initialize `d.next` buffer
   - Use `draw.Draw()` to render src into buffer
   - Call `calculateDiff()` to find minimal changed region
   - Extract only changed bytes
   - Send to display via `writeRect()`
   - Copy buffer to d.buffer for next comparison

**Write(pixels []byte)**: Direct write of HorizontalNibble-formatted bytes

### 5. Differential Update Algorithm

**calculateDiff()**: Returns (minCol, maxCol, minRow, maxRow)
- Scan row by row comparing `d.buffer` vs `d.next.Pix`
- Track minimal bounding rectangle of changes
- Align column boundaries to byte boundaries (every 2 pixels)

**extractRegion()**: Extract rectangular subset of pixel data

**writeRect(x, y, width, height int, pixels []byte)**:
1. Calculate column addresses: `colStart = (x + columnOffset) / 2`, `colEnd = (x + width - 1 + columnOffset) / 2`
2. Send commands: `0x15, colStart, colEnd` (column address), `0x75, y, y+height-1` (row address)
3. Send command: `0x5C` (enable RAM write)
4. Send pixel data with DC pin HIGH

### 6. Communication Layer

**sendCommand(cmd byte)**:
- Set DC pin LOW
- Send via SPI

**sendData(data []byte)**:
- Set DC pin HIGH
- Send via SPI

**SPI Configuration**: Mode 0 (CPOL=0, CPHA=0), 10MHz default

### 7. Additional Features

**SetContrast(contrast byte)**: Send `0xC1, contrast` (0x00-0xFF)

**Invert(invert bool)**: Send `0xA6` (normal) or `0xA7` (inverse)

**Halt()**: Send `0xAE` (display OFF), set `d.halted = true`

**ScrollHorizontal(startRow, endRow byte, speed ScrollSpeed, right bool)**:
- Commands: `0x26` (left) or `0x27` (right)
- Parameters: start/end rows, speed (6/10/100/200 frames)
- Activate with `0x2F`

**StopScroll()**: Send `0x2E`

### 8. Key Commands Reference

```
Display:     0xAE (OFF), 0xAF (ON), 0xA6 (normal), 0xA7 (inverse)
Addressing:  0x15 (column), 0x75 (row), 0x5C (write RAM)
Config:      0xA0 (remap), 0xCA (MUX ratio), 0xFD (unlock)
Contrast:    0xC1 (contrast), 0xC7 (master contrast), 0xB9 (grayscale table)
Scrolling:   0x26 (left), 0x27 (right), 0x2E (stop), 0x2F (start)
```

## Implementation Sequence

### Phase 1: Image Format Foundation
1. Create `image4bit/image4bit.go` with Gray4 color model and HorizontalNibble image type
2. Write comprehensive tests in `image4bit/image4bit_test.go`
3. Verify nibble packing order manually with test patterns

### Phase 2: Basic Driver
1. Implement `ssd1322.go` core structure (Dev, Opts, NewSPI)
2. Implement initialization sequence
3. Implement basic `Write()` method
4. Test on hardware with solid colors and gradients

### Phase 3: Display Interface
1. Implement `ColorModel()`, `Bounds()`, `Draw()`
2. Add double buffering support
3. Test with Go's image package (draw rectangles, gradients)

### Phase 4: Differential Updates
1. Implement `calculateDiff()` and `extractRegion()`
2. Benchmark performance vs full-frame updates
3. Optimize if needed

### Phase 5: Advanced Features
1. Add scrolling methods
2. Add contrast and inversion control
3. Complete documentation in `doc.go`

### Phase 6: Testing & Examples
1. Create unit tests in `ssd1322_test.go`
2. Create demo program in `examples/ssd1322_demo/`
3. Test all 16 grayscale levels
4. Performance testing (target >30 FPS for partial updates)

## Testing Validation

- [ ] Display initializes without artifacts
- [ ] All 16 grayscale levels are distinct and visible
- [ ] Full-frame updates work correctly
- [ ] Differential updates minimize SPI traffic
- [ ] Contrast adjustment works (0-255 range)
- [ ] Inversion works (black↔white swap)
- [ ] Horizontal scrolling works
- [ ] Multiple resolutions work (128x64, 256x64)
- [ ] 180° rotation works
- [ ] Performance: >30 FPS for typical partial updates

## Potential Challenges & Solutions

**Challenge 1: Nibble order confusion** - If display shows scrambled pixels horizontally, toggle the nibble packing order in `pixOffset()` shift calculation.

**Challenge 2: Column offset incorrect** - If image is shifted or wrapped, verify column offset calculation. Try alternatives: 0, (480-width)/2, or consult display datasheet.

**Challenge 3: Grayscale not visible** - If only black/white shows, verify contrast settings and grayscale table initialization (0xB9 command). Some displays may need calibrated tables.

**Challenge 4: Performance issues** - Profile differential update algorithm, optimize byte comparisons, increase SPI speed if stable.

## Critical Files

1. **image4bit/image4bit.go** - Foundation for 4-bit pixel handling; must be implemented first
2. **ssd1322.go** - Core driver with all functionality
3. **doc.go** - Package documentation following periph.io conventions
4. **ssd1322_test.go** - Unit tests for addressing and differential updates
5. **examples/ssd1322_demo/main.go** - Hardware testing and demonstration

## Documentation

Package doc must include:
- Overview of SSD1322 capabilities
- Datasheet link
- Complete usage example
- Connection diagram (SPI wiring)
- Performance characteristics
- Known limitations

All exported types and methods need godoc with:
- Purpose and behavior
- Parameter constraints and valid ranges
- Return value descriptions
- Usage examples where helpful

## Success Criteria

✓ Driver follows periph.io patterns exactly (matches SSD1306 structure)
✓ Implements display.Drawer interface correctly
✓ Supports configurable resolutions
✓ 4-bit grayscale works with all 16 levels visible
✓ Differential updates provide significant performance improvement
✓ Hardware scrolling works smoothly
✓ Code is well-documented and tested
✓ Example program demonstrates all features
