package sff

import (
	"fmt"
	"image/color"
	"io"
)

// SpritePixelDimensionLimit guards ResolveSpritePixels against a corrupt or
// adversarial .sff file declaring an implausibly large sprite: unlike the
// rest of this package's decode path, ResolveSpritePixels is reachable
// directly from untrusted caller-supplied bytes (the WASM boundary, see
// cmd/wasm and .vibe/decisions/020-wasm-sprite-pixel-resolution-batched-stateless-contract.md).
// 4096 per axis comfortably exceeds every known real MUGEN/Ikemen sprite
// (typically well under 1024x1024).
const SpritePixelDimensionLimit = 4096

// ResolveSpritePixels resolves the sprite identified by (group, image)
// within a .sff file — version 1 or 2, auto-detected the same way Load
// is — to its final on-screen RGBA pixels: decoding its pixel data,
// resolving its palette (or using override in its place, if non-nil), and
// applying that palette. override is ignored for a v2 direct-color sprite
// (PNG24/PNG32), which has no palette of its own to replace.
//
// Returns a descriptive error, prefixed "sprite not found: " specifically
// when no sprite matches (group, image) — distinct from every other
// failure (malformed file, corrupt pixel/palette data, an implausibly
// large declared size) — so a caller iterating candidate sprites (e.g.
// browsing a sheet) can tell "doesn't exist" apart from "something is
// broken" without parsing the rest of the message. See
// .vibe/decisions/020-wasm-sprite-pixel-resolution-batched-stateless-contract.md.
//
// A v2 sprite that shares (rather than owns) its pixel data is not yet
// supported and returns a descriptive error: no v2 linked-sprite decode
// path has been validated against real files anywhere in this package yet
// (see .vibe/backlog/029-fixture-driven-v2-sprite-test-suite.md, still
// open) — reporting a clear error here is preferred over guessing at the
// linking rule the way v1's own rule needed correcting once real-file
// testing (item 028) checked it.
func ResolveSpritePixels(r io.ReaderAt, group, image int, override *Palette) ([]color.RGBA, int, int, error) {
	isV2, err := detectVersion(r)
	if err != nil {
		return nil, 0, 0, err
	}
	if isV2 {
		return resolveSpritePixelsV2(r, group, image, override)
	}
	return resolveSpritePixelsV1(r, group, image, override)
}

func resolveSpritePixelsV1(r io.ReaderAt, group, image int, override *Palette) ([]color.RGBA, int, int, error) {
	table, err := ParseV1(r)
	if err != nil {
		return nil, 0, 0, err
	}
	i, ok := table.Index(group, image)
	if !ok {
		return nil, 0, 0, fmt.Errorf("sprite not found: no sprite at group %d, image %d", group, image)
	}

	img, err := ResolveV1Pixels(table, r, i)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("sff: resolving sprite pixels: %w", err)
	}
	if err := checkSpriteDimensions(img.Width, img.Height); err != nil {
		return nil, 0, 0, err
	}

	pal, err := ResolveV1Palette(table, r, i, override)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("sff: resolving sprite palette: %w", err)
	}

	return ResolvePixels(img.Pixels, pal, AlphaForceTransparentAtIndexZero), img.Width, img.Height, nil
}

func resolveSpritePixelsV2(r io.ReaderAt, group, image int, override *Palette) ([]color.RGBA, int, int, error) {
	table, err := ParseV2(r)
	if err != nil {
		return nil, 0, 0, err
	}
	i, ok := table.Index(group, image)
	if !ok {
		return nil, 0, 0, fmt.Errorf("sprite not found: no sprite at group %d, image %d", group, image)
	}
	e := table.Sprites[i]
	if e.Length == 0 {
		return nil, 0, 0, fmt.Errorf("sff: sprite (group %d, image %d): linked/shared v2 sprite pixel data is not yet supported by ResolveSpritePixels", group, image)
	}
	if err := checkSpriteDimensions(e.Width, e.Height); err != nil {
		return nil, 0, 0, err
	}

	raw := make([]byte, e.Length)
	if _, err := r.ReadAt(raw, e.Offset); err != nil {
		return nil, 0, 0, fmt.Errorf("sff: reading sprite pixel data: %w", err)
	}
	img, err := DecodeV2Sprite(e.Format, e.Width, e.Height, e.ColorDepth, raw)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("sff: decoding sprite pixel data: %w", err)
	}

	// A direct-color sprite (PNG24/PNG32) already carries its own colors —
	// no palette bank/override applies, since there is no palette to
	// replace.
	if img.BytesPerPixel != 1 {
		return directColorPixelsToRGBA(img), img.Width, img.Height, nil
	}

	pal, err := ResolveV2Palette(table, r, e.PaletteIndex, override)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("sff: resolving sprite palette: %w", err)
	}

	// RLE8/LZ5-decoded sprites use literal alpha; every other indexed
	// format (raw, PNG8) forces index 0 fully transparent — same rule v1's
	// PCX-decoded sprites use. See ResolvePixels's own AlphaRule doc.
	rule := AlphaForceTransparentAtIndexZero
	if e.Format == V2FormatRLE8 || e.Format == V2FormatLZ5 {
		rule = AlphaLiteral
	}
	return ResolvePixels(img.Pixels, pal, rule), img.Width, img.Height, nil
}

// directColorPixelsToRGBA converts a decoded direct-color V2Image (24-bit
// RGB or 32-bit RGBA, per img.BytesPerPixel) into the same row-major
// []color.RGBA shape ResolvePixels produces for indexed sprites, so both
// paths return one common shape to their caller.
func directColorPixelsToRGBA(img *V2Image) []color.RGBA {
	n := img.Width * img.Height
	out := make([]color.RGBA, n)
	for i := 0; i < n; i++ {
		switch img.BytesPerPixel {
		case 4:
			out[i] = color.RGBA{R: img.Pixels[i*4], G: img.Pixels[i*4+1], B: img.Pixels[i*4+2], A: img.Pixels[i*4+3]}
		case 3:
			out[i] = color.RGBA{R: img.Pixels[i*3], G: img.Pixels[i*3+1], B: img.Pixels[i*3+2], A: 255}
		}
	}
	return out
}

// checkSpriteDimensions rejects a declared sprite size beyond
// SpritePixelDimensionLimit per axis, so a corrupt or adversarial file
// produces a reported error instead of an attempt to allocate an
// implausibly large pixel buffer.
func checkSpriteDimensions(width, height int) error {
	if width > SpritePixelDimensionLimit || height > SpritePixelDimensionLimit {
		return fmt.Errorf("sff: declared sprite size %dx%d exceeds the %dx%d limit", width, height, SpritePixelDimensionLimit, SpritePixelDimensionLimit)
	}
	return nil
}
