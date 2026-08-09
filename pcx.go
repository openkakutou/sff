package sff

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// pcxHeaderSize is the fixed size, in bytes, of a standard PCX file header.
const pcxHeaderSize = 128

// PCXImage is a decoded PCX-encoded sprite: its pixel dimensions and a
// row-major buffer of palette index values (one byte per pixel). It carries
// no palette (RGB) data — a PCX-encoded .sff v1 sprite's actual colors are
// resolved separately against the sprite's (possibly shared) palette.
type PCXImage struct {
	// Width is the image width in pixels.
	Width int
	// Height is the image height in pixels.
	Height int
	// Pixels is the row-major buffer of palette index values, of length
	// Width*Height.
	Pixels []byte
}

// DecodePCX decodes RLE-compressed, 8-bit indexed PCX image data — the
// pixel encoding used by MUGEN/Ikemen GO .sff v1 sprites — into a pure
// pixel buffer. It reads the standard 128-byte PCX header embedded in data
// to recover the image's own width and height, then RLE-decodes the
// scanline data that follows.
//
// Only single-plane, 8-bit, RLE-encoded PCX data is supported, which is
// what .sff v1 sprites use; anything else returns a descriptive error, as
// does corrupted or truncated data, rather than panicking.
func DecodePCX(data []byte) (*PCXImage, error) {
	if len(data) < pcxHeaderSize {
		return nil, fmt.Errorf("sff: pcx data too short for header: got %d bytes, need at least %d", len(data), pcxHeaderSize)
	}
	header := data[:pcxHeaderSize]

	encoding := header[2]
	if encoding != 1 {
		return nil, fmt.Errorf("sff: pcx: unsupported encoding %d, only RLE (1) is supported", encoding)
	}

	bpp := header[3]
	if bpp != 8 {
		return nil, fmt.Errorf("sff: pcx: unsupported bits per pixel %d, only 8 is supported", bpp)
	}

	planes := header[65]
	if planes != 1 {
		return nil, fmt.Errorf("sff: pcx: unsupported color plane count %d, only 1 is supported", planes)
	}

	xmin := int(binary.LittleEndian.Uint16(header[4:6]))
	ymin := int(binary.LittleEndian.Uint16(header[6:8]))
	xmax := int(binary.LittleEndian.Uint16(header[8:10]))
	ymax := int(binary.LittleEndian.Uint16(header[10:12]))
	if xmax < xmin || ymax < ymin {
		return nil, fmt.Errorf("sff: pcx: invalid image bounds (xmin=%d, ymin=%d, xmax=%d, ymax=%d)", xmin, ymin, xmax, ymax)
	}

	bytesPerLine := int(binary.LittleEndian.Uint16(header[66:68]))
	if bytesPerLine <= 0 {
		return nil, errors.New("sff: pcx: invalid bytes-per-line value")
	}

	width := xmax - xmin + 1
	height := ymax - ymin + 1
	if bytesPerLine < width {
		return nil, fmt.Errorf("sff: pcx: bytes-per-line %d is smaller than image width %d", bytesPerLine, width)
	}

	pixels := make([]byte, width*height)
	cursor := pcxHeaderSize
	line := make([]byte, 0, bytesPerLine)

	for y := 0; y < height; y++ {
		line = line[:0]
		for len(line) < bytesPerLine {
			if cursor >= len(data) {
				return nil, fmt.Errorf("sff: pcx: truncated RLE data at row %d: need %d more bytes", y, bytesPerLine-len(line))
			}
			b := data[cursor]
			cursor++

			if b&0xC0 == 0xC0 {
				count := int(b & 0x3F)
				if cursor >= len(data) {
					return nil, fmt.Errorf("sff: pcx: truncated RLE run at row %d: missing value byte", y)
				}
				value := data[cursor]
				cursor++

				remaining := bytesPerLine - len(line)
				if count > remaining {
					return nil, fmt.Errorf("sff: pcx: RLE run at row %d overflows scanline: run of %d exceeds %d remaining bytes", y, count, remaining)
				}
				for i := 0; i < count; i++ {
					line = append(line, value)
				}
			} else {
				line = append(line, b)
			}
		}
		copy(pixels[y*width:(y+1)*width], line[:width])
	}

	return &PCXImage{Width: width, Height: height, Pixels: pixels}, nil
}
