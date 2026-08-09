package sff

import (
	"encoding/binary"
	"fmt"
)

// maxRunLength is the largest pixel count a single PCX RLE unit can encode
// (the run-length byte's low 6 bits, 0x3F).
const maxRunLength = 63

// EncodePCX encodes a pure pixel buffer into RLE-compressed, 8-bit indexed
// PCX image data — the pixel encoding used by MUGEN/Ikemen GO .sff v1
// sprites — so it can be written into a .sff v1 file (see SerializeV1). It
// is the write-path counterpart of DecodePCX.
//
// EncodePCX always emits run-length units (never a bare literal byte), even
// for a run of a single pixel: this keeps the output unambiguous and the
// encoder simple, at the cost of not matching the byte-for-byte output an
// original MUGEN-authored file might use for the same pixels — see
// .vibe/decisions/005-sff-v1-serialize-is-semantic-not-byte-exact-round-trip.md.
// Every unit produced is guaranteed decodable by DecodePCX.
func EncodePCX(img *PCXImage) ([]byte, error) {
	if img.Width <= 0 || img.Height <= 0 {
		return nil, fmt.Errorf("sff: pcx: invalid image dimensions %dx%d, both must be positive", img.Width, img.Height)
	}
	if len(img.Pixels) != img.Width*img.Height {
		return nil, fmt.Errorf("sff: pcx: pixel buffer length %d does not match %dx%d (%d)", len(img.Pixels), img.Width, img.Height, img.Width*img.Height)
	}

	bytesPerLine := img.Width
	if bytesPerLine%2 != 0 {
		bytesPerLine++
	}

	header := make([]byte, pcxHeaderSize)
	header[0] = 0x0A // manufacturer, per the PCX spec
	header[1] = 5    // version
	header[2] = 1    // encoding: RLE
	header[3] = 8    // bits per pixel
	binary.LittleEndian.PutUint16(header[4:6], 0)
	binary.LittleEndian.PutUint16(header[6:8], 0)
	binary.LittleEndian.PutUint16(header[8:10], uint16(img.Width-1))
	binary.LittleEndian.PutUint16(header[10:12], uint16(img.Height-1))
	header[65] = 1 // color planes
	binary.LittleEndian.PutUint16(header[66:68], uint16(bytesPerLine))

	data := make([]byte, len(header))
	copy(data, header)

	row := make([]byte, bytesPerLine)
	for y := 0; y < img.Height; y++ {
		copy(row, img.Pixels[y*img.Width:(y+1)*img.Width])
		for i := img.Width; i < bytesPerLine; i++ {
			row[i] = 0
		}
		data = append(data, encodeRun(row)...)
	}

	return data, nil
}

// encodeRun RLE-encodes a single scanline into PCX run units, splitting any
// run longer than maxRunLength into consecutive units.
func encodeRun(row []byte) []byte {
	out := make([]byte, 0, len(row)*2)

	i := 0
	for i < len(row) {
		value := row[i]
		runLen := 1
		for i+runLen < len(row) && row[i+runLen] == value && runLen < maxRunLength {
			runLen++
		}
		out = append(out, 0xC0|byte(runLen), value)
		i += runLen
	}

	return out
}
