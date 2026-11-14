package sff

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "files", name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return data
}

func jascFile(sep string, count string, lines ...string) []byte {
	return []byte("JASC-PAL" + sep + "0100" + sep + count + sep + strings.Join(lines, sep) + sep)
}

func TestDecodeJASCPalette_NaturalRealFileAndByteRoundTrip(t *testing.T) {
	data := readTestdata(t, "luigi-natural.pal")
	p, err := DecodeJASCPalette(data, PaletteOrderNatural)
	if err != nil {
		t.Fatal(err)
	}
	if want := (color.RGBA{255, 0, 255, 0}); p[0] != want {
		t.Errorf("index 0 = %v, want %v", p[0], want)
	}
	if want := (color.RGBA{240, 240, 240, 255}); p[255] != want {
		t.Errorf("index 255 = %v, want %v", p[255], want)
	}
	if p[1] != (color.RGBA{0, 0, 0, 255}) || p[2] != (color.RGBA{200, 200, 200, 255}) {
		t.Errorf("indices 1,2 = %v %v", p[1], p[2])
	}
	if got := EncodeJASCPalette(p, PaletteOrderNatural); !bytes.Equal(got, data) {
		t.Error("EncodeJASCPalette(decoded) differs from the original file bytes")
	}
}

func TestDecodeJASCPalette_ReversedRealFileMatchesActOrder(t *testing.T) {
	data := readTestdata(t, "raiden-act-order.pal")
	p, err := DecodeJASCPalette(data, PaletteOrderReversed)
	if err != nil {
		t.Fatal(err)
	}
	if want := (color.RGBA{144, 144, 144, 255}); p[255] != want {
		t.Errorf("index 255 = %v, want %v", p[255], want)
	}
	if want := (color.RGBA{248, 200, 160, 255}); p[254] != want {
		t.Errorf("index 254 = %v, want %v", p[254], want)
	}
	if want := (color.RGBA{0, 248, 0, 0}); p[0] != want {
		t.Errorf("index 0 = %v, want %v", p[0], want)
	}
	// Independently build the raw .act buffer from the file's lines.
	lines := strings.Split(strings.TrimRight(string(data), "\r\n"), "\r\n")[3:]
	act := make([]byte, 0, 768)
	for _, l := range lines {
		for _, f := range strings.Fields(l) {
			n := 0
			for _, c := range f {
				n = n*10 + int(c-'0')
			}
			act = append(act, byte(n))
		}
	}
	want, err := DecodeExternalPalette(act)
	if err != nil {
		t.Fatal(err)
	}
	if p != want {
		t.Error("reversed JASC decode differs from DecodeExternalPalette of the same triplets")
	}
	if got := EncodeJASCPalette(p, PaletteOrderReversed); !bytes.Equal(got, data) {
		t.Error("reversed round trip differs from the original file bytes")
	}
}

func TestDecodeJASCPalette_AcceptsLFOnlyLineEndings(t *testing.T) {
	p, err := DecodeJASCPalette(jascFile("\n", "2", "1 2 3", "4 5 6"), PaletteOrderNatural)
	if err != nil {
		t.Fatal(err)
	}
	if p[1] != (color.RGBA{4, 5, 6, 255}) {
		t.Errorf("index 1 = %v", p[1])
	}
}

func jascN(n int) []byte {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = "10 20 " + string(rune('0'+i%10))
	}
	return jascFile("\r\n", strconv.Itoa(n), lines...)
}

func TestDecodeJASCPalette_FewerThan256PadsWithOpaqueBlack(t *testing.T) {
	p, err := DecodeJASCPalette(jascN(16), PaletteOrderNatural)
	if err != nil {
		t.Fatal(err)
	}
	if p[0] != (color.RGBA{10, 20, 0, 0}) {
		t.Errorf("index 0 = %v", p[0])
	}
	if p[15] != (color.RGBA{10, 20, 5, 255}) {
		t.Errorf("index 15 = %v", p[15])
	}
	for i := 16; i < 256; i++ {
		if p[i] != (color.RGBA{0, 0, 0, 255}) {
			t.Fatalf("index %d = %v, want opaque black", i, p[i])
		}
	}
}

func TestDecodeJASCPalette_ReversedFewerThan256MapsEntryZeroToLastIndex(t *testing.T) {
	p, err := DecodeJASCPalette(jascN(16), PaletteOrderReversed)
	if err != nil {
		t.Fatal(err)
	}
	if p[15] != (color.RGBA{10, 20, 0, 255}) {
		t.Errorf("index 15 = %v, want entry 0", p[15])
	}
	if p[0] != (color.RGBA{10, 20, 5, 0}) {
		t.Errorf("index 0 = %v, want last entry, transparent", p[0])
	}
	if p[16] != (color.RGBA{0, 0, 0, 255}) {
		t.Errorf("index 16 = %v", p[16])
	}
}

func TestDecodeJASCPalette_AcceptsTrailingBlankLines(t *testing.T) {
	data := append(jascFile("\r\n", "1", "1 2 3"), "\r\n\r\n"...)
	if _, err := DecodeJASCPalette(data, PaletteOrderNatural); err != nil {
		t.Fatal(err)
	}
}

func TestDecodeJASCPalette_RejectsMalformedFiles(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		frag string
	}{
		{"empty", nil, "line 1"},
		{"bad magic", []byte("JASC-PAX\r\n0100\r\n1\r\n1 2 3\r\n"), "line 1"},
		{"bad version", []byte("JASC-PAL\r\n0200\r\n1\r\n1 2 3\r\n"), "line 2"},
		{"count zero", jascFile("\r\n", "0"), "line 3"},
		{"count 257", jascFile("\r\n", "257", "1 2 3"), "257"},
		{"count non numeric", jascFile("\r\n", "abc", "1 2 3"), "abc"},
		{"component 256", jascFile("\r\n", "2", "1 2 3", "1 256 3"), "line 5"},
		{"component negative", jascFile("\r\n", "2", "1 2 3", "1 -1 3"), "line 5"},
		{"component non numeric", jascFile("\r\n", "2", "1 2 3", "1 x 3"), "line 5"},
		{"two components", jascFile("\r\n", "2", "1 2 3", "1 2"), "line 5"},
		{"fewer lines than count", []byte("JASC-PAL\r\n0100\r\n3\r\n1 2 3\r\n1 2 3"), "expected 3 color entries, found 2"},
		{"fewer lines than count with trailing newline", jascFile("\r\n", "3", "1 2 3", "1 2 3"), "expected 3 color entries, found 2"},
		{"blank line between entries", jascFile("\r\n", "3", "1 2 3", "", "1 2 3"), "line 5: expected 3 components"},
		{"extra line", jascFile("\r\n", "1", "1 2 3", "4 5 6"), "line 5"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := DecodeJASCPalette(c.data, PaletteOrderNatural)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.HasPrefix(err.Error(), "sff: jasc palette: ") || !strings.Contains(err.Error(), c.frag) {
				t.Errorf("error %q lacks prefix or %q", err, c.frag)
			}
		})
	}
}

func TestDecodePNGPalette_MatchesActOnRealFixtures(t *testing.T) {
	p, err := DecodePNGPalette(readTestdata(t, "aba-palette-no-trns.png"))
	if err != nil {
		t.Fatal(err)
	}
	act, err := DecodeExternalPalette(readTestdata(t, "aba-palette.act"))
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < 256; i++ {
		if p[i] != act[i] {
			t.Fatalf("index %d: png %v, act %v", i, p[i], act[i])
		}
	}
	if p[0].A != 0 {
		t.Errorf("index 0 alpha = %d", p[0].A)
	}
	if p[1] != (color.RGBA{255, 255, 206, 255}) {
		t.Errorf("index 1 = %v, want PLTE entry 1", p[1])
	}
}

func TestDecodePNGPalette_TRNSFixtureFollowsAlphaRule(t *testing.T) {
	p, err := DecodePNGPalette(readTestdata(t, "kirby-palette-trns.png"))
	if err != nil {
		t.Fatal(err)
	}
	if p[0].A != 0 {
		t.Errorf("index 0 alpha = %d", p[0].A)
	}
	if p[1] != (color.RGBA{225, 119, 131, 255}) || p[2] != (color.RGBA{97, 78, 80, 255}) {
		t.Errorf("indices 1,2 = %v %v", p[1], p[2])
	}
	for i := 1; i < 256; i++ {
		if p[i].A != 255 {
			t.Fatalf("index %d alpha = %d", i, p[i].A)
		}
	}
}

func TestEncodePNGPalette_RoundTripAndSwatchLayout(t *testing.T) {
	var p Palette
	for i := range p {
		p[i] = color.RGBA{byte(i), byte(255 - i), byte(i * 7), 255}
	}
	p[0].A = 0
	data, err := EncodePNGPalette(p)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodePNGPalette(data)
	if err != nil {
		t.Fatal(err)
	}
	if got != p {
		t.Error("round trip changed the palette")
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	pi, ok := img.(*image.Paletted)
	if !ok {
		t.Fatalf("decoded type %T, want *image.Paletted", img)
	}
	if b := pi.Bounds(); b.Dx() != 16 || b.Dy() != 16 {
		t.Fatalf("bounds %v", b)
	}
	for i := 0; i < 256; i++ {
		if pi.Pix[i] != byte(i) {
			t.Fatalf("Pix[%d] = %d", i, pi.Pix[i])
		}
	}
	if _, _, _, a := pi.Palette[0].RGBA(); a != 0 {
		t.Error("palette index 0 not transparent in PNG")
	}
}

func TestDecodePNGPalette_RejectsNonIndexedAndCorrupt(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	_, err := DecodePNGPalette(buf.Bytes())
	if err == nil || !strings.Contains(err.Error(), "sff: png palette: not an indexed (palette) PNG") {
		t.Errorf("non-indexed error = %v", err)
	}
	_, err = DecodePNGPalette([]byte("not a png at all"))
	if err == nil || !strings.HasPrefix(err.Error(), "sff: png palette: ") {
		t.Errorf("corrupt error = %v", err)
	}
}

func TestDecodePNGPalette_AppliesTRNSIgnoreRule(t *testing.T) {
	pal := color.Palette{
		color.NRGBA{1, 2, 3, 255},
		color.NRGBA{4, 5, 6, 255},
		color.NRGBA{7, 8, 9, 255},
		color.NRGBA{10, 11, 12, 255},
		color.NRGBA{13, 14, 15, 255},
		color.NRGBA{10, 20, 30, 128},
	}
	img := image.NewPaletted(image.Rect(0, 0, 2, 1), pal)
	img.Pix[0], img.Pix[1] = 0, 5
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("tRNS")) {
		t.Fatal("fixture has no tRNS chunk")
	}
	p, err := DecodePNGPalette(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if want := (color.RGBA{10, 20, 30, 255}); p[5] != want {
		t.Errorf("index 5 = %v, want %v (tRNS alpha ignored, RGB kept)", p[5], want)
	}
	if p[0].A != 0 {
		t.Errorf("index 0 alpha = %d, want 0", p[0].A)
	}
}

func pngChunk(typ string, body []byte) []byte {
	out := binary.BigEndian.AppendUint32(nil, uint32(len(body)))
	out = append(out, typ...)
	out = append(out, body...)
	return binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(append([]byte(typ), body...)))
}

func TestDecodePNGPalette_RejectsOversizedDeclaredDimensionsBeforeDecoding(t *testing.T) {
	ihdr := binary.BigEndian.AppendUint32(nil, 100000)
	ihdr = binary.BigEndian.AppendUint32(ihdr, 100000)
	ihdr = append(ihdr, 8, 3, 0, 0, 0) // 8-bit, indexed, default compression/filter/interlace
	data := []byte("\x89PNG\r\n\x1a\n")
	data = append(data, pngChunk("IHDR", ihdr)...)
	data = append(data, pngChunk("PLTE", []byte{1, 2, 3, 4, 5, 6})...)
	data = append(data, pngChunk("IDAT", []byte{0})...) // invalid zlib stream: the pixel data is never needed
	data = append(data, pngChunk("IEND", nil)...)
	_, err := DecodePNGPalette(data)
	if err == nil || !strings.HasPrefix(err.Error(), "sff: png palette: ") || !strings.Contains(err.Error(), "area limit") {
		t.Errorf("error = %v, want png palette error mentioning the area limit", err)
	}
}
