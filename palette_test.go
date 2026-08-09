package sff

import (
	"bytes"
	"image/color"
	"io"
	"testing"
)

func TestResolvePixels_ForcedTransparencyRule_MakesIndexZeroFullyTransparentRegardlessOfStoredValue(t *testing.T) {
	var palette Palette
	// Index 0's stored color is opaque and non-black — the rule must still
	// override it to fully transparent.
	palette[0] = color.RGBA{R: 10, G: 20, B: 30, A: 255}
	palette[5] = color.RGBA{R: 100, G: 150, B: 200, A: 255}

	got := ResolvePixels([]byte{0, 5, 0}, palette, AlphaForceTransparentAtIndexZero)

	want := []color.RGBA{
		{R: 0, G: 0, B: 0, A: 0},
		{R: 100, G: 150, B: 200, A: 255},
		{R: 0, G: 0, B: 0, A: 0},
	}
	if !colorsEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestResolvePixels_LiteralRule_UsesPaletteAlphaUnmodifiedEvenAtIndexZero(t *testing.T) {
	var palette Palette
	palette[0] = color.RGBA{R: 10, G: 20, B: 30, A: 128}
	palette[5] = color.RGBA{R: 100, G: 150, B: 200, A: 64}

	got := ResolvePixels([]byte{0, 5}, palette, AlphaLiteral)

	want := []color.RGBA{
		{R: 10, G: 20, B: 30, A: 128},
		{R: 100, G: 150, B: 200, A: 64},
	}
	if !colorsEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestResolvePixels_EmptyIndices_ReturnsEmptySlice(t *testing.T) {
	var palette Palette
	got := ResolvePixels(nil, palette, AlphaLiteral)
	if len(got) != 0 {
		t.Fatalf("got %d colors, want 0", len(got))
	}
}

func TestResolvePixels_RepeatedIndex_ResolvesToSameColorEachTime(t *testing.T) {
	var palette Palette
	palette[7] = color.RGBA{R: 1, G: 2, B: 3, A: 4}

	got := ResolvePixels([]byte{7, 7, 7}, palette, AlphaLiteral)

	want := []color.RGBA{
		{R: 1, G: 2, B: 3, A: 4},
		{R: 1, G: 2, B: 3, A: 4},
		{R: 1, G: 2, B: 3, A: 4},
	}
	if !colorsEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// buildV1PaletteBlock returns a 768-byte v1 embedded palette block (256 RGB
// triplets) where color i is (i, 255-i, i/2) — distinct per channel so a
// test can catch a channel mix-up, not just a wrong byte count.
func buildV1PaletteBlock() []byte {
	block := make([]byte, 768)
	for i := 0; i < 256; i++ {
		block[i*3] = byte(i)
		block[i*3+1] = byte(255 - i)
		block[i*3+2] = byte(i / 2)
	}
	return block
}

func TestDecodeV1Palette_DecodesRGBTripletsAsOpaqueColors(t *testing.T) {
	got, err := DecodeV1Palette(buildV1PaletteBlock())
	if err != nil {
		t.Fatalf("DecodeV1Palette: unexpected error: %v", err)
	}

	cases := []struct {
		index int
		want  color.RGBA
	}{
		{0, color.RGBA{R: 0, G: 255, B: 0, A: 255}},
		{1, color.RGBA{R: 1, G: 254, B: 0, A: 255}},
		{255, color.RGBA{R: 255, G: 0, B: 127, A: 255}},
	}
	for _, c := range cases {
		if got[c.index] != c.want {
			t.Errorf("index %d: got %v, want %v", c.index, got[c.index], c.want)
		}
	}
	for i, c := range got {
		if c.A != 255 {
			t.Fatalf("index %d: alpha = %d, want 255 (v1 palette is always opaque)", i, c.A)
		}
	}
}

func TestDecodeV1Palette_TooShort_ReturnsError(t *testing.T) {
	_, err := DecodeV1Palette(make([]byte, 767))
	if err == nil {
		t.Fatal("expected an error for a 767-byte block, got nil")
	}
}

func TestDecodeV1Palette_TooLong_ReturnsError(t *testing.T) {
	_, err := DecodeV1Palette(make([]byte, 769))
	if err == nil {
		t.Fatal("expected an error for a 769-byte block, got nil")
	}
}

// buildV1PaletteBlockVariant returns a 768-byte v1 embedded palette block
// distinct from buildV1PaletteBlock's own fixed pattern (color i is
// (255-i, i, i/2)) — used where a test needs two distinguishable real
// palettes to prove which one actually got resolved, not just that some
// palette did.
func buildV1PaletteBlockVariant() []byte {
	block := make([]byte, 768)
	for i := 0; i < 256; i++ {
		block[i*3] = byte(255 - i)
		block[i*3+1] = byte(i)
		block[i*3+2] = byte(i / 2)
	}
	return block
}

// v1TestEntry is one buildV1TableAndData input: a sprite's Offset,
// pixel-only byte count, Group/Image key, and SharedPalette bit.
type v1TestEntry struct {
	offset        int64
	pixelLen      int
	group, image  int
	sharedPalette bool
}

// buildV1TableAndData assembles a synthetic *V1SpriteTable plus its backing
// bytes for ResolveV1Palette tests. A non-shared entry's declared Length is
// pixelLen+V1PaletteBlockSize, with its own 768-byte palette block
// (buildV1PaletteBlock's fixed pattern) written as the *last*
// V1PaletteBlockSize bytes of its own [Offset, Offset+Length) span — the
// real on-disk layout confirmed against genuine .sff v1 files, not a
// trimmed/re-encoded one (see
// .vibe/decisions/018-v1-palette-block-lives-inside-declared-length.md). A
// shared entry's declared Length is pixelLen alone (no embedded palette).
func buildV1TableAndData(entries []v1TestEntry) (*V1SpriteTable, []byte) {
	table := &V1SpriteTable{}
	size := int64(0)
	for _, e := range entries {
		length := e.pixelLen
		if !e.sharedPalette {
			length += V1PaletteBlockSize
		}
		table.Sprites = append(table.Sprites, V1SpriteEntry{
			Offset: e.offset, Length: length,
			Group: e.group, Image: e.image,
			SharedPalette: e.sharedPalette,
		})
		if end := e.offset + int64(length); end > size {
			size = end
		}
	}

	data := make([]byte, size)
	block := buildV1PaletteBlock()
	for i, e := range entries {
		if !e.sharedPalette {
			end := e.offset + int64(table.Sprites[i].Length)
			copy(data[end-V1PaletteBlockSize:end], block)
		}
	}
	return table, data
}

func TestResolveV1Palette_SpriteOwnsItsPalette_DecodesItsOwnTrailingBlock(t *testing.T) {
	table, data := buildV1TableAndData([]v1TestEntry{
		{offset: 100, pixelLen: 20, sharedPalette: false},
	})

	got, err := ResolveV1Palette(table, bytes.NewReader(data), 0, nil)
	if err != nil {
		t.Fatalf("ResolveV1Palette: unexpected error: %v", err)
	}
	want, _ := DecodeV1Palette(buildV1PaletteBlock())
	if got != want {
		t.Fatalf("got %v, want %v", got[0], want[0])
	}
}

func TestResolveV1Palette_SharedSprite_NotGroupZeroImageZero_InheritsImmediatelyPrecedingSprite(t *testing.T) {
	table, data := buildV1TableAndData([]v1TestEntry{
		{offset: 100, pixelLen: 20, group: 3, image: 1, sharedPalette: false}, // sprite 0: owns
		{offset: 1000, pixelLen: 5, group: 3, image: 2, sharedPalette: true},  // sprite 1: shares sprite 0's, not (0,0)
	})

	got, err := ResolveV1Palette(table, bytes.NewReader(data), 1, nil)
	if err != nil {
		t.Fatalf("ResolveV1Palette: unexpected error: %v", err)
	}
	want, _ := DecodeV1Palette(buildV1PaletteBlock())
	if got != want {
		t.Fatalf("got %v, want %v", got[0], want[0])
	}
}

func TestResolveV1Palette_MultipleSharedSprites_WalksBackToRealOwner(t *testing.T) {
	table, data := buildV1TableAndData([]v1TestEntry{
		{offset: 100, pixelLen: 20, group: 3, image: 1, sharedPalette: false}, // sprite 0: owns
		{offset: 1000, pixelLen: 5, group: 3, image: 2, sharedPalette: true},  // sprite 1: shares
		{offset: 2000, pixelLen: 5, group: 3, image: 3, sharedPalette: true},  // sprite 2: shares
	})

	got, err := ResolveV1Palette(table, bytes.NewReader(data), 2, nil)
	if err != nil {
		t.Fatalf("ResolveV1Palette: unexpected error: %v", err)
	}
	want, _ := DecodeV1Palette(buildV1PaletteBlock())
	if got != want {
		t.Fatalf("got %v, want %v", got[0], want[0])
	}
}

func TestResolveV1Palette_SharedSprite_GroupZeroImageZero_InheritsFirstSpriteEvenWhenACloserOwnerExists(t *testing.T) {
	table, data := buildV1TableAndData([]v1TestEntry{
		{offset: 100, pixelLen: 20, group: 0, image: 0, sharedPalette: false}, // sprite 0: owns, block A
		{offset: 1000, pixelLen: 5, group: 7, image: 7, sharedPalette: false}, // sprite 1: owns too, block A (closer, but must not win)
		{offset: 2000, pixelLen: 5, group: 0, image: 0, sharedPalette: true},  // sprite 2: shares, is (0,0) -> must inherit sprite 0's, not sprite 1's
	})
	// Give sprite 1 a palette distinguishable from sprite 0's.
	e1 := table.Sprites[1]
	copy(data[e1.Offset+int64(e1.Length)-V1PaletteBlockSize:e1.Offset+int64(e1.Length)], buildV1PaletteBlockVariant())

	got, err := ResolveV1Palette(table, bytes.NewReader(data), 2, nil)
	if err != nil {
		t.Fatalf("ResolveV1Palette: unexpected error: %v", err)
	}
	want, _ := DecodeV1Palette(buildV1PaletteBlock())
	if got != want {
		t.Fatalf("got %v, want %v (sprite 0's palette, not the nearer sprite 1's)", got[0], want[0])
	}
}

func TestResolveV1Palette_SpriteZero_DecodesItsOwnBlockEvenWhenMarkedShared(t *testing.T) {
	// Table index 0 has no earlier sprite to inherit from: the reference
	// decoder this package tracks always decodes its own trailing block
	// there, regardless of its own SharedPalette bit.
	table, data := buildV1TableAndData([]v1TestEntry{
		{offset: 100, pixelLen: 20, sharedPalette: false}, // Length still includes a real trailing block to decode
	})
	table.Sprites[0].SharedPalette = true

	got, err := ResolveV1Palette(table, bytes.NewReader(data), 0, nil)
	if err != nil {
		t.Fatalf("ResolveV1Palette: unexpected error: %v", err)
	}
	want, _ := DecodeV1Palette(buildV1PaletteBlock())
	if got != want {
		t.Fatalf("got %v, want %v", got[0], want[0])
	}
}

func TestResolveV1Palette_DeclaredLengthTooShortForPaletteBlock_ReturnsError(t *testing.T) {
	table := &V1SpriteTable{Sprites: []V1SpriteEntry{
		{Offset: 100, Length: 20, SharedPalette: false},
	}}
	data := make([]byte, 200)

	_, err := ResolveV1Palette(table, bytes.NewReader(data), 0, nil)
	if err == nil {
		t.Fatal("expected an error when declared length is smaller than the palette block, got nil")
	}
}

func TestResolveV1Palette_IndexOutOfRange_ReturnsError(t *testing.T) {
	table, data := buildV1TableAndData([]v1TestEntry{
		{offset: 100, pixelLen: 20, sharedPalette: false},
	})

	if _, err := ResolveV1Palette(table, bytes.NewReader(data), -1, nil); err == nil {
		t.Error("expected an error for a negative index, got nil")
	}
	if _, err := ResolveV1Palette(table, bytes.NewReader(data), 5, nil); err == nil {
		t.Error("expected an error for an out-of-range index, got nil")
	}
}

// buildV2PaletteColorData returns n colors of already-RGBA v2 palette bank
// data (4 bytes/color): color i is (i, 255-i, i/2, i) — a varying alpha so
// a test can catch DecodeV2Palette wrongly forcing alpha, unlike v1's
// always-opaque embedded palette.
func buildV2PaletteColorData(n int) []byte {
	data := make([]byte, n*4)
	for i := 0; i < n; i++ {
		data[i*4] = byte(i)
		data[i*4+1] = byte(255 - i)
		data[i*4+2] = byte(i / 2)
		data[i*4+3] = byte(i)
	}
	return data
}

func TestDecodeV2Palette_DecodesRGBATupleWithLiteralAlpha(t *testing.T) {
	got, err := DecodeV2Palette(buildV2PaletteColorData(256))
	if err != nil {
		t.Fatalf("DecodeV2Palette: unexpected error: %v", err)
	}

	cases := []struct {
		index int
		want  color.RGBA
	}{
		{0, color.RGBA{R: 0, G: 255, B: 0, A: 0}},
		{1, color.RGBA{R: 1, G: 254, B: 0, A: 1}},
		{255, color.RGBA{R: 255, G: 0, B: 127, A: 255}},
	}
	for _, c := range cases {
		if got[c.index] != c.want {
			t.Errorf("index %d: got %v, want %v", c.index, got[c.index], c.want)
		}
	}
}

func TestDecodeV2Palette_FewerThanMaxColors_LeavesRemainingEntriesZero(t *testing.T) {
	got, err := DecodeV2Palette(buildV2PaletteColorData(4))
	if err != nil {
		t.Fatalf("DecodeV2Palette: unexpected error: %v", err)
	}
	if got[3] != (color.RGBA{R: 3, G: 252, B: 1, A: 3}) {
		t.Errorf("index 3: got %v, want the 4th declared color", got[3])
	}
	if got[4] != (color.RGBA{}) {
		t.Errorf("index 4 (undeclared): got %v, want zero value", got[4])
	}
	if got[255] != (color.RGBA{}) {
		t.Errorf("index 255 (undeclared): got %v, want zero value", got[255])
	}
}

func TestDecodeV2Palette_LengthNotMultipleOfFour_ReturnsError(t *testing.T) {
	_, err := DecodeV2Palette(make([]byte, 15))
	if err == nil {
		t.Fatal("expected an error for a 15-byte block, got nil")
	}
}

func TestDecodeV2Palette_MoreThan256Colors_ReturnsError(t *testing.T) {
	_, err := DecodeV2Palette(make([]byte, 257*4))
	if err == nil {
		t.Fatal("expected an error for 257 colors, got nil")
	}
}

func TestResolveV2Palette_BankOwnsItsData_DecodesItsOwnColorData(t *testing.T) {
	colorData := buildV2PaletteColorData(256)
	data := make([]byte, 100+len(colorData))
	copy(data[100:], colorData)

	table := &V2SpriteTable{Palettes: []V2PaletteEntry{
		{Group: 0, Number: 0, Offset: 100, Length: len(colorData)},
	}}

	got, err := ResolveV2Palette(table, bytes.NewReader(data), 0, nil)
	if err != nil {
		t.Fatalf("ResolveV2Palette: unexpected error: %v", err)
	}
	want, _ := DecodeV2Palette(colorData)
	if got != want {
		t.Fatalf("got %v, want %v", got[1], want[1])
	}
}

func TestResolveV2Palette_ZeroLengthBank_InheritsLinkedBanksColors(t *testing.T) {
	colorData := buildV2PaletteColorData(256)
	data := make([]byte, 100+len(colorData))
	copy(data[100:], colorData)

	table := &V2SpriteTable{Palettes: []V2PaletteEntry{
		{Group: 0, Number: 0, Offset: 100, Length: len(colorData)},
		{Group: 0, Number: 1, Length: 0, LinkedIndex: 0},
	}}

	got, err := ResolveV2Palette(table, bytes.NewReader(data), 1, nil)
	if err != nil {
		t.Fatalf("ResolveV2Palette: unexpected error: %v", err)
	}
	want, _ := DecodeV2Palette(colorData)
	if got != want {
		t.Fatalf("got %v, want %v", got[1], want[1])
	}
}

func TestResolveV2Palette_ZeroLengthBankLinkedToIndexZero_ResolvesToFirstBank(t *testing.T) {
	colorData := buildV2PaletteColorData(256)
	data := make([]byte, 100+len(colorData))
	copy(data[100:], colorData)

	table := &V2SpriteTable{Palettes: []V2PaletteEntry{
		{Group: 0, Number: 0, Offset: 100, Length: len(colorData)}, // the first bank
		{Group: 0, Number: 1, Length: 0, LinkedIndex: 0},
		{Group: 0, Number: 2, Length: 0, LinkedIndex: 0}, // also links to the first bank
	}}

	got, err := ResolveV2Palette(table, bytes.NewReader(data), 2, nil)
	if err != nil {
		t.Fatalf("ResolveV2Palette: unexpected error: %v", err)
	}
	want, _ := DecodeV2Palette(colorData)
	if got != want {
		t.Fatalf("got %v, want %v", got[1], want[1])
	}
}

func TestResolveV2Palette_IndexOutOfRange_ReturnsError(t *testing.T) {
	table := &V2SpriteTable{Palettes: []V2PaletteEntry{
		{Group: 0, Number: 0, Offset: 100, Length: 1024},
	}}
	data := make([]byte, 1200)

	if _, err := ResolveV2Palette(table, bytes.NewReader(data), -1, nil); err == nil {
		t.Error("expected an error for a negative index, got nil")
	}
	if _, err := ResolveV2Palette(table, bytes.NewReader(data), 5, nil); err == nil {
		t.Error("expected an error for an out-of-range index, got nil")
	}
}

func TestResolveV2Palette_LinkChainCyclesBackOnItself_ReturnsError(t *testing.T) {
	table := &V2SpriteTable{Palettes: []V2PaletteEntry{
		{Group: 0, Number: 0, Length: 0, LinkedIndex: 1},
		{Group: 0, Number: 1, Length: 0, LinkedIndex: 0},
	}}
	data := make([]byte, 10)

	_, err := ResolveV2Palette(table, bytes.NewReader(data), 0, nil)
	if err == nil {
		t.Fatal("expected an error for a cyclic link chain, got nil")
	}
}

// buildExternalPaletteBlock returns a 768-byte external .act buffer (256 RGB
// triplets in the file's own on-disk order) using the same fixed pattern as
// buildV1PaletteBlock (triplet i is (i, 255-i, i/2)) — but, unlike v1's
// embedded block, DecodeExternalPalette reads this order in reverse.
func buildExternalPaletteBlock() []byte {
	return buildV1PaletteBlock()
}

func TestDecodeExternalPalette_ReversesTripletOrderAndForcesIndexZeroTransparent(t *testing.T) {
	got, err := DecodeExternalPalette(buildExternalPaletteBlock())
	if err != nil {
		t.Fatalf("DecodeExternalPalette: unexpected error: %v", err)
	}

	// The file's first triplet (i=0: R=0, G=255, B=0) becomes palette index 255.
	if want := (color.RGBA{R: 0, G: 255, B: 0, A: 255}); got[255] != want {
		t.Errorf("index 255: got %v, want %v", got[255], want)
	}
	// The file's second triplet (i=1: R=1, G=254, B=0) becomes palette index 254.
	if want := (color.RGBA{R: 1, G: 254, B: 0, A: 255}); got[254] != want {
		t.Errorf("index 254: got %v, want %v", got[254], want)
	}
	// The file's last triplet (i=255: R=255, G=0, B=127) becomes palette index
	// 0, with its alpha forced to 0 — unlike every other resolved index.
	if want := (color.RGBA{R: 255, G: 0, B: 127, A: 0}); got[0] != want {
		t.Errorf("index 0: got %v, want %v", got[0], want)
	}
	for i := 1; i < 256; i++ {
		if got[i].A != 255 {
			t.Fatalf("index %d: alpha = %d, want 255 (only index 0 is forced transparent)", i, got[i].A)
		}
	}
}

func TestDecodeExternalPalette_TooShort_ReturnsError(t *testing.T) {
	_, err := DecodeExternalPalette(make([]byte, 767))
	if err == nil {
		t.Fatal("expected an error for a 767-byte buffer, got nil")
	}
}

func TestDecodeExternalPalette_TooLong_ReturnsError(t *testing.T) {
	_, err := DecodeExternalPalette(make([]byte, 769))
	if err == nil {
		t.Fatal("expected an error for a 769-byte buffer, got nil")
	}
}

func TestDecodeExternalPalette_Empty_ReturnsError(t *testing.T) {
	_, err := DecodeExternalPalette(nil)
	if err == nil {
		t.Fatal("expected an error for an empty buffer, got nil")
	}
}

func TestResolveV1Palette_OverrideSupplied_ReturnsOverrideInsteadOfSpritesOwnPalette(t *testing.T) {
	table, data := buildV1TableAndData([]v1TestEntry{
		{offset: 100, pixelLen: 20, sharedPalette: false},
	})

	override, _ := DecodeExternalPalette(buildExternalPaletteBlock())
	got, err := ResolveV1Palette(table, bytes.NewReader(data), 0, &override)
	if err != nil {
		t.Fatalf("ResolveV1Palette: unexpected error: %v", err)
	}
	if got != override {
		t.Fatalf("got %v, want the override palette %v", got, override)
	}
}

func TestResolveV1Palette_OverrideSupplied_BypassesIndexRangeCheck(t *testing.T) {
	table := &V1SpriteTable{} // no sprites at all
	override, _ := DecodeExternalPalette(buildExternalPaletteBlock())

	got, err := ResolveV1Palette(table, bytes.NewReader(nil), 999, &override)
	if err != nil {
		t.Fatalf("ResolveV1Palette: unexpected error: %v", err)
	}
	if got != override {
		t.Fatalf("got %v, want the override palette", got)
	}
}

func TestResolveV2Palette_OverrideSupplied_ReturnsOverrideInsteadOfBanksOwnColors(t *testing.T) {
	colorData := buildV2PaletteColorData(256)
	data := make([]byte, 100+len(colorData))
	copy(data[100:], colorData)

	table := &V2SpriteTable{Palettes: []V2PaletteEntry{
		{Group: 0, Number: 0, Offset: 100, Length: len(colorData)},
	}}

	override, _ := DecodeExternalPalette(buildExternalPaletteBlock())
	got, err := ResolveV2Palette(table, bytes.NewReader(data), 0, &override)
	if err != nil {
		t.Fatalf("ResolveV2Palette: unexpected error: %v", err)
	}
	if got != override {
		t.Fatalf("got %v, want the override palette %v", got, override)
	}
}

func TestResolveV2Palette_OverrideSupplied_BypassesIndexRangeCheck(t *testing.T) {
	table := &V2SpriteTable{} // no palette banks at all
	override, _ := DecodeExternalPalette(buildExternalPaletteBlock())

	got, err := ResolveV2Palette(table, bytes.NewReader(nil), 999, &override)
	if err != nil {
		t.Fatalf("ResolveV2Palette: unexpected error: %v", err)
	}
	if got != override {
		t.Fatalf("got %v, want the override palette", got)
	}
}

func TestEncodeV1Palette_RoundTripsByteExactlyWithDecodeV1Palette(t *testing.T) {
	original := buildV1PaletteBlock()
	p, err := DecodeV1Palette(original)
	if err != nil {
		t.Fatalf("test setup: DecodeV1Palette: %v", err)
	}

	got := EncodeV1Palette(p)
	if !bytes.Equal(got, original) {
		t.Fatalf("EncodeV1Palette(DecodeV1Palette(original)) does not reproduce the original 768 bytes")
	}
}

func TestEncodeV1Palette_ProducesExactlyV1PaletteBlockSizeBytes(t *testing.T) {
	var p Palette
	got := EncodeV1Palette(p)
	if len(got) != V1PaletteBlockSize {
		t.Fatalf("got %d bytes, want exactly %d", len(got), V1PaletteBlockSize)
	}
}

func TestEncodeV1Palette_IgnoresAlphaChannel(t *testing.T) {
	var opaque, transparent Palette
	opaque[10] = color.RGBA{R: 11, G: 22, B: 33, A: 255}
	transparent[10] = color.RGBA{R: 11, G: 22, B: 33, A: 0}

	gotOpaque := EncodeV1Palette(opaque)
	gotTransparent := EncodeV1Palette(transparent)
	if !bytes.Equal(gotOpaque, gotTransparent) {
		t.Fatalf("v1 palette encoding must not depend on alpha (v1 has no per-color alpha on disk): got %v vs %v", gotOpaque, gotTransparent)
	}
	if gotOpaque[30] != 11 || gotOpaque[31] != 22 || gotOpaque[32] != 33 {
		t.Fatalf("index 10 RGB triplet: got (%d,%d,%d), want (11,22,33)", gotOpaque[30], gotOpaque[31], gotOpaque[32])
	}
}

func TestEncodeV2Palette_RoundTripsByteExactlyWithDecodeV2Palette(t *testing.T) {
	original := buildV2PaletteColorData(256)
	p, err := DecodeV2Palette(original)
	if err != nil {
		t.Fatalf("test setup: DecodeV2Palette: %v", err)
	}

	got, err := EncodeV2Palette(p, 256)
	if err != nil {
		t.Fatalf("EncodeV2Palette: unexpected error: %v", err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("EncodeV2Palette(DecodeV2Palette(original), 256) does not reproduce the original bytes")
	}
}

func TestEncodeV2Palette_FewerThanMaxColors_EncodesOnlyDeclaredCount(t *testing.T) {
	var p Palette
	p[0] = color.RGBA{R: 1, G: 2, B: 3, A: 4}
	p[3] = color.RGBA{R: 9, G: 8, B: 7, A: 6}
	p[4] = color.RGBA{R: 255, G: 255, B: 255, A: 255} // must not be encoded: beyond colorCount

	got, err := EncodeV2Palette(p, 4)
	if err != nil {
		t.Fatalf("EncodeV2Palette: unexpected error: %v", err)
	}
	if len(got) != 16 {
		t.Fatalf("got %d bytes, want exactly %d (4 colors * 4 bytes)", len(got), 16)
	}
	want := []byte{1, 2, 3, 4, 0, 0, 0, 0, 0, 0, 0, 0, 9, 8, 7, 6}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestEncodeV2Palette_ColorCountOutOfRange_ReturnsError(t *testing.T) {
	var p Palette
	if _, err := EncodeV2Palette(p, -1); err == nil {
		t.Error("expected an error for colorCount -1, got nil")
	}
	if _, err := EncodeV2Palette(p, 257); err == nil {
		t.Error("expected an error for colorCount 257, got nil")
	}
}

func TestEncodeV2Palette_ZeroColorCount_ReturnsEmptySlice(t *testing.T) {
	var p Palette
	got, err := EncodeV2Palette(p, 0)
	if err != nil {
		t.Fatalf("EncodeV2Palette: unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d bytes, want 0", len(got))
	}
}

func TestEncodeExternalPalette_RoundTripsByteExactlyWithDecodeExternalPalette(t *testing.T) {
	original := buildExternalPaletteBlock()
	p, err := DecodeExternalPalette(original)
	if err != nil {
		t.Fatalf("test setup: DecodeExternalPalette: %v", err)
	}

	got := EncodeExternalPalette(p)
	if !bytes.Equal(got, original) {
		t.Fatalf("EncodeExternalPalette(DecodeExternalPalette(original)) does not reproduce the original 768 bytes")
	}
}

func TestEncodeExternalPalette_ProducesExactlyExternalPaletteSizeBytes(t *testing.T) {
	var p Palette
	got := EncodeExternalPalette(p)
	if len(got) != externalPaletteSize {
		t.Fatalf("got %d bytes, want exactly %d", len(got), externalPaletteSize)
	}
}

// TestEncodeExternalPalette_RealFixtureRoundTrips exercises the .act
// export/import round trip against a real, unmodified community .act file
// (not just synthetic data) — see CLAUDE.md's fixture-driven testing
// convention.
func TestEncodeExternalPalette_RealFixtureRoundTrips(t *testing.T) {
	for _, name := range []string{"cyclops-v1-palette1.act", "greenarrow-v1-palette1.act"} {
		t.Run(name, func(t *testing.T) {
			f := openTestdataFile(t, name)
			defer f.Close()
			original, err := io.ReadAll(f)
			if err != nil {
				t.Fatalf("reading fixture: %v", err)
			}

			p, err := DecodeExternalPalette(original)
			if err != nil {
				t.Fatalf("DecodeExternalPalette: %v", err)
			}
			got := EncodeExternalPalette(p)
			if !bytes.Equal(got, original) {
				t.Fatalf("re-encoded .act bytes do not match the original fixture byte-for-byte")
			}
		})
	}
}

func TestPalette_SetColor_SetsColorAtValidIndex(t *testing.T) {
	var p Palette
	if err := p.SetColor(42, 10, 20, 30, 40); err != nil {
		t.Fatalf("SetColor: unexpected error: %v", err)
	}
	want := color.RGBA{R: 10, G: 20, B: 30, A: 40}
	if p[42] != want {
		t.Fatalf("got %v, want %v", p[42], want)
	}
}

func TestPalette_SetColor_BoundaryComponentValues_Accepted(t *testing.T) {
	var p Palette
	if err := p.SetColor(0, 0, 0, 0, 0); err != nil {
		t.Fatalf("SetColor with all-zero components: unexpected error: %v", err)
	}
	if err := p.SetColor(255, 255, 255, 255, 255); err != nil {
		t.Fatalf("SetColor with all-255 components: unexpected error: %v", err)
	}
	want := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	if p[255] != want {
		t.Fatalf("index 255: got %v, want %v", p[255], want)
	}
}

func TestPalette_SetColor_IndexOutOfRange_ReturnsErrorAndLeavesPaletteUnchanged(t *testing.T) {
	var p Palette
	p[0] = color.RGBA{R: 1, G: 2, B: 3, A: 4}

	if err := p.SetColor(-1, 10, 10, 10, 10); err == nil {
		t.Error("expected an error for index -1, got nil")
	}
	if err := p.SetColor(256, 10, 10, 10, 10); err == nil {
		t.Error("expected an error for index 256, got nil")
	}
	if p[0] != (color.RGBA{R: 1, G: 2, B: 3, A: 4}) {
		t.Fatalf("palette was mutated despite the out-of-range index being rejected: %v", p[0])
	}
}

func TestPalette_SetColor_ComponentOutOfRange_ReturnsErrorAndLeavesPaletteUnchanged(t *testing.T) {
	cases := []struct {
		name       string
		r, g, b, a int
	}{
		{"r too low", -1, 0, 0, 0},
		{"r too high", 256, 0, 0, 0},
		{"g too low", 0, -1, 0, 0},
		{"g too high", 0, 256, 0, 0},
		{"b too low", 0, 0, -1, 0},
		{"b too high", 0, 0, 256, 0},
		{"a too low", 0, 0, 0, -1},
		{"a too high", 0, 0, 0, 256},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var p Palette
			p[5] = color.RGBA{R: 9, G: 9, B: 9, A: 9}

			err := p.SetColor(5, c.r, c.g, c.b, c.a)
			if err == nil {
				t.Fatalf("expected an error for components (%d,%d,%d,%d), got nil", c.r, c.g, c.b, c.a)
			}
			if p[5] != (color.RGBA{R: 9, G: 9, B: 9, A: 9}) {
				t.Fatalf("palette was mutated despite the out-of-range component being rejected: %v", p[5])
			}
		})
	}
}

// TestSerializeV1_EditedPaletteReencodesAndSurvivesFullRoundTrip proves
// acceptance criterion "an edited palette re-encodes correctly for v1
// sprites": decode a real embedded block, edit one color, re-encode it,
// write a full .sff v1 file with it, then reparse+reresolve and confirm the
// edit (and only the edit) survived.
func TestSerializeV1_EditedPaletteReencodesAndSurvivesFullRoundTrip(t *testing.T) {
	p, err := DecodeV1Palette(buildV1PaletteBlock())
	if err != nil {
		t.Fatalf("test setup: DecodeV1Palette: %v", err)
	}
	if err := p.SetColor(7, 200, 150, 100, 255); err != nil {
		t.Fatalf("test setup: SetColor: %v", err)
	}

	sprites := []V1WriteSprite{{
		Group: 0, Image: 0, SharedPalette: false,
		PixelData: mustEncodePCX(t, &PCXImage{Width: 1, Height: 1, Pixels: []byte{0}}),
		Palette:   EncodeV1Palette(p),
	}}

	var buf bytes.Buffer
	if err := SerializeV1(&buf, [4]byte{1, 0, 0, 1}, false, sprites); err != nil {
		t.Fatalf("SerializeV1: %v", err)
	}

	table, err := ParseV1(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("ParseV1: %v", err)
	}
	got, err := ResolveV1Palette(table, bytes.NewReader(buf.Bytes()), 0, nil)
	if err != nil {
		t.Fatalf("ResolveV1Palette: %v", err)
	}
	if want := (color.RGBA{R: 200, G: 150, B: 100, A: 255}); got[7] != want {
		t.Errorf("edited index 7: got %v, want %v", got[7], want)
	}
	// An untouched index must still carry its original value.
	if want := (color.RGBA{R: 1, G: 254, B: 0, A: 255}); got[1] != want {
		t.Errorf("untouched index 1: got %v, want %v (edit must not disturb other colors)", got[1], want)
	}
}

// TestSerializeV2_EditedPaletteReencodesAndSurvivesFullRoundTrip mirrors
// TestSerializeV1_EditedPaletteReencodesAndSurvivesFullRoundTrip for v2
// palette banks.
func TestSerializeV2_EditedPaletteReencodesAndSurvivesFullRoundTrip(t *testing.T) {
	p, err := DecodeV2Palette(buildV2PaletteColorData(256))
	if err != nil {
		t.Fatalf("test setup: DecodeV2Palette: %v", err)
	}
	if err := p.SetColor(7, 200, 150, 100, 90); err != nil {
		t.Fatalf("test setup: SetColor: %v", err)
	}
	colorData, err := EncodeV2Palette(p, 256)
	if err != nil {
		t.Fatalf("EncodeV2Palette: %v", err)
	}

	sprites := []V2WriteSprite{{
		Group: 0, Image: 0, Width: 1, Height: 1, Format: 0, ColorDepth: 8,
		PaletteIndex: 0,
		PixelData:    mustEncodeV2Sprite(t, 0, &V2Image{Width: 1, Height: 1, BytesPerPixel: 1, Pixels: []byte{0}}),
	}}
	palettes := []V2WritePalette{{Group: 0, Number: 0, ColorCount: 256, ColorData: colorData}}

	var buf bytes.Buffer
	if err := SerializeV2(&buf, [4]byte{0, 1, 0, 2}, sprites, palettes); err != nil {
		t.Fatalf("SerializeV2: %v", err)
	}

	table, err := ParseV2(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("ParseV2: %v", err)
	}
	got, err := ResolveV2Palette(table, bytes.NewReader(buf.Bytes()), 0, nil)
	if err != nil {
		t.Fatalf("ResolveV2Palette: %v", err)
	}
	if want := (color.RGBA{R: 200, G: 150, B: 100, A: 90}); got[7] != want {
		t.Errorf("edited index 7: got %v, want %v", got[7], want)
	}
	if want := (color.RGBA{R: 1, G: 254, B: 0, A: 1}); got[1] != want {
		t.Errorf("untouched index 1: got %v, want %v (edit must not disturb other colors)", got[1], want)
	}
}

func colorsEqual(a, b []color.RGBA) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
