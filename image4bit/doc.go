// Package image4bit provides a 4-bit grayscale image format for the SSD1322 display controller.
//
// The SSD1322 OLED controller uses 4-bit grayscale (16 intensity levels from 0-15).
// Pixels are stored in horizontal nibble packing where each byte contains 2 pixels.
//
// Memory layout example for a 4-pixel row:
//
//	Pixels: 0  1  2  3
//	Values: 5  10 3  12
//	Bytes:  0x5A     0x3C
//	        (0x5A = high nibble: 5, low nibble: A=10)
//	        (0x3C = high nibble: 3, low nibble: C=12)
//
// This package provides:
//
// - Gray4: A color type representing 4-bit grayscale (0-15)
// - Gray4Model: A color model for converting standard Go colors to Gray4
// - HorizontalNibble: An image.Image implementation optimized for SSD1322
//
// Example usage:
//
//	// Create a 256x64 image
//	img := image4bit.NewHorizontalNibble(image.Rect(0, 0, 256, 64))
//
//	// Set a pixel to gray level 8
//	img.SetGray4(10, 20, image4bit.Gray4{Y: 8})
//
//	// Get a pixel
//	gray := img.Gray4At(10, 20)
//	println(gray.Y)  // Output: 8
//
//	// Use with standard Go image operations
//	draw.Draw(img, img.Bounds(), image.NewUniform(image4bit.Gray4{Y: 15}), image.Point{}, draw.Src)
package image4bit
