package sff

import (
	"fmt"
	"image/color"
	"io"
)

// V1PaletteBlockSize is the fixed size, in bytes, of a .sff v1 sprite's
// embedded 256-color RGB palette block.
const V1PaletteBlockSize = 768

// Palette is a resolved 256-entry RGBA color table, indexed by a decoded
// sprite's palette index bytes (PCXImage.Pixels / V2Image.Pixels with
// BytesPerPixel: 1).
//
// It is kept separate from PCXImage/V2Image/Sprite — the read-path
// pure-data types — as an explicit opt-in helper, mirroring how DecodePCX
// and DecodeV2Sprite are already separate from Load; see
// .vibe/decisions/014-palette-resolution-api-shape.md.
type Palette [256]color.RGBA

// AlphaRule selects how ResolvePixels determines a resolved pixel's alpha
// channel at palette index 0. The reference decoders this package tracks
// apply one of two rules depending on the sprite's own encoding, not a
// single universal rule.
type AlphaRule int

const (
	// AlphaForceTransparentAtIndexZero forces palette index 0 to resolve to
	// fully transparent ((0,0,0,0)), regardless of the palette's own stored
	// value there. Used by PCX (v1) and PNG8 (v2) decoded pixel data.
	AlphaForceTransparentAtIndexZero AlphaRule = iota
	// AlphaLiteral uses the palette's own stored alpha value unmodified,
	// including at index 0. Used by RLE8/LZ5 (v2) decoded pixel data.
	AlphaLiteral
)

// ResolvePixels resolves a row-major palette-index pixel buffer — as
// decoded by DecodePCX or DecodeV2Sprite for an indexed pixel format
// (BytesPerPixel: 1) — against palette into a row-major buffer of final
// RGBA colors, one per index, applying rule to determine index 0's alpha.
func ResolvePixels(indices []byte, palette Palette, rule AlphaRule) []color.RGBA {
	pixels := make([]color.RGBA, len(indices))
	for i, idx := range indices {
		c := palette[idx]
		if rule == AlphaForceTransparentAtIndexZero && idx == 0 {
			c = color.RGBA{}
		}
		pixels[i] = c
	}
	return pixels
}

// DecodeV1Palette decodes a .sff v1 sprite's embedded palette block — 256
// RGB triplets (3 bytes/color), always opaque — into a Palette. data must
// be exactly V1PaletteBlockSize (768) bytes: the palette block itself, not
// the sprite's pixel data it follows (see ResolveV1Palette, which locates
// it within a sprite's raw file bytes).
func DecodeV1Palette(data []byte) (Palette, error) {
	if len(data) != V1PaletteBlockSize {
		return Palette{}, fmt.Errorf("sff: v1 palette block is %d bytes, want exactly %d", len(data), V1PaletteBlockSize)
	}

	var p Palette
	for i := range p {
		p[i] = color.RGBA{R: data[i*3], G: data[i*3+1], B: data[i*3+2], A: 255}
	}
	return p, nil
}

// ResolveV1Palette resolves the palette used by sprite index i in table,
// replicating the reference decoder's exact inheritance rule (see
// .vibe/decisions/017-v1-sprite-linking-and-palette-inheritance-rules.md):
// a sprite that does not share (SharedPalette == false) decodes its own
// embedded block; one that does share inherits table index 0's own
// resolved palette when it is itself (Group 0, Image 0), or the
// immediately preceding sprite's resolved palette otherwise. Table index 0
// always decodes its own block regardless of its own SharedPalette bit —
// there is no earlier sprite for it to inherit from.
//
// An owning sprite's embedded block is read via r from the last
// V1PaletteBlockSize bytes of its own declared [Offset, Offset+Length)
// span, not a suffix starting right after it: a v1 sprite's declared
// Length includes its own trailing palette block when it owns one,
// confirmed against real, unmodified .sff v1 files — see
// .vibe/decisions/018-v1-palette-block-lives-inside-declared-length.md.
//
// If override is non-nil, it is returned immediately instead — a caller-
// supplied external palette (see DecodeExternalPalette) used in place of
// the sprite's own, bypassing the table lookup entirely. See
// .vibe/decisions/016-external-palette-override-api-shape.md.
func ResolveV1Palette(table *V1SpriteTable, r io.ReaderAt, i int, override *Palette) (Palette, error) {
	if override != nil {
		return *override, nil
	}
	if i < 0 || i >= len(table.Sprites) {
		return Palette{}, fmt.Errorf("sff: v1 palette: sprite index %d out of range (have %d sprites)", i, len(table.Sprites))
	}
	return resolveV1Palette(table, r, i)
}

func resolveV1Palette(table *V1SpriteTable, r io.ReaderAt, i int) (Palette, error) {
	e := table.Sprites[i]

	if e.SharedPalette && i > 0 {
		if e.Group == 0 && e.Image == 0 {
			return resolveV1Palette(table, r, 0)
		}
		return resolveV1Palette(table, r, i-1)
	}

	if int64(e.Length) < V1PaletteBlockSize {
		return Palette{}, fmt.Errorf("sff: v1 palette: sprite %d: declared length %d is smaller than the %d-byte palette block it must contain", i, e.Length, V1PaletteBlockSize)
	}
	block := make([]byte, V1PaletteBlockSize)
	if _, err := r.ReadAt(block, e.Offset+int64(e.Length)-V1PaletteBlockSize); err != nil {
		return Palette{}, fmt.Errorf("sff: v1 palette: reading sprite %d's embedded palette block: %w", i, err)
	}
	return DecodeV1Palette(block)
}

// externalPaletteSize is the fixed size, in bytes, of an external .act
// palette file (256 RGB triplets, 3 bytes/color) — numerically the same as
// V1PaletteBlockSize, but named separately since it is a distinct on-disk
// concept (a standalone recolor file, not a sprite's own embedded palette).
const externalPaletteSize = 768

// DecodeExternalPalette decodes an external MUGEN/Ikemen .act palette file
// (256 RGB triplets, always opaque on disk) into a Palette, reproducing
// convertExternalPaletteToRGBA.mjs's two quirks: the file stores its 256
// triplets in reverse index order — the first triplet becomes palette index
// 255, the last becomes index 0 — and only the resulting index 0 is forced
// to fully transparent (alpha 0); every other entry stays opaque. data must
// be exactly externalPaletteSize (768) bytes.
func DecodeExternalPalette(data []byte) (Palette, error) {
	if len(data) != externalPaletteSize {
		return Palette{}, fmt.Errorf("sff: external .act palette is %d bytes, want exactly %d", len(data), externalPaletteSize)
	}

	var p Palette
	for i := 0; i < 256; i++ {
		p[255-i] = color.RGBA{R: data[i*3], G: data[i*3+1], B: data[i*3+2], A: 255}
	}
	p[0].A = 0
	return p, nil
}

// DecodeV2Palette decodes a .sff v2 palette bank's raw color data — already
// stored as RGBA on disk, 4 bytes/color, unlike v1's opaque-only RGB
// triplets — into a Palette. data's length must be a multiple of 4 and
// declare at most 256 colors; any entries beyond the declared color count
// are left at their zero value (fully transparent black).
func DecodeV2Palette(data []byte) (Palette, error) {
	if len(data)%4 != 0 {
		return Palette{}, fmt.Errorf("sff: v2 palette color data length %d is not a multiple of 4", len(data))
	}
	count := len(data) / 4
	if count > 256 {
		return Palette{}, fmt.Errorf("sff: v2 palette color data declares %d colors, more than the maximum 256", count)
	}

	var p Palette
	for i := 0; i < count; i++ {
		p[i] = color.RGBA{R: data[i*4], G: data[i*4+1], B: data[i*4+2], A: data[i*4+3]}
	}
	return p, nil
}

// ResolveV2Palette resolves the palette bank at index within table.Palettes
// to its final Palette: its own RGBA color data (read via r) when it
// stores one (Length > 0), or the colors of the bank its LinkedIndex points
// to otherwise — following the chain across further zero-length banks if
// needed. A link chain that is out of range or cycles back on itself
// returns a descriptive error instead of looping or panicking, mirroring
// v1 linked-sprite resolution (ResolveV1Pixels).
//
// If override is non-nil, it is returned immediately instead — a caller-
// supplied external palette (see DecodeExternalPalette) used in place of
// the bank's own, bypassing the table lookup entirely. See
// .vibe/decisions/016-external-palette-override-api-shape.md.
func ResolveV2Palette(table *V2SpriteTable, r io.ReaderAt, index int, override *Palette) (Palette, error) {
	if override != nil {
		return *override, nil
	}
	return resolveV2Palette(table, r, index, nil)
}

func resolveV2Palette(table *V2SpriteTable, r io.ReaderAt, index int, seen map[int]bool) (Palette, error) {
	if index < 0 || index >= len(table.Palettes) {
		return Palette{}, fmt.Errorf("sff: v2 palette: bank index %d out of range (have %d banks)", index, len(table.Palettes))
	}
	if seen == nil {
		seen = make(map[int]bool, len(table.Palettes))
	}
	if seen[index] {
		return Palette{}, fmt.Errorf("sff: v2 palette: linked bank chain cycles back to bank index %d", index)
	}
	seen[index] = true

	e := table.Palettes[index]
	if e.Length > 0 {
		raw := make([]byte, e.Length)
		if _, err := r.ReadAt(raw, e.Offset); err != nil {
			return Palette{}, fmt.Errorf("sff: v2 palette: reading bank %d color data: %w", index, err)
		}
		return DecodeV2Palette(raw)
	}

	return resolveV2Palette(table, r, e.LinkedIndex, seen)
}
