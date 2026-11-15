package sff

import (
	"errors"
	"fmt"
	"strings"
)

// This file holds the v2 sprite-table editing API: AddV2Sprite,
// ReplaceV2Sprite, RemoveV2Sprite, AddV2Palette and RemoveV2Palette. They
// operate on the write-side []V2WriteSprite / []V2WritePalette slices
// SerializeV2 consumes, not on a parsed V2SpriteTable — see
// .vibe/decisions/021-sprite-table-editing-api-shape.md.
//
// Aliasing contract (all functions): returned slices always have fresh
// backing arrays and the inputs are never modified. The byte slices
// (PixelData, ColorData) are shared by reference between input and result,
// never copied or mutated; callers must not modify them in place
// afterwards.

// V2SpritePixels is the pixel payload ReplaceV2Sprite swaps into a sprite:
// the encoded data plus the declared fields that describe how to decode it.
type V2SpritePixels struct {
	Width, Height int
	Format        int
	ColorDepth    int
	// PixelData is the already-encoded data (see EncodeV2Sprite); non-empty.
	PixelData []byte
}

func findV2Sprite(sprites []V2WriteSprite, group, image int) int {
	for i, s := range sprites {
		if s.Group == group && s.Image == image {
			return i
		}
	}
	return -1
}

func findV2Palette(palettes []V2WritePalette, group, number int) int {
	for i, p := range palettes {
		if p.Group == group && p.Number == number {
			return i
		}
	}
	return -1
}

// AddV2Sprite returns a copy of sprites with s appended at the end, so
// every existing LinkedIndex stays valid. It rejects a (Group, Image) key
// already present, a link (empty PixelData) whose LinkedIndex is not an
// existing sprite, and a PaletteIndex outside palettes (an empty palette
// table accepts index 0, as direct-color sprites carry one regardless).
func AddV2Sprite(sprites []V2WriteSprite, palettes []V2WritePalette, s V2WriteSprite) ([]V2WriteSprite, error) {
	if findV2Sprite(sprites, s.Group, s.Image) >= 0 {
		return nil, fmt.Errorf("sff: v2: sprite (group %d, image %d) already exists", s.Group, s.Image)
	}
	if len(s.PixelData) == 0 && (s.LinkedIndex < 0 || s.LinkedIndex >= len(sprites)) {
		return nil, fmt.Errorf("sff: v2: sprite links to out-of-range index %d (have %d sprites)", s.LinkedIndex, len(sprites))
	}
	if s.PaletteIndex < 0 || (s.PaletteIndex >= len(palettes) && !(len(palettes) == 0 && s.PaletteIndex == 0)) {
		return nil, fmt.Errorf("sff: v2: sprite palette index %d out of range (have %d palettes)", s.PaletteIndex, len(palettes))
	}
	out := make([]V2WriteSprite, len(sprites), len(sprites)+1)
	copy(out, sprites)
	return append(out, s), nil
}

// ReplaceV2Sprite returns a copy of sprites in which the sprite at (group,
// image) carries px as its own pixel data, dimensions and format. A sprite
// that was linked becomes an owner (LinkedIndex reset to 0). Key, axis and
// palette index are kept.
//
// Dependents of the replaced sprite are not touched: they keep following
// it and so decode the new data — with their own declared Width/Height,
// which the caller must keep consistent. It fails if no sprite has that
// key or px.PixelData is empty.
func ReplaceV2Sprite(sprites []V2WriteSprite, group, image int, px V2SpritePixels) ([]V2WriteSprite, error) {
	i := findV2Sprite(sprites, group, image)
	if i < 0 {
		return nil, fmt.Errorf("sff: v2: sprite (group %d, image %d) not found", group, image)
	}
	if len(px.PixelData) == 0 {
		return nil, errors.New("sff: v2: replacement pixel data is empty")
	}
	out := make([]V2WriteSprite, len(sprites))
	copy(out, sprites)
	s := out[i]
	s.Width, s.Height, s.Format, s.ColorDepth = px.Width, px.Height, px.Format, px.ColorDepth
	s.PixelData, s.LinkedIndex = px.PixelData, 0
	out[i] = s
	return out, nil
}

// RemoveV2Sprite returns a copy of sprites without the sprite at (group,
// image). Sprites linking to it (empty PixelData, LinkedIndex pointing at
// it) are repaired: if it owned pixel data, the first dependent in table
// order inherits its PixelData, Format, ColorDepth, Width and Height
// (the data is only decodable with them) and keeps its own key, axis and
// palette index, and the other dependents relink to that first one; if it
// was itself linked, its dependents relink to its own target. Every
// LinkedIndex is renumbered for the shifted indices, so chains stay
// intact. It fails if no sprite has that key.
func RemoveV2Sprite(sprites []V2WriteSprite, group, image int) ([]V2WriteSprite, error) {
	r := findV2Sprite(sprites, group, image)
	if r < 0 {
		return nil, fmt.Errorf("sff: v2: sprite (group %d, image %d) not found", group, image)
	}
	removed := sprites[r]
	isDep := func(k int) bool {
		return k != r && len(sprites[k].PixelData) == 0 && sprites[k].LinkedIndex == r
	}
	first := -1
	target := removed.LinkedIndex
	if len(removed.PixelData) > 0 {
		for k := range sprites {
			if isDep(k) {
				first = k
				break
			}
		}
		target = first
	}
	newIdx := func(i int) int {
		if i > r {
			return i - 1
		}
		return i
	}
	out := make([]V2WriteSprite, 0, len(sprites)-1)
	for k, s := range sprites {
		switch {
		case k == r:
			continue
		case k == first:
			s.PixelData, s.Format, s.ColorDepth = removed.PixelData, removed.Format, removed.ColorDepth
			s.Width, s.Height, s.LinkedIndex = removed.Width, removed.Height, 0
		case isDep(k):
			s.LinkedIndex = newIdx(target)
		case len(s.PixelData) == 0:
			s.LinkedIndex = newIdx(s.LinkedIndex)
		}
		out = append(out, s)
	}
	return out, nil
}

// AddV2Palette returns a copy of palettes with p appended at the end, so
// existing PaletteIndex and LinkedIndex values stay valid. It rejects a
// (Group, Number) key already present and a link (empty ColorData) whose
// LinkedIndex is not an existing bank.
func AddV2Palette(palettes []V2WritePalette, p V2WritePalette) ([]V2WritePalette, error) {
	if findV2Palette(palettes, p.Group, p.Number) >= 0 {
		return nil, fmt.Errorf("sff: v2: palette (group %d, number %d) already exists", p.Group, p.Number)
	}
	if len(p.ColorData) == 0 && (p.LinkedIndex < 0 || p.LinkedIndex >= len(palettes)) {
		return nil, fmt.Errorf("sff: v2: palette links to out-of-range index %d (have %d palettes)", p.LinkedIndex, len(palettes))
	}
	out := make([]V2WritePalette, len(palettes), len(palettes)+1)
	copy(out, palettes)
	return append(out, p), nil
}

// RemoveV2Palette returns copies of palettes and sprites without the bank
// at (group, number). A bank that sprites still draw with is rejected, not
// repaired — there is no right replacement to pick — and the error lists
// those sprites; remove or re-point them first. Banks linking to the
// removed one are repaired like RemoveV2Sprite does (first dependent
// inherits ColorData and ColorCount, the rest relink to it; a removed
// linked bank sends its dependents to its own target). Palette links and
// every sprite's PaletteIndex are renumbered. It fails if no bank has that
// key.
func RemoveV2Palette(palettes []V2WritePalette, sprites []V2WriteSprite, group, number int) ([]V2WritePalette, []V2WriteSprite, error) {
	r := findV2Palette(palettes, group, number)
	if r < 0 {
		return nil, nil, fmt.Errorf("sff: v2: palette (group %d, number %d) not found", group, number)
	}
	var users []string
	for _, s := range sprites {
		if s.PaletteIndex == r {
			users = append(users, fmt.Sprintf("(group %d, image %d)", s.Group, s.Image))
		}
	}
	if len(users) > 0 {
		return nil, nil, fmt.Errorf("sff: v2: cannot remove palette (group %d, number %d): still used by sprites %s", group, number, strings.Join(users, ", "))
	}
	removed := palettes[r]
	isDep := func(k int) bool {
		return k != r && len(palettes[k].ColorData) == 0 && palettes[k].LinkedIndex == r
	}
	first := -1
	target := removed.LinkedIndex
	if len(removed.ColorData) > 0 {
		for k := range palettes {
			if isDep(k) {
				first = k
				break
			}
		}
		target = first
	}
	newIdx := func(i int) int {
		if i > r {
			return i - 1
		}
		return i
	}
	outP := make([]V2WritePalette, 0, len(palettes)-1)
	for k, p := range palettes {
		switch {
		case k == r:
			continue
		case k == first:
			p.ColorData, p.ColorCount, p.LinkedIndex = removed.ColorData, removed.ColorCount, 0
		case isDep(k):
			p.LinkedIndex = newIdx(target)
		case len(p.ColorData) == 0:
			p.LinkedIndex = newIdx(p.LinkedIndex)
		}
		outP = append(outP, p)
	}
	outS := make([]V2WriteSprite, len(sprites))
	for k, s := range sprites {
		s.PaletteIndex = newIdx(s.PaletteIndex)
		outS[k] = s
	}
	return outP, outS, nil
}
