package sff

import (
	"encoding/binary"
	"strings"
	"testing"
)

// buildPCXHeader assembles a 128-byte PCX header with the given fields; the
// remaining header bytes (manufacturer, version, dpi, EGA palette, reserved,
// palette info, filler) are left at zero since the decoder does not inspect
// them.
func buildPCXHeader(encoding, bpp byte, xmin, ymin, xmax, ymax int, planes byte, bytesPerLine int) []byte {
	h := make([]byte, 128)
	h[0] = 0x0A // manufacturer, per the PCX spec, unused by the decoder
	h[1] = 5    // version, unused by the decoder
	h[2] = encoding
	h[3] = bpp
	binary.LittleEndian.PutUint16(h[4:6], uint16(xmin))
	binary.LittleEndian.PutUint16(h[6:8], uint16(ymin))
	binary.LittleEndian.PutUint16(h[8:10], uint16(xmax))
	binary.LittleEndian.PutUint16(h[10:12], uint16(ymax))
	h[65] = planes
	binary.LittleEndian.PutUint16(h[66:68], uint16(bytesPerLine))
	return h
}

func TestDecodePCX_DecodesMixedRunsAndLiterals(t *testing.T) {
	// 4x2 image. Row 0: a run of three 1s then a literal 2. Row 1: four
	// literal bytes (5, 6, 7, 8).
	data := buildPCXHeader(1, 8, 0, 0, 3, 1, 1, 4)
	data = append(data, 0xC3, 0x01, 0x02) // row 0: run(3, value=1), literal 2
	data = append(data, 5, 6, 7, 8)       // row 1: four literals

	img, err := DecodePCX(data)
	if err != nil {
		t.Fatalf("DecodePCX returned error: %v", err)
	}
	if img.Width != 4 || img.Height != 2 {
		t.Fatalf("got Width=%d Height=%d, want Width=4 Height=2", img.Width, img.Height)
	}
	want := []byte{1, 1, 1, 2, 5, 6, 7, 8}
	if !bytesEqual(img.Pixels, want) {
		t.Fatalf("got Pixels=%v, want %v", img.Pixels, want)
	}
}

func TestDecodePCX_HandlesRunSplitAcrossMaxRunLength(t *testing.T) {
	// A single RLE unit can only encode a run of up to 63 (0x3F) pixels, so
	// a 70-pixel-wide solid row must be split into two consecutive run
	// units: one of 63 and one of 7, both ending exactly at the
	// bytes-per-line boundary.
	data := buildPCXHeader(1, 8, 0, 0, 69, 0, 1, 70)
	data = append(data, 0xC0|63, 9) // run(63, value=9)
	data = append(data, 0xC0|7, 9)  // run(7, value=9)

	img, err := DecodePCX(data)
	if err != nil {
		t.Fatalf("DecodePCX returned error: %v", err)
	}
	if img.Width != 70 || img.Height != 1 {
		t.Fatalf("got Width=%d Height=%d, want Width=70 Height=1", img.Width, img.Height)
	}
	for i, p := range img.Pixels {
		if p != 9 {
			t.Fatalf("pixel %d = %d, want 9", i, p)
		}
	}
	if len(img.Pixels) != 70 {
		t.Fatalf("got %d pixels, want 70", len(img.Pixels))
	}
}

func TestDecodePCX_EscapesLiteralBytesThatLookLikeRunMarkers(t *testing.T) {
	// A raw pixel value >= 0xC0 must be encoded as a run of length 1 (its
	// top two bits would otherwise be mistaken for a run marker).
	data := buildPCXHeader(1, 8, 0, 0, 2, 0, 1, 3)
	data = append(data, 0xC1, 0xC5) // run(1, value=0xC5)
	data = append(data, 0x10)       // literal
	data = append(data, 0x05)       // literal

	img, err := DecodePCX(data)
	if err != nil {
		t.Fatalf("DecodePCX returned error: %v", err)
	}
	want := []byte{0xC5, 0x10, 0x05}
	if !bytesEqual(img.Pixels, want) {
		t.Fatalf("got Pixels=%v, want %v", img.Pixels, want)
	}
}

func TestDecodePCX_ErrorsOnDataTooShortForHeader(t *testing.T) {
	_, err := DecodePCX(make([]byte, 50))
	if err == nil {
		t.Fatal("expected an error for data shorter than the PCX header, got nil")
	}
	if !strings.Contains(err.Error(), "too short") {
		t.Fatalf("error %q does not mention the data being too short", err.Error())
	}
}

func TestDecodePCX_ErrorsOnUnsupportedEncoding(t *testing.T) {
	data := buildPCXHeader(2, 8, 0, 0, 3, 1, 1, 4) // encoding=2 is not RLE

	_, err := DecodePCX(data)
	if err == nil {
		t.Fatal("expected an error for unsupported encoding, got nil")
	}
	if !strings.Contains(err.Error(), "encoding") {
		t.Fatalf("error %q does not mention encoding", err.Error())
	}
}

func TestDecodePCX_ErrorsOnUnsupportedBitsPerPixel(t *testing.T) {
	data := buildPCXHeader(1, 4, 0, 0, 3, 1, 1, 4) // 4 bits per pixel unsupported

	_, err := DecodePCX(data)
	if err == nil {
		t.Fatal("expected an error for unsupported bits per pixel, got nil")
	}
	if !strings.Contains(err.Error(), "bits per pixel") {
		t.Fatalf("error %q does not mention bits per pixel", err.Error())
	}
}

func TestDecodePCX_ErrorsOnInvalidImageBounds(t *testing.T) {
	data := buildPCXHeader(1, 8, 3, 0, 0, 1, 1, 4) // xmax < xmin

	_, err := DecodePCX(data)
	if err == nil {
		t.Fatal("expected an error for xmax < xmin, got nil")
	}
}

func TestDecodePCX_ErrorsOnTruncatedRunMissingValueByte(t *testing.T) {
	data := buildPCXHeader(1, 8, 0, 0, 1, 0, 1, 2)
	data = append(data, 0xC1) // run marker with no following value byte

	_, err := DecodePCX(data)
	if err == nil {
		t.Fatal("expected an error for a run marker missing its value byte, got nil")
	}
	if !strings.Contains(err.Error(), "truncated") {
		t.Fatalf("error %q does not mention truncation", err.Error())
	}
}

func TestDecodePCX_ErrorsOnDataEndingMidScanline(t *testing.T) {
	data := buildPCXHeader(1, 8, 0, 0, 3, 0, 1, 4)
	data = append(data, 0x01, 0x02) // only 2 of the 4 declared bytes present

	_, err := DecodePCX(data)
	if err == nil {
		t.Fatal("expected an error for data ending mid-scanline, got nil")
	}
	if !strings.Contains(err.Error(), "truncated") {
		t.Fatalf("error %q does not mention truncation", err.Error())
	}
}

func TestDecodePCX_ErrorsOnRunExceedingScanlineBounds(t *testing.T) {
	data := buildPCXHeader(1, 8, 0, 0, 1, 0, 1, 2) // width=2, bytesPerLine=2
	data = append(data, 0xC3, 0x05)                // run(3, ...) overflows the 2-byte line

	_, err := DecodePCX(data)
	if err == nil {
		t.Fatal("expected an error for a run overflowing the scanline, got nil")
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
