package sff

import (
	"encoding/binary"
	"fmt"
	"io"
	"sort"
)

// v1MaxPixelCount matches the reference decoder's own guard against
// corrupt PCX header dimensions: a declared width*height at or beyond this
// (the theoretical maximum for 16-bit-bounded PCX header fields) indicates
// unusable sprite metadata, not a real image — real .sff v1 sprites never
// approach this size. Falling back to a synthetic, fully-transparent 1x1
// image (rather than erroring, or attempting to allocate a pixel buffer
// sized from untrusted header bytes) matches the reference project's own
// behavior for this case; palette index 0 always resolves to fully
// transparent under AlphaForceTransparentAtIndexZero, so the specific
// fallback pixel value does not otherwise matter. See
// .vibe/decisions/017-v1-sprite-linking-and-palette-inheritance-rules.md.
const v1MaxPixelCount int64 = 65536 * 65536

// peekPCXDimensions reads a PCX image's declared width/height directly from
// its 128-byte header (the same four fields DecodePCX itself reads), without
// RLE-decoding any scanline data — cheap enough to call before deciding
// whether decoding the rest is worthwhile at all. ok is false if data is too
// short to contain a header or its bounds are inverted (DecodePCX will
// produce its own descriptive error for that case).
func peekPCXDimensions(data []byte) (width, height int, ok bool) {
	if len(data) < pcxHeaderSize {
		return 0, 0, false
	}
	xmin := int(binary.LittleEndian.Uint16(data[4:6]))
	ymin := int(binary.LittleEndian.Uint16(data[6:8]))
	xmax := int(binary.LittleEndian.Uint16(data[8:10]))
	ymax := int(binary.LittleEndian.Uint16(data[10:12]))
	if xmax < xmin || ymax < ymin {
		return 0, 0, false
	}
	return xmax - xmin + 1, ymax - ymin + 1, true
}

// signaturePeekSize is the number of leading bytes Load reads to identify a
// file: the 12-byte signature both .sff versions share, plus the 4 version
// bytes that distinguish them (see ParseV2's Version[3] check).
const signaturePeekSize = 16

// Load reads a full MUGEN/Ikemen GO .sff file from r — version 1 or version
// 2, auto-detected from the file's own signature and version bytes — and
// assembles it into the shared read-path SpriteGroup model, decoding
// whatever pixel data is needed to do so.
//
// v1's sprite table carries no width/height (see
// .vibe/decisions/004-sff-v1-table-is-a-separate-low-level-type.md): Load
// decodes each sprite's PCX pixel data to recover them, resolving a linked
// sprite (one with no pixel data of its own) to its link target first. A
// link chain that is out of range or cycles back on itself returns a
// descriptive error instead of looping or panicking. v1's table also only
// records whether a sprite reuses the previous sprite's palette, not a
// numeric reference, so Sprite.Palette is derived by incrementing a counter
// on every sprite that does not share, and having sharing sprites inherit
// the current value — see
// .vibe/decisions/010-def-loader-assembles-character-from-referenced-files.md.
//
// v2's sprite table already carries width/height and an explicit palette
// bank index, so each V2SpriteEntry maps directly to a Sprite without
// decoding any pixel data.
func Load(r io.ReaderAt) ([]SpriteGroup, error) {
	isV2, err := detectVersion(r)
	if err != nil {
		return nil, err
	}
	if isV2 {
		return loadV2(r)
	}
	return loadV1(r)
}

// detectVersion peeks at a .sff file's own signature and version bytes to
// tell v1 and v2 files apart, the same way Load and ResolveSpritePixels
// both need to before picking which version-specific parser to use.
func detectVersion(r io.ReaderAt) (isV2 bool, err error) {
	peek := make([]byte, signaturePeekSize)
	if _, err := r.ReadAt(peek, 0); err != nil {
		return false, fmt.Errorf("sff: reading file header: %w", err)
	}
	if sig := string(peek[0:12]); sig != v1Signature {
		return false, fmt.Errorf("sff: not a .sff file: unexpected signature %q", sig)
	}

	// verhi (the version's high byte) is stored at this offset and is 2
	// only for a v2 file — matches ParseV2's own check.
	return peek[15] == 2, nil
}

// loadV1 assembles a v1 .sff file's sprite table into SpriteGroups,
// decoding each sprite's PCX pixel data to recover its dimensions and
// deriving a Palette value from the table's per-sprite palette-sharing
// flag.
func loadV1(r io.ReaderAt) ([]SpriteGroup, error) {
	table, err := ParseV1(r)
	if err != nil {
		return nil, err
	}

	sprites := make([]Sprite, len(table.Sprites))
	palette := -1
	for i, e := range table.Sprites {
		if !e.SharedPalette || palette == -1 {
			palette++
		}

		img, err := resolveV1Pixels(table, r, i, nil)
		if err != nil {
			return nil, fmt.Errorf("sff: sprite %d (group %d, image %d): %w", i, e.Group, e.Image, err)
		}

		sprites[i] = Sprite{
			Group:   e.Group,
			Image:   e.Image,
			Width:   img.Width,
			Height:  img.Height,
			AxisX:   e.AxisX,
			AxisY:   e.AxisY,
			Palette: palette,
		}
	}

	return groupSprites(sprites), nil
}

// ResolveV1Pixels resolves sprite index i's actual decoded PCX pixel data —
// following v1's sprite-linking rule so a caller (e.g. a future editor
// wanting a sprite's real image, not just its dimensions the way loadV1
// uses this internally) gets the same result Load itself would derive
// dimensions from, without having to reimplement the linking rule.
// Mirrors ResolveV1Palette's public shape.
func ResolveV1Pixels(table *V1SpriteTable, r io.ReaderAt, i int) (*PCXImage, error) {
	if i < 0 || i >= len(table.Sprites) {
		return nil, fmt.Errorf("sff: v1 pixels: sprite index %d out of range (have %d sprites)", i, len(table.Sprites))
	}
	return resolveV1Pixels(table, r, i, nil)
}

// resolveV1Pixels returns the decoded PCX pixel data belonging to sprite
// index i in table, replicating the reference decoder's exact linking rule
// (see .vibe/decisions/017-v1-sprite-linking-and-palette-inheritance-rules.md):
// a sprite with no pixel data of its own (Length == 0) always inherits the
// immediately preceding table entry's already-resolved image — its own
// LinkedIndex field, despite being stored, is not consulted for this case.
// A sprite that does carry its own pixel data (Length > 0) uses it as-is
// unless its own LinkedIndex is a genuine backward reference (strictly less
// than i, and not 0): a self- or forward-reference (LinkedIndex >= i) is
// treated as "no link" and falls back to the sprite's own data, the same
// as a LinkedIndex of exactly 0 always does (0 can never be a real link
// target through this field — matching the reference decoder, an
// unavoidable format quirk, not a bug in this package).
//
// seen tracks indices already visited on the current chain, so a link
// cycle is reported as an error rather than recursing forever; pass nil on
// the initial call. Every recursive call here strictly decreases the
// index, so a cycle is not actually reachable, but the guard is kept for
// defense in depth and to give a descriptive error instead of an
// unbounded/panicking read if that invariant is ever wrong.
func resolveV1Pixels(table *V1SpriteTable, r io.ReaderAt, i int, seen map[int]bool) (*PCXImage, error) {
	if seen == nil {
		seen = make(map[int]bool, len(table.Sprites))
	}
	if seen[i] {
		return nil, fmt.Errorf("linked sprite chain cycles back to sprite index %d", i)
	}
	seen[i] = true

	e := table.Sprites[i]
	if e.Length == 0 {
		if i == 0 {
			return nil, fmt.Errorf("sprite 0 has no pixel data of its own and no earlier sprite to inherit from")
		}
		return resolveV1Pixels(table, r, i-1, seen)
	}

	linked := e.LinkedIndex
	if linked >= i {
		linked = 0
	}
	if linked != 0 {
		if linked < 0 || linked >= len(table.Sprites) {
			return nil, fmt.Errorf("links to out-of-range sprite index %d", linked)
		}
		return resolveV1Pixels(table, r, linked, seen)
	}

	// Own data: a sprite that owns its palette (SharedPalette == false)
	// declares Length inclusive of its own trailing palette block — see
	// ResolveV1Palette — so the real pixel-only byte count is shorter than
	// Length by that fixed size.
	pixelLen := int64(e.Length)
	if !e.SharedPalette {
		pixelLen -= V1PaletteBlockSize
		if pixelLen < 0 {
			return nil, fmt.Errorf("sprite %d: declared length %d is smaller than the %d-byte palette block it must contain", i, e.Length, V1PaletteBlockSize)
		}
	}

	raw := make([]byte, pixelLen)
	if _, err := r.ReadAt(raw, e.Offset); err != nil {
		return nil, fmt.Errorf("reading pixel data: %w", err)
	}
	if w, h, ok := peekPCXDimensions(raw); ok && int64(w)*int64(h) >= v1MaxPixelCount {
		return &PCXImage{Width: 1, Height: 1, Pixels: []byte{0}}, nil
	}
	return DecodePCX(raw)
}

// loadV2 assembles a v2 .sff file's sprite table into SpriteGroups. Every
// field a Sprite needs is already present in a V2SpriteEntry, so no pixel
// data is decoded.
func loadV2(r io.ReaderAt) ([]SpriteGroup, error) {
	table, err := ParseV2(r)
	if err != nil {
		return nil, err
	}

	sprites := make([]Sprite, len(table.Sprites))
	for i, e := range table.Sprites {
		sprites[i] = Sprite{
			Group:   e.Group,
			Image:   e.Image,
			Width:   e.Width,
			Height:  e.Height,
			AxisX:   e.AxisX,
			AxisY:   e.AxisY,
			Palette: e.PaletteIndex,
		}
	}

	return groupSprites(sprites), nil
}

// groupSprites buckets sprites by Group into SpriteGroup values, preserving
// each sprite's relative order within its group, and returns the groups
// sorted by ascending group index.
func groupSprites(sprites []Sprite) []SpriteGroup {
	var order []int
	byGroup := make(map[int][]Sprite)
	for _, s := range sprites {
		if _, ok := byGroup[s.Group]; !ok {
			order = append(order, s.Group)
		}
		byGroup[s.Group] = append(byGroup[s.Group], s)
	}

	sort.Ints(order)
	groups := make([]SpriteGroup, len(order))
	for i, g := range order {
		groups[i] = SpriteGroup{Index: g, Sprites: byGroup[g]}
	}
	return groups
}
