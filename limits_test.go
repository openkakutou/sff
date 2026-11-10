package sff

import (
	"bytes"
	"encoding/binary"
	"runtime"
	"strings"
	"testing"
)

// assertLimitError fails unless err is non-nil and names a limit.
func assertLimitError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "limit") {
		t.Errorf("error should mention the limit, got: %v", err)
	}
}

// assertAllocUnder fails if running fn allocated more than maxBytes in
// total: proves an oversize declaration is rejected before any big make.
func assertAllocUnder(t *testing.T, maxBytes uint64, fn func()) {
	t.Helper()
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	fn()
	runtime.ReadMemStats(&after)
	if got := after.TotalAlloc - before.TotalAlloc; got > maxBytes {
		t.Errorf("allocated %d bytes, want at most %d", got, maxBytes)
	}
}

func TestDecodePCX_OversizeDeclaredArea_ReturnsLimitErrorWithoutAllocating(t *testing.T) {
	data := make([]byte, pcxHeaderSize)
	data[2], data[3], data[65] = 1, 8, 1
	binary.LittleEndian.PutUint16(data[8:], 40000-1)  // xmax
	binary.LittleEndian.PutUint16(data[10:], 40000-1) // ymax
	binary.LittleEndian.PutUint16(data[66:], 40000)   // bytes per line

	assertAllocUnder(t, 1<<20, func() {
		_, err := DecodePCX(data)
		assertLimitError(t, err)
	})
}

func TestDecodePCX_WideThinSpriteWithinAreaLimit_Decodes(t *testing.T) {
	// Real extreme-aspect sprites (e.g. 17637x249) must keep decoding.
	for _, dim := range [][2]int{{17637, 249}, {16384, 1024}, {1, 16 * 1024 * 1024}} {
		if dim[0] > 65535 || dim[1] > 65535 {
			continue // PCX bounds are 16-bit
		}
		img := &PCXImage{Width: dim[0], Height: dim[1], Pixels: make([]byte, dim[0]*dim[1])}
		data := mustEncodePCX(t, img)
		got, err := DecodePCX(data)
		if err != nil {
			t.Fatalf("%dx%d: %v", dim[0], dim[1], err)
		}
		if got.Width != dim[0] || got.Height != dim[1] {
			t.Errorf("got %dx%d, want %dx%d", got.Width, got.Height, dim[0], dim[1])
		}
	}
}

func TestDecodePCX_OneBeyondAreaLimit_ReturnsLimitError(t *testing.T) {
	// 16384 x 1025 = 16,793,600 > 4096*4096 = 16,777,216.
	data := make([]byte, pcxHeaderSize)
	data[2], data[3], data[65] = 1, 8, 1
	binary.LittleEndian.PutUint16(data[8:], 16384-1)
	binary.LittleEndian.PutUint16(data[10:], 1025-1)
	binary.LittleEndian.PutUint16(data[66:], 16384)
	_, err := DecodePCX(data)
	assertLimitError(t, err)
}

func TestDecodeV2Sprite_OversizeDeclaredDimensions_ReturnsLimitErrorWithoutAllocating(t *testing.T) {
	formats := map[string]struct {
		format, depth int
		data          []byte
	}{
		"raw":   {V2FormatRaw, 8, make([]byte, 16)},
		"rle8":  {V2FormatRLE8, 8, make([]byte, 16)},
		"rle5":  {V2FormatRLE5, 5, make([]byte, 16)},
		"lz5":   {V2FormatLZ5, 5, make([]byte, 16)},
		"png8":  {V2FormatPNG8, 8, make([]byte, 16)},
		"png24": {V2FormatPNG24, 24, make([]byte, 16)},
		"png32": {V2FormatPNG32, 32, make([]byte, 16)},
	}
	for name, c := range formats {
		t.Run(name, func(t *testing.T) {
			assertAllocUnder(t, 1<<20, func() {
				_, err := DecodeV2Sprite(c.format, 65535, 65535, c.depth, c.data)
				assertLimitError(t, err)
			})
		})
	}
}

func TestDecodeV2Sprite_WideThinSpriteWithinAreaLimit_Decodes(t *testing.T) {
	w, h := 17637, 249
	img, err := DecodeV2Sprite(V2FormatRaw, w, h, 8, make([]byte, w*h))
	if err != nil {
		t.Fatal(err)
	}
	if len(img.Pixels) != w*h {
		t.Errorf("got %d pixels, want %d", len(img.Pixels), w*h)
	}
}

func TestDecodeV2Sprite_PNGHeaderOversizeButDeclaredSmall_ReturnsLimitErrorWithoutDecoding(t *testing.T) {
	// A PNG whose own header claims 6000x6000 (36M px, over the limit)
	// while the table declares 2x2: must be refused from the header alone.
	data := buildPNG8(t, 6000, 6000, make([]byte, 6000*6000))
	assertAllocUnder(t, 8<<20, func() {
		_, err := DecodeV2Sprite(V2FormatPNG8, 2, 2, 8, data)
		assertLimitError(t, err)
	})
}

func TestParseV1_ImageCountBeyondTableLimit_ReturnsLimitErrorWithoutAllocating(t *testing.T) {
	file := buildV1File(t, 1, 0xFFFFFFFF, false, nil)
	assertAllocUnder(t, 1<<20, func() {
		_, err := ParseV1(bytes.NewReader(file))
		assertLimitError(t, err)
	})
}

func TestParseV2_SpriteOrPaletteCountBeyondTableLimit_ReturnsLimitErrorWithoutAllocating(t *testing.T) {
	cases := map[string]struct{ sprites, palettes uint32 }{
		"sprites":  {0xFFFFFFFF, 0},
		"palettes": {0, 0xFFFFFFFF},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			file := buildV2File(t, 2, c.sprites, c.palettes, 0, 0, nil, nil)
			assertAllocUnder(t, 1<<20, func() {
				_, err := ParseV2(bytes.NewReader(file))
				assertLimitError(t, err)
			})
		})
	}
}

func TestResolveV1Pixels_OversizeDeclaredLength_ReturnsLimitErrorWithoutAllocating(t *testing.T) {
	file := buildV1File(t, 1, 1, true, []v1SubfileSpec{{length: 0xFFFFFFF0, sharedPalette: true}})
	table, err := ParseV1(bytes.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	assertAllocUnder(t, 1<<20, func() {
		_, err := ResolveV1Pixels(table, bytes.NewReader(file), 0)
		assertLimitError(t, err)
	})
}

func TestResolveSpritePixels_V2OversizeSpriteOrPaletteLength_ReturnsLimitErrorWithoutAllocating(t *testing.T) {
	cases := map[string]struct {
		spriteLen, palLen uint32
	}{
		"sprite data":  {0xFFFFFFF0, 16},
		"palette data": {4, 0xFFFFFFF0},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			file := buildV2File(t, 2, 1, 1, 0, 0,
				[]v2PaletteSpec{{colorCount: 4, length: c.palLen}},
				[]v2SpriteSpec{{width: 2, height: 2, format: V2FormatRaw, colorDepth: 8, length: c.spriteLen}})
			assertAllocUnder(t, 1<<20, func() {
				_, _, _, err := ResolveSpritePixels(bytes.NewReader(file), 0, 0, nil)
				assertLimitError(t, err)
			})
		})
	}
}
