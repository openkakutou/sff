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
// format. Only V2FormatRaw and the PNG formats (V2FormatPNG8/24/32) are
// supported, matching DecodeV2Sprite's own scope; any other format code —
// including the real but unimplemented compressed formats
// V2FormatRLE8/RLE5/LZ5 — returns a descriptive error. See
// .vibe/decisions/007-sff-v2-serialize-shape-and-scope.md.
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
