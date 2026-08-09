package sff

import "testing"

func TestEncodeV2Sprite_RawFormat_RoundTripsThroughDecodeV2Sprite(t *testing.T) {
	img := &V2Image{Width: 3, Height: 2, BytesPerPixel: 1, Pixels: []byte{10, 20, 30, 40, 50, 60}}

	data, err := EncodeV2Sprite(V2FormatRaw, img)
	if err != nil {
		t.Fatalf("EncodeV2Sprite returned error: %v", err)
	}

	got, err := DecodeV2Sprite(V2FormatRaw, img.Width, img.Height, 8, data)
	if err != nil {
		t.Fatalf("DecodeV2Sprite on encoded raw data: %v", err)
	}
	if !bytesEqual(got.Pixels, img.Pixels) {
		t.Errorf("expected pixels %v, got %v", img.Pixels, got.Pixels)
	}
}

func TestEncodeV2Sprite_PNG8Format_RoundTripsThroughDecodeV2Sprite(t *testing.T) {
	img := &V2Image{Width: 4, Height: 2, BytesPerPixel: 1, Pixels: []byte{1, 2, 3, 4, 5, 6, 7, 8}}

	data, err := EncodeV2Sprite(V2FormatPNG8, img)
	if err != nil {
		t.Fatalf("EncodeV2Sprite returned error: %v", err)
	}

	got, err := DecodeV2Sprite(V2FormatPNG8, img.Width, img.Height, 8, data)
	if err != nil {
		t.Fatalf("DecodeV2Sprite on encoded PNG8 data: %v", err)
	}
	if !bytesEqual(got.Pixels, img.Pixels) {
		t.Errorf("expected pixels %v, got %v", img.Pixels, got.Pixels)
	}
}

func TestEncodeV2Sprite_PNG24Format_RoundTripsThroughDecodeV2Sprite(t *testing.T) {
	// 2x1: pure red, pure blue.
	img := &V2Image{Width: 2, Height: 1, BytesPerPixel: 3, Pixels: []byte{255, 0, 0, 0, 0, 255}}

	data, err := EncodeV2Sprite(V2FormatPNG24, img)
	if err != nil {
		t.Fatalf("EncodeV2Sprite returned error: %v", err)
	}

	got, err := DecodeV2Sprite(V2FormatPNG24, img.Width, img.Height, 24, data)
	if err != nil {
		t.Fatalf("DecodeV2Sprite on encoded PNG24 data: %v", err)
	}
	if !bytesEqual(got.Pixels, img.Pixels) {
		t.Errorf("expected pixels %v, got %v", img.Pixels, got.Pixels)
	}
}

func TestEncodeV2Sprite_PNG32Format_RoundTripsThroughDecodeV2SpriteWithAlpha(t *testing.T) {
	// 2x1: opaque green, half-transparent white — exercises non-255 alpha.
	// Pixels are alpha-premultiplied (see EncodeV2Sprite/decodeV2PNG's
	// V2FormatPNG32 doc comments): straight-alpha white (255,255,255) at
	// alpha 128 premultiplies to (128,128,128,128), not (255,255,255,128)
	// — a premultiplied color's R/G/B can never exceed its own A.
	img := &V2Image{Width: 2, Height: 1, BytesPerPixel: 4, Pixels: []byte{0, 255, 0, 255, 128, 128, 128, 128}}

	data, err := EncodeV2Sprite(V2FormatPNG32, img)
	if err != nil {
		t.Fatalf("EncodeV2Sprite returned error: %v", err)
	}

	got, err := DecodeV2Sprite(V2FormatPNG32, img.Width, img.Height, 32, data)
	if err != nil {
		t.Fatalf("DecodeV2Sprite on encoded PNG32 data: %v", err)
	}
	// Approximate, not byte-exact: un-premultiplying then re-premultiplying
	// an 8-bit premultiplied value is inherently lossy (each step
	// truncates), the same "semantic round trip, not byte-exact" nature
	// this package's other binary write paths already document (see
	// .vibe/decisions/005-sff-v1-serialize-is-semantic-not-byte-exact-round-trip.md).
	if !bytesApproxEqual(got.Pixels, img.Pixels, 1) {
		t.Errorf("expected pixels %v (±1), got %v", img.Pixels, got.Pixels)
	}
}

func TestEncodeV2Sprite_SinglePixelRawSprite_RoundTrips(t *testing.T) {
	img := &V2Image{Width: 1, Height: 1, BytesPerPixel: 1, Pixels: []byte{42}}

	data, err := EncodeV2Sprite(V2FormatRaw, img)
	if err != nil {
		t.Fatalf("EncodeV2Sprite returned error: %v", err)
	}
	if !bytesEqual(data, img.Pixels) {
		t.Errorf("expected raw encoded data %v, got %v", img.Pixels, data)
	}
}

func TestEncodeV2Sprite_ReturnsErrorOnUnsupportedFormat(t *testing.T) {
	img := &V2Image{Width: 2, Height: 2, BytesPerPixel: 1, Pixels: []byte{1, 2, 3, 4}}

	// RLE5 is a real .sff v2 format code but stays deliberately unimplemented
	// on both the decode and encode side — see
	// .vibe/decisions/001-v2-rle8-lz5-encode-scope-and-rle5-deferred.md.
	_, err := EncodeV2Sprite(V2FormatRLE5, img)
	if err == nil {
		t.Fatal("expected an error for the unsupported RLE5 format, got nil")
	}
}

func TestEncodeV2Sprite_RLE8Format_RoundTripsThroughDecodeV2Sprite(t *testing.T) {
	// A mix of: a long run (9 repeats of 7, forcing the encoder to use the
	// run-marker path rather than a literal byte), a run long enough to
	// require more than one 6-bit run-marker chunk (70 repeats of 3, max
	// chunk size is 63), a single literal byte, and a single-occurrence
	// value that falls in the run-marker's own reserved bit pattern
	// (0x40-0x7F, top two bits 0b01) which cannot be written as a plain
	// literal byte without being misread as a run marker on decode.
	pixels := make([]byte, 0, 9+70+1+1)
	for i := 0; i < 9; i++ {
		pixels = append(pixels, 7)
	}
	for i := 0; i < 70; i++ {
		pixels = append(pixels, 3)
	}
	pixels = append(pixels, 200)
	pixels = append(pixels, 0x55) // ambiguous: 0x55&0xc0 == 0x40
	img := &V2Image{Width: len(pixels), Height: 1, BytesPerPixel: 1, Pixels: pixels}

	data, err := EncodeV2Sprite(V2FormatRLE8, img)
	if err != nil {
		t.Fatalf("EncodeV2Sprite returned error: %v", err)
	}

	got, err := DecodeV2Sprite(V2FormatRLE8, img.Width, img.Height, 8, data)
	if err != nil {
		t.Fatalf("DecodeV2Sprite on encoded RLE8 data: %v", err)
	}
	if !bytesEqual(got.Pixels, img.Pixels) {
		t.Errorf("expected pixels %v, got %v", img.Pixels, got.Pixels)
	}
}

func TestEncodeV2Sprite_RLE8Format_SinglePixelRoundTrips(t *testing.T) {
	img := &V2Image{Width: 1, Height: 1, BytesPerPixel: 1, Pixels: []byte{42}}

	data, err := EncodeV2Sprite(V2FormatRLE8, img)
	if err != nil {
		t.Fatalf("EncodeV2Sprite returned error: %v", err)
	}

	got, err := DecodeV2Sprite(V2FormatRLE8, img.Width, img.Height, 8, data)
	if err != nil {
		t.Fatalf("DecodeV2Sprite on encoded RLE8 data: %v", err)
	}
	if !bytesEqual(got.Pixels, img.Pixels) {
		t.Errorf("expected pixels %v, got %v", img.Pixels, got.Pixels)
	}
}

func TestEncodeV2Sprite_RLE8Format_RealFixtureRoundTrips(t *testing.T) {
	// Real, unmodified fixture (see testdata/README.md): decode it, encode
	// the decoded pixels back out, decode the encoder's own output, and
	// compare — the encoder does not need to reproduce the original file's
	// exact byte layout (see .vibe/decisions/001-...), only decode back to
	// the same pixels (a semantic round trip).
	// v2-rle8.sff (despite its name) actually stores its one sprite as LZ5,
	// not RLE8 — v2-empty-palette-use-first.sff is the fixture that
	// genuinely carries a real RLE8-encoded sprite.
	f := openTestdataFile(t, "v2-empty-palette-use-first.sff")
	defer f.Close()

	table, err := ParseV2(f)
	if err != nil {
		t.Fatalf("ParseV2: %v", err)
	}
	idx, ok := table.Index(0, 0)
	if !ok {
		t.Fatal("v2-empty-palette-use-first.sff: no sprite in group 0")
	}
	entry := table.Sprites[idx]
	if entry.Format != V2FormatRLE8 {
		t.Fatalf("v2-empty-palette-use-first.sff: expected sprite format %d (RLE8), got %d", V2FormatRLE8, entry.Format)
	}

	raw := make([]byte, entry.Length)
	if _, err := f.ReadAt(raw, entry.Offset); err != nil {
		t.Fatalf("reading real RLE8 sprite data: %v", err)
	}
	want, err := DecodeV2Sprite(V2FormatRLE8, entry.Width, entry.Height, entry.ColorDepth, raw)
	if err != nil {
		t.Fatalf("DecodeV2Sprite on real fixture data: %v", err)
	}

	encoded, err := EncodeV2Sprite(V2FormatRLE8, want)
	if err != nil {
		t.Fatalf("EncodeV2Sprite on real fixture pixels: %v", err)
	}
	got, err := DecodeV2Sprite(V2FormatRLE8, entry.Width, entry.Height, entry.ColorDepth, encoded)
	if err != nil {
		t.Fatalf("DecodeV2Sprite on re-encoded real fixture data: %v", err)
	}
	if !bytesEqual(got.Pixels, want.Pixels) {
		t.Errorf("real RLE8 fixture: pixels differ after encode/decode round trip")
	}
}

func TestEncodeV2Sprite_RLE8Format_ReturnsErrorOnBytesPerPixelMismatch(t *testing.T) {
	img := &V2Image{Width: 2, Height: 1, BytesPerPixel: 3, Pixels: []byte{1, 2, 3, 4, 5, 6}}

	_, err := EncodeV2Sprite(V2FormatRLE8, img)
	if err == nil {
		t.Fatal("expected an error for a BytesPerPixel mismatch, got nil")
	}
}

func TestEncodeV2Sprite_ReturnsErrorOnBytesPerPixelMismatch(t *testing.T) {
	// Raw format expects 1 byte per pixel (indexed); this image claims 3
	// (RGB), which raw pixel data can never represent.
	img := &V2Image{Width: 2, Height: 1, BytesPerPixel: 3, Pixels: []byte{1, 2, 3, 4, 5, 6}}

	_, err := EncodeV2Sprite(V2FormatRaw, img)
	if err == nil {
		t.Fatal("expected an error for a BytesPerPixel mismatch, got nil")
	}
}

func TestEncodeV2Sprite_LZ5Format_RoundTripsThroughDecodeV2Sprite(t *testing.T) {
	// LZ5 pixel data is a 5-bit palette-index format (real files declare
	// ColorDepth 5) — every value here stays within 0-31. Includes: a run
	// longer than a single literal op can hold (12 repeats of 9, forcing
	// the encoder to split into multiple ops), enough total pixels to span
	// more than one 8-op control byte group (over 20 pixels total), and a
	// couple of single, non-repeating values.
	pixels := make([]byte, 0, 40)
	for i := 0; i < 12; i++ {
		pixels = append(pixels, 9)
	}
	for i := 0; i < 20; i++ {
		pixels = append(pixels, byte(i%32))
	}
	pixels = append(pixels, 31, 0)
	img := &V2Image{Width: len(pixels), Height: 1, BytesPerPixel: 1, Pixels: pixels}

	data, err := EncodeV2Sprite(V2FormatLZ5, img)
	if err != nil {
		t.Fatalf("EncodeV2Sprite returned error: %v", err)
	}

	got, err := DecodeV2Sprite(V2FormatLZ5, img.Width, img.Height, 5, data)
	if err != nil {
		t.Fatalf("DecodeV2Sprite on encoded LZ5 data: %v", err)
	}
	if !bytesEqual(got.Pixels, img.Pixels) {
		t.Errorf("expected pixels %v, got %v", img.Pixels, got.Pixels)
	}
}

func TestEncodeV2Sprite_LZ5Format_SinglePixelRoundTrips(t *testing.T) {
	img := &V2Image{Width: 1, Height: 1, BytesPerPixel: 1, Pixels: []byte{17}}

	data, err := EncodeV2Sprite(V2FormatLZ5, img)
	if err != nil {
		t.Fatalf("EncodeV2Sprite returned error: %v", err)
	}

	got, err := DecodeV2Sprite(V2FormatLZ5, img.Width, img.Height, 5, data)
	if err != nil {
		t.Fatalf("DecodeV2Sprite on encoded LZ5 data: %v", err)
	}
	if !bytesEqual(got.Pixels, img.Pixels) {
		t.Errorf("expected pixels %v, got %v", img.Pixels, got.Pixels)
	}
}

func TestEncodeV2Sprite_LZ5Format_RealFixtureRoundTrips(t *testing.T) {
	// Real, unmodified fixture (see testdata/README.md): decode it, encode
	// the decoded pixels back out, decode the encoder's own output, and
	// compare — a semantic round trip, not a byte-exact reproduction of the
	// original encoder's output (see
	// .vibe/decisions/001-v2-rle8-lz5-encode-scope-and-rle5-deferred.md).
	f := openTestdataFile(t, "v2-lz5.sff")
	defer f.Close()

	table, err := ParseV2(f)
	if err != nil {
		t.Fatalf("ParseV2: %v", err)
	}
	idx, ok := table.Index(0, 0)
	if !ok {
		t.Fatal("v2-lz5.sff: no sprite in group 0")
	}
	entry := table.Sprites[idx]
	if entry.Format != V2FormatLZ5 {
		t.Fatalf("v2-lz5.sff: expected sprite format %d (LZ5), got %d", V2FormatLZ5, entry.Format)
	}

	raw := make([]byte, entry.Length)
	if _, err := f.ReadAt(raw, entry.Offset); err != nil {
		t.Fatalf("reading real LZ5 sprite data: %v", err)
	}
	want, err := DecodeV2Sprite(V2FormatLZ5, entry.Width, entry.Height, entry.ColorDepth, raw)
	if err != nil {
		t.Fatalf("DecodeV2Sprite on real fixture data: %v", err)
	}

	encoded, err := EncodeV2Sprite(V2FormatLZ5, want)
	if err != nil {
		t.Fatalf("EncodeV2Sprite on real fixture pixels: %v", err)
	}
	got, err := DecodeV2Sprite(V2FormatLZ5, entry.Width, entry.Height, entry.ColorDepth, encoded)
	if err != nil {
		t.Fatalf("DecodeV2Sprite on re-encoded real fixture data: %v", err)
	}
	if !bytesEqual(got.Pixels, want.Pixels) {
		t.Errorf("real LZ5 fixture: pixels differ after encode/decode round trip")
	}
}

func TestEncodeV2Sprite_LZ5Format_ReturnsErrorOnBytesPerPixelMismatch(t *testing.T) {
	img := &V2Image{Width: 2, Height: 1, BytesPerPixel: 3, Pixels: []byte{1, 2, 3, 4, 5, 6}}

	_, err := EncodeV2Sprite(V2FormatLZ5, img)
	if err == nil {
		t.Fatal("expected an error for a BytesPerPixel mismatch, got nil")
	}
}

func TestEncodeV2Sprite_LZ5Format_ReturnsErrorOnPixelValueOutOfRange(t *testing.T) {
	// LZ5 encodes palette indices in 5 bits (0-31); a value of 32 or above
	// cannot be represented by this format.
	img := &V2Image{Width: 2, Height: 1, BytesPerPixel: 1, Pixels: []byte{10, 32}}

	_, err := EncodeV2Sprite(V2FormatLZ5, img)
	if err == nil {
		t.Fatal("expected an error for a pixel value outside LZ5's 5-bit index range, got nil")
	}
}

// bytesApproxEqual reports whether a and b are the same length and every
// byte pair differs by at most tolerance — used for PNG32's premultiplied-
// alpha round trip, which is not byte-exact (see the test using this).
func bytesApproxEqual(a, b []byte, tolerance int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		d := int(a[i]) - int(b[i])
		if d < -tolerance || d > tolerance {
			return false
		}
	}
	return true
}
