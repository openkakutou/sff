package sff

// Tests for ResolveSpritePixels (backlog item 034): resolving one sprite's
// final on-screen RGBA pixels directly from a .sff file's raw bytes,
// combining what ParseV1/ParseV2, ResolveV1Pixels/DecodeV2Sprite, and
// ResolveV1Palette/ResolveV2Palette already do individually — needed so
// character-viewer-web can render real sprite images, not just metadata
// (.vibe/decisions/019-...md deferred this; .vibe/decisions/020-...md
// designs this function's contract). Reuses the same real fixtures and
// expected reference PNGs as the v1 fixture-driven suite (item 028) so
// results are checked against ground truth, not just "didn't error".

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestResolveSpritePixels_V1MatchesReferencePNG exercises the v1 path
// end-to-end against a real fixture, reusing v1-basic.sff and its expected
// output the same way TestV1Fixtures_DecodedPixelsMatchReferencePNGs does.
func TestResolveSpritePixels_V1MatchesReferencePNG(t *testing.T) {
	f := openTestdataFile(t, "v1-basic.sff")
	defer f.Close()

	table, err := ParseV1(f)
	if err != nil {
		t.Fatalf("ParseV1: %v", err)
	}
	target := table.Sprites[len(table.Sprites)-1]

	got, width, height, err := ResolveSpritePixels(f, target.Group, target.Image, nil)
	if err != nil {
		t.Fatalf("ResolveSpritePixels: %v", err)
	}

	want := decodeExpectedPNG(t, "v1-sprite-001.png")
	assertPixelsMatch(t, got, width, height, want)
}

// TestResolveSpritePixels_V1WithExternalPaletteOverride mirrors the
// "ExternalPalette" v1 fixture scenario: supplying an override must
// actually change the resolved colors to the override's, not the sprite's
// own.
func TestResolveSpritePixels_V1WithExternalPaletteOverride(t *testing.T) {
	f := openTestdataFile(t, "v1-external-palette-source.sff")
	defer f.Close()

	table, err := ParseV1(f)
	if err != nil {
		t.Fatalf("ParseV1: %v", err)
	}
	target := table.Sprites[len(table.Sprites)-1]

	actData, err := os.ReadFile(filepath.Join("testdata", "files", "cyclops-v1-palette1.act"))
	if err != nil {
		t.Fatalf("reading external palette: %v", err)
	}
	override, err := DecodeExternalPalette(actData)
	if err != nil {
		t.Fatalf("DecodeExternalPalette: %v", err)
	}

	got, width, height, err := ResolveSpritePixels(f, target.Group, target.Image, &override)
	if err != nil {
		t.Fatalf("ResolveSpritePixels: %v", err)
	}

	want := decodeExpectedPNG(t, "v1-sprite-007.png")
	assertPixelsMatch(t, got, width, height, want)
}

// TestResolveSpritePixels_V2IndexedMatchesReferencePNG exercises the v2
// path for an indexed (palette-based) sprite against a real fixture.
func TestResolveSpritePixels_V2IndexedMatchesReferencePNG(t *testing.T) {
	cases := []struct {
		name string
		sff  string
		png  string
	}{
		{"RLE8", "v2-empty-palette-use-first.sff", "v2-sprite-009.png"},
		{"LZ5", "v2-lz5.sff", "v2-sprite-002.png"},
		{"PNG8", "v2-png8.sff", "v2-sprite-003.png"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := openTestdataFile(t, c.sff)
			defer f.Close()

			table, err := ParseV2(f)
			if err != nil {
				t.Fatalf("ParseV2: %v", err)
			}
			target := table.Sprites[len(table.Sprites)-1]

			got, width, height, err := ResolveSpritePixels(f, target.Group, target.Image, nil)
			if err != nil {
				t.Fatalf("ResolveSpritePixels: %v", err)
			}

			want := decodeExpectedPNG(t, c.png)
			assertPixelsMatch(t, got, width, height, want)
		})
	}
}

// TestResolveSpritePixels_V2DirectColorMatchesReferencePNG exercises the v2
// path for a direct-color (PNG24/32, no palette) sprite.
func TestResolveSpritePixels_V2DirectColorMatchesReferencePNG(t *testing.T) {
	cases := []struct {
		name string
		sff  string
		png  string
	}{
		{"PNG24", "v2-png24.sff", "v2-sprite-004.png"},
		{"PNG32", "v2-png32.sff", "v2-sprite-005.png"}, // alpha-premultiplied, per decodeV2PNG's V2FormatPNG32 case
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := openTestdataFile(t, c.sff)
			defer f.Close()

			table, err := ParseV2(f)
			if err != nil {
				t.Fatalf("ParseV2: %v", err)
			}
			target := table.Sprites[len(table.Sprites)-1]

			got, width, height, err := ResolveSpritePixels(f, target.Group, target.Image, nil)
			if err != nil {
				t.Fatalf("ResolveSpritePixels: %v", err)
			}

			want := decodeExpectedPNG(t, c.png)
			assertPixelsMatch(t, got, width, height, want)
		})
	}
}

// TestResolveSpritePixels_SpriteNotFound asserts the "not found" error is
// distinguishable (a stable "sprite not found: " prefix, per
// .vibe/decisions/020-...md) from every other kind of failure, so a caller
// probing candidate (group, image) pairs can tell "doesn't exist" apart
// from "something is broken".
func TestResolveSpritePixels_SpriteNotFound(t *testing.T) {
	f := openTestdataFile(t, "v1-basic.sff")
	defer f.Close()

	_, _, _, err := ResolveSpritePixels(f, 999999, 999999, nil)
	if err == nil {
		t.Fatal("expected an error for a nonexistent sprite, got nil")
	}
	const wantPrefix = "sprite not found: "
	if len(err.Error()) < len(wantPrefix) || err.Error()[:len(wantPrefix)] != wantPrefix {
		t.Errorf("error %q does not start with %q", err.Error(), wantPrefix)
	}
}

// TestResolveSpritePixels_MalformedFile asserts malformed/truncated input
// reports a descriptive error rather than panicking.
func TestResolveSpritePixels_MalformedFile(t *testing.T) {
	r := bytes.NewReader([]byte("not a real sff file"))

	_, _, _, err := ResolveSpritePixels(r, 0, 0, nil)
	if err == nil {
		t.Fatal("expected an error for a malformed file, got nil")
	}
}

// TestResolveSpritePixels_V2LinkedSpriteNotYetSupported documents and
// verifies the deliberate scope cut: v2 sprites that share (rather than
// own) their pixel data are not yet resolved by this function — no v2
// linked-sprite decode path has been validated against real files anywhere
// in this package yet (that is item 029's job, still open). Resolving one
// must report a clear, distinct error instead of guessing at the linking
// rule the way an unvalidated implementation risks.
func TestResolveSpritePixels_V2LinkedSpriteNotYetSupported(t *testing.T) {
	f := openTestdataFile(t, "v2-zero-length-copy.sff")
	defer f.Close()

	table, err := ParseV2(f)
	if err != nil {
		t.Fatalf("ParseV2: %v", err)
	}
	target := table.Sprites[len(table.Sprites)-1]
	if target.Length != 0 {
		t.Fatalf("fixture assumption broken: expected the target sprite (group %d, image %d) to have zero-length (linked) pixel data", target.Group, target.Image)
	}

	_, _, _, err = ResolveSpritePixels(f, target.Group, target.Image, nil)
	if err == nil {
		t.Fatal("expected an error for a linked v2 sprite, got nil")
	}
}

// TestCheckSpriteDimensions guards ResolveSpritePixels against a corrupt or
// adversarial file declaring an implausibly large sprite (this function is
// reachable directly from untrusted caller-supplied bytes via the WASM
// boundary — see .vibe/decisions/020-...md).
func TestCheckSpriteDimensions(t *testing.T) {
	cases := []struct {
		name          string
		width, height int
		wantErr       bool
	}{
		{"Zero", 0, 0, false},
		{"TypicalSprite", 200, 300, false},
		{"AtLimit", SpritePixelDimensionLimit, SpritePixelDimensionLimit, false},
		{"WidthBeyondLimit", SpritePixelDimensionLimit + 1, 10, true},
		{"HeightBeyondLimit", 10, SpritePixelDimensionLimit + 1, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := checkSpriteDimensions(c.width, c.height)
			if c.wantErr && err == nil {
				t.Errorf("checkSpriteDimensions(%d, %d): expected an error, got nil", c.width, c.height)
			}
			if !c.wantErr && err != nil {
				t.Errorf("checkSpriteDimensions(%d, %d): unexpected error: %v", c.width, c.height, err)
			}
		})
	}
}
