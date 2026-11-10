package sff

import "fmt"

// Resource caps for untrusted input. A .sff file (or a bare pixel buffer
// handed to a decoder) declares sizes in its own bytes; none of them is
// trusted before it is checked against one of these limits, so a corrupt or
// hostile file produces an error instead of a multi-gigabyte allocation.
//
//   - Pixel area: checkSpriteDimensions (SpritePixelDimensionLimit squared,
//     total area, never per axis: see resolve_sprite.go and
//     .vibe/decisions/016).
//   - Encoded data length (sprite pixel data, palette data):
//     MaxDataLength.
//   - Sprite/palette table entry count: MaxTableEntries.
const (
	// MaxDataLength is the largest declared encoded-data length, in bytes,
	// accepted for one sprite or palette. It is 4 bytes per pixel at the
	// pixel-area limit (the worst case, an uncompressed 32-bit sprite).
	MaxDataLength = SpritePixelDimensionLimit * SpritePixelDimensionLimit * 4

	// MaxTableEntries is the largest sprite or palette count accepted from
	// a file header. Real files hold at most a few thousand entries.
	MaxTableEntries = 1 << 18

	// tablePreallocEntries bounds how many table entries are preallocated
	// up front; larger tables grow on demand as entries are actually read.
	tablePreallocEntries = 1024
)

// checkDataLength rejects a declared encoded-data length above
// MaxDataLength (or negative).
func checkDataLength(what string, n int) error {
	if n < 0 || n > MaxDataLength {
		return fmt.Errorf("sff: %s declared length %d exceeds the %d-byte limit", what, n, MaxDataLength)
	}
	return nil
}

// checkTableCount rejects a declared table entry count above
// MaxTableEntries.
func checkTableCount(what string, n int) error {
	if n < 0 || n > MaxTableEntries {
		return fmt.Errorf("sff: %s count %d exceeds the %d-entry limit", what, n, MaxTableEntries)
	}
	return nil
}

// tableCap returns a safe initial capacity for a table of n declared entries.
func tableCap(n int) int {
	return min(n, tablePreallocEntries)
}
