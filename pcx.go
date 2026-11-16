package sff

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// pcxHeaderSize is the fixed size, in bytes, of a standard PCX file header.
const pcxHeaderSize = 128

// pcxPlanesOffset is the header offset of the color-plane count byte.
const pcxPlanesOffset = 65

// pcxRGBPlanes is the color-plane count of a 24-bit (direct RGB) PCX.
const pcxRGBPlanes = 3

// isPCX3Plane reports whether data starts with a PCX header declaring 3 color
// planes (direct RGB, no trailing palette block).
func isPCX3Plane(data []byte) bool {
	return len(data) > pcxPlanesOffset && data[pcxPlanesOffset] == pcxRGBPlanes
}

// PCXImage is a decoded PCX-encoded sprite: its pixel dimensions and a
// row-major pixel buffer. A 1-plane sprite holds palette index values (one
// byte per pixel) and carries no palette (RGB) data — its actual colors are
// resolved separately against the sprite's (possibly shared) palette. A
// 3-plane (24-bit) sprite holds its real colors directly, as interleaved
// R, G, B bytes — see .vibe/decisions/022-pcx-3-plane-sprites-decode-to-direct-rgb.md.
type PCXImage struct {
	// Width is the image width in pixels.
	Width int
	// Height is the image height in pixels.
	Height int
	// BytesPerPixel is 1 when Pixels holds palette indices, 3 when it holds
	// interleaved R, G, B bytes. 0 means indexed (an image built by hand
	// without setting it).
	BytesPerPixel int
	// Pixels is the row-major pixel buffer, of length
	// Width*Height*BytesPerPixel.
	Pixels []byte
}

// DecodePCX decodes RLE-compressed, 8-bit indexed PCX image data — the
// pixel encoding used by MUGEN/Ikemen GO .sff v1 sprites — into a pure
// pixel buffer. It reads the standard 128-byte PCX header embedded in data
// to recover the image's own width and height, then RLE-decodes the
// scanline data that follows.
//
// Only 8-bit, RLE-encoded PCX data with 1 plane (palette indices) or 3
// planes (24-bit RGB, returned as interleaved R, G, B bytes with
// BytesPerPixel 3) is supported; anything else returns a descriptive error,
// as does corrupted or truncated data, rather than panicking. A 3-plane
// scanline is an R row, a G row, then a B row, each bytesPerLine long; an
// RLE run may cross plane boundaries within a scanline.
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

	bytesPerPixel := int(header[pcxPlanesOffset])
	if bytesPerPixel != 1 && bytesPerPixel != pcxRGBPlanes {
		return nil, fmt.Errorf("sff: pcx: unsupported color plane count %d, only 1 or 3 is supported", bytesPerPixel)
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

	if err := checkSpriteDimensions(width, height); err != nil {
		return nil, err
	}

	lineLen := bytesPerLine * bytesPerPixel
	pixels := make([]byte, width*height*bytesPerPixel)
	cursor := pcxHeaderSize
	line := make([]byte, 0, lineLen)

	for y := 0; y < height; y++ {
		line = line[:0]
		for len(line) < lineLen {
			if cursor >= len(data) {
				return nil, fmt.Errorf("sff: pcx: truncated RLE data at row %d: need %d more bytes", y, lineLen-len(line))
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

				remaining := lineLen - len(line)
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
		// Fast path: a 1-plane scanline is already the pixel row.
		if bytesPerPixel == 1 {
			copy(pixels[y*width:(y+1)*width], line[:width])
			continue
		}
		out := pixels[y*width*3 : (y+1)*width*3]
		for x := 0; x < width; x++ {
			out[x*3] = line[x]
			out[x*3+1] = line[bytesPerLine+x]
			out[x*3+2] = line[2*bytesPerLine+x]
		}
	}

	return &PCXImage{Width: width, Height: height, BytesPerPixel: bytesPerPixel, Pixels: pixels}, nil
}
