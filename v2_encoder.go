package sff

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
)

// EncodeV2Sprite is the exact inverse of DecodeV2Sprite: it encodes a
// decoded pixel buffer back into the on-disk byte representation for
// format. V2FormatRaw, V2FormatRLE8, V2FormatLZ5, and the PNG formats
// (V2FormatPNG8/24/32) are supported; V2FormatRLE5 — a real but
// unimplemented format on both the decode and encode side — returns a
// descriptive error instead, per
// .vibe/decisions/001-v2-rle8-lz5-encode-scope-and-rle5-deferred.md.
func EncodeV2Sprite(format int, img *V2Image) ([]byte, error) {
	if img == nil {
		return nil, fmt.Errorf("sff: v2 sprite: cannot encode a nil image")
	}
	if img.Width <= 0 || img.Height <= 0 {
		return nil, fmt.Errorf("sff: v2 sprite: invalid dimensions %dx%d", img.Width, img.Height)
	}

	switch format {
	case V2FormatRaw:
		return encodeV2Raw(img)
	case V2FormatRLE8:
		return encodeV2RLE8(img)
	case V2FormatLZ5:
		return encodeV2LZ5(img)
	case V2FormatPNG8:
		return encodeV2PNG8(img)
	case V2FormatPNG24:
		return encodeV2PNG24(img)
	case V2FormatPNG32:
		return encodeV2PNG32(img)
	default:
		return nil, fmt.Errorf("sff: v2 sprite: unsupported pixel format %d", format)
	}
}

// encodeV2Raw encodes V2FormatRaw pixel data: the pixel buffer written out
// literally, one index byte per pixel, row-major.
func encodeV2Raw(img *V2Image) ([]byte, error) {
	if img.BytesPerPixel != 1 {
		return nil, fmt.Errorf("sff: v2 sprite: raw format requires BytesPerPixel 1 (indexed), got %d", img.BytesPerPixel)
	}
	want := img.Width * img.Height
	if len(img.Pixels) != want {
		return nil, fmt.Errorf("sff: v2 sprite: pixel buffer length %d does not match declared %dx%d size (%d bytes)", len(img.Pixels), img.Width, img.Height, want)
	}

	data := make([]byte, want)
	copy(data, img.Pixels)
	return data, nil
}

// rle8MaxRun is the largest repeat count a single RLE8 run marker can carry
// — its low 6 bits, per decodeV2RLE8's doc comment.
const rle8MaxRun = 0x3f

// isRLE8AmbiguousLiteral reports whether b, written as a plain literal
// byte, would be misread by decodeV2RLE8 as a run marker instead (its top
// two bits are 0b01, mask 0xc0 value 0x40) — decodeV2RLE8's own
// discriminator test.
func isRLE8AmbiguousLiteral(b byte) bool {
	return b&0xc0 == 0x40
}

// encodeV2RLE8 encodes V2FormatRLE8 pixel data, the inverse of
// decodeV2RLE8: a 4-byte little-endian declared decompressed length
// (matching decodeV2RLE8's own prefix, though — per
// .vibe/decisions/001-v2-rle8-lz5-encode-scope-and-rle5-deferred.md — this
// encoder targets a semantic round trip through DecodeV2Sprite, not
// byte-exact reproduction of any particular real encoder's output)
// followed by a run-length-encoded control stream.
//
// Each maximal run of identical bytes is written as a run marker (0x40 |
// count, count capped at rle8MaxRun since the marker's repeat count field
// is only 6 bits, splitting a longer run into multiple markers) followed by
// the repeated byte — except a run of length 1 whose byte would not be
// misread as a run marker on its own, which is written as that single
// literal byte instead (isRLE8AmbiguousLiteral guards against the one case
// where that shortcut is unsafe).
func encodeV2RLE8(img *V2Image) ([]byte, error) {
	if img.BytesPerPixel != 1 {
		return nil, fmt.Errorf("sff: v2 sprite: RLE8 format requires BytesPerPixel 1 (indexed), got %d", img.BytesPerPixel)
	}
	want := img.Width * img.Height
	if len(img.Pixels) != want {
		return nil, fmt.Errorf("sff: v2 sprite: pixel buffer length %d does not match declared %dx%d size (%d bytes)", len(img.Pixels), img.Width, img.Height, want)
	}

	var stream []byte
	pos := 0
	for pos < want {
		v := img.Pixels[pos]
		run := 1
		for pos+run < want && img.Pixels[pos+run] == v && run < rle8MaxRun {
			run++
		}

		if run == 1 && !isRLE8AmbiguousLiteral(v) {
			stream = append(stream, v)
		} else {
			stream = append(stream, 0x40|byte(run), v)
		}
		pos += run
	}

	return withV2LengthPrefix(uint32(want), stream), nil
}

// lz5MaxLiteralRun is the longest run a single short-form literal op can
// carry — its 3-bit run-length field, per decodeV2LZ5's doc comment.
const lz5MaxLiteralRun = 7

// lz5MaxIndex is the largest palette index LZ5's literal encoding can carry
// — its value is packed into the format's 5-bit index field (decodeV2LZ5's
// "lit = d & 0x1f"), matching real LZ5 sprites' own declared ColorDepth 5
// (decodeV2LZ5's doc comment).
const lz5MaxIndex = 0x1f

// encodeV2LZ5 encodes V2FormatLZ5 pixel data, the inverse of decodeV2LZ5,
// targeting a semantic round trip through DecodeV2Sprite rather than
// byte-for-byte reproduction of any real encoder's output (see
// .vibe/decisions/001-v2-rle8-lz5-encode-scope-and-rle5-deferred.md).
//
// It always emits literal runs (decodeV2LZ5's "backRef == false" path),
// never back-reference copies: every pixel value must therefore fit LZ5's
// 5-bit index field (0-31), matching real LZ5 sprites' own ColorDepth 5.
// Each control byte covers up to 8 operations, one bit per operation
// (decodeV2LZ5 reads them low-bit-first); since every operation here is a
// literal, every control byte is written as 0x00. Each literal operation
// covers a run of up to lz5MaxLiteralRun identical pixels, encoded in the
// short form decodeV2LZ5 expects: one byte packing the run length (top 3
// bits) and the repeated index (bottom 5 bits) — always safe here since a
// run of at least 1 with a top-3-bit run length is never all-zero, so it's
// never misread as the format's alternative "extended run length in the
// next byte" form.
func encodeV2LZ5(img *V2Image) ([]byte, error) {
	if img.BytesPerPixel != 1 {
		return nil, fmt.Errorf("sff: v2 sprite: LZ5 format requires BytesPerPixel 1 (indexed), got %d", img.BytesPerPixel)
	}
	want := img.Width * img.Height
	if len(img.Pixels) != want {
		return nil, fmt.Errorf("sff: v2 sprite: pixel buffer length %d does not match declared %dx%d size (%d bytes)", len(img.Pixels), img.Width, img.Height, want)
	}

	var ops []byte
	pos := 0
	for pos < want {
		v := img.Pixels[pos]
		if v > lz5MaxIndex {
			return nil, fmt.Errorf("sff: v2 sprite: LZ5 format only supports palette indices 0-%d (5-bit), got %d at pixel %d", lz5MaxIndex, v, pos)
		}
		run := 1
		for pos+run < want && img.Pixels[pos+run] == v && run < lz5MaxLiteralRun {
			run++
		}
		ops = append(ops, byte(run<<5)|v)
		pos += run
	}

	var stream []byte
	for i := 0; i < len(ops); i += 8 {
		end := i + 8
		if end > len(ops) {
			end = len(ops)
		}
		stream = append(stream, 0x00) // control byte: every op below is a literal.
		stream = append(stream, ops[i:end]...)
	}

	return withV2LengthPrefix(uint32(want), stream), nil
}

// identityPalette is a synthetic 256-entry grayscale palette used to encode
// PNG8 sprites. Its actual colors are never round-tripped: DecodeV2Sprite
// only reads back a PNG8 sprite's index bytes, never its embedded palette,
// so any valid, distinct-enough palette works.
var identityPalette = func() color.Palette {
	p := make(color.Palette, 256)
	for i := range p {
		p[i] = color.RGBA{R: uint8(i), G: uint8(i), B: uint8(i), A: 255}
	}
	return p
}()

// withV2LengthPrefix prepends the 4-byte little-endian declared-length
// prefix real .sff v2 PNG (and RLE8/LZ5) pixel data starts with on disk —
// see decodeV2PNG's doc comment — ahead of already-encoded bytes. n is the
// declared decompressed pixel count, matching what real files store there
// (decodeV2PNG itself never validates this value, only skips it).
func withV2LengthPrefix(n uint32, encoded []byte) []byte {
	data := make([]byte, 4+len(encoded))
	data[0] = byte(n)
	data[1] = byte(n >> 8)
	data[2] = byte(n >> 16)
	data[3] = byte(n >> 24)
	copy(data[4:], encoded)
	return data
}

// encodeV2PNG8 encodes an indexed (8-bit palette) pixel buffer as a PNG8
// image, the inverse of decodeV2PNG's V2FormatPNG8 branch.
func encodeV2PNG8(img *V2Image) ([]byte, error) {
	if img.BytesPerPixel != 1 {
		return nil, fmt.Errorf("sff: v2 sprite: PNG8 format requires BytesPerPixel 1 (indexed), got %d", img.BytesPerPixel)
	}
	want := img.Width * img.Height
	if len(img.Pixels) != want {
		return nil, fmt.Errorf("sff: v2 sprite: pixel buffer length %d does not match declared %dx%d size (%d bytes)", len(img.Pixels), img.Width, img.Height, want)
	}

	dst := image.NewPaletted(image.Rect(0, 0, img.Width, img.Height), identityPalette)
	for y := 0; y < img.Height; y++ {
		copy(dst.Pix[y*dst.Stride:y*dst.Stride+img.Width], img.Pixels[y*img.Width:(y+1)*img.Width])
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return nil, fmt.Errorf("sff: v2 sprite: encoding PNG8 pixel data: %w", err)
	}
	return withV2LengthPrefix(uint32(want), buf.Bytes()), nil
}

// encodeV2PNG24 encodes an RGB pixel buffer as a PNG, the inverse of
// decodeV2PNG's V2FormatPNG24 branch. Alpha is always written fully opaque
// so RGB values survive premultiplication unchanged.
func encodeV2PNG24(img *V2Image) ([]byte, error) {
	if img.BytesPerPixel != 3 {
		return nil, fmt.Errorf("sff: v2 sprite: PNG24 format requires BytesPerPixel 3 (RGB), got %d", img.BytesPerPixel)
	}
	want := img.Width * img.Height * 3
	if len(img.Pixels) != want {
		return nil, fmt.Errorf("sff: v2 sprite: pixel buffer length %d does not match declared %dx%d size (%d bytes)", len(img.Pixels), img.Width, img.Height, want)
	}

	dst := image.NewRGBA(image.Rect(0, 0, img.Width, img.Height))
	for y := 0; y < img.Height; y++ {
		for x := 0; x < img.Width; x++ {
			i := (y*img.Width + x) * 3
			dst.SetRGBA(x, y, color.RGBA{R: img.Pixels[i], G: img.Pixels[i+1], B: img.Pixels[i+2], A: 255})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return nil, fmt.Errorf("sff: v2 sprite: encoding PNG24 pixel data: %w", err)
	}
	return withV2LengthPrefix(uint32(img.Width*img.Height), buf.Bytes()), nil
}

// encodeV2PNG32 encodes an alpha-premultiplied RGBA pixel buffer (matching
// decodeV2PNG's V2FormatPNG32 output shape) as a PNG, the inverse of
// decodeV2PNG's V2FormatPNG32 branch. A PNG file itself always stores
// straight alpha (per the PNG format spec), so each pixel is un-
// premultiplied via color.NRGBAModel before writing — the exact inverse of
// decode's own color.RGBAModel.Convert premultiplication step.
func encodeV2PNG32(img *V2Image) ([]byte, error) {
	if img.BytesPerPixel != 4 {
		return nil, fmt.Errorf("sff: v2 sprite: PNG32 format requires BytesPerPixel 4 (RGBA), got %d", img.BytesPerPixel)
	}
	want := img.Width * img.Height * 4
	if len(img.Pixels) != want {
		return nil, fmt.Errorf("sff: v2 sprite: pixel buffer length %d does not match declared %dx%d size (%d bytes)", len(img.Pixels), img.Width, img.Height, want)
	}

	dst := image.NewNRGBA(image.Rect(0, 0, img.Width, img.Height))
	for y := 0; y < img.Height; y++ {
		for x := 0; x < img.Width; x++ {
			i := (y*img.Width + x) * 4
			premultiplied := color.RGBA{R: img.Pixels[i], G: img.Pixels[i+1], B: img.Pixels[i+2], A: img.Pixels[i+3]}
			straight := color.NRGBAModel.Convert(premultiplied).(color.NRGBA)
			dst.SetNRGBA(x, y, straight)
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return nil, fmt.Errorf("sff: v2 sprite: encoding PNG32 pixel data: %w", err)
	}
	return withV2LengthPrefix(uint32(img.Width*img.Height), buf.Bytes()), nil
}
