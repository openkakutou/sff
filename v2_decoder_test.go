package sff

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// buildPNG8 encodes a paletted (8-bit indexed) PNG from a row-major index
// buffer, with the 4-byte length prefix real .sff v2 PNG8 sprite pixel data
// is stored with on disk.
func buildPNG8(t *testing.T, width, height int, indices []byte) []byte {
	t.Helper()

	palette := make(color.Palette, 256)
	for i := range palette {
		palette[i] = color.RGBA{R: uint8(i), G: uint8(i), B: uint8(i), A: 255}
	}

	img := image.NewPaletted(image.Rect(0, 0, width, height), palette)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetColorIndex(x, y, indices[y*width+x])
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encoding test PNG8 fixture: %v", err)
	}
	return withV2LengthPrefix(uint32(len(indices)), buf.Bytes())
}

// buildPNG24 encodes an RGB (no alpha) PNG from a row-major RGB byte
// buffer, with the 4-byte length prefix real .sff v2 PNG24 sprite pixel
// data is stored with on disk.
func buildPNG24(t *testing.T, width, height int, rgb []byte) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := (y*width + x) * 3
			img.SetRGBA(x, y, color.RGBA{R: rgb[i], G: rgb[i+1], B: rgb[i+2], A: 255})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encoding test PNG24 fixture: %v", err)
	}
	return withV2LengthPrefix(uint32(len(rgb)), buf.Bytes())
}

// buildPNG32 encodes an RGBA PNG from a row-major RGBA byte buffer, with
// the 4-byte length prefix real .sff v2 PNG32 sprite pixel data is stored
// with on disk.
func buildPNG32(t *testing.T, width, height int, rgba []byte) []byte {
	t.Helper()

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := (y*width + x) * 4
			img.SetNRGBA(x, y, color.NRGBA{R: rgba[i], G: rgba[i+1], B: rgba[i+2], A: rgba[i+3]})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encoding test PNG32 fixture: %v", err)
	}
	return withV2LengthPrefix(uint32(len(rgba)), buf.Bytes())
}

func TestDecodeV2Sprite_DecodesRawIndexedData(t *testing.T) {
	// 3x2 raw (uncompressed) 8-bit indexed sprite: literally one byte per
	// pixel, row-major, no header of its own.
	data := []byte{10, 20, 30, 40, 50, 60}

	img, err := DecodeV2Sprite(V2FormatRaw, 3, 2, 8, data)
	if err != nil {
		t.Fatalf("DecodeV2Sprite returned error: %v", err)
	}
	if img.Width != 3 || img.Height != 2 {
		t.Fatalf("got Width=%d Height=%d, want Width=3 Height=2", img.Width, img.Height)
	}
	if img.BytesPerPixel != 1 {
		t.Fatalf("got BytesPerPixel=%d, want 1", img.BytesPerPixel)
	}
	if !bytesEqual(img.Pixels, data) {
		t.Fatalf("got Pixels=%v, want %v", img.Pixels, data)
	}
}

func TestDecodeV2Sprite_DecodesPNG8IndexedData(t *testing.T) {
	indices := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	data := buildPNG8(t, 4, 2, indices)

	img, err := DecodeV2Sprite(V2FormatPNG8, 4, 2, 8, data)
	if err != nil {
		t.Fatalf("DecodeV2Sprite returned error: %v", err)
	}
	if img.Width != 4 || img.Height != 2 {
		t.Fatalf("got Width=%d Height=%d, want Width=4 Height=2", img.Width, img.Height)
	}
	if img.BytesPerPixel != 1 {
		t.Fatalf("got BytesPerPixel=%d, want 1", img.BytesPerPixel)
	}
	if !bytesEqual(img.Pixels, indices) {
		t.Fatalf("got Pixels=%v, want %v", img.Pixels, indices)
	}
}

func TestDecodeV2Sprite_DecodesPNG24ColorData(t *testing.T) {
	// 2x1 image: pixel 0 pure red, pixel 1 pure blue.
	rgb := []byte{255, 0, 0, 0, 0, 255}
	data := buildPNG24(t, 2, 1, rgb)

	img, err := DecodeV2Sprite(V2FormatPNG24, 2, 1, 24, data)
	if err != nil {
		t.Fatalf("DecodeV2Sprite returned error: %v", err)
	}
	if img.BytesPerPixel != 3 {
		t.Fatalf("got BytesPerPixel=%d, want 3", img.BytesPerPixel)
	}
	if !bytesEqual(img.Pixels, rgb) {
		t.Fatalf("got Pixels=%v, want %v", img.Pixels, rgb)
	}
}

func TestDecodeV2Sprite_DecodesPNG32ColorDataWithAlpha(t *testing.T) {
	// 2x1 image: pixel 0 opaque green (alpha premultiplication is a no-op
	// at full alpha), pixel 1 half-transparent white (RGB scaled down by
	// alpha) — confirmed against a real, unmodified .sff v2 file that
	// V2FormatPNG32 pixel data is alpha-premultiplied, like V2FormatPNG24.
	// buildPNG32 still encodes its input as a straight-alpha source PNG (a
	// PNG file itself always stores straight alpha, per the PNG format
	// spec), so the expected premultiplied output is derived here via the
	// same color.RGBAModel conversion DecodeV2Sprite itself uses, rather
	// than hand-computed bytes that could hide a rounding mistake.
	rgba := []byte{0, 255, 0, 255, 255, 255, 255, 128}
	data := buildPNG32(t, 2, 1, rgba)

	img, err := DecodeV2Sprite(V2FormatPNG32, 2, 1, 32, data)
	if err != nil {
		t.Fatalf("DecodeV2Sprite returned error: %v", err)
	}
	if img.BytesPerPixel != 4 {
		t.Fatalf("got BytesPerPixel=%d, want 4", img.BytesPerPixel)
	}
	want := make([]byte, len(rgba))
	for i := 0; i < len(rgba)/4; i++ {
		src := color.NRGBA{R: rgba[i*4], G: rgba[i*4+1], B: rgba[i*4+2], A: rgba[i*4+3]}
		c := color.RGBAModel.Convert(src).(color.RGBA)
		want[i*4], want[i*4+1], want[i*4+2], want[i*4+3] = c.R, c.G, c.B, c.A
	}
	if !bytesEqual(img.Pixels, want) {
		t.Fatalf("got Pixels=%v, want %v (premultiplied)", img.Pixels, want)
	}
}

func TestDecodeV2Sprite_DecodesSinglePixelRawSprite(t *testing.T) {
	// 1x1 boundary case.
	data := []byte{42}

	img, err := DecodeV2Sprite(V2FormatRaw, 1, 1, 8, data)
	if err != nil {
		t.Fatalf("DecodeV2Sprite returned error: %v", err)
	}
	if img.Width != 1 || img.Height != 1 {
		t.Fatalf("got Width=%d Height=%d, want Width=1 Height=1", img.Width, img.Height)
	}
	if !bytesEqual(img.Pixels, data) {
		t.Fatalf("got Pixels=%v, want %v", img.Pixels, data)
	}
}

func TestDecodeV2Sprite_ReturnsErrorOnRawDataLengthMismatch(t *testing.T) {
	// Declares a 3x2 sprite (6 bytes needed) but supplies only 4.
	data := []byte{1, 2, 3, 4}

	_, err := DecodeV2Sprite(V2FormatRaw, 3, 2, 8, data)
	if err == nil {
		t.Fatal("expected an error for truncated raw pixel data, got nil")
	}
}

func TestDecodeV2Sprite_ReturnsErrorOnUnrecognizedFormat(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5, 6}

	_, err := DecodeV2Sprite(99, 3, 2, 8, data)
	if err == nil {
		t.Fatal("expected an error for an unrecognized format code, got nil")
	}
}

func TestDecodeV2Sprite_ReturnsErrorOnCorruptPNGData(t *testing.T) {
	data := withV2LengthPrefix(0, []byte{0x89, 'P', 'N', 'G', 0, 0, 0, 0, 'g', 'a', 'r', 'b', 'a', 'g', 'e'})

	_, err := DecodeV2Sprite(V2FormatPNG8, 4, 2, 8, data)
	if err == nil {
		t.Fatal("expected an error for corrupt PNG data, got nil")
	}
}

func TestDecodeV2Sprite_ReturnsErrorOnPNGDataShorterThanLengthPrefix(t *testing.T) {
	data := []byte{1, 2, 3}

	_, err := DecodeV2Sprite(V2FormatPNG8, 4, 2, 8, data)
	if err == nil {
		t.Fatal("expected an error for PNG pixel data shorter than the 4-byte length prefix, got nil")
	}
}

func TestDecodeV2Sprite_ReturnsErrorOnPNGDimensionMismatch(t *testing.T) {
	// Encodes a 4x2 PNG but declares (via the table entry's own
	// width/height) that it should be 2x2.
	data := buildPNG8(t, 4, 2, []byte{1, 2, 3, 4, 5, 6, 7, 8})

	_, err := DecodeV2Sprite(V2FormatPNG8, 2, 2, 8, data)
	if err == nil {
		t.Fatal("expected an error for a PNG whose dimensions disagree with the declared sprite size, got nil")
	}
}

// buildRLE8 prepends the 4-byte little-endian declared-decompressed-length
// prefix real .sff v2 RLE8 pixel data starts with (see decodeRLE8's doc
// comment) to a hand-built control stream, mirroring how a sprite's raw
// pixel data is actually laid out on disk.
func buildRLE8(prefixLength uint32, stream []byte) []byte {
	return withV2LengthPrefix(prefixLength, stream)
}

func TestDecodeV2Sprite_DecodesRLE8LiteralBytes(t *testing.T) {
	// Every byte here has its top two bits clear, so none of them is
	// mistaken for a run-length marker (0b01xxxxxx): a pure literal stream,
	// one byte per pixel.
	indices := []byte{10, 20, 30, 40}
	data := buildRLE8(4, indices)

	img, err := DecodeV2Sprite(V2FormatRLE8, 4, 1, 8, data)
	if err != nil {
		t.Fatalf("DecodeV2Sprite returned error: %v", err)
	}
	if img.Width != 4 || img.Height != 1 {
		t.Fatalf("got Width=%d Height=%d, want Width=4 Height=1", img.Width, img.Height)
	}
	if img.BytesPerPixel != 1 {
		t.Fatalf("got BytesPerPixel=%d, want 1", img.BytesPerPixel)
	}
	if !bytesEqual(img.Pixels, indices) {
		t.Fatalf("got Pixels=%v, want %v", img.Pixels, indices)
	}
}

func TestDecodeV2Sprite_DecodesRLE8RunLengthEncodedData(t *testing.T) {
	// A 3-pixel run of index 5 (marker 0x40|3, value 5), followed by three
	// literal pixels (200, 210, 220 — all >= 0xC0, so none collides with the
	// 0b01xxxxxx marker pattern).
	stream := []byte{0x43, 5, 200, 210, 220}
	data := buildRLE8(6, stream)
	want := []byte{5, 5, 5, 200, 210, 220}

	img, err := DecodeV2Sprite(V2FormatRLE8, 6, 1, 8, data)
	if err != nil {
		t.Fatalf("DecodeV2Sprite returned error: %v", err)
	}
	if !bytesEqual(img.Pixels, want) {
		t.Fatalf("got Pixels=%v, want %v", img.Pixels, want)
	}
}

func TestDecodeV2Sprite_DecodesRLE8SingleRunFillingEntireImage(t *testing.T) {
	// A single run marker (0x40|5, value 7) covers the whole declared image.
	stream := []byte{0x45, 7}
	data := buildRLE8(5, stream)
	want := []byte{7, 7, 7, 7, 7}

	img, err := DecodeV2Sprite(V2FormatRLE8, 5, 1, 8, data)
	if err != nil {
		t.Fatalf("DecodeV2Sprite returned error: %v", err)
	}
	if !bytesEqual(img.Pixels, want) {
		t.Fatalf("got Pixels=%v, want %v", img.Pixels, want)
	}
}

func TestDecodeV2Sprite_DecodesSinglePixelRLE8Sprite(t *testing.T) {
	// 1x1 boundary case: one literal byte.
	data := buildRLE8(1, []byte{9})

	img, err := DecodeV2Sprite(V2FormatRLE8, 1, 1, 8, data)
	if err != nil {
		t.Fatalf("DecodeV2Sprite returned error: %v", err)
	}
	if img.Width != 1 || img.Height != 1 {
		t.Fatalf("got Width=%d Height=%d, want Width=1 Height=1", img.Width, img.Height)
	}
	if !bytesEqual(img.Pixels, []byte{9}) {
		t.Fatalf("got Pixels=%v, want [9]", img.Pixels)
	}
}

func TestDecodeV2Sprite_ReturnsErrorOnRLE8DataShorterThanLengthPrefix(t *testing.T) {
	// Real RLE8 data always starts with a 4-byte declared-length prefix;
	// anything shorter than that can't even hold the prefix.
	data := []byte{1, 2, 3}

	_, err := DecodeV2Sprite(V2FormatRLE8, 3, 2, 8, data)
	if err == nil {
		t.Fatal("expected an error for RLE8 data shorter than the length prefix, got nil")
	}
}

func TestDecodeV2Sprite_ReturnsErrorOnRLE8RunOverrunningImageBounds(t *testing.T) {
	// Declares a 2x1 image (2 pixels) but the control stream's run marker
	// asks for 5 repeats of a single value.
	stream := []byte{0x45, 7}
	data := buildRLE8(2, stream)

	_, err := DecodeV2Sprite(V2FormatRLE8, 2, 1, 8, data)
	if err == nil {
		t.Fatal("expected an error for a run overrunning the declared image size, got nil")
	}
}

func TestDecodeV2Sprite_ReturnsErrorOnRLE8TruncatedMidRun(t *testing.T) {
	// A run-length marker byte with no following value byte.
	stream := []byte{0x41}
	data := buildRLE8(3, stream)

	_, err := DecodeV2Sprite(V2FormatRLE8, 3, 1, 8, data)
	if err == nil {
		t.Fatal("expected an error for a run marker truncated before its value byte, got nil")
	}
}

func TestDecodeV2Sprite_ReturnsErrorOnRLE8InsufficientPixelData(t *testing.T) {
	// Declares a 3-pixel image but the control stream only ever produces 1
	// pixel before running out of input.
	stream := []byte{10}
	data := buildRLE8(3, stream)

	_, err := DecodeV2Sprite(V2FormatRLE8, 3, 1, 8, data)
	if err == nil {
		t.Fatal("expected an error for RLE8 data that ends before filling the declared image size, got nil")
	}
}

func TestDecodeV2Sprite_ReturnsErrorOnRLE8UnsupportedColorDepth(t *testing.T) {
	data := buildRLE8(4, []byte{1, 2, 3, 4})

	_, err := DecodeV2Sprite(V2FormatRLE8, 4, 1, 4, data)
	if err == nil {
		t.Fatal("expected an error for an unsupported RLE8 color depth, got nil")
	}
}

func TestDecodeV2Sprite_ReturnsErrorOnRLE8InvalidDimensions(t *testing.T) {
	data := buildRLE8(0, nil)

	_, err := DecodeV2Sprite(V2FormatRLE8, 0, 0, 8, data)
	if err == nil {
		t.Fatal("expected an error for zero declared dimensions, got nil")
	}
}

// buildLZ5 prepends the 4-byte little-endian declared-decompressed-length
// prefix real .sff v2 LZ5 pixel data starts with (mirroring buildRLE8) to a
// hand-built control stream.
func buildLZ5(prefixLength uint32, stream []byte) []byte {
	data := make([]byte, 4+len(stream))
	data[0] = byte(prefixLength)
	data[1] = byte(prefixLength >> 8)
	data[2] = byte(prefixLength >> 16)
	data[3] = byte(prefixLength >> 24)
	copy(data[4:], stream)
	return data
}

func TestDecodeV2Sprite_DecodesLZ5LiteralAndPackedBackReferencePattern(t *testing.T) {
	// Control byte 0x04 (0b00000100): op1/op2 are literal (bit clear), op3 is
	// a back-reference (bit set).
	//   op1: d=0x25 (0b001_00101) -> top 3 bits give run=1, low 5 bits give
	//        literal value 5. Writes [5].
	//   op2: d=0x29 (0b001_01001) -> run=1, literal value 9. Writes [9].
	//   op3: d=0x03 -> low 6 bits (3) are non-zero, so this is the "packed"
	//        back-reference form: run = d&0x3f = 3 (copies run+1 = 4 times),
	//        and since this is the first packed op this decode, the
	//        distance comes directly from the next byte: dist = 1+1 = 2.
	//        Copies pixels[j-2] four times starting at j=2, alternating the
	//        two already-written values.
	stream := []byte{0x04, 0x25, 0x29, 0x03, 0x01}
	data := buildLZ5(6, stream)
	want := []byte{5, 9, 5, 9, 5, 9}

	img, err := DecodeV2Sprite(V2FormatLZ5, 6, 1, 8, data)
	if err != nil {
		t.Fatalf("DecodeV2Sprite returned error: %v", err)
	}
	if img.Width != 6 || img.Height != 1 {
		t.Fatalf("got Width=%d Height=%d, want Width=6 Height=1", img.Width, img.Height)
	}
	if img.BytesPerPixel != 1 {
		t.Fatalf("got BytesPerPixel=%d, want 1", img.BytesPerPixel)
	}
	if !bytesEqual(img.Pixels, want) {
		t.Fatalf("got Pixels=%v, want %v", img.Pixels, want)
	}
}

func TestDecodeV2Sprite_DecodesLZ5ExtendedLiteralRun(t *testing.T) {
	// Control byte 0x00: op1 is literal. d=0x03 has its top 3 bits clear,
	// selecting the extended-count form: the run length comes from the next
	// byte (+8), and the literal value is d itself (3). A single op fills
	// all 10 declared pixels.
	stream := []byte{0x00, 0x03, 0x02}
	data := buildLZ5(10, stream)
	want := []byte{3, 3, 3, 3, 3, 3, 3, 3, 3, 3}

	img, err := DecodeV2Sprite(V2FormatLZ5, 10, 1, 8, data)
	if err != nil {
		t.Fatalf("DecodeV2Sprite returned error: %v", err)
	}
	if !bytesEqual(img.Pixels, want) {
		t.Fatalf("got Pixels=%v, want %v", img.Pixels, want)
	}
}

func TestDecodeV2Sprite_DecodesLZ5LongDistanceBackReference(t *testing.T) {
	// Control byte 0x04: op1/op2 literal (as in the packed-back-reference
	// test above), writing [5, 9]. op3's d=0x00 has low 6 bits all zero,
	// selecting the long-distance back-reference form: distance is built
	// from two bytes (d and the next byte) instead of one, here
	// (0<<2|1)+1 = 2, and the run length from a further byte (0+2, copied
	// run+1 = 3 times) — copying the 2-pixel [5, 9] pattern once more.
	stream := []byte{0x04, 0x25, 0x29, 0x00, 0x01, 0x00}
	data := buildLZ5(5, stream)
	want := []byte{5, 9, 5, 9, 5}

	img, err := DecodeV2Sprite(V2FormatLZ5, 5, 1, 8, data)
	if err != nil {
		t.Fatalf("DecodeV2Sprite returned error: %v", err)
	}
	if !bytesEqual(img.Pixels, want) {
		t.Fatalf("got Pixels=%v, want %v", img.Pixels, want)
	}
}

func TestDecodeV2Sprite_ReturnsErrorOnLZ5DataShorterThanLengthPrefix(t *testing.T) {
	data := []byte{1, 2, 3}

	_, err := DecodeV2Sprite(V2FormatLZ5, 3, 2, 8, data)
	if err == nil {
		t.Fatal("expected an error for LZ5 data shorter than the length prefix, got nil")
	}
}

func TestDecodeV2Sprite_ReturnsErrorOnLZ5TruncatedMidOperation(t *testing.T) {
	// Control byte selects a back-reference for op1 (bit 0 set), but the
	// stream ends right after the op's own byte, before the distance byte
	// the packed back-reference form needs.
	stream := []byte{0x01, 0x03}
	data := buildLZ5(4, stream)

	_, err := DecodeV2Sprite(V2FormatLZ5, 4, 1, 8, data)
	if err == nil {
		t.Fatal("expected an error for LZ5 data truncated mid-operation, got nil")
	}
}

func TestDecodeV2Sprite_ReturnsErrorOnLZ5RunOverrunningImageBounds(t *testing.T) {
	// Same stream as the extended-literal-run test (which fills 10 pixels)
	// but declared against a 5-pixel image.
	stream := []byte{0x00, 0x03, 0x02}
	data := buildLZ5(5, stream)

	_, err := DecodeV2Sprite(V2FormatLZ5, 5, 1, 8, data)
	if err == nil {
		t.Fatal("expected an error for a literal run overrunning the declared image size, got nil")
	}
}

func TestDecodeV2Sprite_DecodesLZ5RegardlessOfDeclaredColorDepth(t *testing.T) {
	// Real LZ5 sprites are declared with ColorDepth 5 (a reduced color
	// count), not 8, yet still decode into a plain one-byte-per-pixel index
	// buffer — the declared depth must not gate decoding the way it does
	// for raw/RLE8 pixel data.
	stream := []byte{0x00, 0x03, 0x02}
	data := buildLZ5(10, stream)
	want := []byte{3, 3, 3, 3, 3, 3, 3, 3, 3, 3}

	img, err := DecodeV2Sprite(V2FormatLZ5, 10, 1, 5, data)
	if err != nil {
		t.Fatalf("DecodeV2Sprite returned error: %v", err)
	}
	if !bytesEqual(img.Pixels, want) {
		t.Fatalf("got Pixels=%v, want %v", img.Pixels, want)
	}
}

func TestDecodeV2Sprite_ReturnsErrorOnLZ5InvalidDimensions(t *testing.T) {
	data := buildLZ5(0, nil)

	_, err := DecodeV2Sprite(V2FormatLZ5, 0, 0, 8, data)
	if err == nil {
		t.Fatal("expected an error for zero declared dimensions, got nil")
	}
}
