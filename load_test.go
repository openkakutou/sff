package sff

import (
	"bytes"
	"testing"
)

func TestLoad_V1File_AssemblesSpriteGroupsWithDecodedDimensionsAndDerivedPalettes(t *testing.T) {
	sprites := []V1WriteSprite{
		{
			Group: 0, Image: 0, AxisX: 4, AxisY: -2, SharedPalette: false,
			PixelData: mustEncodePCX(t, &PCXImage{Width: 4, Height: 2, Pixels: []byte{1, 1, 1, 1, 2, 2, 2, 2}}),
			Palette:   v1TestPalette(),
		},
		{
			// Shares sprite 0's palette: same derived Palette value.
			Group: 0, Image: 1, AxisX: 5, AxisY: -1, SharedPalette: true,
			PixelData: mustEncodePCX(t, &PCXImage{Width: 3, Height: 1, Pixels: []byte{3, 3, 3}}),
		},
		{
			// Its own palette again: derived Palette value bumps up.
			Group: 1, Image: 0, AxisX: 0, AxisY: 0, SharedPalette: false,
			PixelData: mustEncodePCX(t, &PCXImage{Width: 2, Height: 2, Pixels: []byte{9, 9, 9, 9}}),
			Palette:   v1TestPalette(),
		},
	}

	var buf bytes.Buffer
	if err := SerializeV1(&buf, [4]byte{1, 0, 0, 1}, false, sprites); err != nil {
		t.Fatalf("test setup: SerializeV1 failed: %v", err)
	}

	groups, err := Load(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if len(groups) != 2 {
		t.Fatalf("expected 2 sprite groups, got %d", len(groups))
	}
	if groups[0].Index != 0 || groups[1].Index != 1 {
		t.Fatalf("expected groups in ascending index order [0, 1], got [%d, %d]", groups[0].Index, groups[1].Index)
	}

	if len(groups[0].Sprites) != 2 {
		t.Fatalf("expected 2 sprites in group 0, got %d", len(groups[0].Sprites))
	}
	want := Sprite{Group: 0, Image: 0, Width: 4, Height: 2, AxisX: 4, AxisY: -2, Palette: 0}
	if got := groups[0].Sprites[0]; got != want {
		t.Errorf("sprite (0,0): expected %+v, got %+v", want, got)
	}
	want = Sprite{Group: 0, Image: 1, Width: 3, Height: 1, AxisX: 5, AxisY: -1, Palette: 0}
	if got := groups[0].Sprites[1]; got != want {
		t.Errorf("sprite (0,1): expected %+v, got %+v", want, got)
	}

	if len(groups[1].Sprites) != 1 {
		t.Fatalf("expected 1 sprite in group 1, got %d", len(groups[1].Sprites))
	}
	want = Sprite{Group: 1, Image: 0, Width: 2, Height: 2, AxisX: 0, AxisY: 0, Palette: 1}
	if got := groups[1].Sprites[0]; got != want {
		t.Errorf("sprite (1,0): expected %+v, got %+v", want, got)
	}
}

func TestLoad_V2File_AssemblesSpriteGroupsDirectlyFromTableWithoutDecodingPixels(t *testing.T) {
	sprites := []V2WriteSprite{
		{
			Group: 0, Image: 0, Width: 3, Height: 2, AxisX: 1, AxisY: -1,
			Format: V2FormatRaw, ColorDepth: 8, PaletteIndex: 0,
			// Deliberately unsupported pixel encoding — Load must not need
			// to decode pixel data for v2 to succeed.
			PixelData: []byte{0xFF, 0xFF, 0xFF},
		},
		{
			Group: 1, Image: 0, Width: 1, Height: 1, AxisX: 0, AxisY: 0,
			Format: V2FormatRLE8, ColorDepth: 8, PaletteIndex: 2,
			PixelData: []byte{0x00},
		},
	}

	var buf bytes.Buffer
	if err := SerializeV2(&buf, [4]byte{0, 1, 0, 2}, sprites, nil); err != nil {
		t.Fatalf("test setup: SerializeV2 failed: %v", err)
	}

	groups, err := Load(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if len(groups) != 2 {
		t.Fatalf("expected 2 sprite groups, got %d", len(groups))
	}

	want := Sprite{Group: 0, Image: 0, Width: 3, Height: 2, AxisX: 1, AxisY: -1, Palette: 0}
	if got := groups[0].Sprites[0]; got != want {
		t.Errorf("sprite (0,0): expected %+v, got %+v", want, got)
	}
	want = Sprite{Group: 1, Image: 0, Width: 1, Height: 1, AxisX: 0, AxisY: 0, Palette: 2}
	if got := groups[1].Sprites[0]; got != want {
		t.Errorf("sprite (1,0): expected %+v, got %+v", want, got)
	}
}

func TestLoad_V1LinkedSprite_ResolvesDimensionsFromLinkTarget(t *testing.T) {
	targetPixels := &PCXImage{Width: 5, Height: 3, Pixels: make([]byte, 15)}
	for i := range targetPixels.Pixels {
		targetPixels.Pixels[i] = byte(i)
	}

	sprites := []V1WriteSprite{
		{Group: 0, Image: 0, PixelData: mustEncodePCX(t, targetPixels), Palette: v1TestPalette()},
		{Group: 0, Image: 1, SharedPalette: true, LinkedIndex: 0}, // no PixelData, links to sprite 0
	}

	var buf bytes.Buffer
	if err := SerializeV1(&buf, [4]byte{1, 0, 0, 1}, true, sprites); err != nil {
		t.Fatalf("test setup: SerializeV1 failed: %v", err)
	}

	groups, err := Load(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if len(groups) != 1 || len(groups[0].Sprites) != 2 {
		t.Fatalf("expected 1 group with 2 sprites, got %+v", groups)
	}
	linked := groups[0].Sprites[1]
	if linked.Width != 5 || linked.Height != 3 {
		t.Errorf("expected linked sprite to inherit dimensions 5x3 from its link target, got %dx%d", linked.Width, linked.Height)
	}
}

// TestLoad_V1FirstSpriteHasNoPixelDataOfItsOwn_ReturnsDescriptiveError covers
// the terminal case of the corrected v1 linking rule (see
// .vibe/decisions/017-v1-sprite-linking-and-palette-inheritance-rules.md):
// table index 0 can never inherit pixel data from an earlier entry, since
// none exists. Replaces a previous test built around a mutual LinkedIndex
// cycle — no longer reachable now that a zero-length sprite always
// inherits the immediately preceding table entry (never its own
// LinkedIndex), so every resolution step strictly decreases the index and
// can never cycle back on itself.
func TestLoad_V1FirstSpriteHasNoPixelDataOfItsOwn_ReturnsDescriptiveError(t *testing.T) {
	sprites := []V1WriteSprite{
		// index 0: no PixelData; SerializeV1 requires a valid (in-range,
		// non-self) LinkedIndex to accept a linked sprite at all, but the
		// corrected read-side rule never consults it for a zero-length
		// sprite — table index 0 has no earlier entry to inherit from
		// regardless of what it points to.
		{Group: 0, Image: 0, SharedPalette: true, LinkedIndex: 1},
		{Group: 0, Image: 1, PixelData: mustEncodePCX(t, &PCXImage{Width: 1, Height: 1, Pixels: []byte{1}}), Palette: v1TestPalette()},
	}

	var buf bytes.Buffer
	if err := SerializeV1(&buf, [4]byte{1, 0, 0, 1}, false, sprites); err != nil {
		t.Fatalf("test setup: SerializeV1 failed: %v", err)
	}

	_, err := Load(bytes.NewReader(buf.Bytes()))
	if err == nil {
		t.Fatal("expected an error for a first sprite with no pixel data of its own, got nil")
	}
}

// TestResolveV1Pixels_ChainRevisitsAnIndex_ReturnsErrorInsteadOfPanicking
// exercises resolveV1Pixels's own cycle guard directly: every real
// resolution step strictly decreases the index (see the test above), so a
// cycle can no longer occur through Load itself, but the guard stays in
// place as defense in depth and must still behave correctly if ever
// reached — e.g. through a future caller that seeds its own seen map.
func TestResolveV1Pixels_ChainRevisitsAnIndex_ReturnsErrorInsteadOfPanicking(t *testing.T) {
	table := &V1SpriteTable{Sprites: []V1SpriteEntry{
		{Group: 0, Image: 0, Length: 0},
	}}
	seen := map[int]bool{0: true} // pretend index 0 was already visited on this chain

	_, err := resolveV1Pixels(table, bytes.NewReader(nil), 0, seen)
	if err == nil {
		t.Fatal("expected an error for a chain that revisits an index, got nil")
	}
}

func TestResolveV1Pixels_IndexOutOfRange_ReturnsError(t *testing.T) {
	table := &V1SpriteTable{Sprites: []V1SpriteEntry{
		{Group: 0, Image: 0, Length: 4, SharedPalette: true},
	}}
	data := make([]byte, 100)

	if _, err := ResolveV1Pixels(table, bytes.NewReader(data), -1); err == nil {
		t.Error("expected an error for a negative index, got nil")
	}
	if _, err := ResolveV1Pixels(table, bytes.NewReader(data), 5); err == nil {
		t.Error("expected an error for an out-of-range index, got nil")
	}
}

func TestLoad_UnrecognizedSignature_ReturnsDescriptiveError(t *testing.T) {
	data := bytes.Repeat([]byte("not-an-sff-file!"), 4)

	_, err := Load(bytes.NewReader(data))
	if err == nil {
		t.Fatal("expected an error for data with an unrecognized signature, got nil")
	}
}

func TestLoad_TruncatedFile_ReturnsErrorRatherThanPanicking(t *testing.T) {
	data := []byte{1, 2, 3} // shorter than the 16-byte signature/version peek

	_, err := Load(bytes.NewReader(data))
	if err == nil {
		t.Fatal("expected an error for a file too short to contain a header, got nil")
	}
}
