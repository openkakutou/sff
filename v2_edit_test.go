package sff

import (
	"bytes"
	"image/color"
	"reflect"
	"strings"
	"testing"
)

// v2FromFixture rebuilds the write slices of a real, unmodified v2 fixture.
func v2FromFixture(t *testing.T, name string) ([]V2WriteSprite, []V2WritePalette) {
	t.Helper()
	f := openTestdataFile(t, name)
	defer f.Close()
	table, err := ParseV2(f)
	if err != nil {
		t.Fatalf("ParseV2(%s): %v", name, err)
	}
	var sprites []V2WriteSprite
	for _, e := range table.Sprites {
		s := V2WriteSprite{Group: e.Group, Image: e.Image, Width: e.Width, Height: e.Height, AxisX: e.AxisX, AxisY: e.AxisY,
			Format: e.Format, ColorDepth: e.ColorDepth, PaletteIndex: e.PaletteIndex, LinkedIndex: e.LinkedIndex}
		if e.Length > 0 {
			s.PixelData = make([]byte, e.Length)
			if _, err := f.ReadAt(s.PixelData, e.Offset); err != nil {
				t.Fatal(err)
			}
		}
		sprites = append(sprites, s)
	}
	var palettes []V2WritePalette
	for _, e := range table.Palettes {
		p := V2WritePalette{Group: e.Group, Number: e.Number, ColorCount: e.ColorCount, LinkedIndex: e.LinkedIndex}
		if e.Length > 0 {
			p.ColorData = make([]byte, e.Length)
			if _, err := f.ReadAt(p.ColorData, e.Offset); err != nil {
				t.Fatal(err)
			}
		}
		palettes = append(palettes, p)
	}
	return sprites, palettes
}

type v2Rendered struct {
	px   []color.RGBA
	w, h int
}

// renderV2 serializes, re-parses and decodes every sprite to RGBA, keyed by
// (group, image). A linked sprite is decoded from the pixel data of the
// sprite its chain ends at, with that sprite's format and dimensions, and
// its own palette bank.
func renderV2(t *testing.T, sprites []V2WriteSprite, palettes []V2WritePalette) map[[2]int]v2Rendered {
	t.Helper()
	var buf bytes.Buffer
	if err := SerializeV2(&buf, [4]byte{0, 1, 0, 2}, sprites, palettes); err != nil {
		t.Fatalf("SerializeV2: %v", err)
	}
	r := bytes.NewReader(buf.Bytes())
	table, err := ParseV2(r)
	if err != nil {
		t.Fatalf("re-parsing: %v", err)
	}
	out := map[[2]int]v2Rendered{}
	for i, e := range table.Sprites {
		owner := table.Sprites[i]
		for hops := 0; owner.Length == 0; hops++ {
			if hops > len(table.Sprites) {
				t.Fatalf("link cycle from sprite %d", i)
			}
			owner = table.Sprites[owner.LinkedIndex]
		}
		raw := make([]byte, owner.Length)
		if _, err := r.ReadAt(raw, owner.Offset); err != nil {
			t.Fatal(err)
		}
		img, err := DecodeV2Sprite(owner.Format, owner.Width, owner.Height, owner.ColorDepth, raw)
		if err != nil {
			t.Fatalf("decoding sprite %d: %v", i, err)
		}
		pal, err := ResolveV2Palette(table, r, e.PaletteIndex, nil)
		if err != nil {
			t.Fatalf("palette of sprite %d: %v", i, err)
		}
		rule := AlphaForceTransparentAtIndexZero
		if owner.Format == V2FormatRLE8 || owner.Format == V2FormatLZ5 {
			rule = AlphaLiteral
		}
		out[[2]int{e.Group, e.Image}] = v2Rendered{ResolvePixels(img.Pixels, pal, rule), img.Width, img.Height}
	}
	return out
}

func assertV2RenderedLike(t *testing.T, got, want map[[2]int]v2Rendered, skip ...[2]int) {
	t.Helper()
	skipped := map[[2]int]bool{}
	for _, k := range skip {
		skipped[k] = true
	}
	for k, w := range want {
		if skipped[k] {
			continue
		}
		if g, ok := got[k]; !ok || !reflect.DeepEqual(g, w) {
			t.Errorf("sprite %v missing or decodes differently after edit", k)
		}
	}
}

// v2EditSheet combines real fixtures into one sheet:
//
//	sprites: 0 (9000,0) rle8/pal0   1 (0,0) lz5/pal1   2 (9000,1) png8/pal2
//	         3 (0,5) png8/pal3      4 (7,1) -> 0, pal0   5 (7,2) -> 4, pal 7
//	palettes: 0..3 own banks from the fixtures, 4 (2,2) own copy of bank 0's
//	          colors, 5 (1,9) -> 4, 6 (1,10) -> 5, 7 (1,11) -> 6
//
// Sprite 5 draws with bank 7, which resolves (through three links) to bank
// 4's data, equal to bank 0's colors.
func v2EditSheet(t *testing.T) ([]V2WriteSprite, []V2WritePalette) {
	t.Helper()
	var sprites []V2WriteSprite
	var palettes []V2WritePalette
	for i, name := range []string{"v2-rle8.sff", "v2-lz5.sff", "v2-png8-b.sff", "v2-png8.sff"} {
		s, p := v2FromFixture(t, name)
		s[0].PaletteIndex = i
		p[0].Number = 100 + i
		sprites = append(sprites, s[0])
		palettes = append(palettes, p[0])
	}
	sprites[3].Image = 5
	palettes = append(palettes, V2WritePalette{Group: 2, Number: 2, ColorCount: palettes[0].ColorCount, ColorData: palettes[0].ColorData},
		V2WritePalette{Group: 1, Number: 9, LinkedIndex: 4},
		V2WritePalette{Group: 1, Number: 10, LinkedIndex: 5},
		V2WritePalette{Group: 1, Number: 11, LinkedIndex: 6})
	sprites = append(sprites,
		V2WriteSprite{Group: 7, Image: 1, Width: 25, Height: 25, LinkedIndex: 0},
		V2WriteSprite{Group: 7, Image: 2, Width: 25, Height: 25, LinkedIndex: 4, PaletteIndex: 7})
	return sprites, palettes
}

func TestV2EditSheet_OwnersMatchRealFixtureDecoding(t *testing.T) {
	sprites, palettes := v2EditSheet(t)
	got := renderV2(t, sprites, palettes)
	f := openTestdataFile(t, "v2-rle8.sff")
	defer f.Close()
	px, w, h, err := ResolveSpritePixels(f, 9000, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got[[2]int{9000, 0}], v2Rendered{px, w, h}) {
		t.Errorf("rebuilt rle8 sprite differs from the original fixture's decoding")
	}
	// The linked sprite 5 draws sprite 0's pixels through bank 7 == bank 0's colors.
	if !reflect.DeepEqual(got[[2]int{7, 2}], got[[2]int{9000, 0}]) {
		t.Errorf("linked sprite via linked palette should decode like sprite 0")
	}
}

func TestAddV2Sprite_AppendsAndDecodes(t *testing.T) {
	sprites, palettes := v2EditSheet(t)
	img := &V2Image{Width: 2, Height: 2, BytesPerPixel: 1, Pixels: []byte{1, 2, 3, 4}}
	s := V2WriteSprite{Group: 9, Image: 9, Width: 2, Height: 2, Format: V2FormatRaw, ColorDepth: 8, PaletteIndex: 2,
		PixelData: mustEncodeV2Sprite(t, V2FormatRaw, img)}

	out, err := AddV2Sprite(sprites, palettes, s)
	if err != nil {
		t.Fatal(err)
	}
	if len(sprites) != 6 || len(out) != 7 || out[6].Group != 9 {
		t.Fatalf("lens in/out = %d/%d, last group %d", len(sprites), len(out), out[len(out)-1].Group)
	}
	got := renderV2(t, out, palettes)
	// Bank 2 is the png8-b fixture's own colors, read straight from its file.
	f := openTestdataFile(t, "v2-png8-b.sff")
	defer f.Close()
	fixTable, err := ParseV2(f)
	if err != nil {
		t.Fatal(err)
	}
	pal, err := ResolveV2Palette(fixTable, f, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := v2Rendered{px: []color.RGBA{pal[1], pal[2], pal[3], pal[4]}, w: 2, h: 2}
	if !reflect.DeepEqual(got[[2]int{9, 9}], want) {
		t.Errorf("added sprite decodes to %+v, want %+v", got[[2]int{9, 9}], want)
	}
	assertV2RenderedLike(t, got, renderV2(t, sprites, palettes))
}

func TestAddV2Sprite_Errors(t *testing.T) {
	sprites, palettes := v2EditSheet(t)
	data := []byte{1}
	cases := []struct {
		name string
		s    V2WriteSprite
		want string
	}{
		{"duplicate key", V2WriteSprite{Group: 7, Image: 1, PixelData: data}, "already exists"},
		{"link out of range", V2WriteSprite{Group: 9, Image: 9, LinkedIndex: 6}, "out-of-range index 6"},
		{"palette out of range", V2WriteSprite{Group: 9, Image: 9, PixelData: data, PaletteIndex: 8}, "palette index 8"},
		{"negative palette", V2WriteSprite{Group: 9, Image: 9, PixelData: data, PaletteIndex: -1}, "palette index -1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := AddV2Sprite(sprites, palettes, c.s)
			if err == nil || !strings.HasPrefix(err.Error(), "sff: v2: ") || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want \"sff: v2: \" containing %q", err, c.want)
			}
		})
	}
	if _, err := AddV2Sprite(nil, nil, V2WriteSprite{Group: 1, PixelData: data}); err != nil {
		t.Errorf("a direct-color style sprite with palette index 0 and no palettes should be accepted: %v", err)
	}
}

func TestReplaceV2Sprite_ReplacesDataAndDimensions(t *testing.T) {
	sprites, palettes := v2EditSheet(t)
	img := &V2Image{Width: 3, Height: 1, BytesPerPixel: 1, Pixels: []byte{2, 3, 4}}
	px := V2SpritePixels{Width: 3, Height: 1, Format: V2FormatRaw, ColorDepth: 8, PixelData: mustEncodeV2Sprite(t, V2FormatRaw, img)}

	out, err := ReplaceV2Sprite(sprites, 0, 0, px) // lz5 sprite, bank 1
	if err != nil {
		t.Fatal(err)
	}
	got := renderV2(t, out, palettes)
	if g := got[[2]int{0, 0}]; g.w != 3 || g.h != 1 || len(g.px) != 3 {
		t.Errorf("replaced sprite is %dx%d with %d pixels, want 3x1", g.w, g.h, len(g.px))
	}
	assertV2RenderedLike(t, got, renderV2(t, sprites, palettes), [2]int{0, 0})
	if sprites[1].Width == 3 {
		t.Errorf("input was modified")
	}
}

func TestReplaceV2Sprite_LinkedSpriteBecomesOwnerAndDependentsFollow(t *testing.T) {
	sprites, palettes := v2EditSheet(t)
	img := &V2Image{Width: 25, Height: 25, BytesPerPixel: 1, Pixels: bytes.Repeat([]byte{3}, 625)}
	px := V2SpritePixels{Width: 25, Height: 25, Format: V2FormatRaw, ColorDepth: 8, PixelData: mustEncodeV2Sprite(t, V2FormatRaw, img)}

	out, err := ReplaceV2Sprite(sprites, 7, 1, px) // (7,1) was linked to 0; (7,2) links to it
	if err != nil {
		t.Fatal(err)
	}
	if out[4].LinkedIndex != 0 || len(out[4].PixelData) == 0 || out[4].Format != V2FormatRaw {
		t.Errorf("replaced sprite should own raw data: %+v", out[4])
	}
	got := renderV2(t, out, palettes)
	for _, p := range got[[2]int{7, 2}].px {
		if p != got[[2]int{7, 2}].px[0] {
			t.Fatalf("dependent (7,2) should be a flat fill of the new pixels")
		}
	}
	assertV2RenderedLike(t, got, renderV2(t, sprites, palettes), [2]int{7, 1}, [2]int{7, 2})
}

func TestReplaceV2Sprite_Errors(t *testing.T) {
	sprites, _ := v2EditSheet(t)
	if _, err := ReplaceV2Sprite(sprites, 99, 99, V2SpritePixels{PixelData: []byte{1}}); err == nil || !strings.HasPrefix(err.Error(), "sff: v2: ") || !strings.Contains(err.Error(), "not found") {
		t.Errorf("not found: err = %v", err)
	}
	if _, err := ReplaceV2Sprite(sprites, 0, 0, V2SpritePixels{}); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Errorf("empty data: err = %v", err)
	}
}

func TestRemoveV2Sprite_OwnerWithChain_FirstDependentInheritsAndOthersRelink(t *testing.T) {
	sprites, palettes := v2EditSheet(t)
	want := renderV2(t, sprites, palettes)

	out, err := RemoveV2Sprite(sprites, 9000, 0) // first sprite, owner of 4 <- 5
	if err != nil {
		t.Fatal(err)
	}
	// (7,1) moves 4 -> 3 and inherits; (7,2) moves 5 -> 4 and links to 3.
	if out[3].Image != 1 || !bytes.Equal(out[3].PixelData, sprites[0].PixelData) || out[3].Format != sprites[0].Format {
		t.Errorf("(7,1) did not inherit the removed sprite's data: %+v", out[3])
	}
	if out[3].Width != 25 || out[3].Height != 25 || out[3].ColorDepth != 5 || out[3].LinkedIndex != 0 {
		t.Errorf("(7,1) inherited fields wrong: %+v", out[3])
	}
	if len(out[4].PixelData) != 0 || out[4].LinkedIndex != 3 || out[4].PaletteIndex != 7 {
		t.Errorf("(7,2) should link to 3 keeping its palette index: %+v", out[4])
	}
	assertV2RenderedLike(t, renderV2(t, out, palettes), want, [2]int{9000, 0})
}

func TestRemoveV2Sprite_MiddleOfChain_DependentRelinksToOwner(t *testing.T) {
	sprites, palettes := v2EditSheet(t)
	want := renderV2(t, sprites, palettes)

	out, err := RemoveV2Sprite(sprites, 7, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 5 || out[4].Image != 2 || out[4].LinkedIndex != 0 {
		t.Fatalf("(7,2) should link to index 0: %+v", out[len(out)-1])
	}
	assertV2RenderedLike(t, renderV2(t, out, palettes), want, [2]int{7, 1})
}

func TestRemoveV2Sprite_MiddleOwnerWithoutDependents_RenumbersLinks(t *testing.T) {
	sprites, palettes := v2EditSheet(t)
	want := renderV2(t, sprites, palettes)

	out, err := RemoveV2Sprite(sprites, 9000, 1) // index 2, below both linked sprites
	if err != nil {
		t.Fatal(err)
	}
	if out[3].LinkedIndex != 0 || out[4].LinkedIndex != 3 {
		t.Errorf("links should be 0 and 3, got %d and %d", out[3].LinkedIndex, out[4].LinkedIndex)
	}
	assertV2RenderedLike(t, renderV2(t, out, palettes), want, [2]int{9000, 1})
}

func TestRemoveV2Sprite_LastSprite_LeavesOthersIdentical(t *testing.T) {
	sprites, _ := v2EditSheet(t)
	out, err := RemoveV2Sprite(sprites, 7, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out, sprites[:5]) {
		t.Errorf("first five sprites should be unchanged")
	}
}

func TestRemoveV2Sprite_NotFound_ReturnsError(t *testing.T) {
	sprites, _ := v2EditSheet(t)
	_, err := RemoveV2Sprite(sprites, 1, 1)
	if err == nil || !strings.HasPrefix(err.Error(), "sff: v2: ") || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("err = %v", err)
	}
}

func TestAddV2Palette_AppendsAndRejectsBadInput(t *testing.T) {
	_, palettes := v2EditSheet(t)
	out, err := AddV2Palette(palettes, V2WritePalette{Group: 3, Number: 3, ColorCount: 1, ColorData: []byte{1, 2, 3, 255}})
	if err != nil || len(out) != 9 || len(palettes) != 8 {
		t.Fatalf("err=%v len out/in = %d/%d", err, len(out), len(palettes))
	}
	if _, err := AddV2Palette(palettes, V2WritePalette{Group: 2, Number: 2, ColorData: []byte{1}}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("duplicate: err = %v", err)
	}
	if _, err := AddV2Palette(palettes, V2WritePalette{Group: 3, Number: 3, LinkedIndex: 8}); err == nil || !strings.HasPrefix(err.Error(), "sff: v2: ") || !strings.Contains(err.Error(), "out-of-range index 8") {
		t.Errorf("link range: err = %v", err)
	}
}

func TestRemoveV2Palette_UnusedOwnerWithLinkChain_RepairsAndRenumbersSpritePalettes(t *testing.T) {
	sprites, palettes := v2EditSheet(t)
	want := renderV2(t, sprites, palettes)

	// Bank 4 owns the colors that banks 5 <- 6 <- 7 chain to; sprite 5
	// draws with bank 7.
	outP, outS, err := RemoveV2Palette(palettes, sprites, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(outP) != 7 {
		t.Fatalf("got %d banks, want 7", len(outP))
	}
	// Old bank 5 (1,9) inherits the data; old 6 -> relinks to it; old 7 -> 6 keeps pointing at old 6.
	if outP[4].Number != 9 || !bytes.Equal(outP[4].ColorData, palettes[4].ColorData) || outP[4].ColorCount != palettes[4].ColorCount || outP[4].LinkedIndex != 0 {
		t.Errorf("bank (1,9) did not inherit: %+v", outP[4])
	}
	if len(outP[5].ColorData) != 0 || outP[5].LinkedIndex != 4 || len(outP[6].ColorData) != 0 || outP[6].LinkedIndex != 5 {
		t.Errorf("links wrong: %+v %+v", outP[5], outP[6])
	}
	if outS[5].PaletteIndex != 6 || outS[0].PaletteIndex != 0 {
		t.Errorf("sprite palette indices: sprite 5 -> %d (want 6), sprite 0 -> %d (want 0)", outS[5].PaletteIndex, outS[0].PaletteIndex)
	}
	assertV2RenderedLike(t, renderV2(t, outS, outP), want)
}

func TestRemoveV2Palette_MiddleOfChain_DependentRelinksToTarget(t *testing.T) {
	sprites, palettes := v2EditSheet(t)
	want := renderV2(t, sprites, palettes)
	outP, outS, err := RemoveV2Palette(palettes, sprites, 1, 9) // bank 5: links 4, linked by bank 6
	if err != nil {
		t.Fatal(err)
	}
	if outP[5].Number != 10 || outP[5].LinkedIndex != 4 || outS[5].PaletteIndex != 6 {
		t.Errorf("(1,10) should link to bank 4 and sprite 5 use bank 6: %+v, %d", outP[5], outS[5].PaletteIndex)
	}
	assertV2RenderedLike(t, renderV2(t, outS, outP), want)
}

func TestRemoveV2Palette_Errors(t *testing.T) {
	sprites, palettes := v2EditSheet(t)
	_, _, err := RemoveV2Palette(palettes, sprites, 1, 100) // bank 0, used by (9000,0) and (7,1)
	if err == nil || !strings.HasPrefix(err.Error(), "sff: v2: ") || !strings.Contains(err.Error(), "(group 9000, image 0)") || !strings.Contains(err.Error(), "(group 7, image 1)") {
		t.Errorf("in use: err = %v, want both users listed", err)
	}
	if _, _, err := RemoveV2Palette(palettes, sprites, 5, 5); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("not found: err = %v", err)
	}
}

// TestRemoveV2_RealFixtures_SpriteAndUnusedPalette_RemainingSpritesMatchOriginalFiles
// builds a sheet purely from real, unmodified fixtures, removes a
// non-linked sprite and then its now-unused palette bank, and compares the
// remaining sprites with the original files' own decoding.
func TestRemoveV2_RealFixtures_SpriteAndUnusedPalette_RemainingSpritesMatchOriginalFiles(t *testing.T) {
	var sprites []V2WriteSprite
	var palettes []V2WritePalette
	for i, name := range []string{"v2-rle8.sff", "v2-png8-b.sff", "v2-lz5.sff"} {
		s, p := v2FromFixture(t, name)
		s[0].PaletteIndex = i
		sprites = append(sprites, s[0])
		palettes = append(palettes, p[0])
	}
	palettes[2].Number = 7 // the rle8 and lz5 fixtures both use (1,1)

	sprites, err := RemoveV2Sprite(sprites, 9000, 1)
	if err != nil {
		t.Fatal(err)
	}
	palettes, sprites, err = RemoveV2Palette(palettes, sprites, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(sprites) != 2 || len(palettes) != 2 || sprites[1].PaletteIndex != 1 {
		t.Fatalf("expected 2 sprites, 2 banks, lz5 sprite on bank 1; got %d, %d, %d", len(sprites), len(palettes), sprites[1].PaletteIndex)
	}

	got := renderV2(t, sprites, palettes)
	for _, c := range []struct {
		file         string
		group, image int
	}{{"v2-rle8.sff", 9000, 0}, {"v2-lz5.sff", 0, 0}} {
		f := openTestdataFile(t, c.file)
		px, w, h, err := ResolveSpritePixels(f, c.group, c.image, nil)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got[[2]int{c.group, c.image}], v2Rendered{px, w, h}) {
			t.Errorf("%s sprite differs from the original file after edits", c.file)
		}
	}
}
