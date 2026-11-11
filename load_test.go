package sff

import (
	"bytes"
	"os"
	"runtime"
	"testing"
)

func TestLoad_V1File_AssemblesSpriteGroupsWithDecodedDimensionsAndDerivedPalettes(t *testing.T) {
	sprites := []V1WriteSprite{
		{
			Group: 0, Image: 0, AxisX: 4, AxisY: -2, SharedPalette: false,
			PixelData: mustEncodePCX(t, &PCXImage{Width: 4, Height: 2, Pixels: []byte{1, 1, 1, 1, 2, 2, 2, 2}}),
			Palette:   v1TestPalette(),
		},
		{
			// Shares sprite 0's palette: same derived Palette value.
			Group: 0, Image: 1, AxisX: 5, AxisY: -1, SharedPalette: true,
			PixelData: mustEncodePCX(t, &PCXImage{Width: 3, Height: 1, Pixels: []byte{3, 3, 3}}),
		},
		{
			// Its own palette again: derived Palette value bumps up.
			Group: 1, Image: 0, AxisX: 0, AxisY: 0, SharedPalette: false,
			PixelData: mustEncodePCX(t, &PCXImage{Width: 2, Height: 2, Pixels: []byte{9, 9, 9, 9}}),
			Palette:   v1TestPalette(),
		},
	}

	var buf bytes.Buffer
	if err := SerializeV1(&buf, [4]byte{1, 0, 0, 1}, false, sprites); err != nil {
		t.Fatalf("test setup: SerializeV1 failed: %v", err)
	}

	groups, err := Load(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if len(groups) != 2 {
		t.Fatalf("expected 2 sprite groups, got %d", len(groups))
	}
	if groups[0].Index != 0 || groups[1].Index != 1 {
		t.Fatalf("expected groups in ascending index order [0, 1], got [%d, %d]", groups[0].Index, groups[1].Index)
	}

	if len(groups[0].Sprites) != 2 {
		t.Fatalf("expected 2 sprites in group 0, got %d", len(groups[0].Sprites))
	}
	want := Sprite{Group: 0, Image: 0, Width: 4, Height: 2, AxisX: 4, AxisY: -2, Palette: 0}
	if got := groups[0].Sprites[0]; got != want {
		t.Errorf("sprite (0,0): expected %+v, got %+v", want, got)
	}
	want = Sprite{Group: 0, Image: 1, Width: 3, Height: 1, AxisX: 5, AxisY: -1, Palette: 0}
	if got := groups[0].Sprites[1]; got != want {
		t.Errorf("sprite (0,1): expected %+v, got %+v", want, got)
	}

	if len(groups[1].Sprites) != 1 {
		t.Fatalf("expected 1 sprite in group 1, got %d", len(groups[1].Sprites))
	}
	want = Sprite{Group: 1, Image: 0, Width: 2, Height: 2, AxisX: 0, AxisY: 0, Palette: 1}
	if got := groups[1].Sprites[0]; got != want {
		t.Errorf("sprite (1,0): expected %+v, got %+v", want, got)
	}
}

func TestLoad_V2File_AssemblesSpriteGroupsDirectlyFromTableWithoutDecodingPixels(t *testing.T) {
	sprites := []V2WriteSprite{
		{
			Group: 0, Image: 0, Width: 3, Height: 2, AxisX: 1, AxisY: -1,
			Format: V2FormatRaw, ColorDepth: 8, PaletteIndex: 0,
			// Deliberately unsupported pixel encoding — Load must not need
			// to decode pixel data for v2 to succeed.
			PixelData: []byte{0xFF, 0xFF, 0xFF},
		},
		{
			Group: 1, Image: 0, Width: 1, Height: 1, AxisX: 0, AxisY: 0,
			Format: V2FormatRLE8, ColorDepth: 8, PaletteIndex: 2,
			PixelData: []byte{0x00},
		},
	}

	var buf bytes.Buffer
	if err := SerializeV2(&buf, [4]byte{0, 1, 0, 2}, sprites, nil); err != nil {
		t.Fatalf("test setup: SerializeV2 failed: %v", err)
	}

	groups, err := Load(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if len(groups) != 2 {
		t.Fatalf("expected 2 sprite groups, got %d", len(groups))
	}

	want := Sprite{Group: 0, Image: 0, Width: 3, Height: 2, AxisX: 1, AxisY: -1, Palette: 0}
	if got := groups[0].Sprites[0]; got != want {
		t.Errorf("sprite (0,0): expected %+v, got %+v", want, got)
	}
	want = Sprite{Group: 1, Image: 0, Width: 1, Height: 1, AxisX: 0, AxisY: 0, Palette: 2}
	if got := groups[1].Sprites[0]; got != want {
		t.Errorf("sprite (1,0): expected %+v, got %+v", want, got)
	}
}

func TestLoad_V1LinkedSprite_ResolvesDimensionsFromLinkTarget(t *testing.T) {
	targetPixels := &PCXImage{Width: 5, Height: 3, Pixels: make([]byte, 15)}
	for i := range targetPixels.Pixels {
		targetPixels.Pixels[i] = byte(i)
	}

	sprites := []V1WriteSprite{
		{Group: 0, Image: 0, PixelData: mustEncodePCX(t, targetPixels), Palette: v1TestPalette()},
		{Group: 0, Image: 1, SharedPalette: true, LinkedIndex: 0}, // no PixelData, links to sprite 0
	}

	var buf bytes.Buffer
	if err := SerializeV1(&buf, [4]byte{1, 0, 0, 1}, true, sprites); err != nil {
		t.Fatalf("test setup: SerializeV1 failed: %v", err)
	}

	groups, err := Load(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if len(groups) != 1 || len(groups[0].Sprites) != 2 {
		t.Fatalf("expected 1 group with 2 sprites, got %+v", groups)
	}
	linked := groups[0].Sprites[1]
	if linked.Width != 5 || linked.Height != 3 {
		t.Errorf("expected linked sprite to inherit dimensions 5x3 from its link target, got %dx%d", linked.Width, linked.Height)
	}
}

// TestResolveV1Pixels_LastSharedPaletteSpriteOverstatesLength_DecodesAnyway
// covers a real-file shape found by backlog item 007's corpus scan (17
// sprites across 7 real character files, e.g. Marvel's Thor and the
// bundled Kung Fu Man's own ending/intro sheets): a v1 file's last sprite
// that owns real pixel data can declare a Length up to 768 bytes more than
// the file actually has left to give — the file simply ends there. Since
// this only happens on a SharedPalette sprite, and a SharedPalette-false
// sprite's declared Length already legitimately includes its own trailing
// 768-byte palette block, the fix mirrors that: retry once without those
// 768 bytes when the full declared read overruns the underlying reader.
// See .vibe/decisions/017-v1-last-shared-palette-sprite-trailing-block.md.
func TestResolveV1Pixels_LastSharedPaletteSpriteOverstatesLength_DecodesAnyway(t *testing.T) {
	f := openTestdataFile(t, "v1-last-sprite-shared-palette-trailing-block.sff")
	defer f.Close()

	table, err := ParseV1(f)
	if err != nil {
		t.Fatalf("ParseV1: %v", err)
	}
	last := len(table.Sprites) - 1
	target := table.Sprites[last]
	if !target.SharedPalette || target.Length == 0 {
		t.Fatalf("test fixture assumption broken: last entry must be a real, SharedPalette sprite, got %+v", target)
	}

	img, err := ResolveV1Pixels(table, f, last)
	if err != nil {
		t.Fatalf("ResolveV1Pixels: expected the overstated length to be tolerated, got error: %v", err)
	}
	if img.Width <= 0 || img.Height <= 0 {
		t.Errorf("expected a real decoded image, got %dx%d", img.Width, img.Height)
	}
	if len(img.Pixels) != img.Width*img.Height {
		t.Errorf("expected %d pixels (%dx%d), got %d", img.Width*img.Height, img.Width, img.Height, len(img.Pixels))
	}
}

// TestResolveV1Pixels_NonSharedSpriteEOF_StillReturnsError confirms the
// retry above is scoped to SharedPalette sprites only: a SharedPalette-
// false sprite whose declared Length already overruns the reader is a
// different, still-undiagnosed real-file shape (backlog item 007's other
// remaining gap) and must keep erroring rather than silently trying yet
// another guessed length.
func TestResolveV1Pixels_NonSharedSpriteEOF_StillReturnsError(t *testing.T) {
	sprites := []V1WriteSprite{
		{
			Group: 0, Image: 0, SharedPalette: false,
			PixelData: mustEncodePCX(t, &PCXImage{Width: 2, Height: 2, Pixels: []byte{1, 1, 1, 1}}),
			Palette:   v1TestPalette(),
		},
	}
	var buf bytes.Buffer
	if err := SerializeV1(&buf, [4]byte{1, 0, 0, 1}, false, sprites); err != nil {
		t.Fatalf("test setup: SerializeV1 failed: %v", err)
	}
	full := buf.Bytes()

	table, err := ParseV1(bytes.NewReader(full))
	if err != nil {
		t.Fatalf("ParseV1: %v", err)
	}
	// Truncate right after the sprite's own pixel-data offset, well short
	// of its declared (palette-block-inclusive) Length, so the read of
	// its actual pixel data — not just its trailing palette block — fails.
	truncated := full[:table.Sprites[0].Offset+1]

	if _, err := ResolveV1Pixels(table, bytes.NewReader(truncated), 0); err == nil {
		t.Fatal("expected an error for a truncated non-shared sprite, got nil")
	}
}

// TestLoad_V1FirstSpriteHasNoPixelDataOfItsOwn_ReturnsDescriptiveError covers
// the terminal case of the v1 linking rule (decision 011): table index 0
// can never inherit pixel data from an earlier entry, since none exists.
func TestLoad_V1FirstSpriteHasNoPixelDataOfItsOwn_ReturnsDescriptiveError(t *testing.T) {
	sprites := []V1WriteSprite{
		// index 0: no PixelData; SerializeV1 requires a valid (in-range,
		// non-self) LinkedIndex to accept a linked sprite at all, but the
		// corrected read-side rule never consults it for a zero-length
		// sprite — table index 0 has no earlier entry to inherit from
		// regardless of what it points to.
		{Group: 0, Image: 0, SharedPalette: true, LinkedIndex: 1},
		{Group: 0, Image: 1, PixelData: mustEncodePCX(t, &PCXImage{Width: 1, Height: 1, Pixels: []byte{1}}), Palette: v1TestPalette()},
	}

	var buf bytes.Buffer
	if err := SerializeV1(&buf, [4]byte{1, 0, 0, 1}, false, sprites); err != nil {
		t.Fatalf("test setup: SerializeV1 failed: %v", err)
	}

	_, err := Load(bytes.NewReader(buf.Bytes()))
	if err == nil {
		t.Fatal("expected an error for a first sprite with no pixel data of its own, got nil")
	}
}

func TestResolveV1Pixels_IndexOutOfRange_ReturnsError(t *testing.T) {
	table := &V1SpriteTable{Sprites: []V1SpriteEntry{
		{Group: 0, Image: 0, Length: 4, SharedPalette: true},
	}}
	data := make([]byte, 100)

	if _, err := ResolveV1Pixels(table, bytes.NewReader(data), -1); err == nil {
		t.Error("expected an error for a negative index, got nil")
	}
	if _, err := ResolveV1Pixels(table, bytes.NewReader(data), 5); err == nil {
		t.Error("expected an error for an out-of-range index, got nil")
	}
}

func TestLoad_UnrecognizedSignature_ReturnsDescriptiveError(t *testing.T) {
	data := bytes.Repeat([]byte("not-an-sff-file!"), 4)

	_, err := Load(bytes.NewReader(data))
	if err == nil {
		t.Fatal("expected an error for data with an unrecognized signature, got nil")
	}
}

func TestLoad_TruncatedFile_ReturnsErrorRatherThanPanicking(t *testing.T) {
	data := []byte{1, 2, 3} // shorter than the 16-byte signature/version peek

	_, err := Load(bytes.NewReader(data))
	if err == nil {
		t.Fatal("expected an error for a file too short to contain a header, got nil")
	}
}

// FuzzLoad checks Load and ResolveSpritePixels (both versions, auto-
// detected) never panic or hang on arbitrary bytes, and that every
// resolved sprite's pixel buffer matches its reported size.
func FuzzLoad(f *testing.F) {
	addFixtureSeeds(f, "v")

	f.Fuzz(func(t *testing.T, data []byte) {
		groups, err := Load(bytes.NewReader(data))
		if err != nil {
			return
		}
		for _, g := range groups {
			for _, s := range g.Sprites {
				px, w, h, err := ResolveSpritePixels(bytes.NewReader(data), s.Group, s.Image, nil)
				if err == nil && len(px) != w*h {
					t.Fatalf("len(pixels)=%d, want %dx%d", len(px), w, h)
				}
			}
		}
	})
}

// largeSheetSprites is the sprite count of the in-memory sheets the
// benchmarks build: the order of magnitude of a big real character sheet.
const largeSheetSprites = 2000

// largeSheetGroup and largeSheetImage give sprite i its (group, image) key,
// shared by the sheet builders and the benchmarks that look a sprite up.
func largeSheetGroup(i int) int { return i / 50 }
func largeSheetImage(i int) int { return i % 50 }

// largeSheetLinked reports whether sprite i is written as a zero-length
// linked sprite (every fourth one except sprite 0, a realistic share).
func largeSheetLinked(i int) bool { return i > 0 && i%4 == 3 }

// largeSheetBenchTarget is the sprite the resolve benchmarks look up: near
// the end of the table, and not linked (v2 cannot resolve linked sprites).
const largeSheetBenchTarget = largeSheetSprites - 2

func buildLargeV1Sheet(tb testing.TB) []byte {
	tb.Helper()
	img := &PCXImage{Width: 8, Height: 8, Pixels: make([]byte, 64)}
	pcx, err := EncodePCX(img)
	if err != nil {
		tb.Fatalf("test setup: EncodePCX failed: %v", err)
	}
	sprites := make([]V1WriteSprite, largeSheetSprites)
	for i := range sprites {
		s := V1WriteSprite{Group: largeSheetGroup(i), Image: largeSheetImage(i), SharedPalette: i > 0}
		if i == 0 {
			s.Palette = v1TestPalette()
		}
		if largeSheetLinked(i) {
			s.LinkedIndex = i - 1
		} else {
			s.PixelData = pcx
		}
		sprites[i] = s
	}
	var buf bytes.Buffer
	if err := SerializeV1(&buf, [4]byte{1, 0, 0, 1}, true, sprites); err != nil {
		tb.Fatalf("test setup: SerializeV1 failed: %v", err)
	}
	return buf.Bytes()
}

func buildLargeV2Sheet(tb testing.TB) []byte {
	tb.Helper()
	sprites := make([]V2WriteSprite, largeSheetSprites)
	for i := range sprites {
		s := V2WriteSprite{
			Group: largeSheetGroup(i), Image: largeSheetImage(i), Width: 8, Height: 8,
			Format: V2FormatRaw, ColorDepth: 8,
		}
		if largeSheetLinked(i) {
			s.LinkedIndex = i - 1
		} else {
			s.PixelData = make([]byte, 64)
		}
		sprites[i] = s
	}
	palettes := []V2WritePalette{{Group: 1, Number: 1, ColorCount: 256, ColorData: make([]byte, 256*4)}}
	var buf bytes.Buffer
	if err := SerializeV2(&buf, [4]byte{0, 1, 0, 2}, sprites, palettes); err != nil {
		tb.Fatalf("test setup: SerializeV2 failed: %v", err)
	}
	return buf.Bytes()
}

func benchmarkLoad(b *testing.B, data []byte) {
	r := bytes.NewReader(data)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Load(r); err != nil {
			b.Fatalf("Load: %v", err)
		}
	}
}

func BenchmarkLoad_V1LargeSheet(b *testing.B) { benchmarkLoad(b, buildLargeV1Sheet(b)) }

func BenchmarkLoad_V2LargeSheet(b *testing.B) { benchmarkLoad(b, buildLargeV2Sheet(b)) }

// sffBenchFileEnv names the optional environment variable pointing
// BenchmarkLoad_RealFile at a real .sff file on the local machine. Like
// sffCorpusDirEnv, it is never hardcoded: the benchmark is skipped when unset.
const sffBenchFileEnv = "SFF_BENCH_FILE"

func BenchmarkLoad_RealFile(b *testing.B) {
	path := os.Getenv(sffBenchFileEnv)
	if path == "" {
		b.Skipf("%s not set", sffBenchFileEnv)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		b.Fatalf("reading %s: %v", sffBenchFileEnv, err)
	}
	benchmarkLoad(b, data)
}

// zeroLengthChainTable returns a v1 table of n sprites where sprite 0 owns a
// 3x2 image with pixels 10..15 and every later sprite is a zero-length
// sprite, plus the reader holding sprite 0's PCX data.
func zeroLengthChainTable(t *testing.T, n int) (*V1SpriteTable, *bytes.Reader) {
	t.Helper()
	pcx := mustEncodePCX(t, &PCXImage{Width: 3, Height: 2, Pixels: []byte{10, 11, 12, 13, 14, 15}})
	table := &V1SpriteTable{Sprites: make([]V1SpriteEntry, n)}
	table.Sprites[0] = V1SpriteEntry{Offset: 0, Length: len(pcx), SharedPalette: true}
	for i := 1; i < n; i++ {
		table.Sprites[i] = V1SpriteEntry{Image: i, SharedPalette: true}
	}
	return table, bytes.NewReader(pcx)
}

func assertPCXPixels(t *testing.T, got *PCXImage, width, height int, want []byte) {
	t.Helper()
	if got.Width != width || got.Height != height || !bytes.Equal(got.Pixels, want) {
		t.Errorf("expected %dx%d pixels %v, got %dx%d pixels %v", width, height, want, got.Width, got.Height, got.Pixels)
	}
}

// bytesAllocatedBy reports the heap bytes f allocates, best of a few runs
// to ignore unrelated background allocation.
func bytesAllocatedBy(f func()) uint64 {
	best := ^uint64(0)
	for range 5 {
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		f()
		runtime.ReadMemStats(&after)
		if d := after.TotalAlloc - before.TotalAlloc; d < best {
			best = d
		}
	}
	return best
}

func TestResolveV1Pixels_AllocationDoesNotGrowWithTableSize(t *testing.T) {
	small, smallR := zeroLengthChainTable(t, 10)
	large, largeR := zeroLengthChainTable(t, 5000)

	resolveLast := func(table *V1SpriteTable, r *bytes.Reader) func() {
		return func() {
			if _, err := ResolveV1Pixels(table, r, len(table.Sprites)-1); err != nil {
				t.Fatalf("ResolveV1Pixels: %v", err)
			}
		}
	}
	smallBytes := bytesAllocatedBy(resolveLast(small, smallR))
	largeBytes := bytesAllocatedBy(resolveLast(large, largeR))

	// Resolving the same single image must cost the same however many
	// sprites the table has; 4 KiB of slack covers measurement noise only.
	// (A per-call visited set sized to the table costs far more than that
	// at 5000 sprites.)
	if largeBytes > smallBytes+4096 {
		t.Errorf("resolving through a 5000-sprite table allocated %d bytes vs %d for a 10-sprite table", largeBytes, smallBytes)
	}
}

func TestResolveV1Pixels_LongZeroLengthChain_ResolvesToFirstSpritesPixels(t *testing.T) {
	table, r := zeroLengthChainTable(t, 5000)

	got, err := ResolveV1Pixels(table, r, 4999)
	if err != nil {
		t.Fatalf("ResolveV1Pixels: %v", err)
	}
	assertPCXPixels(t, got, 3, 2, []byte{10, 11, 12, 13, 14, 15})
}

func TestResolveV1Pixels_BackwardLinkedIndex_ResolvesLinkTargetNotOwnData(t *testing.T) {
	a := mustEncodePCX(t, &PCXImage{Width: 1, Height: 2, Pixels: []byte{1, 2}})
	b := mustEncodePCX(t, &PCXImage{Width: 2, Height: 1, Pixels: []byte{3, 4}})
	c := mustEncodePCX(t, &PCXImage{Width: 1, Height: 1, Pixels: []byte{9}})
	data := append(append(append([]byte{}, a...), b...), c...)
	offB, offC := int64(len(a)), int64(len(a)+len(b))
	table := &V1SpriteTable{Sprites: []V1SpriteEntry{
		{Offset: 0, Length: len(a), SharedPalette: true},
		{Image: 1, Offset: offB, Length: len(b), SharedPalette: true},
		{Image: 2, Offset: offC, Length: len(c), SharedPalette: true, LinkedIndex: 1}, // backward link: wins over own data
		{Image: 3, Offset: offC, Length: len(c), SharedPalette: true, LinkedIndex: 3}, // self link: ignored, own data
		{Image: 4, SharedPalette: true},                                               // zero-length: inherits sprite 3
	}}
	r := bytes.NewReader(data)

	for _, tc := range []struct {
		index         int
		width, height int
		want          []byte
	}{
		{2, 2, 1, []byte{3, 4}},
		{3, 1, 1, []byte{9}},
		{4, 1, 1, []byte{9}},
	} {
		got, err := ResolveV1Pixels(table, r, tc.index)
		if err != nil {
			t.Fatalf("sprite %d: ResolveV1Pixels: %v", tc.index, err)
		}
		assertPCXPixels(t, got, tc.width, tc.height, tc.want)
	}
}

func TestResolveV1Pixels_MultiHopLinkThroughZeroLengthSprite_ResolvesFinalOwner(t *testing.T) {
	a := mustEncodePCX(t, &PCXImage{Width: 1, Height: 2, Pixels: []byte{1, 2}})
	b := mustEncodePCX(t, &PCXImage{Width: 2, Height: 1, Pixels: []byte{3, 4}})
	c := mustEncodePCX(t, &PCXImage{Width: 1, Height: 1, Pixels: []byte{9}})
	data := append(append(append([]byte{}, a...), b...), c...)
	offB, offC := int64(len(a)), int64(len(a)+len(b))
	table := &V1SpriteTable{Sprites: []V1SpriteEntry{
		{Offset: 0, Length: len(a), SharedPalette: true},
		{Image: 1, Offset: offB, Length: len(b), SharedPalette: true},
		{Image: 2, SharedPalette: true},                                               // zero-length: inherits sprite 1
		{Image: 3, Offset: offC, Length: len(c), SharedPalette: true, LinkedIndex: 2}, // links to a zero-length sprite
		{Image: 4, Offset: offC, Length: len(c), SharedPalette: true, LinkedIndex: 3}, // links to a sprite that links again
	}}
	r := bytes.NewReader(data)

	for _, index := range []int{3, 4} {
		got, err := ResolveV1Pixels(table, r, index)
		if err != nil {
			t.Fatalf("sprite %d: ResolveV1Pixels: %v", index, err)
		}
		assertPCXPixels(t, got, 2, 1, []byte{3, 4})
	}
}

func TestResolveV1Pixels_ZeroLengthChainReachingSpriteZeroWithoutData_ReturnsError(t *testing.T) {
	table := &V1SpriteTable{Sprites: []V1SpriteEntry{{}, {Image: 1}, {Image: 2}}}

	if _, err := ResolveV1Pixels(table, bytes.NewReader(nil), 2); err == nil {
		t.Fatal("expected an error when the chain ends at a zero-length sprite 0, got nil")
	}
}

func TestResolveV1Pixels_LinkToOutOfRangeIndex_ReturnsError(t *testing.T) {
	table := &V1SpriteTable{Sprites: []V1SpriteEntry{
		{Length: 4, SharedPalette: true},
		{Image: 1, Length: 4, SharedPalette: true, LinkedIndex: -5},
		{Image: 2}, // zero-length: inherits sprite 1, whose link is invalid
	}}

	for _, i := range []int{1, 2} {
		if _, err := ResolveV1Pixels(table, bytes.NewReader(make([]byte, 16)), i); err == nil {
			t.Errorf("sprite %d: expected an error for an out-of-range link, got nil", i)
		}
	}
}

func TestLoad_V1File_ZeroLengthSpriteNotMarkedShared_ReportsInheritedPaletteNumber(t *testing.T) {
	sprites := []V1WriteSprite{
		{
			Group: 0, Image: 0, SharedPalette: false,
			PixelData: mustEncodePCX(t, &PCXImage{Width: 2, Height: 1, Pixels: []byte{1, 2}}),
			Palette:   v1TestPalette(),
		},
		{
			// Linked sprite with no pixel data of its own; the shared bit
			// is cleared below, since the serializer cannot write a
			// non-shared entry without a palette block.
			Group: 0, Image: 1, SharedPalette: true, LinkedIndex: 0,
		},
		{
			Group: 1, Image: 0, SharedPalette: false,
			PixelData: mustEncodePCX(t, &PCXImage{Width: 2, Height: 1, Pixels: []byte{3, 4}}),
			Palette:   v1TestPalette(),
		},
	}

	data := serializeV1ForTest(t, sprites)
	table, err := ParseV1(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("test setup: ParseV1 failed: %v", err)
	}
	// The sprite is drawn with sprite 0's palette whatever its shared bit
	// says (decision 011): clear the bit in its subheader, which sits in the
	// 32 bytes before its pixel data offset (palette flag at byte 18).
	data[table.Sprites[1].Offset-subheaderSize+18] = 0

	assertLoadedV1PaletteNumbers(t, data, sprites, []int{0, 0, 1})
}

func TestLoad_V1File_SharedGroupZeroImageZeroSpriteAboveIndexZero_ReportsIndexZerosPaletteNumber(t *testing.T) {
	sprites := []V1WriteSprite{
		{
			Group: 0, Image: 1, SharedPalette: false,
			PixelData: mustEncodePCX(t, &PCXImage{Width: 2, Height: 1, Pixels: []byte{1, 2}}),
			Palette:   v1TestPalette(),
		},
		{
			Group: 1, Image: 0, SharedPalette: false,
			PixelData: mustEncodePCX(t, &PCXImage{Width: 2, Height: 1, Pixels: []byte{3, 4}}),
			Palette:   v1TestPalette(),
		},
		{
			// Shared (0, 0) entry: inherits table index 0's palette, not the
			// immediately preceding sprite's (decision 011).
			Group: 0, Image: 0, SharedPalette: true,
			PixelData: mustEncodePCX(t, &PCXImage{Width: 2, Height: 1, Pixels: []byte{5, 6}}),
		},
	}

	assertLoadedV1PaletteNumbers(t, serializeV1ForTest(t, sprites), sprites, []int{0, 1, 0})
}

func serializeV1ForTest(t *testing.T, sprites []V1WriteSprite) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := SerializeV1(&buf, [4]byte{1, 0, 0, 1}, false, sprites); err != nil {
		t.Fatalf("test setup: SerializeV1 failed: %v", err)
	}
	return buf.Bytes()
}

// assertLoadedV1PaletteNumbers loads data and checks want[i], the expected
// Palette of sprites[i].
func assertLoadedV1PaletteNumbers(t *testing.T, data []byte, sprites []V1WriteSprite, want []int) {
	t.Helper()
	groups, err := Load(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	palettes := map[[2]int]int{}
	for _, g := range groups {
		for _, s := range g.Sprites {
			palettes[[2]int{s.Group, s.Image}] = s.Palette
		}
	}
	for i, s := range sprites {
		got, ok := palettes[[2]int{s.Group, s.Image}]
		if !ok {
			t.Errorf("sprite (%d,%d) missing from Load result", s.Group, s.Image)
		} else if got != want[i] {
			t.Errorf("sprite (%d,%d): Palette = %d, want %d", s.Group, s.Image, got, want[i])
		}
	}
}
