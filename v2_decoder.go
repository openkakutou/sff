package sff

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
)

// V2Image is a decoded .sff v2 sprite: its pixel dimensions and a row-major
// pixel buffer.
//
// For indexed pixel data (V2FormatRaw, V2FormatRLE8, V2FormatPNG8),
// BytesPerPixel is 1 and each byte in Pixels is a palette index — no
// RGB/palette resolution performed, mirroring PCXImage. For direct-color
// pixel data (V2FormatPNG24, V2FormatPNG32), BytesPerPixel is 3 (RGB) or 4
// (RGBA, straight alpha) and each pixel's actual color channels are stored
// directly, since no palette is involved for those formats.
type V2Image struct {
	// Width is the image width in pixels.
	Width int
	// Height is the image height in pixels.
	Height int
	// BytesPerPixel is the number of bytes each pixel occupies in Pixels:
	// 1 for indexed data, 3 for RGB, 4 for RGBA.
	BytesPerPixel int
	// Pixels is the row-major pixel buffer, of length
	// Width*Height*BytesPerPixel.
	Pixels []byte
}

// DecodeV2Sprite decodes a .sff v2 sprite's stored pixel data into a pure
// pixel buffer. format is the sprite table entry's Format code
// (V2Format* constant); width, height, and colorDepth are the entry's own
// declared dimensions and color depth, since not every encoding
// self-describes them the way PCX/PNG headers do.
//
// Only V2FormatRaw (uncompressed indexed data), V2FormatRLE8, V2FormatLZ5,
// and the PNG formats (V2FormatPNG8/24/32, decoded via the standard
// library) are supported. Any other format code — including the real but
// unimplemented compressed format V2FormatRLE5 — returns a descriptive
// error rather than panicking or silently misinterpreting the data; see
// .vibe/decisions/006-sff-v2-pixel-decode-shape-and-scope.md.
func DecodeV2Sprite(format, width, height, colorDepth int, data []byte) (*V2Image, error) {
	switch format {
	case V2FormatRaw:
		return decodeV2Raw(width, height, colorDepth, data)
	case V2FormatRLE8:
		return decodeV2RLE8(width, height, colorDepth, data)
	case V2FormatLZ5:
		return decodeV2LZ5(width, height, colorDepth, data)
	case V2FormatPNG8, V2FormatPNG24, V2FormatPNG32:
		return decodeV2PNG(format, width, height, data)
	default:
		return nil, fmt.Errorf("sff: v2 sprite: unsupported pixel format %d", format)
	}
}

// decodeV2Raw decodes V2FormatRaw pixel data: literal, uncompressed
// palette-index bytes, one per pixel, row-major, with no encoding of its
// own.
func decodeV2Raw(width, height, colorDepth int, data []byte) (*V2Image, error) {
	if colorDepth != 8 {
		return nil, fmt.Errorf("sff: v2 sprite: unsupported color depth %d for raw format, only 8 is supported", colorDepth)
	}
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("sff: v2 sprite: invalid dimensions %dx%d for raw format", width, height)
	}

	want := width * height
	if len(data) != want {
		return nil, fmt.Errorf("sff: v2 sprite: raw pixel data length %d does not match declared %dx%d size (%d bytes)", len(data), width, height, want)
	}

	pixels := make([]byte, want)
	copy(pixels, data)
	return &V2Image{Width: width, Height: height, BytesPerPixel: 1, Pixels: pixels}, nil
}

// decodeV2RLE8 decodes V2FormatRLE8 pixel data: a run-length-compressed
// stream of palette-index bytes. On disk it starts with a 4-byte
// little-endian declared decompressed length, which this decoder skips
// without validating — real files are not always reliable about its value
// (empirically observed against sff-extractor, the reference project this
// algorithm is ported from), so the declared width/height are trusted
// instead, matching decodeV2Raw's own source of truth.
//
// The control stream that follows is a sequence of bytes: one whose top two
// bits are 0b01 (mask 0xc0, value 0x40) is a run marker — its low 6 bits
// give a repeat count, and the byte right after it is the palette index
// repeated that many times. Any other byte is a single literal palette
// index. Decoding stops as soon as enough pixels have been produced to
// fill the declared image size; trailing input bytes, if any, are ignored,
// matching the real engine's own decoder (Ikemen-GO's Rle8Decode stops once
// its output buffer is full).
func decodeV2RLE8(width, height, colorDepth int, data []byte) (*V2Image, error) {
	if colorDepth != 8 {
		return nil, fmt.Errorf("sff: v2 sprite: unsupported color depth %d for RLE8 format, only 8 is supported", colorDepth)
	}
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("sff: v2 sprite: invalid dimensions %dx%d for RLE8 format", width, height)
	}
	if len(data) < 4 {
		return nil, fmt.Errorf("sff: v2 sprite: RLE8 pixel data too short (%d bytes, need at least 4 for the length prefix)", len(data))
	}

	want := width * height
	pixels := make([]byte, want)
	stream := data[4:]

	pos, i := 0, 0
	for pos < want {
		if i >= len(stream) {
			return nil, fmt.Errorf("sff: v2 sprite: RLE8 pixel data ends before filling declared %dx%d size (decoded %d of %d pixels)", width, height, pos, want)
		}
		b := stream[i]
		i++

		run := 1
		if b&0xc0 == 0x40 {
			run = int(b & 0x3f)
			if i >= len(stream) {
				return nil, fmt.Errorf("sff: v2 sprite: RLE8 data ends mid-run (missing run byte)")
			}
			b = stream[i]
			i++
		}

		if pos+run > want {
			return nil, fmt.Errorf("sff: v2 sprite: RLE8 run of %d pixels at position %d overruns declared %dx%d image size", run, pos, width, height)
		}
		for k := 0; k < run; k++ {
			pixels[pos] = b
			pos++
		}
	}

	return &V2Image{Width: width, Height: height, BytesPerPixel: 1, Pixels: pixels}, nil
}

// decodeV2LZ5 decodes V2FormatLZ5 pixel data: a dictionary-based
// compressed stream of palette-index bytes, ported from decodeLZ5.mjs
// (sff-extractor) and cross-checked against Ikemen-GO's Lz5Decode — both
// implement the same algorithm. Like RLE8, it starts with a 4-byte
// little-endian declared decompressed length, skipped without validation
// (decodeV2RLE8's doc comment explains why), followed by the actual
// compressed stream.
//
// The stream is driven by control bytes: each control byte's 8 bits (read
// low bit first) each select one operation, either a literal run or a
// back-reference copy.
//
//   - Literal run: the operation byte's top 3 bits select the run length
//     (1-7) and its bottom 5 bits the repeated palette index — unless the
//     top 3 bits are all zero, in which case the run length instead comes
//     from the next byte (+8, for longer runs) and the operation byte
//     itself, unmasked, is the repeated index.
//   - Back-reference copy: the operation byte's bottom 6 bits give a
//     distance/length pair packed against 1-2 more bytes — unless those 6
//     bits are all zero, in which case a wider distance is built from the
//     operation byte and one more byte instead. Either way, the resulting
//     distance and length describe a copy from already-decoded output
//     earlier in the same buffer (allowing overlapping copies, the same
//     "repeat what was just written" trick RLE-family formats use for
//     runs).
//
// Unlike the reference decoders, which tolerate running out of input by
// clamping their read cursor to the last valid byte (silently reusing
// stale data) and let an over-long run stop writing once the output
// buffer is full without reporting it, this port treats both as a
// descriptive error instead — matching decodeV2RLE8's own error-handling
// contract for the same "malformed compressed data" class of problem; see
// .vibe/decisions/015-v2-lz5-decode-error-handling-diverges-from-reference.md.
//
// Unlike decodeV2Raw/decodeV2RLE8, colorDepth is not validated here: real
// LZ5 sprites (checked against a corpus of real characters) are declared
// with ColorDepth 5, not 8 — the field records the sprite's actual color
// count, not the pixel data's on-disk byte width, which for LZ5 is always
// one index byte per pixel regardless of declared depth. This also
// matches decodeLZ5.mjs, whose signature has no color-depth parameter at
// all.
func decodeV2LZ5(width, height, colorDepth int, data []byte) (*V2Image, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("sff: v2 sprite: invalid dimensions %dx%d for LZ5 format", width, height)
	}
	if len(data) < 4 {
		return nil, fmt.Errorf("sff: v2 sprite: LZ5 pixel data too short (%d bytes, need at least 4 for the length prefix)", len(data))
	}

	want := width * height
	stream := data[4:]
	pixels := make([]byte, want)

	pos := 0
	next := func() (byte, error) {
		if pos >= len(stream) {
			return 0, fmt.Errorf("sff: v2 sprite: LZ5 data ends before filling declared %dx%d size", width, height)
		}
		b := stream[pos]
		pos++
		return b, nil
	}

	var ct byte
	ctBits := 0
	var rb byte
	rbBits := 0

	j := 0
	for j < want {
		if ctBits == 0 {
			b, err := next()
			if err != nil {
				return nil, err
			}
			ct = b
			ctBits = 8
		}
		backRef := ct&(1<<uint(8-ctBits)) != 0
		ctBits--

		d, err := next()
		if err != nil {
			return nil, err
		}

		if backRef {
			var dist, run int
			if d&0x3f == 0 {
				d2, err := next()
				if err != nil {
					return nil, fmt.Errorf("sff: v2 sprite: LZ5 data truncated mid back-reference (distance byte): %w", err)
				}
				dist = (int(d)<<2 | int(d2)) + 1
				nb, err := next()
				if err != nil {
					return nil, fmt.Errorf("sff: v2 sprite: LZ5 data truncated mid back-reference (length byte): %w", err)
				}
				run = int(nb) + 2
			} else {
				rb |= (d & 0xc0) >> uint(rbBits)
				rbBits += 2
				run = int(d & 0x3f)
				if rbBits < 8 {
					d2, err := next()
					if err != nil {
						return nil, fmt.Errorf("sff: v2 sprite: LZ5 data truncated mid back-reference (distance byte): %w", err)
					}
					dist = int(d2) + 1
				} else {
					dist = int(rb) + 1
					rb, rbBits = 0, 0
				}
			}
			run++ // do-while semantics in the reference: decrement then check, so it always copies once more than the raw count.

			if dist <= 0 || dist > j {
				return nil, fmt.Errorf("sff: v2 sprite: LZ5 back-reference distance %d invalid at output position %d", dist, j)
			}
			if j+run > want {
				return nil, fmt.Errorf("sff: v2 sprite: LZ5 back-reference run of %d pixels at position %d overruns declared %dx%d image size", run, j, width, height)
			}
			for k := 0; k < run; k++ {
				pixels[j] = pixels[j-dist]
				j++
			}
		} else {
			var run int
			lit := d
			if d&0xe0 == 0 {
				nb, err := next()
				if err != nil {
					return nil, fmt.Errorf("sff: v2 sprite: LZ5 data truncated mid literal run (count byte): %w", err)
				}
				run = int(nb) + 8
			} else {
				run = int(d >> 5)
				lit = d & 0x1f
			}

			if j+run > want {
				return nil, fmt.Errorf("sff: v2 sprite: LZ5 literal run of %d pixels at position %d overruns declared %dx%d image size", run, j, width, height)
			}
			for k := 0; k < run; k++ {
				pixels[j] = lit
				j++
			}
		}
	}

	return &V2Image{Width: width, Height: height, BytesPerPixel: 1, Pixels: pixels}, nil
}

// decodeV2PNG decodes a PNG-encoded sprite (V2FormatPNG8/24/32) using the
// standard library's PNG decoder, then extracts a row-major byte buffer
// matching format's expected shape. Like decodeV2RLE8/decodeV2LZ5, on disk
// it starts with a 4-byte little-endian declared length before the actual
// PNG bytes, which this decoder skips without validating — confirmed
// against real, unmodified .sff v2 files (see testdata/README.md), where
// this prefix precedes the PNG magic bytes for every PNG-format sprite.
func decodeV2PNG(format, width, height int, data []byte) (*V2Image, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("sff: v2 sprite: PNG pixel data too short (%d bytes, need at least 4 for the length prefix)", len(data))
	}
	img, err := png.Decode(bytes.NewReader(data[4:]))
	if err != nil {
		return nil, fmt.Errorf("sff: v2 sprite: decoding PNG pixel data: %w", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != width || bounds.Dy() != height {
		return nil, fmt.Errorf("sff: v2 sprite: PNG dimensions %dx%d do not match declared %dx%d size", bounds.Dx(), bounds.Dy(), width, height)
	}

	switch format {
	case V2FormatPNG8:
		paletted, ok := img.(*image.Paletted)
		if !ok {
			return nil, fmt.Errorf("sff: v2 sprite: PNG8 pixel data is not an indexed (8-bit palette) PNG")
		}
		pixels := make([]byte, width*height)
		for y := 0; y < height; y++ {
			srcRow := paletted.Pix[y*paletted.Stride : y*paletted.Stride+width]
			copy(pixels[y*width:(y+1)*width], srcRow)
		}
		return &V2Image{Width: width, Height: height, BytesPerPixel: 1, Pixels: pixels}, nil

	case V2FormatPNG24:
		pixels := make([]byte, width*height*3)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				c := color.RGBAModel.Convert(img.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.RGBA)
				i := (y*width + x) * 3
				pixels[i] = c.R
				pixels[i+1] = c.G
				pixels[i+2] = c.B
			}
		}
		return &V2Image{Width: width, Height: height, BytesPerPixel: 3, Pixels: pixels}, nil

	case V2FormatPNG32:
		// Alpha-premultiplied, like V2FormatPNG24's color.RGBAModel
		// conversion just above — confirmed against a real, unmodified
		// .sff v2 file: a non-premultiplied (color.NRGBAModel) conversion
		// preserves each pixel's original RGB even where alpha is low or
		// zero, but the real file's own semi-transparent pixels decode
		// correctly only once RGB is scaled down by alpha the same way
		// color.RGBAModel already does for every other color.Color value
		// in this package.
		pixels := make([]byte, width*height*4)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				c := color.RGBAModel.Convert(img.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.RGBA)
				i := (y*width + x) * 4
				pixels[i] = c.R
				pixels[i+1] = c.G
				pixels[i+2] = c.B
				pixels[i+3] = c.A
			}
		}
		return &V2Image{Width: width, Height: height, BytesPerPixel: 4, Pixels: pixels}, nil

	default:
		// Unreachable: DecodeV2Sprite only routes V2FormatPNG8/24/32 here.
		return nil, fmt.Errorf("sff: v2 sprite: unsupported PNG pixel format %d", format)
	}
}
