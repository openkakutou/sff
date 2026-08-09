package sff

import (
	"bytes"
	"errors"
	"testing"
)

// mustEncodePCX is a test helper wrapping EncodePCX for building fixture
// pixel data; it fails the test immediately on error since these fixtures
// are always valid.
func mustEncodePCX(t *testing.T, img *PCXImage) []byte {
	t.Helper()
	data, err := EncodePCX(img)
	if err != nil {
		t.Fatalf("test setup: EncodePCX failed: %v", err)
	}
	return data
}

// v1TestPalette returns a valid V1PaletteBlockSize-byte block for a
// V1WriteSprite that does not share its palette — its exact colors are
// irrelevant to these serializer tests, only its size.
func v1TestPalette() []byte {
	return make([]byte, V1PaletteBlockSize)
}

func TestSerializeV1_MultiSpriteRoundTrip_ReparsesToEquivalentMetadataAndPixels(t *testing.T) {
	spriteAPixels := &PCXImage{Width: 4, Height: 2, Pixels: []byte{1, 1, 1, 2, 5, 6, 7, 8}}
	spriteBPixels := &PCXImage{Width: 2, Height: 3, Pixels: []byte{9, 9, 8, 8, 7, 7}}

	sprites := []V1WriteSprite{
		{
			Group: 0, Image: 0, AxisX: 5, AxisY: -3, SharedPalette: false,
			PixelData: mustEncodePCX(t, spriteAPixels), Palette: v1TestPalette(),
		},
		{
			Group: 0, Image: 1, AxisX: 1, AxisY: 2, SharedPalette: true,
			PixelData: mustEncodePCX(t, spriteBPixels),
		},
		{
			Group: 1, Image: 0, AxisX: 0, AxisY: 0, SharedPalette: false,
			PixelData: mustEncodePCX(t, &PCXImage{Width: 1, Height: 1, Pixels: []byte{42}}), Palette: v1TestPalette(),
		},
	}

	var buf bytes.Buffer
	if err := SerializeV1(&buf, [4]byte{1, 0, 0, 1}, true, sprites); err != nil {
		t.Fatalf("SerializeV1 returned error: %v", err)
	}

	table, err := ParseV1(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("re-parsing serialized file: %v", err)
	}

	if table.Header.Version != [4]byte{1, 0, 0, 1} {
		t.Errorf("expected version {1,0,0,1}, got %v", table.Header.Version)
	}
	if !table.Header.SharedPalette {
		t.Errorf("expected header SharedPalette true")
	}
	if table.Header.ImageCount != 3 {
		t.Errorf("expected ImageCount 3, got %d", table.Header.ImageCount)
	}
	if table.Header.GroupCount != 2 {
		t.Errorf("expected GroupCount 2 (distinct groups 0 and 1), got %d", table.Header.GroupCount)
	}
	if len(table.Sprites) != 3 {
		t.Fatalf("expected 3 sprites, got %d", len(table.Sprites))
	}

	wantMeta := []struct {
		group, image, axisX, axisY int
		sharedPalette              bool
	}{
		{0, 0, 5, -3, false},
		{0, 1, 1, 2, true},
		{1, 0, 0, 0, false},
	}
	for i, want := range wantMeta {
		got := table.Sprites[i]
		if got.Group != want.group || got.Image != want.image {
			t.Errorf("sprite %d: expected (group,image)=(%d,%d), got (%d,%d)", i, want.group, want.image, got.Group, got.Image)
		}
		if got.AxisX != want.axisX || got.AxisY != want.axisY {
			t.Errorf("sprite %d: expected axis (%d,%d), got (%d,%d)", i, want.axisX, want.axisY, got.AxisX, got.AxisY)
		}
		if got.SharedPalette != want.sharedPalette {
			t.Errorf("sprite %d: expected SharedPalette %v, got %v", i, want.sharedPalette, got.SharedPalette)
		}
	}

	wantPixels := []*PCXImage{spriteAPixels, spriteBPixels, {Width: 1, Height: 1, Pixels: []byte{42}}}
	for i, want := range wantPixels {
		entry := table.Sprites[i]
		raw := make([]byte, entry.Length)
		if _, err := bytes.NewReader(buf.Bytes()).ReadAt(raw, entry.Offset); err != nil {
			t.Fatalf("sprite %d: reading pixel data at offset %d: %v", i, entry.Offset, err)
		}
		decoded, err := DecodePCX(raw)
		if err != nil {
			t.Fatalf("sprite %d: DecodePCX: %v", i, err)
		}
		if decoded.Width != want.Width || decoded.Height != want.Height {
			t.Errorf("sprite %d: expected dimensions %dx%d, got %dx%d", i, want.Width, want.Height, decoded.Width, decoded.Height)
		}
		if !bytesEqual(decoded.Pixels, want.Pixels) {
			t.Errorf("sprite %d: expected pixels %v, got %v", i, want.Pixels, decoded.Pixels)
		}
	}
}

func TestSerializeV1_ZeroSprites_ProducesHeaderOnlyFileThatReparsesEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := SerializeV1(&buf, [4]byte{1, 0, 0, 1}, false, nil); err != nil {
		t.Fatalf("SerializeV1 returned error: %v", err)
	}

	if buf.Len() != headerSize {
		t.Errorf("expected a %d-byte header-only file, got %d bytes", headerSize, buf.Len())
	}

	table, err := ParseV1(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("re-parsing serialized file: %v", err)
	}
	if len(table.Sprites) != 0 {
		t.Errorf("expected 0 sprites, got %d", len(table.Sprites))
	}
	if table.Header.GroupCount != 0 || table.Header.ImageCount != 0 {
		t.Errorf("expected GroupCount=ImageCount=0, got GroupCount=%d ImageCount=%d", table.Header.GroupCount, table.Header.ImageCount)
	}
}

func TestSerializeV1_LinkedSprite_PreservesLinkageAndTargetPixelsSurviveRoundTrip(t *testing.T) {
	targetPixels := &PCXImage{Width: 3, Height: 2, Pixels: []byte{1, 2, 3, 4, 5, 6}}

	sprites := []V1WriteSprite{
		{Group: 0, Image: 0, PixelData: mustEncodePCX(t, targetPixels), Palette: v1TestPalette()},
		{Group: 0, Image: 1, SharedPalette: true, LinkedIndex: 0}, // links to sprite 0, no PixelData
	}

	var buf bytes.Buffer
	if err := SerializeV1(&buf, [4]byte{1, 0, 0, 1}, true, sprites); err != nil {
		t.Fatalf("SerializeV1 returned error: %v", err)
	}

	table, err := ParseV1(bytes.NewReader(buf.Bytes()))
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
	if !linked.SharedPalette {
		t.Errorf("expected linked sprite SharedPalette true")
	}

	// The sprite it links to must still carry its full, undamaged pixel
	// data after the round trip.
	target := table.Sprites[linked.LinkedIndex]
	raw := make([]byte, target.Length)
	if _, err := bytes.NewReader(buf.Bytes()).ReadAt(raw, target.Offset); err != nil {
		t.Fatalf("reading linked target pixel data: %v", err)
	}
	decoded, err := DecodePCX(raw)
	if err != nil {
		t.Fatalf("DecodePCX on linked target: %v", err)
	}
	if !bytesEqual(decoded.Pixels, targetPixels.Pixels) {
		t.Errorf("expected linked target pixels %v, got %v", targetPixels.Pixels, decoded.Pixels)
	}
}

func TestSerializeV1_LinkedIndexOutOfBounds_ReturnsError(t *testing.T) {
	sprites := []V1WriteSprite{
		{Group: 0, Image: 0, PixelData: mustEncodePCX(t, &PCXImage{Width: 1, Height: 1, Pixels: []byte{1}}), Palette: v1TestPalette()},
		{Group: 0, Image: 1, SharedPalette: true, LinkedIndex: 7}, // no sprite at index 7
	}

	var buf bytes.Buffer
	err := SerializeV1(&buf, [4]byte{1, 0, 0, 1}, false, sprites)

	if err == nil {
		t.Fatalf("expected an error for an out-of-bounds LinkedIndex, got nil")
	}
}

func TestSerializeV1_SelfReferencingLinkedIndex_ReturnsError(t *testing.T) {
	sprites := []V1WriteSprite{
		{Group: 0, Image: 0, SharedPalette: true, LinkedIndex: 0}, // links to itself, no PixelData anywhere
	}

	var buf bytes.Buffer
	err := SerializeV1(&buf, [4]byte{1, 0, 0, 1}, false, sprites)

	if err == nil {
		t.Fatalf("expected an error for a self-referencing LinkedIndex, got nil")
	}
}

// failingWriter always returns an error, used to verify SerializeV1
// propagates write failures instead of swallowing them.
type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) {
	return 0, errors.New("simulated write failure")
}

func TestSerializeV1_WriterError_IsPropagated(t *testing.T) {
	sprites := []V1WriteSprite{
		{Group: 0, Image: 0, PixelData: mustEncodePCX(t, &PCXImage{Width: 1, Height: 1, Pixels: []byte{1}}), Palette: v1TestPalette()},
	}

	err := SerializeV1(failingWriter{}, [4]byte{1, 0, 0, 1}, false, sprites)

	if err == nil {
		t.Fatalf("expected the writer's error to be propagated, got nil")
	}
}
