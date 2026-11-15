package sff

import (
	"bytes"
	"errors"
	"fmt"
)

// This file holds the v1 sprite-table editing API: AddV1Sprite,
// ReplaceV1Sprite and RemoveV1Sprite. They operate on the write-side
// []V1WriteSprite slice SerializeV1 consumes, not on a parsed
// V1SpriteTable — see
// .vibe/decisions/021-sprite-table-editing-api-shape.md.
//
// Aliasing contract (all three): the returned slice always has a fresh
// backing array and the input slice is never modified. The byte slices a
// V1WriteSprite holds (PixelData, Palette) are shared by reference between
// the input and the result, never copied or mutated; callers must not
// modify them in place afterwards.
//
// The slice's linking semantics are the ones ParseV1 + ResolveV1Pixels /
// ResolveV1Palette apply when the serialized file is read back (see
// .vibe/decisions/011-v1-sprite-linking-and-palette-inheritance-rules.md and
// 015): a sprite with SharedPalette true and no PixelData is written with
// Length 0 and always copies the previous sprite's pixels and palette
// (LinkedIndex is not consulted); a sprite with a non-empty own span whose
// LinkedIndex is a backward, non-zero index uses that sprite's pixels.

// v1Length is the subheader Length SerializeV1 writes for s.
func v1Length(s V1WriteSprite) int {
	n := len(s.PixelData)
	if !s.SharedPalette {
		n += V1PaletteBlockSize
	}
	return n
}

func v1Entry(s V1WriteSprite) V1SpriteEntry {
	return V1SpriteEntry{
		Group:         s.Group,
		Image:         s.Image,
		Length:        v1Length(s),
		LinkedIndex:   s.LinkedIndex,
		SharedPalette: s.SharedPalette,
	}
}

func v1TableOf(sprites []V1WriteSprite) *V1SpriteTable {
	t := &V1SpriteTable{Sprites: make([]V1SpriteEntry, len(sprites))}
	for i, s := range sprites {
		t.Sprites[i] = v1Entry(s)
	}
	return t
}

// v1Resolved is what a reader decodes for one sprite: the PCX bytes it
// draws from, the 768-byte palette block it uses (nil when empty), and the
// index of the sprite owning those pixels.
type v1Resolved struct {
	pix, pal []byte
	owner    int
	ok       bool
}

// v1Resolve replicates, on the write slice, what the readers resolve for
// sprite k: pixel owner via v1PixelOwner, palette via v1PaletteSource.
func v1Resolve(sprites []V1WriteSprite, table *V1SpriteTable, k int) v1Resolved {
	owner, err := v1PixelOwner(table, k, nil)
	if err != nil || len(sprites[owner].PixelData) == 0 {
		return v1Resolved{}
	}
	src := k
	for {
		next := v1PaletteSource(table, src)
		if next == src {
			break
		}
		src = next
	}
	var pal []byte
	s := sprites[src]
	switch {
	case !s.SharedPalette:
		pal = s.Palette
	case len(s.PixelData) >= V1PaletteBlockSize:
		// Only reachable for sprite 0 marked shared: the reader takes the
		// last block of its declared span as its palette.
		pal = s.PixelData[len(s.PixelData)-V1PaletteBlockSize:]
	}
	return v1Resolved{pix: sprites[owner].PixelData, pal: pal, owner: owner, ok: true}
}

func findV1Sprite(sprites []V1WriteSprite, group, image int) int {
	for i, s := range sprites {
		if s.Group == group && s.Image == image {
			return i
		}
	}
	return -1
}

// AddV1Sprite returns a copy of sprites with s appended at the end, so
// every existing LinkedIndex stays valid. It rejects what SerializeV1
// would reject (a non-shared sprite without a 768-byte Palette, a link
// that is out of range or self-referencing) and a (Group, Image) key
// already present.
func AddV1Sprite(sprites []V1WriteSprite, s V1WriteSprite) ([]V1WriteSprite, error) {
	if findV1Sprite(sprites, s.Group, s.Image) >= 0 {
		return nil, fmt.Errorf("sff: v1: sprite (group %d, image %d) already exists", s.Group, s.Image)
	}
	if !s.SharedPalette && len(s.Palette) != V1PaletteBlockSize {
		return nil, fmt.Errorf("sff: v1: sprite does not share a palette and needs its own %d-byte Palette, got %d bytes", V1PaletteBlockSize, len(s.Palette))
	}
	if len(s.PixelData) == 0 && (s.LinkedIndex < 0 || s.LinkedIndex >= len(sprites)) {
		return nil, fmt.Errorf("sff: v1: sprite links to out-of-range index %d (have %d sprites)", s.LinkedIndex, len(sprites))
	}
	out := make([]V1WriteSprite, len(sprites), len(sprites)+1)
	copy(out, sprites)
	return append(out, s), nil
}

// ReplaceV1Sprite returns a copy of sprites in which the sprite at (group,
// image) draws pixelData (non-empty PCX bytes) instead. A sprite that was
// linked becomes an owner of its own pixels (LinkedIndex reset to 0, which
// is "no link" for readers). palette, when non-nil, must be 768 bytes and
// replaces the sprite's own palette block; it is rejected for a sprite
// with SharedPalette true, which has none. Axis and key are kept.
//
// Dependents of the replaced sprite are not touched: they keep following
// it, so they now draw the new pixels, and sprites sharing its palette get
// the new palette. It fails if no sprite has that key.
func ReplaceV1Sprite(sprites []V1WriteSprite, group, image int, pixelData, palette []byte) ([]V1WriteSprite, error) {
	i := findV1Sprite(sprites, group, image)
	if i < 0 {
		return nil, fmt.Errorf("sff: v1: sprite (group %d, image %d) not found", group, image)
	}
	if len(pixelData) == 0 {
		return nil, errors.New("sff: v1: replacement pixel data is empty")
	}
	s := sprites[i]
	if palette != nil {
		if s.SharedPalette {
			return nil, fmt.Errorf("sff: v1: sprite (group %d, image %d) shares its palette and cannot take its own", group, image)
		}
		if len(palette) != V1PaletteBlockSize {
			return nil, fmt.Errorf("sff: v1: replacement palette must be %d bytes, got %d", V1PaletteBlockSize, len(palette))
		}
		s.Palette = palette
	}
	s.PixelData = pixelData
	s.LinkedIndex = 0
	out := make([]V1WriteSprite, len(sprites))
	copy(out, sprites)
	out[i] = s
	return out, nil
}

// RemoveV1Sprite returns a copy of sprites without the sprite at (group,
// image), repairing every sprite that depended on it so each remaining
// sprite still resolves to exactly the pixels and palette it did before:
//
//   - If the removed sprite owned pixel data, the first sprite (in table
//     order) that resolved to it inherits its PixelData, and the other
//     sprites that linked to it directly relink to that first one. If the
//     removed sprite was itself linked, its direct dependents relink to
//     the sprite it resolved to.
//   - All LinkedIndex values are renumbered for the shifted indices.
//   - Because readers resolve a zero-length shared-palette sprite from the
//     previous table entry and a shared palette from the previous
//     sprite's, a sprite whose result would change is made explicit: it
//     first gets its resolved palette as its own block, and if its pixels
//     still differ (for instance a link that would have to point at table
//     index 0, which readers cannot express) it also gets a copy of the
//     resolved PixelData.
//
// It fails if no sprite has that key, or if a sprite that depends on the
// removed one cannot be resolved in the input.
func RemoveV1Sprite(sprites []V1WriteSprite, group, image int) ([]V1WriteSprite, error) {
	r := findV1Sprite(sprites, group, image)
	if r < 0 {
		return nil, fmt.Errorf("sff: v1: sprite (group %d, image %d) not found", group, image)
	}
	oldTable := v1TableOf(sprites)
	old := make([]v1Resolved, len(sprites))
	for k := range sprites {
		old[k] = v1Resolve(sprites, oldTable, k)
	}
	explicit := func(k int) bool {
		s := sprites[k]
		return v1Length(s) > 0 && s.LinkedIndex > 0 && s.LinkedIndex < k
	}

	first, target := -1, -1
	if old[r].ok {
		if old[r].owner == r {
			for k := range sprites {
				if k != r && old[k].ok && old[k].owner == r {
					first = k
					break
				}
			}
		} else {
			target = old[r].owner
		}
	} else {
		for k := range sprites {
			if k != r && explicit(k) && sprites[k].LinkedIndex == r {
				return nil, fmt.Errorf("sff: v1: cannot remove sprite (group %d, image %d): sprite (group %d, image %d) links to it and it does not resolve", group, image, sprites[k].Group, sprites[k].Image)
			}
		}
	}

	newIdx := func(i int) int {
		if i > r {
			return i - 1
		}
		return i
	}
	out := make([]V1WriteSprite, 0, len(sprites)-1)
	oldOf := make([]int, 0, len(sprites)-1)
	for k, s := range sprites {
		if k == r {
			continue
		}
		switch {
		case k == first:
			s.PixelData = sprites[r].PixelData
			s.LinkedIndex = 0
		case explicit(k):
			t := s.LinkedIndex
			if t == r {
				t = target
				if first >= 0 {
					t = first
				}
			}
			s.LinkedIndex = newIdx(t)
		case len(s.PixelData) > 0:
			s.LinkedIndex = 0
		default:
			// A zero-length copy follows the previous table entry whatever
			// its stored LinkedIndex says, but SerializeV1 still demands a
			// valid, non-self index: use the previous entry (or the next
			// one for the very first sprite).
			s.LinkedIndex = max(len(out)-1, 0)
			if len(out) == 0 && len(sprites) > 2 {
				s.LinkedIndex = 1
			}
		}
		out = append(out, s)
		oldOf = append(oldOf, k)
	}

	table := v1TableOf(out)
	for j := range out {
		want := old[oldOf[j]]
		if !want.ok {
			continue
		}
		matches := func() (pixOK, ok bool) {
			got := v1Resolve(out, table, j)
			pixOK = got.ok && bytes.Equal(got.pix, want.pix)
			return pixOK, pixOK && bytes.Equal(got.pal, want.pal)
		}
		if _, ok := matches(); ok {
			continue
		}
		orig := out[j]
		withPalette := func(s V1WriteSprite) V1WriteSprite {
			if len(want.pal) == V1PaletteBlockSize {
				s.Palette, s.SharedPalette = want.pal, false
			} else {
				s.Palette, s.SharedPalette = nil, true
			}
			return s
		}
		candidates := []V1WriteSprite{withPalette(orig)}
		full := withPalette(orig)
		full.PixelData, full.LinkedIndex = want.pix, 0
		candidates = append(candidates, full)
		repaired := false
		for _, c := range candidates {
			out[j] = c
			table.Sprites[j] = v1Entry(c)
			if _, ok := matches(); ok {
				repaired = true
				break
			}
		}
		if !repaired {
			return nil, fmt.Errorf("sff: v1: cannot preserve sprite (group %d, image %d) after removing (group %d, image %d)", orig.Group, orig.Image, group, image)
		}
	}
	return out, nil
}
