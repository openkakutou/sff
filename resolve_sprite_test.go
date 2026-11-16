package sff

// Tests for ResolveSpritePixels (backlog item 034): resolving one sprite's
// final on-screen RGBA pixels directly from a .sff file's raw bytes,
// combining what ParseV1/ParseV2, ResolveV1Pixels/DecodeV2Sprite, and
// ResolveV1Palette/ResolveV2Palette already do individually — needed so
// character-viewer-web can render real sprite images, not just metadata
// (earlier work only exposed sprite metadata; this function's own batched,
// stateless contract is decision
// .vibe/decisions/013-wasm-sprite-pixel-resolution-batched-stateless-contract.md).
// Reuses the same real fixtures and expected reference PNGs as the v1
// fixture-driven suite (item 028) so results are checked against ground
// truth, not just "didn't error".

import (
	"bytes"
	"image/color"
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
// .vibe/decisions/013-wasm-sprite-pixel-resolution-batched-stateless-contract.md)
// from every other kind of failure, so a caller probing candidate (group,
// image) pairs can tell "doesn't exist" apart from "something is broken".
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
// boundary — see
// .vibe/decisions/013-wasm-sprite-pixel-resolution-batched-stateless-contract.md).
// The bound is on total pixel *area* (width*height), not either axis
// independently — see
// .vibe/decisions/016-sprite-dimension-limit-is-area-based-not-per-axis.md:
// a real corpus scan (backlog item 007) found legitimate extreme-aspect-
// ratio "filmstrip" sprites (one axis far beyond SpritePixelDimensionLimit,
// total area well under it) that a per-axis check rejected outright, while
// the memory-safety guarantee this function exists for (bounding the pixel
// buffer ResolveSpritePixels allocates) only ever depended on area.
func TestCheckSpriteDimensions(t *testing.T) {
	cases := []struct {
		name          string
		width, height int
		wantErr       bool
	}{
		{"Zero", 0, 0, false},
		{"TypicalSprite", 200, 300, false},
		{"AtLimit", SpritePixelDimensionLimit, SpritePixelDimensionLimit, false},
		{"OneAxisOverLimitButAreaWithinBound", SpritePixelDimensionLimit + 1, 1, false},
		{"AreaJustOverLimit", SpritePixelDimensionLimit, SpritePixelDimensionLimit + 1, true},
		{"BothAxesHuge", SpritePixelDimensionLimit * 2, SpritePixelDimensionLimit * 2, true},
		// Real corpus sprites (backlog item 007), exact declared dimensions:
		// legitimate filmstrip-shaped sprites, not corrupt.
		{"RealCorpusSupergirlFilmstrip", 17637, 249, false},
		{"RealCorpusSatanZ2Filmstrip", 9979, 24, false},
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

func benchmarkResolveSpritePixels(b *testing.B, data []byte) {
	r := bytes.NewReader(data)
	group, image := largeSheetGroup(largeSheetBenchTarget), largeSheetImage(largeSheetBenchTarget)
	b.ReportAllocs()
	for b.Loop() {
		if _, _, _, err := ResolveSpritePixels(r, group, image, nil); err != nil {
			b.Fatalf("ResolveSpritePixels: %v", err)
		}
	}
}

func BenchmarkResolveSpritePixels_V1LargeSheet(b *testing.B) {
	benchmarkResolveSpritePixels(b, buildLargeV1Sheet(b))
}

func BenchmarkResolveSpritePixels_V2LargeSheet(b *testing.B) {
	benchmarkResolveSpritePixels(b, buildLargeV2Sheet(b))
}

// The v1-pcx-3plane.sff fixture is a real 3-plane (24-bit) sprite (Donkey
// Kong SD, group 2000 image 18, 52x52, magenta background) — see
// .vibe/decisions/022-pcx-3-plane-sprites-decode-to-direct-rgb.md. Expected
// colors below were read once with a separate throwaway Python PCX decoder.

func resolve3PlaneFixture(t *testing.T, override *Palette) ([]color.RGBA, int, int) {
	t.Helper()
	f := openTestdataFile(t, "v1-pcx-3plane.sff")
	defer f.Close()
	got, w, h, err := ResolveSpritePixels(f, 2000, 18, override)
	if err != nil {
		t.Fatalf("ResolveSpritePixels: %v", err)
	}
	return got, w, h
}

func TestResolveSpritePixels_ThreePlaneSprite_ReturnsRealColorsWithKeyColorTransparent(t *testing.T) {
	got, w, h := resolve3PlaneFixture(t, nil)
	if w != 52 || h != 52 || len(got) != 52*52 {
		t.Fatalf("got %dx%d with %d pixels, want 52x52 with %d", w, h, len(got), 52*52)
	}
	if got[0].A != 0 {
		t.Errorf("top-left pixel = %v, want fully transparent", got[0])
	}
	if want := (color.RGBA{192, 64, 136, 255}); got[0*52+21] != want {
		t.Errorf("pixel (21,0) = %v, want %v", got[0*52+21], want)
	}
	if want := (color.RGBA{232, 96, 200, 255}); got[36*52+4] != want {
		t.Errorf("pixel (4,36) = %v, want %v", got[36*52+4], want)
	}
	opaque := 0
	for _, p := range got {
		if p.A == 255 {
			opaque++
		}
	}
	if opaque != 196 {
		t.Errorf("opaque pixel count = %d, want 196", opaque)
	}
}

func TestResolveSpritePixels_ThreePlaneSprite_IgnoresOverridePalette(t *testing.T) {
	var override Palette
	for i := range override {
		override[i] = color.RGBA{1, 2, 3, 255}
	}
	want, _, _ := resolve3PlaneFixture(t, nil)
	got, _, _ := resolve3PlaneFixture(t, &override)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pixel %d = %v with override, want %v (override must have no effect)", i, got[i], want[i])
		}
	}
}

func TestResolveSpritePixels_ThreePlaneSpriteShorterThanPaletteBlock_ResolvesWithoutPaletteError(t *testing.T) {
	// 2x2, bytesPerLine 2: 128-byte header + 12 data bytes = 140 bytes,
	// well under the 768 bytes an indexed sprite's palette block needs.
	pcx := buildPCXHeader(1, 8, 0, 0, 1, 1, 3, 2)
	pcx = append(pcx, 10, 20, 30, 40, 50, 60)    // row 0: R(10,20) G(30,40) B(50,60)
	pcx = append(pcx, 70, 80, 90, 100, 110, 120) // row 1: R(70,80) G(90,100) B(110,120)
	data := buildV1File(t, 1, 1, false, []v1SubfileSpec{
		{length: uint32(len(pcx)), group: 2000, image: 20, pixelDataLen: len(pcx)},
	})
	copy(data[len(data)-len(pcx):], pcx)

	got, w, h, err := ResolveSpritePixels(bytes.NewReader(data), 2000, 20, nil)
	if err != nil {
		t.Fatalf("ResolveSpritePixels: %v", err)
	}
	if w != 2 || h != 2 {
		t.Fatalf("got %dx%d, want 2x2", w, h)
	}
	if want := (color.RGBA{20, 40, 60, 255}); got[1] != want {
		t.Errorf("pixel (1,0) = %v, want %v", got[1], want)
	}
	if want := (color.RGBA{80, 100, 120, 255}); got[3] != want {
		t.Errorf("pixel (1,1) = %v, want %v", got[3], want)
	}
}

func TestResolveSpritePixels_ZeroLengthSpriteLinkedToThreePlaneOwner_ReturnsOwnersPixelsAndLoadsSameSize(t *testing.T) {
	pcx := buildPCXHeader(1, 8, 0, 0, 1, 1, 3, 2)
	pcx = append(pcx, 10, 20, 30, 40, 50, 60)
	pcx = append(pcx, 70, 80, 90, 100, 110, 120)
	data := buildV1File(t, 1, 2, false, []v1SubfileSpec{
		{length: uint32(len(pcx)), group: 2000, image: 20, pixelDataLen: len(pcx)},
		{length: 0, group: 2000, image: 21, pixelDataLen: 0},
	})
	copy(data[headerSize+subheaderSize:], pcx) // sprite 0's pixel data

	owner, ow, oh, err := ResolveSpritePixels(bytes.NewReader(data), 2000, 20, nil)
	if err != nil {
		t.Fatalf("owner: %v", err)
	}
	got, w, h, err := ResolveSpritePixels(bytes.NewReader(data), 2000, 21, nil)
	if err != nil {
		t.Fatalf("linked: %v", err)
	}
	if w != 2 || h != 2 || ow != 2 || oh != 2 || len(got) != 4 {
		t.Fatalf("got %dx%d (%d px), owner %dx%d, want 2x2", w, h, len(got), ow, oh)
	}
	for i := range got {
		if got[i] != owner[i] {
			t.Fatalf("pixel %d = %v, owner has %v", i, got[i], owner[i])
		}
	}
	if want := (color.RGBA{80, 100, 120, 255}); got[3] != want {
		t.Errorf("pixel (1,1) = %v, want %v", got[3], want)
	}
	groups, err := Load(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(groups) != 1 || len(groups[0].Sprites) != 2 {
		t.Fatalf("got %d groups, want 1 with 2 sprites", len(groups))
	}
	if s := groups[0].Sprites[1]; s.Image != 21 || s.Width != 2 || s.Height != 2 {
		t.Errorf("linked sprite: image %d %dx%d, want image 21 2x2", s.Image, s.Width, s.Height)
	}
}
