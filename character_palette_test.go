package sff

import (
	"bytes"
	"image/color"
	"testing"
)

// characterSheet is an SFF v1 sheet of four 1x1 sprites of palette index 1:
// 0,0 and 0,1 on the first palette (red at index 1), 1,0 with its own
// (green at index 1) and 1,1 sharing that one.
func characterSheet(t *testing.T) *bytes.Reader {
	t.Helper()
	px, err := EncodePCX(&PCXImage{Width: 1, Height: 1, BytesPerPixel: 1, Pixels: []byte{1}})
	if err != nil {
		t.Fatal(err)
	}
	palette := func(c color.RGBA) []byte {
		p := make([]byte, V1PaletteBlockSize)
		p[3], p[4], p[5] = c.R, c.G, c.B
		return p
	}
	red, green := color.RGBA{R: 255, A: 255}, color.RGBA{G: 255, A: 255}
	sprites := []V1WriteSprite{
		{Group: 0, Image: 0, PixelData: px, Palette: palette(red)},
		{Group: 0, Image: 1, PixelData: px, SharedPalette: true},
		{Group: 1, Image: 0, PixelData: px, Palette: palette(green)},
		{Group: 1, Image: 1, PixelData: px, SharedPalette: true},
	}
	var buf bytes.Buffer
	if err := SerializeV1(&buf, [4]byte{0, 1, 0, 1}, false, sprites); err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(buf.Bytes())
}

// Ikemen GO (char.go:4762-4800, commit 54d4148a) puts an SFF v1
// character's selected .act in place of its first palette: the sprites that
// use it take the .act's colours, those with their own palette keep it.
func TestResolveCharacterSpritePixels_TheActReplacesOnlyTheFirstPalette(t *testing.T) {
	sheet := characterSheet(t)
	var act Palette
	act[1] = color.RGBA{B: 255, A: 255}
	for _, c := range []struct {
		group, image int
		want         color.RGBA
	}{
		{0, 0, color.RGBA{B: 255, A: 255}}, {0, 1, color.RGBA{B: 255, A: 255}},
		{1, 0, color.RGBA{G: 255, A: 255}}, {1, 1, color.RGBA{G: 255, A: 255}},
	} {
		px, _, _, err := ResolveCharacterSpritePixels(sheet, c.group, c.image, &act)
		if err != nil {
			t.Fatal(err)
		}
		if px[0] != c.want {
			t.Errorf("sprite %d,%d = %v, want %v", c.group, c.image, px[0], c.want)
		}
	}
}

func TestResolveCharacterSpritePixels_WithoutAnActIsResolveSpritePixels(t *testing.T) {
	sheet := characterSheet(t)
	px, _, _, err := ResolveCharacterSpritePixels(sheet, 0, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if px[0] != (color.RGBA{R: 255, A: 255}) {
		t.Errorf("sprite 0,1 = %v, want its own red", px[0])
	}
	if _, _, _, err := ResolveCharacterSpritePixels(sheet, 9, 9, nil); err == nil {
		t.Error("a missing sprite must be an error")
	}
}
