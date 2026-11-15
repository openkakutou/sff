package sff

import (
	"bytes"
	"image/color"
	"os"
	"reflect"
	"strings"
	"testing"
)

type v1Rendered struct {
	px   []color.RGBA
	w, h int
}

// v1WriteSpritesFromFixture rebuilds the write slice of a real, unmodified
// testdata fixture from its parsed table and raw bytes. Only fixtures whose
// every sprite's declared span lies within the file are supported.
func v1WriteSpritesFromFixture(t *testing.T, name string) []V1WriteSprite {
	t.Helper()
	f := openTestdataFile(t, name)
	defer f.Close()
	table, err := ParseV1(f)
	if err != nil {
		t.Fatalf("ParseV1(%s): %v", name, err)
	}
	var out []V1WriteSprite
	for _, e := range table.Sprites {
		s := V1WriteSprite{Group: e.Group, Image: e.Image, AxisX: e.AxisX, AxisY: e.AxisY, SharedPalette: e.SharedPalette, LinkedIndex: e.LinkedIndex}
		if e.Length > 0 {
			raw := make([]byte, e.Length)
			if _, err := f.ReadAt(raw, e.Offset); err != nil {
				t.Fatalf("reading %s sprite span: %v", name, err)
			}
			if e.SharedPalette {
				s.PixelData = raw
			} else {
				s.PixelData, s.Palette = raw[:e.Length-V1PaletteBlockSize], raw[e.Length-V1PaletteBlockSize:]
			}
		}
		out = append(out, s)
	}
	return out
}

func serializeV1Sheet(t *testing.T, sprites []V1WriteSprite) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := SerializeV1(&buf, [4]byte{1, 0, 0, 1}, true, sprites); err != nil {
		t.Fatalf("SerializeV1: %v", err)
	}
	return buf.Bytes()
}

// renderV1 serializes sprites, re-parses them and resolves every sprite to
// final RGBA pixels, keyed by (group, image).
func renderV1(t *testing.T, sprites []V1WriteSprite) map[[2]int]v1Rendered {
	t.Helper()
	data := serializeV1Sheet(t, sprites)
	table, err := ParseV1(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("re-parsing: %v", err)
	}
	if len(table.Sprites) != len(sprites) {
		t.Fatalf("re-parsed %d sprites, want %d", len(table.Sprites), len(sprites))
	}
	out := map[[2]int]v1Rendered{}
	for _, s := range sprites {
		px, w, h, err := ResolveSpritePixels(bytes.NewReader(data), s.Group, s.Image, nil)
		if err != nil {
			t.Fatalf("ResolveSpritePixels(%d,%d): %v", s.Group, s.Image, err)
		}
		out[[2]int{s.Group, s.Image}] = v1Rendered{px, w, h}
	}
	return out
}

func assertRenderedLike(t *testing.T, got, want map[[2]int]v1Rendered, skip ...[2]int) {
	t.Helper()
	skipped := map[[2]int]bool{}
	for _, k := range skip {
		skipped[k] = true
	}
	for k, w := range want {
		if skipped[k] {
			continue
		}
		g, ok := got[k]
		if !ok {
			t.Errorf("sprite %v missing after edit", k)
			continue
		}
		if !reflect.DeepEqual(g, w) {
			t.Errorf("sprite %v decodes differently after edit", k)
		}
	}
}

// v1EditSheet builds a seven-sprite sheet from real fixtures plus two
// linked sprites:
//
//	0 (0,74)  own pixels + palette P1          (zero-length-copy fixture)
//	1 (0,75)  zero-length copy of 0, shared palette (same fixture)
//	2 (5,0)   own pixels + palette P2          (same-palette-a-group5)
//	3 (5,1)   own pixels, shares previous palette (renamed from (5,0))
//	4 (0,0)   own pixels + palette P3          (basic)
//	5 (7,7)   linked to 2, own palette
//	6 (7,8)   linked to 5, own palette
func v1EditSheet(t *testing.T) []V1WriteSprite {
	t.Helper()
	sheet := v1WriteSpritesFromFixture(t, "v1-zero-length-copy.sff")
	g5 := v1WriteSpritesFromFixture(t, "v1-same-palette-a-group5.sff")
	g5[1].Image = 1
	sheet = append(sheet, g5...)
	sheet = append(sheet, v1WriteSpritesFromFixture(t, "v1-basic.sff")...)
	pal := make([]byte, V1PaletteBlockSize)
	for i := range pal {
		pal[i] = byte(i * 7)
	}
	sheet = append(sheet,
		V1WriteSprite{Group: 7, Image: 7, Palette: pal, LinkedIndex: 2},
		V1WriteSprite{Group: 7, Image: 8, Palette: pal, LinkedIndex: 5},
	)
	return sheet
}

func TestV1EditSheet_BaselineMatchesRealFixtureDecoding(t *testing.T) {
	got := renderV1(t, v1EditSheet(t))
	for _, k := range [][2]int{{0, 74}, {0, 75}} {
		f := openTestdataFile(t, "v1-zero-length-copy.sff")
		px, w, h, err := ResolveSpritePixels(f, k[0], k[1], nil)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got[k], v1Rendered{px, w, h}) {
			t.Errorf("rebuilt sprite %v differs from the original fixture's decoding", k)
		}
	}
	if got[[2]int{7, 7}].w != got[[2]int{5, 0}].w || got[[2]int{7, 8}].h != got[[2]int{5, 0}].h {
		t.Errorf("linked sprites should have the dimensions of their target")
	}
}

func TestAddV1Sprite_AppendsWithoutMutatingInput(t *testing.T) {
	sheet := v1EditSheet(t)
	before := len(sheet)
	pal := make([]byte, V1PaletteBlockSize)
	pal[3*1], pal[3*1+1], pal[3*1+2] = 10, 20, 30 // palette index 1 -> (10,20,30)
	data := mustEncodePCX(t, &PCXImage{Width: 2, Height: 1, Pixels: []byte{1, 1}})

	out, err := AddV1Sprite(sheet, V1WriteSprite{Group: 9, Image: 1, PixelData: data, Palette: pal})
	if err != nil {
		t.Fatal(err)
	}
	if len(sheet) != before || len(out) != before+1 {
		t.Fatalf("input len %d (want %d), output len %d (want %d)", len(sheet), before, len(out), before+1)
	}
	got := renderV1(t, out)
	want := v1Rendered{px: []color.RGBA{{R: 10, G: 20, B: 30, A: 255}, {R: 10, G: 20, B: 30, A: 255}}, w: 2, h: 1}
	if !reflect.DeepEqual(got[[2]int{9, 1}], want) {
		t.Errorf("added sprite decodes to %+v, want %+v", got[[2]int{9, 1}], want)
	}
	assertRenderedLike(t, got, renderV1(t, sheet))
}

func TestAddV1Sprite_LinkedSpriteDecodesLikeItsTarget(t *testing.T) {
	sheet := v1EditSheet(t)
	out, err := AddV1Sprite(sheet, V1WriteSprite{Group: 9, Image: 2, Palette: v1TestPalette(), LinkedIndex: 4})
	if err != nil {
		t.Fatal(err)
	}
	got := renderV1(t, out)
	if got[[2]int{9, 2}].w != got[[2]int{0, 0}].w || got[[2]int{9, 2}].h != got[[2]int{0, 0}].h {
		t.Errorf("linked sprite should have the dimensions of sprite 4 (0,0)")
	}
}

func TestAddV1Sprite_Errors(t *testing.T) {
	sheet := v1EditSheet(t)
	data := mustEncodePCX(t, &PCXImage{Width: 1, Height: 1, Pixels: []byte{1}})
	cases := []struct {
		name string
		s    V1WriteSprite
		want string
	}{
		{"duplicate key", V1WriteSprite{Group: 5, Image: 0, PixelData: data, Palette: v1TestPalette()}, "already exists"},
		{"missing own palette", V1WriteSprite{Group: 9, Image: 1, PixelData: data}, "768-byte Palette"},
		{"link out of range", V1WriteSprite{Group: 9, Image: 1, Palette: v1TestPalette(), LinkedIndex: 7}, "out-of-range index 7"},
		{"negative link", V1WriteSprite{Group: 9, Image: 1, Palette: v1TestPalette(), LinkedIndex: -1}, "out-of-range"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := AddV1Sprite(sheet, c.s)
			if err == nil || !strings.Contains(err.Error(), c.want) || !strings.HasPrefix(err.Error(), "sff: v1: ") {
				t.Fatalf("err = %v, want prefix \"sff: v1: \" containing %q", err, c.want)
			}
		})
	}
}

func TestReplaceV1Sprite_ReplacesPixelsAndPaletteKeepingOthers(t *testing.T) {
	sheet := v1EditSheet(t)
	pal := make([]byte, V1PaletteBlockSize)
	pal[3*5], pal[3*5+1], pal[3*5+2] = 1, 2, 3 // index 5 -> (1,2,3)
	data := mustEncodePCX(t, &PCXImage{Width: 3, Height: 1, Pixels: []byte{5, 5, 5}})

	out, err := ReplaceV1Sprite(sheet, 0, 74, data, pal)
	if err != nil {
		t.Fatal(err)
	}
	got := renderV1(t, out)
	px := color.RGBA{R: 1, G: 2, B: 3, A: 255}
	want := v1Rendered{px: []color.RGBA{px, px, px}, w: 3, h: 1}
	if !reflect.DeepEqual(got[[2]int{0, 74}], want) {
		t.Errorf("replaced sprite decodes to %+v, want %+v", got[[2]int{0, 74}], want)
	}
	// Its zero-length copy follows it, with the new palette too.
	if !reflect.DeepEqual(got[[2]int{0, 75}], want) {
		t.Errorf("dependent (0,75) should follow the replaced owner, got %+v", got[[2]int{0, 75}])
	}
	assertRenderedLike(t, got, renderV1(t, sheet), [2]int{0, 74}, [2]int{0, 75})
	if &out[0] == &sheet[0] {
		t.Errorf("result shares the input's backing array")
	}
}

func TestReplaceV1Sprite_LinkedSpriteBecomesOwnerAndKeepsDependents(t *testing.T) {
	sheet := v1EditSheet(t)
	data := mustEncodePCX(t, &PCXImage{Width: 2, Height: 2, Pixels: []byte{1, 2, 3, 4}})

	out, err := ReplaceV1Sprite(sheet, 7, 7, data, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out[5].LinkedIndex != 0 || !bytes.Equal(out[5].PixelData, data) {
		t.Errorf("replaced linked sprite should own its pixels: %+v", out[5])
	}
	got := renderV1(t, out)
	if got[[2]int{7, 7}].w != 2 || got[[2]int{7, 7}].h != 2 {
		t.Errorf("replaced sprite is %dx%d, want 2x2", got[[2]int{7, 7}].w, got[[2]int{7, 7}].h)
	}
	if !reflect.DeepEqual(got[[2]int{7, 8}], got[[2]int{7, 7}]) {
		t.Errorf("dependent (7,8) should now draw the new pixels")
	}
	assertRenderedLike(t, got, renderV1(t, sheet), [2]int{7, 7}, [2]int{7, 8})
}

func TestReplaceV1Sprite_Errors(t *testing.T) {
	sheet := v1EditSheet(t)
	data := mustEncodePCX(t, &PCXImage{Width: 1, Height: 1, Pixels: []byte{1}})
	cases := []struct {
		name         string
		group, image int
		pixels, pal  []byte
		want         string
	}{
		{"not found", 99, 99, data, nil, "not found"},
		{"empty pixels", 0, 74, nil, nil, "pixel data is empty"},
		{"palette on shared sprite", 5, 1, data, v1TestPalette(), "shares its palette"},
		{"wrong palette size", 0, 74, data, []byte{1, 2, 3}, "must be 768 bytes"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ReplaceV1Sprite(sheet, c.group, c.image, c.pixels, c.pal)
			if err == nil || !strings.Contains(err.Error(), c.want) || !strings.HasPrefix(err.Error(), "sff: v1: ") {
				t.Fatalf("err = %v, want prefix \"sff: v1: \" containing %q", err, c.want)
			}
		})
	}
}

func TestRemoveV1Sprite_FirstSpriteWithZeroLengthCopy_CopyInheritsPixelsAndPalette(t *testing.T) {
	sheet := v1EditSheet(t)
	want := renderV1(t, sheet)

	out, err := RemoveV1Sprite(sheet, 0, 74)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 6 || out[0].Group != 0 || out[0].Image != 75 {
		t.Fatalf("expected (0,75) to become the first of 6 sprites, got %d sprites, first (%d,%d)", len(out), out[0].Group, out[0].Image)
	}
	// The copy inherited the removed owner's pixels and, since its shared
	// palette (the removed sprite's) is gone, that palette as its own block.
	if !bytes.Equal(out[0].PixelData, sheet[0].PixelData) {
		t.Errorf("(0,75) did not inherit the removed sprite's pixel data")
	}
	if out[0].SharedPalette || !bytes.Equal(out[0].Palette, sheet[0].Palette) {
		t.Errorf("(0,75) should own the removed sprite's palette block, shared=%v", out[0].SharedPalette)
	}
	got := renderV1(t, out)
	assertRenderedLike(t, got, want, [2]int{0, 74})
	if _, still := got[[2]int{0, 74}]; still {
		t.Errorf("removed sprite still present")
	}
}

func TestRemoveV1Sprite_OwnerWithLinkChain_FirstDependentInheritsAndOthersRelink(t *testing.T) {
	sheet := v1EditSheet(t)
	want := renderV1(t, sheet)

	out, err := RemoveV1Sprite(sheet, 5, 0) // owner of 5 <- 6 <- 7 chain; also sprite (5,1) shares its palette
	if err != nil {
		t.Fatal(err)
	}
	// Old indices 5,6 become 4,5; (7,7) inherits the data, (7,8) links to it.
	if out[4].Image != 7 || !bytes.Equal(out[4].PixelData, sheet[2].PixelData) || out[4].LinkedIndex != 0 {
		t.Errorf("(7,7) should own the removed sprite's data: %+v", out[4])
	}
	if len(out[5].PixelData) != 0 || out[5].LinkedIndex != 4 {
		t.Errorf("(7,8) should link to index 4, got LinkedIndex %d with %d data bytes", out[5].LinkedIndex, len(out[5].PixelData))
	}
	assertRenderedLike(t, renderV1(t, out), want, [2]int{5, 0})
}

func TestRemoveV1Sprite_MiddleOfChain_DependentRelinksToOwner(t *testing.T) {
	sheet := v1EditSheet(t)
	want := renderV1(t, sheet)

	out, err := RemoveV1Sprite(sheet, 7, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 6 || out[5].Image != 8 || out[5].LinkedIndex != 2 {
		t.Fatalf("(7,8) should link to index 2, got %d sprites, %+v", len(out), out[len(out)-1])
	}
	assertRenderedLike(t, renderV1(t, out), want, [2]int{7, 7})
}

func TestRemoveV1Sprite_LastSprite_LeavesOthersUntouched(t *testing.T) {
	sheet := v1EditSheet(t)
	want := renderV1(t, sheet)

	out, err := RemoveV1Sprite(sheet, 7, 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 6 || !reflect.DeepEqual(out, sheet[:6]) {
		t.Errorf("removing the last sprite should leave the first six unchanged")
	}
	assertRenderedLike(t, renderV1(t, out), want, [2]int{7, 8})
}

func TestRemoveV1Sprite_SpriteSharingRemovedPalette_KeepsItsColors(t *testing.T) {
	sheet := v1EditSheet(t)
	want := renderV1(t, sheet)

	out, err := RemoveV1Sprite(sheet, 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	// (5,1) shared (5,0)'s palette: it must now carry it itself.
	if out[2].Image != 1 || out[2].SharedPalette || !bytes.Equal(out[2].Palette, sheet[2].Palette) {
		t.Errorf("(5,1) should own the removed sprite's palette, got shared=%v", out[2].SharedPalette)
	}
	assertRenderedLike(t, renderV1(t, out), want, [2]int{5, 0})
}

func TestRemoveV1Sprite_DoesNotMutateInput(t *testing.T) {
	sheet := v1EditSheet(t)
	snapshot := append([]V1WriteSprite(nil), sheet...)
	if _, err := RemoveV1Sprite(sheet, 5, 0); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sheet, snapshot) {
		t.Errorf("input slice was modified")
	}
}

func TestRemoveV1Sprite_NotFound_ReturnsError(t *testing.T) {
	_, err := RemoveV1Sprite(v1EditSheet(t), 99, 1)
	if err == nil || !strings.HasPrefix(err.Error(), "sff: v1: ") || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("err = %v, want \"sff: v1: ... not found\"", err)
	}
}

func TestRemoveV1Sprite_UnresolvableRemovedSpriteWithDependent_ReturnsError(t *testing.T) {
	// Sprite 1 has no pixel data and a link readers ignore (index 0 means
	// "no link"), so it resolves to nothing; sprite 2 links to it.
	sheet := []V1WriteSprite{
		{Group: 0, Image: 0, PixelData: []byte{1}, Palette: v1TestPalette()},
		{Group: 0, Image: 1, Palette: v1TestPalette()},
		{Group: 0, Image: 2, Palette: v1TestPalette(), LinkedIndex: 1},
	}
	_, err := RemoveV1Sprite(sheet, 0, 1)
	if err == nil || !strings.Contains(err.Error(), "does not resolve") {
		t.Fatalf("err = %v, want a \"does not resolve\" error", err)
	}
}

// TestRemoveV1Sprite_RealFixture_RemainingSpriteMatchesOriginalFile pins the end-to-end
// property on an unmodified file: removing the trailing sprite of a real
// fixture and re-serializing keeps the remaining one pixel-identical to the
// original file.
func TestRemoveV1Sprite_RealFixture_RemainingSpriteMatchesOriginalFile(t *testing.T) {
	sprites := v1WriteSpritesFromFixture(t, "v1-zero-length-copy.sff")
	out, err := RemoveV1Sprite(sprites, 0, 75)
	if err != nil {
		t.Fatal(err)
	}
	data := serializeV1Sheet(t, out)
	got, gw, gh, err := ResolveSpritePixels(bytes.NewReader(data), 0, 74, nil)
	if err != nil {
		t.Fatal(err)
	}
	orig, err := os.Open("testdata/files/v1-zero-length-copy.sff")
	if err != nil {
		t.Fatal(err)
	}
	defer orig.Close()
	want, ww, wh, err := ResolveSpritePixels(orig, 0, 74, nil)
	if err != nil {
		t.Fatal(err)
	}
	if gw != ww || gh != wh || !reflect.DeepEqual(got, want) {
		t.Errorf("sprite (0,74) differs from the original fixture file")
	}
}

func TestRemoveV1Sprite_ZeroLengthCopyAfterRemovedSprite_StillSerializes(t *testing.T) {
	own := func(g int) V1WriteSprite {
		return V1WriteSprite{Group: g, PixelData: mustEncodePCX(t, &PCXImage{Width: 1, Height: 1, Pixels: []byte{byte(g + 1)}}), Palette: v1TestPalette()}
	}
	cases := []struct {
		name  string
		sheet []V1WriteSprite
	}{
		{"link becomes self-referencing", []V1WriteSprite{own(0), own(1), {Group: 2, SharedPalette: true, LinkedIndex: 1}}},
		{"link becomes out of range", []V1WriteSprite{own(0), own(1), {Group: 2, SharedPalette: true, LinkedIndex: 3}, own(3)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			want := renderV1(t, c.sheet)
			out, err := RemoveV1Sprite(c.sheet, 0, 0)
			if err != nil {
				t.Fatal(err)
			}
			got := renderV1(t, out) // serializes, re-parses and decodes
			assertRenderedLike(t, got, want, [2]int{0, 0})
			copyOut := out[1]
			if len(copyOut.PixelData) != 0 || copyOut.LinkedIndex == 1 || copyOut.LinkedIndex < 0 || copyOut.LinkedIndex >= len(out) {
				t.Errorf("zero-length copy has an invalid link: %+v", copyOut)
			}
		})
	}
}
