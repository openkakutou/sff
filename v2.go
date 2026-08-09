package sff

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	// v2HeaderSize is the number of header bytes ParseV2 reads: the
	// signature, version, and every field it needs (table offsets/counts,
	// data section bases). The remainder of a real 512-byte .sff v2 header
	// (comment padding) is not needed to locate the tables and is not read.
	v2HeaderSize = 64
	// v2PaletteEntrySize is the fixed size, in bytes, of one .sff v2
	// palette table entry.
	v2PaletteEntrySize = 16
	// v2SpriteEntrySize is the fixed size, in bytes, of one .sff v2 sprite
	// table entry.
	v2SpriteEntrySize = 28
)

// Pixel-data format codes, as stored in each V2SpriteEntry's Format field.
// The v2 pixel decoder (a later, separate piece of work) is responsible for
// interpreting these; ParseV2 only records them.
const (
	V2FormatRaw   = 0
	V2FormatRLE8  = 2
	V2FormatRLE5  = 3
	V2FormatLZ5   = 4
	V2FormatPNG8  = 10
	V2FormatPNG24 = 11
	V2FormatPNG32 = 12
)

// V2Header is the parsed MUGEN/Ikemen GO .sff v2 file header.
type V2Header struct {
	// Version holds the four raw version bytes as stored in the file
	// (verlo3, verlo2, verlo1, verhi). verhi (Version[3]) is 2 for a v2
	// file.
	Version [4]byte
	// SpriteCount is the total number of sprites declared in the header.
	SpriteCount int
	// PaletteCount is the total number of palette banks declared in the
	// header.
	PaletteCount int
}

// V2SpriteEntry is one sprite's entry in a .sff v2 sprite index table: its
// (group, image) key, its pixel dimensions, its axis (pivot) point, which
// palette bank it uses, and where its pixel data lives in the file. Pixel
// data itself is not decoded here — see the v2 pixel decoder.
type V2SpriteEntry struct {
	// Group is the sprite group index.
	Group int
	// Image is the sprite's image index within Group.
	Image int
	// Width is the sprite's width in pixels, as declared in the table
	// (independent of its encoded pixel data).
	Width int
	// Height is the sprite's height in pixels, as declared in the table.
	Height int
	// AxisX is the horizontal offset from the sprite's top-left corner to
	// its axis (pivot) point.
	AxisX int
	// AxisY is the vertical offset from the sprite's top-left corner to its
	// axis (pivot) point.
	AxisY int
	// Offset is the absolute file offset of this sprite's encoded pixel
	// data, already resolved against the file's literal or translated data
	// section as indicated by the sprite's on-disk flag.
	Offset int64
	// Length is the encoded pixel data length in bytes. Zero means this
	// sprite does not store pixel data of its own: it links to a previous
	// sprite's data, identified by LinkedIndex.
	Length int
	// LinkedIndex is the index, within the sprite table's Sprites slice, of
	// the sprite this one shares pixel data with. Only meaningful when
	// Length is 0.
	LinkedIndex int
	// Format identifies how the pixel data at Offset is encoded (see the
	// V2Format* constants). Only meaningful when Length is non-zero.
	Format int
	// ColorDepth is the color depth, in bits, of the encoded pixel data.
	ColorDepth int
	// PaletteIndex is the index, within the sprite table's Palettes slice,
	// of the palette bank this sprite is drawn with.
	PaletteIndex int
}

// V2PaletteEntry is one palette bank's entry in a .sff v2 palette table:
// its (group, number) key, its color count, and where its color data lives
// in the file.
type V2PaletteEntry struct {
	// Group is the palette bank's group index.
	Group int
	// Number is the palette bank's index within Group.
	Number int
	// ColorCount is the number of colors declared for this palette bank.
	ColorCount int
	// Offset is the absolute file offset of this palette bank's RGBA color
	// data.
	Offset int64
	// Length is the color data length in bytes. Zero means this palette
	// bank does not store color data of its own: it links to a previous
	// bank's data, identified by LinkedIndex.
	Length int
	// LinkedIndex is the index, within the sprite table's Palettes slice,
	// of the palette bank this one shares color data with. Only meaningful
	// when Length is 0.
	LinkedIndex int
}

// V2SpriteTable is a parsed .sff v2 file header plus its sprite and palette
// index tables, as produced by ParseV2.
type V2SpriteTable struct {
	Header   V2Header
	Sprites  []V2SpriteEntry
	Palettes []V2PaletteEntry
}

// Offset resolves the (group, image) pair to the absolute file offset of
// that sprite's encoded pixel data. The second return value is false if no
// sprite with that (group, image) exists in the table.
func (t *V2SpriteTable) Offset(group, image int) (int64, bool) {
	i, ok := t.Index(group, image)
	if !ok {
		return 0, false
	}
	return t.Sprites[i].Offset, true
}

// Index resolves the (group, image) pair to its position within
// t.Sprites. The second return value is false if no sprite with that
// (group, image) exists in the table.
func (t *V2SpriteTable) Index(group, image int) (int, bool) {
	for i, s := range t.Sprites {
		if s.Group == group && s.Image == image {
			return i, true
		}
	}
	return 0, false
}

// PaletteOffset resolves the (group, number) pair to the absolute file
// offset of that palette bank's RGBA color data. The second return value is
// false if no palette bank with that (group, number) exists in the table.
func (t *V2SpriteTable) PaletteOffset(group, number int) (int64, bool) {
	for _, p := range t.Palettes {
		if p.Group == group && p.Number == number {
			return p.Offset, true
		}
	}
	return 0, false
}

// ParseV2 reads a MUGEN/Ikemen GO .sff v2 file header and its sprite and
// palette index tables from r. Sprite and palette color data is located
// (offset and length) but not decoded.
func ParseV2(r io.ReaderAt) (*V2SpriteTable, error) {
	header := make([]byte, v2HeaderSize)
	if _, err := r.ReadAt(header, 0); err != nil {
		return nil, fmt.Errorf("sff: reading v2 header: %w", err)
	}

	if sig := string(header[0:12]); sig != v1Signature {
		return nil, fmt.Errorf("sff: not a v2 .sff file: unexpected signature %q", sig)
	}

	var h V2Header
	copy(h.Version[:], header[12:16])
	if h.Version[3] != 2 {
		return nil, fmt.Errorf("sff: not a v2 .sff file: version high byte is %d, want 2", h.Version[3])
	}

	firstSpriteOffset := int64(binary.LittleEndian.Uint32(header[36:40]))
	h.SpriteCount = int(binary.LittleEndian.Uint32(header[40:44]))
	firstPaletteOffset := int64(binary.LittleEndian.Uint32(header[44:48]))
	h.PaletteCount = int(binary.LittleEndian.Uint32(header[48:52]))
	literalDataOffset := int64(binary.LittleEndian.Uint32(header[52:56]))
	translatedDataOffset := int64(binary.LittleEndian.Uint32(header[60:64]))

	if h.SpriteCount < 0 || h.PaletteCount < 0 {
		return nil, errors.New("sff: v2 header declares a negative sprite or palette count")
	}

	table := &V2SpriteTable{
		Header:   h,
		Sprites:  make([]V2SpriteEntry, 0, h.SpriteCount),
		Palettes: make([]V2PaletteEntry, 0, h.PaletteCount),
	}

	paletteBuf := make([]byte, v2PaletteEntrySize)
	for i := 0; i < h.PaletteCount; i++ {
		at := firstPaletteOffset + int64(i)*v2PaletteEntrySize
		if _, err := r.ReadAt(paletteBuf, at); err != nil {
			return nil, fmt.Errorf("sff: reading v2 palette entry %d: %w", i, err)
		}

		ofs := int64(binary.LittleEndian.Uint32(paletteBuf[8:12]))
		table.Palettes = append(table.Palettes, V2PaletteEntry{
			Group:       int(binary.LittleEndian.Uint16(paletteBuf[0:2])),
			Number:      int(binary.LittleEndian.Uint16(paletteBuf[2:4])),
			ColorCount:  int(binary.LittleEndian.Uint16(paletteBuf[4:6])),
			LinkedIndex: int(binary.LittleEndian.Uint16(paletteBuf[6:8])),
			Offset:      literalDataOffset + ofs,
			Length:      int(binary.LittleEndian.Uint32(paletteBuf[12:16])),
		})
	}

	spriteBuf := make([]byte, v2SpriteEntrySize)
	for i := 0; i < h.SpriteCount; i++ {
		at := firstSpriteOffset + int64(i)*v2SpriteEntrySize
		if _, err := r.ReadAt(spriteBuf, at); err != nil {
			return nil, fmt.Errorf("sff: reading v2 sprite entry %d: %w", i, err)
		}

		ofs := int64(binary.LittleEndian.Uint32(spriteBuf[16:20]))
		flag := binary.LittleEndian.Uint16(spriteBuf[26:28])
		offset := literalDataOffset + ofs
		if flag&1 != 0 {
			offset = translatedDataOffset + ofs
		}

		table.Sprites = append(table.Sprites, V2SpriteEntry{
			Group:        int(binary.LittleEndian.Uint16(spriteBuf[0:2])),
			Image:        int(binary.LittleEndian.Uint16(spriteBuf[2:4])),
			Width:        int(binary.LittleEndian.Uint16(spriteBuf[4:6])),
			Height:       int(binary.LittleEndian.Uint16(spriteBuf[6:8])),
			AxisX:        int(int16(binary.LittleEndian.Uint16(spriteBuf[8:10]))),
			AxisY:        int(int16(binary.LittleEndian.Uint16(spriteBuf[10:12]))),
			LinkedIndex:  int(binary.LittleEndian.Uint16(spriteBuf[12:14])),
			Format:       int(spriteBuf[14]),
			ColorDepth:   int(spriteBuf[15]),
			Offset:       offset,
			Length:       int(binary.LittleEndian.Uint32(spriteBuf[20:24])),
			PaletteIndex: int(binary.LittleEndian.Uint16(spriteBuf[24:26])),
		})
	}

	return table, nil
}
