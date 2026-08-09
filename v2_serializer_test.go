package sff

import (
	"bytes"
	"testing"
)

// mustEncodeV2Sprite is a test helper wrapping EncodeV2Sprite for building
// fixture pixel data; it fails the test immediately on error since these
// fixtures are always valid.
func mustEncodeV2Sprite(t *testing.T, format int, img *V2Image) []byte {
	t.Helper()
	data, err := EncodeV2Sprite(format, img)
	if err != nil {
		t.Fatalf("test setup: EncodeV2Sprite failed: %v", err)
	}
	return data
}

func TestSerializeV2_MultiSpriteRoundTrip_ReparsesToEquivalentMetadataAndPixels(t *testing.T) {
	rawImg := &V2Image{Width: 3, Height: 2, BytesPerPixel: 1, Pixels: []byte{1, 2, 3, 4, 5, 6}}
	png8Img := &V2Image{Width: 2, Height: 2, BytesPerPixel: 1, Pixels: []byte{9, 8, 7, 6}}
	png24Img := &V2Image{Width: 2, Height: 1, BytesPerPixel: 3, Pixels: []byte{255, 0, 0, 0, 255, 0}}
	png32Img := &V2Image{Width: 1, Height: 1, BytesPerPixel: 4, Pixels: []byte{10, 20, 30, 128}} // alpha-premultiplied (R/G/B <= A)

	sprites := []V2WriteSprite{
		{
			Group: 0, Image: 0, Width: 3, Height: 2, AxisX: 1, AxisY: -1,
			Format: V2FormatRaw, ColorDepth: 8, PaletteIndex: 0,
			PixelData: mustEncodeV2Sprite(t, V2FormatRaw, rawImg),
		},
		{
			Group: 0, Image: 1, Width: 2, Height: 2, AxisX: 0, AxisY: 0,
			Format: V2FormatPNG8, ColorDepth: 8, PaletteIndex: 0,
			PixelData: mustEncodeV2Sprite(t, V2FormatPNG8, png8Img),
		},
		{
			Group: 1, Image: 0, Width: 2, Height: 1, AxisX: 2, AxisY: 3,
			Format: V2FormatPNG24, ColorDepth: 24, PaletteIndex: -1,
			PixelData: mustEncodeV2Sprite(t, V2FormatPNG24, png24Img),
		},
		{
			Group: 1, Image: 1, Width: 1, Height: 1, AxisX: 0, AxisY: 0,
			Format: V2FormatPNG32, ColorDepth: 32, PaletteIndex: -1,
			PixelData: mustEncodeV2Sprite(t, V2FormatPNG32, png32Img),
		},
	}

	palettes := []V2WritePalette{
		{Group: 0, Number: 0, ColorCount: 2, ColorData: []byte{255, 0, 0, 255, 0, 255, 0, 255}},
	}

	var buf bytes.Buffer
	if err := SerializeV2(&buf, [4]byte{0, 1, 0, 2}, sprites, palettes); err != nil {
		t.Fatalf("SerializeV2 returned error: %v", err)
	}

	table, err := ParseV2(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("re-parsing serialized file: %v", err)
	}

	if table.Header.Version != [4]byte{0, 1, 0, 2} {
		t.Errorf("expected version {0,1,0,2}, got %v", table.Header.Version)
	}
	if len(table.Sprites) != 4 {
		t.Fatalf("expected 4 sprites, got %d", len(table.Sprites))
	}
	if len(table.Palettes) != 1 {
		t.Fatalf("expected 1 palette, got %d", len(table.Palettes))
	}

	wantImages := []*V2Image{rawImg, png8Img, png24Img, png32Img}
	wantFormats := []int{V2FormatRaw, V2FormatPNG8, V2FormatPNG24, V2FormatPNG32}
	for i, want := range wantImages {
		entry := table.Sprites[i]
		raw := make([]byte, entry.Length)
		if _, err := bytes.NewReader(buf.Bytes()).ReadAt(raw, entry.Offset); err != nil {
			t.Fatalf("sprite %d: reading pixel data at offset %d: %v", i, entry.Offset, err)
		}
		decoded, err := DecodeV2Sprite(entry.Format, entry.Width, entry.Height, entry.ColorDepth, raw)
		if err != nil {
			t.Fatalf("sprite %d: DecodeV2Sprite: %v", i, err)
		}
		if entry.Format != wantFormats[i] {
			t.Errorf("sprite %d: expected format %d, got %d", i, wantFormats[i], entry.Format)
		}
		// PNG32 pixels are alpha-premultiplied; round-tripping them through
		// a straight-alpha PNG is inherently lossy by up to ±1 per channel
		// (see TestEncodeV2Sprite_PNG32Format_RoundTripsThroughDecodeV2SpriteWithAlpha).
		// Every other format's round trip stays byte-exact.
		tolerance := 0
		if wantFormats[i] == V2FormatPNG32 {
			tolerance = 1
		}
		if !bytesApproxEqual(decoded.Pixels, want.Pixels, tolerance) {
			t.Errorf("sprite %d: expected pixels %v, got %v", i, want.Pixels, decoded.Pixels)
		}
	}

	// Palette bank reference preserved: sprite 0/1 point at palette (0,0).
	if table.Sprites[0].PaletteIndex != 0 || table.Sprites[1].PaletteIndex != 0 {
		t.Errorf("expected sprites 0 and 1 to reference palette index 0, got %d and %d", table.Sprites[0].PaletteIndex, table.Sprites[1].PaletteIndex)
	}

	paletteEntry := table.Palettes[0]
	if paletteEntry.Group != 0 || paletteEntry.Number != 0 || paletteEntry.ColorCount != 2 {
		t.Errorf("unexpected palette metadata: %+v", paletteEntry)
	}
	colorData := make([]byte, paletteEntry.Length)
	if _, err := bytes.NewReader(buf.Bytes()).ReadAt(colorData, paletteEntry.Offset); err != nil {
		t.Fatalf("reading palette color data at offset %d: %v", paletteEntry.Offset, err)
	}
	if !bytesEqual(colorData, palettes[0].ColorData) {
		t.Errorf("expected palette color data %v, got %v", palettes[0].ColorData, colorData)
	}
}

func TestSerializeV2_ZeroSpritesAndPalettes_ProducesFileThatReparsesEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := SerializeV2(&buf, [4]byte{0, 1, 0, 2}, nil, nil); err != nil {
		t.Fatalf("SerializeV2 returned error: %v", err)
	}

	table, err := ParseV2(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("re-parsing serialized file: %v", err)
	}
	if len(table.Sprites) != 0 {
		t.Errorf("expected 0 sprites, got %d", len(table.Sprites))
	}
	if len(table.Palettes) != 0 {
		t.Errorf("expected 0 palettes, got %d", len(table.Palettes))
	}
}

func TestSerializeV2_LinkedSprite_PreservesLinkageAndTargetPixelsSurviveRoundTrip(t *testing.T) {
	targetImg := &V2Image{Width: 2, Height: 2, BytesPerPixel: 1, Pixels: []byte{1, 2, 3, 4}}

	sprites := []V2WriteSprite{
		{
			Group: 0, Image: 0, Width: 2, Height: 2, Format: V2FormatRaw, ColorDepth: 8,
			PixelData: mustEncodeV2Sprite(t, V2FormatRaw, targetImg),
		},
		{Group: 0, Image: 1, Width: 2, Height: 2, LinkedIndex: 0}, // links to sprite 0, no PixelData
	}

	var buf bytes.Buffer
	if err := SerializeV2(&buf, [4]byte{0, 1, 0, 2}, sprites, nil); err != nil {
		t.Fatalf("SerializeV2 returned error: %v", err)
	}

	table, err := ParseV2(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("re-parsing serialized file: %v", err)
	}

	linked := table.Sprites[1]
	if linked.Length != 0 {
		t.Errorf("expected linked sprite Length 0, got %d", linked.Length)
	}
	if linked.LinkedIndex != 0 {
		t.Errorf("expected linked sprite LinkedIndex 0, got %d", linked.LinkedIndex)
	}

	target := table.Sprites[linked.LinkedIndex]
	raw := make([]byte, target.Length)
	if _, err := bytes.NewReader(buf.Bytes()).ReadAt(raw, target.Offset); err != nil {
		t.Fatalf("reading linked target pixel data: %v", err)
	}
	decoded, err := DecodeV2Sprite(target.Format, target.Width, target.Height, target.ColorDepth, raw)
	if err != nil {
		t.Fatalf("DecodeV2Sprite on linked target: %v", err)
	}
	if !bytesEqual(decoded.Pixels, targetImg.Pixels) {
		t.Errorf("expected linked target pixels %v, got %v", targetImg.Pixels, decoded.Pixels)
	}
}

func TestSerializeV2_LinkedPalette_PreservesLinkageAndTargetColorDataSurvives(t *testing.T) {
	targetColors := []byte{1, 2, 3, 4, 5, 6, 7, 8}

	palettes := []V2WritePalette{
		{Group: 0, Number: 0, ColorCount: 2, ColorData: targetColors},
		{Group: 0, Number: 1, ColorCount: 2, LinkedIndex: 0}, // links to palette 0, no ColorData
	}

	var buf bytes.Buffer
	if err := SerializeV2(&buf, [4]byte{0, 1, 0, 2}, nil, palettes); err != nil {
		t.Fatalf("SerializeV2 returned error: %v", err)
	}

	table, err := ParseV2(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("re-parsing serialized file: %v", err)
	}

	linked := table.Palettes[1]
	if linked.Length != 0 {
		t.Errorf("expected linked palette Length 0, got %d", linked.Length)
	}
	if linked.LinkedIndex != 0 {
		t.Errorf("expected linked palette LinkedIndex 0, got %d", linked.LinkedIndex)
	}

	target := table.Palettes[linked.LinkedIndex]
	raw := make([]byte, target.Length)
	if _, err := bytes.NewReader(buf.Bytes()).ReadAt(raw, target.Offset); err != nil {
		t.Fatalf("reading linked target color data: %v", err)
	}
	if !bytesEqual(raw, targetColors) {
		t.Errorf("expected linked target color data %v, got %v", targetColors, raw)
	}
}

func TestSerializeV2_LinkedIndexOutOfBounds_ReturnsError(t *testing.T) {
	sprites := []V2WriteSprite{
		{Group: 0, Image: 0, Width: 1, Height: 1, Format: V2FormatRaw, ColorDepth: 8, PixelData: []byte{1}},
		{Group: 0, Image: 1, LinkedIndex: 7}, // no sprite at index 7
	}

	var buf bytes.Buffer
	err := SerializeV2(&buf, [4]byte{0, 1, 0, 2}, sprites, nil)
	if err == nil {
		t.Fatal("expected an error for an out-of-bounds sprite LinkedIndex, got nil")
	}
}

func TestSerializeV2_SelfReferencingLinkedIndex_ReturnsError(t *testing.T) {
	sprites := []V2WriteSprite{
		{Group: 0, Image: 0, LinkedIndex: 0}, // links to itself, no PixelData anywhere
	}

	var buf bytes.Buffer
	err := SerializeV2(&buf, [4]byte{0, 1, 0, 2}, sprites, nil)
	if err == nil {
		t.Fatal("expected an error for a self-referencing sprite LinkedIndex, got nil")
	}
}

func TestSerializeV2_PaletteLinkedIndexOutOfBounds_ReturnsError(t *testing.T) {
	palettes := []V2WritePalette{
		{Group: 0, Number: 0, LinkedIndex: 3}, // no palette at index 3
	}

	var buf bytes.Buffer
	err := SerializeV2(&buf, [4]byte{0, 1, 0, 2}, nil, palettes)
	if err == nil {
		t.Fatal("expected an error for an out-of-bounds palette LinkedIndex, got nil")
	}
}

func TestSerializeV2_WriterError_IsPropagated(t *testing.T) {
	sprites := []V2WriteSprite{
		{Group: 0, Image: 0, Width: 1, Height: 1, Format: V2FormatRaw, ColorDepth: 8, PixelData: []byte{1}},
	}

	err := SerializeV2(failingWriter{}, [4]byte{0, 1, 0, 2}, sprites, nil)
	if err == nil {
		t.Fatal("expected the writer's error to be propagated, got nil")
	}
}
