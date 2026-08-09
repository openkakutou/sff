package sff

import (
	"encoding/binary"
	"testing"
)

// pcxHeaderField reads back one of the header fields EncodePCX is expected
// to populate, from a full encoded byte stream.
func pcxHeaderField(t *testing.T, data []byte) (encoding, bpp byte, xmin, ymin, xmax, ymax int, planes byte, bytesPerLine int) {
	t.Helper()
	if len(data) < pcxHeaderSize {
		t.Fatalf("encoded data too short for a PCX header: got %d bytes", len(data))
	}
	h := data[:pcxHeaderSize]
	encoding = h[2]
	bpp = h[3]
	xmin = int(binary.LittleEndian.Uint16(h[4:6]))
	ymin = int(binary.LittleEndian.Uint16(h[6:8]))
	xmax = int(binary.LittleEndian.Uint16(h[8:10]))
	ymax = int(binary.LittleEndian.Uint16(h[10:12]))
	planes = h[65]
	bytesPerLine = int(binary.LittleEndian.Uint16(h[66:68]))
	return
}

func TestEncodePCX_MixedRunsAndLiterals_ProducesExpectedHeaderAndData(t *testing.T) {
	// 4x2 image: row 0 has a run of three 1s then a single 2; row 1 has
	// four distinct values (each its own run of length 1). EncodePCX always
	// emits run-length units (0xC0|count, value) — see
	// .vibe/decisions/005-sff-v1-serialize-is-semantic-not-byte-exact-round-trip.md
	// — so every byte of output is independently predictable.
	img := &PCXImage{
		Width:  4,
		Height: 2,
		Pixels: []byte{1, 1, 1, 2, 5, 6, 7, 8},
	}

	data, err := EncodePCX(img)
	if err != nil {
		t.Fatalf("EncodePCX returned error: %v", err)
	}

	encoding, bpp, xmin, ymin, xmax, ymax, planes, bytesPerLine := pcxHeaderField(t, data)
	if encoding != 1 {
		t.Errorf("expected encoding 1 (RLE), got %d", encoding)
	}
	if bpp != 8 {
		t.Errorf("expected 8 bits per pixel, got %d", bpp)
	}
	if xmin != 0 || ymin != 0 {
		t.Errorf("expected xmin=ymin=0, got xmin=%d ymin=%d", xmin, ymin)
	}
	if xmax != 3 || ymax != 1 {
		t.Errorf("expected xmax=3 ymax=1 for a 4x2 image, got xmax=%d ymax=%d", xmax, ymax)
	}
	if planes != 1 {
		t.Errorf("expected 1 color plane, got %d", planes)
	}
	if bytesPerLine != 4 {
		t.Errorf("expected bytesPerLine 4 for width 4, got %d", bytesPerLine)
	}

	wantScanlines := []byte{
		0xC0 | 3, 1, // run of three 1s
		0xC0 | 1, 2, // run of one 2
		0xC0 | 1, 5,
		0xC0 | 1, 6,
		0xC0 | 1, 7,
		0xC0 | 1, 8,
	}
	got := data[pcxHeaderSize:]
	if !bytesEqual(got, wantScanlines) {
		t.Fatalf("got scanline bytes %v, want %v", got, wantScanlines)
	}
}

func TestEncodePCX_RunLongerThan63_SplitsAcrossTwoRunUnits(t *testing.T) {
	// A single RLE unit can only encode a run of up to 63 pixels, so a
	// 70-pixel-wide solid row must become two consecutive units: 63 then 7.
	pixels := make([]byte, 70)
	for i := range pixels {
		pixels[i] = 9
	}
	img := &PCXImage{Width: 70, Height: 1, Pixels: pixels}

	data, err := EncodePCX(img)
	if err != nil {
		t.Fatalf("EncodePCX returned error: %v", err)
	}

	want := []byte{0xC0 | 63, 9, 0xC0 | 7, 9}
	got := data[pcxHeaderSize:]
	if !bytesEqual(got, want) {
		t.Fatalf("got scanline bytes %v, want %v", got, want)
	}
}

func TestEncodePCX_SinglePixelImage_PadsOddWidthToEvenBytesPerLine(t *testing.T) {
	// Width 1 is odd; PCX convention pads bytesPerLine to the next even
	// number, so the scanline carries the pixel plus one zero padding byte,
	// each its own run unit.
	img := &PCXImage{Width: 1, Height: 1, Pixels: []byte{42}}

	data, err := EncodePCX(img)
	if err != nil {
		t.Fatalf("EncodePCX returned error: %v", err)
	}

	_, _, _, _, _, _, _, bytesPerLine := pcxHeaderField(t, data)
	if bytesPerLine != 2 {
		t.Fatalf("expected bytesPerLine 2 (padded from odd width 1), got %d", bytesPerLine)
	}

	want := []byte{0xC0 | 1, 42, 0xC0 | 1, 0}
	got := data[pcxHeaderSize:]
	if !bytesEqual(got, want) {
		t.Fatalf("got scanline bytes %v, want %v", got, want)
	}
}

func TestEncodePCX_ThenDecodePCX_RecoversOriginalPixels(t *testing.T) {
	img := &PCXImage{
		Width:  5,
		Height: 3,
		Pixels: []byte{
			0, 0, 0, 0, 0,
			1, 2, 3, 4, 5,
			9, 9, 9, 9, 1,
		},
	}

	data, err := EncodePCX(img)
	if err != nil {
		t.Fatalf("EncodePCX returned error: %v", err)
	}

	decoded, err := DecodePCX(data)
	if err != nil {
		t.Fatalf("DecodePCX(EncodePCX(img)) returned error: %v", err)
	}
	if decoded.Width != img.Width || decoded.Height != img.Height {
		t.Fatalf("got decoded dimensions %dx%d, want %dx%d", decoded.Width, decoded.Height, img.Width, img.Height)
	}
	if !bytesEqual(decoded.Pixels, img.Pixels) {
		t.Fatalf("got decoded pixels %v, want %v", decoded.Pixels, img.Pixels)
	}
}

func TestEncodePCX_PixelBufferLengthMismatch_ReturnsError(t *testing.T) {
	img := &PCXImage{Width: 4, Height: 2, Pixels: []byte{1, 2, 3}} // want 8 bytes, have 3

	_, err := EncodePCX(img)

	if err == nil {
		t.Fatalf("expected an error for a mismatched pixel buffer length, got nil")
	}
}

func TestEncodePCX_NonPositiveDimensions_ReturnsError(t *testing.T) {
	cases := []*PCXImage{
		{Width: 0, Height: 2, Pixels: []byte{}},
		{Width: 4, Height: 0, Pixels: []byte{}},
		{Width: -1, Height: 2, Pixels: []byte{1, 2}},
	}

	for _, img := range cases {
		if _, err := EncodePCX(img); err == nil {
			t.Errorf("expected an error for dimensions %dx%d, got nil", img.Width, img.Height)
		}
	}
}
