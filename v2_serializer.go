package sff

import (
	"encoding/binary"
	"fmt"
	"io"
)

// V2WriteSprite describes one sprite to serialize into a .sff v2 file via
// SerializeV2: its (group, image) key, declared dimensions, axis (pivot)
// point, palette bank reference, and either its own encoded pixel data
// (e.g. produced by EncodeV2Sprite) or a link to another sprite already in
// the same call's Sprites slice.
//
// This is a write-only counterpart to V2SpriteEntry, not that same type
// reused: V2SpriteEntry carries Offset/Length, facts ParseV2 computes while
// reading a file, not values a caller should be trusted to supply when
// writing one — see
// .vibe/decisions/007-sff-v2-serialize-shape-and-scope.md.
type V2WriteSprite struct {
	// Group is the sprite group index.
	Group int
	// Image is the sprite's image index within Group.
	Image int
	// Width is the sprite's width in pixels.
	Width int
	// Height is the sprite's height in pixels.
	Height int
	// AxisX is the horizontal offset from the sprite's top-left corner to
	// its axis (pivot) point.
	AxisX int
	// AxisY is the vertical offset from the sprite's top-left corner to its
	// axis (pivot) point.
	AxisY int
	// Format identifies how PixelData is encoded (see the V2Format*
	// constants). Only meaningful when PixelData is non-empty.
	Format int
	// ColorDepth is the color depth, in bits, of PixelData.
	ColorDepth int
	// PaletteIndex is the index, within the Palettes slice passed to
	// SerializeV2, of the palette bank this sprite is drawn with.
	PaletteIndex int
	// PixelData is this sprite's already-encoded pixel data, e.g. produced
	// by EncodeV2Sprite. Leave it nil/empty to write this sprite as a
	// linked sprite that reuses another sprite's pixel data instead — see
	// LinkedIndex.
	PixelData []byte
	// LinkedIndex is the index, within the Sprites slice passed to
	// SerializeV2, of the sprite this one shares pixel data with. Only
	// meaningful when PixelData is empty.
	LinkedIndex int
}

// V2WritePalette describes one palette bank to serialize into a .sff v2
// file via SerializeV2: its (group, number) key, color count, and either
// its own RGBA color data or a link to another palette bank already in the
// same call's Palettes slice.
//
// This is a write-only counterpart to V2PaletteEntry, for the same reason
// V2WriteSprite is a write-only counterpart to V2SpriteEntry.
type V2WritePalette struct {
	// Group is the palette bank's group index.
	Group int
	// Number is the palette bank's index within Group.
	Number int
	// ColorCount is the number of colors in this palette bank.
	ColorCount int
	// ColorData is this palette bank's own RGBA color data. Leave it
	// nil/empty to write this bank as linked, reusing another bank's color
	// data instead — see LinkedIndex.
	ColorData []byte
	// LinkedIndex is the index, within the Palettes slice passed to
	// SerializeV2, of the palette bank this one shares color data with.
	// Only meaningful when ColorData is empty.
	LinkedIndex int
}

// SerializeV2 writes a .sff v2 file to w: the v2HeaderSize-byte header
// portion ParseV2 reads, followed by the sprite table, the palette table,
// and finally a single data section holding every sprite's pixel data and
// every palette bank's color data, in that order.
//
// version is written verbatim as the file's four raw version bytes.
// SerializeV2 always writes every sprite's/palette's data into the file's
// literal data section (the on-disk "translated" flag bit is always 0) and
// only writes the v2HeaderSize header bytes ParseV2 itself understands —
// see .vibe/decisions/007-sff-v2-serialize-shape-and-scope.md for why.
func SerializeV2(w io.Writer, version [4]byte, sprites []V2WriteSprite, palettes []V2WritePalette) error {
	for i, s := range sprites {
		if len(s.PixelData) > 0 {
			continue
		}
		if s.LinkedIndex < 0 || s.LinkedIndex >= len(sprites) {
			return fmt.Errorf("sff: v2: sprite %d links to out-of-range index %d (have %d sprites)", i, s.LinkedIndex, len(sprites))
		}
		if s.LinkedIndex == i {
			return fmt.Errorf("sff: v2: sprite %d has no pixel data and links to itself", i)
		}
	}
	for i, p := range palettes {
		if len(p.ColorData) > 0 {
			continue
		}
		if p.LinkedIndex < 0 || p.LinkedIndex >= len(palettes) {
			return fmt.Errorf("sff: v2: palette %d links to out-of-range index %d (have %d palettes)", i, p.LinkedIndex, len(palettes))
		}
		if p.LinkedIndex == i {
			return fmt.Errorf("sff: v2: palette %d has no color data and links to itself", i)
		}
	}

	spriteTableOffset := uint32(v2HeaderSize)
	paletteTableOffset := spriteTableOffset + uint32(len(sprites))*v2SpriteEntrySize
	literalDataOffset := paletteTableOffset + uint32(len(palettes))*v2PaletteEntrySize

	spriteOfs := make([]uint32, len(sprites))
	cur := uint32(0)
	for i, s := range sprites {
		spriteOfs[i] = cur
		cur += uint32(len(s.PixelData))
	}
	paletteOfs := make([]uint32, len(palettes))
	for i, p := range palettes {
		paletteOfs[i] = cur
		cur += uint32(len(p.ColorData))
	}
	translatedDataOffset := literalDataOffset + cur

	if err := writeV2Header(w, version, len(sprites), len(palettes), spriteTableOffset, paletteTableOffset, literalDataOffset, translatedDataOffset); err != nil {
		return err
	}

	for i, s := range sprites {
		if err := writeV2SpriteEntry(w, s, spriteOfs[i]); err != nil {
			return fmt.Errorf("sff: v2: writing sprite %d entry: %w", i, err)
		}
	}
	for i, p := range palettes {
		if err := writeV2PaletteEntry(w, p, paletteOfs[i]); err != nil {
			return fmt.Errorf("sff: v2: writing palette %d entry: %w", i, err)
		}
	}

	for i, s := range sprites {
		if len(s.PixelData) == 0 {
			continue
		}
		if _, err := w.Write(s.PixelData); err != nil {
			return fmt.Errorf("sff: v2: writing sprite %d pixel data: %w", i, err)
		}
	}
	for i, p := range palettes {
		if len(p.ColorData) == 0 {
			continue
		}
		if _, err := w.Write(p.ColorData); err != nil {
			return fmt.Errorf("sff: v2: writing palette %d color data: %w", i, err)
		}
	}

	return nil
}

// writeV2Header writes the v2HeaderSize-byte .sff v2 file header, matching
// the layout ParseV2 reads.
func writeV2Header(w io.Writer, version [4]byte, spriteCount, paletteCount int, spriteTableOffset, paletteTableOffset, literalDataOffset, translatedDataOffset uint32) error {
	header := make([]byte, v2HeaderSize)
	copy(header[0:12], v1Signature)
	copy(header[12:16], version[:])
	// header[16:36] (compatibility version + legacy v1-only fields) is
	// unused by ParseV2 and left zeroed.
	binary.LittleEndian.PutUint32(header[36:40], spriteTableOffset)
	binary.LittleEndian.PutUint32(header[40:44], uint32(spriteCount))
	binary.LittleEndian.PutUint32(header[44:48], paletteTableOffset)
	binary.LittleEndian.PutUint32(header[48:52], uint32(paletteCount))
	binary.LittleEndian.PutUint32(header[52:56], literalDataOffset)
	// header[56:60] is unused by ParseV2 and left zeroed.
	binary.LittleEndian.PutUint32(header[60:64], translatedDataOffset)

	_, err := w.Write(header)
	if err != nil {
		return fmt.Errorf("sff: v2: writing header: %w", err)
	}
	return nil
}

// writeV2SpriteEntry writes one sprite's fixed v2SpriteEntrySize-byte .sff
// v2 sprite table entry. ofs is the sprite's pixel data offset relative to
// the file's literal data section base — SerializeV2 always writes into
// the literal section, so the on-disk translated-data flag bit is always 0.
func writeV2SpriteEntry(w io.Writer, s V2WriteSprite, ofs uint32) error {
	entry := make([]byte, v2SpriteEntrySize)
	binary.LittleEndian.PutUint16(entry[0:2], uint16(s.Group))
	binary.LittleEndian.PutUint16(entry[2:4], uint16(s.Image))
	binary.LittleEndian.PutUint16(entry[4:6], uint16(s.Width))
	binary.LittleEndian.PutUint16(entry[6:8], uint16(s.Height))
	binary.LittleEndian.PutUint16(entry[8:10], uint16(int16(s.AxisX)))
	binary.LittleEndian.PutUint16(entry[10:12], uint16(int16(s.AxisY)))
	binary.LittleEndian.PutUint16(entry[12:14], uint16(s.LinkedIndex))
	entry[14] = byte(s.Format)
	entry[15] = byte(s.ColorDepth)
	binary.LittleEndian.PutUint32(entry[16:20], ofs)
	binary.LittleEndian.PutUint32(entry[20:24], uint32(len(s.PixelData)))
	binary.LittleEndian.PutUint16(entry[24:26], uint16(s.PaletteIndex))
	// entry[26:28] is the literal/translated data section flag, always 0
	// (literal) — see SerializeV2's doc comment.

	_, err := w.Write(entry)
	return err
}

// writeV2PaletteEntry writes one palette bank's fixed v2PaletteEntrySize-byte
// .sff v2 palette table entry. ofs is the bank's color data offset relative
// to the file's literal data section base.
func writeV2PaletteEntry(w io.Writer, p V2WritePalette, ofs uint32) error {
	entry := make([]byte, v2PaletteEntrySize)
	binary.LittleEndian.PutUint16(entry[0:2], uint16(p.Group))
	binary.LittleEndian.PutUint16(entry[2:4], uint16(p.Number))
	binary.LittleEndian.PutUint16(entry[4:6], uint16(p.ColorCount))
	binary.LittleEndian.PutUint16(entry[6:8], uint16(p.LinkedIndex))
	binary.LittleEndian.PutUint32(entry[8:12], ofs)
	binary.LittleEndian.PutUint32(entry[12:16], uint32(len(p.ColorData)))

	_, err := w.Write(entry)
	return err
}
