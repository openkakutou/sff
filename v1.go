package sff

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	// headerSize is the fixed size, in bytes, of a .sff v1 file header.
	headerSize = 512
	// subheaderSize is the fixed size, in bytes, of a .sff v1 sprite
	// subheader.
	subheaderSize = 32
	// v1Signature is the fixed 12-byte signature every .sff v1 file
	// starts with.
	v1Signature = "ElecbyteSpr\x00"
)

// V1Header is the parsed MUGEN/Ikemen GO .sff v1 file header.
type V1Header struct {
	// Version holds the four raw version bytes as stored in the file
	// (verhi, verlo1, verlo2, verlo3).
	Version [4]byte
	// GroupCount is the number of sprite groups declared in the header.
	GroupCount int
	// ImageCount is the total number of sprites declared in the header.
	ImageCount int
	// SharedPalette is true when sprites share a common palette table
	// (SPRPALTYPE_SHARED in the MUGEN spec), false when each sprite
	// carries its own individual palette (SPRPALTYPE_INDIV).
	SharedPalette bool
}

// V1SpriteEntry is one sprite's entry in a .sff v1 sprite index table: its
// (group, image) key, its axis (pivot) point, and where its pixel data
// lives in the file. Pixel data itself is not decoded here — see the v1
// pixel decoder.
type V1SpriteEntry struct {
	// Group is the sprite group index.
	Group int
	// Image is the sprite's image index within Group.
	Image int
	// AxisX is the horizontal offset from the sprite's top-left corner to
	// its axis (pivot) point.
	AxisX int
	// AxisY is the vertical offset from the sprite's top-left corner to
	// its axis (pivot) point.
	AxisY int
	// Offset is the absolute file offset of this sprite's pixel data,
	// immediately following its own subheader.
	Offset int64
	// Length is the pixel data length in bytes. Zero means this sprite
	// does not store pixel data of its own: it links to a previous
	// sprite's data, identified by LinkedIndex.
	Length int
	// LinkedIndex is the index, within the sprite table's Sprites slice,
	// of the sprite this one shares pixel data with. Only meaningful
	// when Length is 0.
	LinkedIndex int
	// SharedPalette is true when this sprite reuses the previous
	// sprite's palette instead of carrying its own.
	SharedPalette bool
}

// V1SpriteTable is a parsed .sff v1 file header plus its sprite index
// table, as produced by ParseV1.
type V1SpriteTable struct {
	Header  V1Header
	Sprites []V1SpriteEntry
}

// Offset resolves the (group, image) pair to the absolute file offset of
// that sprite's pixel data. The second return value is false if no sprite
// with that (group, image) exists in the table.
func (t *V1SpriteTable) Offset(group, image int) (int64, bool) {
	i, ok := t.Index(group, image)
	if !ok {
		return 0, false
	}
	return t.Sprites[i].Offset, true
}

// Index resolves the (group, image) pair to its position within
// t.Sprites — the index ResolveV1Pixels/ResolveV1Palette take. The second
// return value is false if no sprite with that (group, image) exists in
// the table.
func (t *V1SpriteTable) Index(group, image int) (int, bool) {
	for i, s := range t.Sprites {
		if s.Group == group && s.Image == image {
			return i, true
		}
	}
	return 0, false
}

// ParseV1 reads a MUGEN/Ikemen GO .sff v1 file header and its sprite index
// table from r. Sprite pixel data is located (offset and length) but not
// decoded.
func ParseV1(r io.ReaderAt) (*V1SpriteTable, error) {
	header := make([]byte, headerSize)
	if _, err := r.ReadAt(header, 0); err != nil {
		return nil, fmt.Errorf("sff: reading v1 header: %w", err)
	}

	if sig := string(header[0:12]); sig != v1Signature {
		return nil, fmt.Errorf("sff: not a v1 .sff file: unexpected signature %q", sig)
	}

	var h V1Header
	copy(h.Version[:], header[12:16])
	h.GroupCount = int(binary.LittleEndian.Uint32(header[16:20]))
	h.ImageCount = int(binary.LittleEndian.Uint32(header[20:24]))
	nextOffset := int64(binary.LittleEndian.Uint32(header[24:28]))
	h.SharedPalette = header[32] == 1

	if h.GroupCount < 0 || h.ImageCount < 0 {
		return nil, errors.New("sff: v1 header declares a negative group or image count")
	}

	table := &V1SpriteTable{
		Header:  h,
		Sprites: make([]V1SpriteEntry, 0, h.ImageCount),
	}

	for i := 0; i < h.ImageCount; i++ {
		if nextOffset == 0 {
			return nil, fmt.Errorf("sff: v1 sprite table ended after %d of %d declared sprites", i, h.ImageCount)
		}

		sub := make([]byte, subheaderSize)
		if _, err := r.ReadAt(sub, nextOffset); err != nil {
			return nil, fmt.Errorf("sff: reading v1 sprite subheader %d: %w", i, err)
		}

		entry := V1SpriteEntry{
			Offset:        nextOffset + subheaderSize,
			Length:        int(binary.LittleEndian.Uint32(sub[4:8])),
			AxisX:         int(int16(binary.LittleEndian.Uint16(sub[8:10]))),
			AxisY:         int(int16(binary.LittleEndian.Uint16(sub[10:12]))),
			Group:         int(binary.LittleEndian.Uint16(sub[12:14])),
			Image:         int(binary.LittleEndian.Uint16(sub[14:16])),
			LinkedIndex:   int(binary.LittleEndian.Uint16(sub[16:18])),
			SharedPalette: sub[18] != 0,
		}
		table.Sprites = append(table.Sprites, entry)

		nextOffset = int64(binary.LittleEndian.Uint32(sub[0:4]))
	}

	return table, nil
}
