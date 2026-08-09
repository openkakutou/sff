package sff

import (
	"encoding/binary"
	"fmt"
	"io"
)

// V1WriteSprite describes one sprite to serialize into a .sff v1 file via
// SerializeV1: its (group, image) key, its axis (pivot) point, and either
// its own PCX-encoded pixel data or a link to another sprite already in the
// same call's Sprites slice.
//
// This is a write-only counterpart to V1SpriteEntry, not that same type
// reused: V1SpriteEntry carries Offset/Length, facts ParseV1 computes while
// reading a file, not values a caller should be trusted to supply when
// writing one — see
// .vibe/decisions/005-sff-v1-serialize-is-semantic-not-byte-exact-round-trip.md.
type V1WriteSprite struct {
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
	// SharedPalette marks this sprite as reusing the previous sprite's
	// palette instead of carrying its own.
	SharedPalette bool
	// PixelData is this sprite's PCX-encoded pixel data, e.g. produced by
	// EncodePCX. Leave it nil/empty to write this sprite as a linked
	// sprite that reuses another sprite's pixel data instead — see
	// LinkedIndex.
	PixelData []byte
	// Palette is this sprite's own embedded 768-byte RGB palette block
	// (256 triplets, 3 bytes/color, matching DecodeV1Palette's input
	// shape). Required whenever SharedPalette is false: real .sff v1 files
	// always embed an owning sprite's palette immediately after its own
	// pixel data, folded into the subheader's declared Length rather than
	// following it — see
	// .vibe/decisions/018-v1-palette-block-lives-inside-declared-length.md.
	// Ignored when SharedPalette is true.
	Palette []byte
	// LinkedIndex is the index, within the Sprites slice passed to
	// SerializeV1, of the sprite this one shares pixel data with. Only
	// meaningful when PixelData is empty.
	LinkedIndex int
}

// SerializeV1 writes a .sff v1 file to w: the fixed 512-byte header
// followed by one 32-byte subheader plus pixel data blob per sprite, in
// order, chained via each subheader's "next subfile" offset — the layout
// ParseV1 reads back.
//
// version is written verbatim as the file's four raw version bytes.
// sharedPalette sets the header's file-level palette-sharing flag
// (SPRPALTYPE_SHARED); it is independent of each V1WriteSprite's own
// SharedPalette field, which governs whether that individual sprite reuses
// the previous sprite's palette. The header's group/image counts are
// derived from sprites rather than taken from the caller, so they can never
// disagree with what is actually written: ImageCount is len(sprites), and
// GroupCount is the number of distinct Group values present.
//
// SerializeV1 always produces a fresh, valid layout — it does not attempt
// to reproduce any original file's exact byte layout or PCX encoding
// choices; see
// .vibe/decisions/005-sff-v1-serialize-is-semantic-not-byte-exact-round-trip.md.
func SerializeV1(w io.Writer, version [4]byte, sharedPalette bool, sprites []V1WriteSprite) error {
	for i, s := range sprites {
		if !s.SharedPalette && len(s.Palette) != V1PaletteBlockSize {
			return fmt.Errorf("sff: v1: sprite %d does not share a palette and needs its own %d-byte Palette, got %d bytes", i, V1PaletteBlockSize, len(s.Palette))
		}
		if len(s.PixelData) > 0 {
			continue
		}
		if s.LinkedIndex < 0 || s.LinkedIndex >= len(sprites) {
			return fmt.Errorf("sff: v1: sprite %d links to out-of-range index %d (have %d sprites)", i, s.LinkedIndex, len(sprites))
		}
		if s.LinkedIndex == i {
			return fmt.Errorf("sff: v1: sprite %d has no pixel data and links to itself", i)
		}
	}

	groups := make(map[int]struct{}, len(sprites))
	for _, s := range sprites {
		groups[s.Group] = struct{}{}
	}

	// A sprite that does not share its palette embeds its own trailing
	// V1PaletteBlockSize-byte block, folded into its declared Length rather
	// than following it — see .vibe/decisions/018-v1-palette-block-lives-
	// inside-declared-length.md.
	ownLength := func(s V1WriteSprite) uint32 {
		n := uint32(len(s.PixelData))
		if !s.SharedPalette {
			n += V1PaletteBlockSize
		}
		return n
	}

	offsets := make([]uint32, len(sprites))
	cur := uint32(headerSize)
	for i, s := range sprites {
		offsets[i] = cur
		cur += subheaderSize + ownLength(s)
	}

	if err := writeV1Header(w, version, sharedPalette, len(groups), len(sprites), offsets); err != nil {
		return err
	}

	for i, s := range sprites {
		next := uint32(0)
		if i+1 < len(sprites) {
			next = offsets[i+1]
		}
		if err := writeV1Subheader(w, next, ownLength(s), s); err != nil {
			return fmt.Errorf("sff: v1: writing sprite %d subheader: %w", i, err)
		}
		if len(s.PixelData) > 0 {
			if _, err := w.Write(s.PixelData); err != nil {
				return fmt.Errorf("sff: v1: writing sprite %d pixel data: %w", i, err)
			}
		}
		if !s.SharedPalette {
			if _, err := w.Write(s.Palette); err != nil {
				return fmt.Errorf("sff: v1: writing sprite %d palette: %w", i, err)
			}
		}
	}

	return nil
}

// writeV1Header writes the fixed 512-byte .sff v1 file header.
func writeV1Header(w io.Writer, version [4]byte, sharedPalette bool, groupCount, imageCount int, offsets []uint32) error {
	header := make([]byte, headerSize)
	copy(header[0:12], v1Signature)
	copy(header[12:16], version[:])
	binary.LittleEndian.PutUint32(header[16:20], uint32(groupCount))
	binary.LittleEndian.PutUint32(header[20:24], uint32(imageCount))
	firstOffset := uint32(0)
	if len(offsets) > 0 {
		firstOffset = offsets[0]
	}
	binary.LittleEndian.PutUint32(header[24:28], firstOffset)
	// header[28:32] is the subfile header length, unused by ParseV1, left 0.
	if sharedPalette {
		header[32] = 1
	}
	// header[33:512] (reserved + comment) is left zeroed.

	_, err := w.Write(header)
	if err != nil {
		return fmt.Errorf("sff: v1: writing header: %w", err)
	}
	return nil
}

// writeV1Subheader writes one sprite's fixed 32-byte .sff v1 subheader.
// length is the sprite's full declared Length (pixel data plus its own
// trailing palette block, when it has one — see ownLength in SerializeV1),
// not simply len(s.PixelData).
func writeV1Subheader(w io.Writer, next, length uint32, s V1WriteSprite) error {
	sub := make([]byte, subheaderSize)
	binary.LittleEndian.PutUint32(sub[0:4], next)
	binary.LittleEndian.PutUint32(sub[4:8], length)
	binary.LittleEndian.PutUint16(sub[8:10], uint16(int16(s.AxisX)))
	binary.LittleEndian.PutUint16(sub[10:12], uint16(int16(s.AxisY)))
	binary.LittleEndian.PutUint16(sub[12:14], uint16(s.Group))
	binary.LittleEndian.PutUint16(sub[14:16], uint16(s.Image))
	binary.LittleEndian.PutUint16(sub[16:18], uint16(s.LinkedIndex))
	if s.SharedPalette {
		sub[18] = 1
	}
	// sub[19:32] is subheader padding, left zeroed.

	_, err := w.Write(sub)
	return err
}
