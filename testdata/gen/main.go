// Command gen regenerates the trimmed .sff test fixtures under
// testdata/files from real, full-size character files. It is a
// standalone regeneration tool, not part of the sff package's public API:
// see .vibe/backlog/done/001-extract-and-migrate-sff-code-from-character.md
// for context (migrated as-is from character/sff/testdata/gen).
//
// It is not meant to run in CI. Run it manually, pointing SRC_DIR at a
// local checkout of the source files (see testdata/README.md for where
// they come from), whenever a fixture needs to be regenerated:
//
//	SRC_DIR=/path/to/sff-extractor/tests/files go run ./testdata/gen
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	"github.com/openkakutou/sff"
)

// scenario describes one trimmed fixture to produce: which real source
// file to pull from, which sprite the test needs (selected the same way
// the reference JS project's own test suite selects it: filter the sprite
// table to one group, then take the nth match), and the output filename.
type scenario struct {
	name       string // output file, under testdata/files
	src        string // source file, under SRC_DIR
	version    int    // 1 or 2
	group      int    // group filter, matching the JS test's spriteGroups option
	nthInGroup int    // index into the group-filtered list (0-based)
	// includePrevOwnPalette also copies the nearest earlier sprite that
	// owns its palette (v1 only), needed when the target sprite shares a
	// palette rather than carrying its own.
}

func main() {
	srcDir := os.Getenv("SRC_DIR")
	if srcDir == "" {
		fmt.Fprintln(os.Stderr, "SRC_DIR environment variable must point at the real .sff files (see testdata/README.md)")
		os.Exit(1)
	}
	outDir := "testdata/files"
	if _, err := os.Stat(outDir); err != nil {
		fmt.Fprintln(os.Stderr, "run this from the repo root:", err)
		os.Exit(1)
	}

	scenarios := []scenario{
		{"v1-basic.sff", "cvsryu-v1.sff", 1, 0, 0},
		{"v1-last-sprite.sff", "arale-v1.sff", 1, 10302, 0},
		{"v1-zero-length-copy.sff", "crab-v1.sff", 1, 0, 75},
		{"v1-self-link.sff", "greenarrow-v1.sff", 1, 5040, 2},
		{"v1-forward-link.sff", "cvssakura-v1.sff", 1, 5072, 0},
		{"v1-invalid-size.sff", "vivi-v1.sff", 1, 10017, 7},
		{"v1-external-palette-source.sff", "cyclops-v1.sff", 1, 0, 0},
		{"v1-same-palette-a.sff", "gray-v1.sff", 1, 0, 0},
		{"v1-same-palette-a-group5.sff", "gray-v1.sff", 1, 5, 0},
		{"v1-same-palette-b.sff", "lucifer-v1.sff", 1, 0, 0},

		{"v2-rle8.sff", "kfm-v2.sff", 2, 9000, 1},
		{"v2-lz5.sff", "kfm-v2.sff", 2, 0, 0},
		{"v2-png8.sff", "kim-v2.sff", 2, 0, 0},
		{"v2-png24.sff", "ruby-v2.sff", 2, 6053, 0},
		{"v2-png32.sff", "batman-v2.sff", 2, 9000, 2},
		{"v2-zero-length-copy.sff", "piccolo-v2.sff", 2, 186, 0},
		{"v2-png8-forced-alpha.sff", "kfm720-v2.sff", 2, 9000, 0},
		{"v2-external-palette-source.sff", "ruby-v2.sff", 2, 0, 0},
		{"v2-empty-palette-use-first.sff", "makina-v2.sff", 2, 0, 0},
		{"v2-empty-palette-use-previous.sff", "sonic-v2.sff", 2, 0, 0},
		{"v2-empty-palette-use-previous2.sff", "piccolo-sd-v2.sff", 2, 0, 0},
		{"v2-png8-b.sff", "kumagawa-v2.sff", 2, 9000, 1},
		{"v2-loadmode1.sff", "kazuki-v2.sff", 2, 3096, 14},
	}

	for _, sc := range scenarios {
		srcPath := filepath.Join(srcDir, sc.src)
		data, err := os.ReadFile(srcPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v (skipping)\n", sc.name, err)
			continue
		}

		var out []byte
		if sc.version == 1 {
			out, err = trimV1(data, sc)
		} else {
			out, err = trimV2(data, sc)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", sc.name, err)
			continue
		}

		if err := os.WriteFile(filepath.Join(outDir, sc.name), out, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "%s: writing: %v\n", sc.name, err)
			continue
		}
		fmt.Printf("%s: %d bytes (from %d)\n", sc.name, len(out), len(data))
	}
}

// resolveGroupV1 replicates the reference JS project's own sprite
// selection: it filters ParseV1's sprite table to entries whose Group
// matches, in original table order, and returns the table index of the
// nth (0-based) match.
func resolveGroupV1(table *sff.V1SpriteTable, group, nth int) (int, error) {
	n := 0
	for i, s := range table.Sprites {
		if s.Group != group {
			continue
		}
		if n == nth {
			return i, nil
		}
		n++
	}
	return 0, fmt.Errorf("group %d: only %d matching sprites, want index %d", group, n, nth)
}

func resolveGroupV2(table *sff.V2SpriteTable, group, nth int) (int, error) {
	n := 0
	for i, s := range table.Sprites {
		if s.Group != group {
			continue
		}
		if n == nth {
			return i, nil
		}
		n++
	}
	return 0, fmt.Errorf("group %d: only %d matching sprites, want index %d", group, n, nth)
}

// resolvePaletteOwnerV1 mirrors the sff package's own (corrected)
// palette-inheritance rule (see
// .vibe/decisions/011-v1-sprite-linking-and-palette-inheritance-rules.md):
// a sprite that shares (SharedPalette == true) inherits table index 0's
// own owner when it is itself (Group 0, Image 0), or the immediately
// preceding sprite's owner otherwise. Table index 0 always owns — there is
// no earlier sprite for it to inherit from.
func resolvePaletteOwnerV1(table *sff.V1SpriteTable, i int) int {
	e := table.Sprites[i]
	if e.SharedPalette && i > 0 {
		if e.Group == 0 && e.Image == 0 {
			return resolvePaletteOwnerV1(table, 0)
		}
		return resolvePaletteOwnerV1(table, i-1)
	}
	return i
}

// findOwnPalette returns real sprite index i's true resolved 768-byte RGB
// palette block: its owner's (per resolvePaletteOwnerV1) own trailing
// block, living inside the *last* 768 bytes of the owner's own declared
// [Offset, Offset+Length) span — a v1 sprite's declared Length includes
// its own trailing palette block when it owns one, confirmed against real
// files (see
// .vibe/decisions/012-v1-palette-block-lives-inside-declared-length.md),
// not a suffix starting right after it.
func findOwnPalette(table *sff.V1SpriteTable, data []byte, i int) ([]byte, error) {
	owner := resolvePaletteOwnerV1(table, i)
	e := table.Sprites[owner]
	end := e.Offset + int64(e.Length)
	start := end - 768
	if start < 0 || end > int64(len(data)) {
		return nil, fmt.Errorf("sprite %d (owner %d): palette block out of range [%d,%d)", i, owner, start, end)
	}
	return data[start:end], nil
}

// resolvePixelOwnerV1 mirrors the sff package's own (corrected) linking
// rule (see
// .vibe/decisions/011-v1-sprite-linking-and-palette-inheritance-rules.md):
// a zero-length sprite always inherits the immediately preceding table
// entry's pixel owner — its own LinkedIndex field is not consulted for
// this case; a sprite with its own pixel data uses it unless its own
// LinkedIndex is a genuine backward reference (strictly less than i, and
// not 0 — 0 can never be a real link target through this field).
func resolvePixelOwnerV1(table *sff.V1SpriteTable, i int) (int, error) {
	e := table.Sprites[i]
	if e.Length == 0 {
		if i == 0 {
			return 0, fmt.Errorf("sprite 0 has no pixel data and no earlier sprite to inherit from")
		}
		return resolvePixelOwnerV1(table, i-1)
	}
	linked := e.LinkedIndex
	if linked >= i {
		linked = 0
	}
	if linked == 0 {
		return i, nil
	}
	if linked < 0 || linked >= len(table.Sprites) {
		return 0, fmt.Errorf("sprite %d: linked index %d out of range", i, linked)
	}
	return resolvePixelOwnerV1(table, linked)
}

// ownPixelLength returns real sprite index i's actual pixel-data-only byte
// count: its declared Length, minus its own trailing 768-byte palette block
// when it owns one (SharedPalette == false) — see findOwnPalette's doc
// comment for why Length alone over-counts in that case.
func ownPixelLength(table *sff.V1SpriteTable, i int) int {
	e := table.Sprites[i]
	if e.SharedPalette {
		return e.Length
	}
	return e.Length - 768
}

// trimV1 builds a minimal, real-bytes-only .sff v1 file exposing exactly
// the sprite scenario sc asks for. When the target needs anything from an
// earlier real sprite — its pixel data (Length == 0) and/or its palette
// (SharedPalette == true) — it keeps exactly one donor entry, placed
// immediately before the target: sff's corrected v1 resolution (see
// .vibe/decisions/011-v1-sprite-linking-and-palette-inheritance-rules.md)
// always inherits pixel data from the table position right before a
// zero-length sprite, and — for the (Group, Image) != (0, 0) case this
// package's scenarios exercise — palette from that same position for a
// sharing sprite, so a single donor there satisfies both regardless of
// whether the target's real pixel owner and real palette owner happen to
// be the same source sprite. The donor's own palette bytes are always the
// target's own true resolved palette (via findOwnPalette, which follows
// the real source file's full inheritance chain regardless of length) —
// not necessarily the pixel owner's own natural palette — precisely so
// this single-donor collapse stays correct even when the two would
// otherwise diverge.
func trimV1(data []byte, sc scenario) ([]byte, error) {
	table, err := sff.ParseV1(readerAt(data))
	if err != nil {
		return nil, err
	}

	idx, err := resolveGroupV1(table, sc.group, sc.nthInGroup)
	if err != nil {
		return nil, err
	}
	target := table.Sprites[idx]

	pixelOwner, err := resolvePixelOwnerV1(table, idx)
	if err != nil {
		return nil, err
	}
	paletteOwner := resolvePaletteOwnerV1(table, idx)

	var sprites []v1Sprite

	if pixelOwner != idx || paletteOwner != idx {
		pe := table.Sprites[pixelOwner]
		pal, err := findOwnPalette(table, data, idx)
		if err != nil {
			return nil, err
		}
		sprites = append(sprites, v1Sprite{
			group: pe.Group, image: pe.Image, axisX: pe.AxisX, axisY: pe.AxisY,
			linkedIndex: -1, shared: false,
			pixel:   data[pe.Offset : pe.Offset+int64(ownPixelLength(table, pixelOwner))],
			palette: pal,
		})
	}

	te := v1Sprite{group: target.Group, image: target.Image, axisX: target.AxisX, axisY: target.AxisY}
	if pixelOwner == idx {
		te.linkedIndex = -1
		te.pixel = data[target.Offset : target.Offset+int64(ownPixelLength(table, idx))]
	} else {
		te.linkedIndex = 0 // unused by the corrected reader (it inherits by position, not LinkedIndex), kept well-formed
	}
	if paletteOwner == idx {
		te.shared = false
		pal, err := findOwnPalette(table, data, idx)
		if err != nil {
			return nil, err
		}
		te.palette = pal
	} else {
		te.shared = true
	}
	sprites = append(sprites, te)

	return encodeV1(table.Header, sprites)
}

// v1Sprite is the trimming tool's own minimal per-sprite write shape,
// carrying real bytes copied verbatim from a source file.
type v1Sprite struct {
	group, image, axisX, axisY int
	linkedIndex                int // -1 = none, own pixel+palette
	shared                     bool
	pixel                      []byte
	palette                    []byte
}

// encodeV1 hand-writes a minimal, valid .sff v1 file: a 512-byte header
// followed by one 32-byte subheader + pixel data (+ trailing 768-byte
// palette, for sprites that carry their own) per sprite, chained via each
// subheader's next-subfile offset. This bypasses SerializeV1 because
// V1WriteSprite has no field for embedding a sprite's own real palette
// bytes (that capability doesn't exist yet — see backlog item 026); this
// tool needs the real bytes preserved verbatim, not decoded/re-encoded.
func encodeV1(h sff.V1Header, sprites []v1Sprite) ([]byte, error) {
	var buf bytes.Buffer
	header := make([]byte, 512)
	copy(header[0:12], []byte("ElecbyteSpr\x00"))
	copy(header[12:16], h.Version[:])
	binary.LittleEndian.PutUint32(header[16:20], uint32(len(sprites)))
	binary.LittleEndian.PutUint32(header[20:24], uint32(len(sprites)))
	binary.LittleEndian.PutUint32(header[24:28], 512)
	if h.SharedPalette {
		header[32] = 1
	}
	buf.Write(header)

	offsets := make([]int, len(sprites))
	pos := 512
	for i, s := range sprites {
		offsets[i] = pos
		pos += 32 + len(s.pixel)
		if !s.shared {
			pos += len(s.palette)
		}
	}

	for i, s := range sprites {
		sub := make([]byte, 32)
		if i+1 < len(sprites) {
			binary.LittleEndian.PutUint32(sub[0:4], uint32(offsets[i+1]))
		}
		// A non-shared entry's declared Length includes its own trailing
		// palette block, matching real files — see
		// .vibe/decisions/012-v1-palette-block-lives-inside-declared-length.md.
		length := len(s.pixel)
		if !s.shared {
			length += len(s.palette)
		}
		binary.LittleEndian.PutUint32(sub[4:8], uint32(length))
		binary.LittleEndian.PutUint16(sub[8:10], uint16(int16(s.axisX)))
		binary.LittleEndian.PutUint16(sub[10:12], uint16(int16(s.axisY)))
		binary.LittleEndian.PutUint16(sub[12:14], uint16(s.group))
		binary.LittleEndian.PutUint16(sub[14:16], uint16(s.image))
		li := s.linkedIndex
		if li < 0 {
			li = 0
		}
		binary.LittleEndian.PutUint16(sub[16:18], uint16(li))
		if s.shared {
			sub[18] = 1
		}
		buf.Write(sub)
		buf.Write(s.pixel)
		if !s.shared {
			buf.Write(s.palette)
		}
	}

	return buf.Bytes(), nil
}

func trimV2(data []byte, sc scenario) ([]byte, error) {
	table, err := sff.ParseV2(readerAt(data))
	if err != nil {
		return nil, err
	}

	idx, err := resolveGroupV2(table, sc.group, sc.nthInGroup)
	if err != nil {
		return nil, err
	}
	target := table.Sprites[idx]

	pixelOwner := idx
	seen := map[int]bool{}
	for table.Sprites[pixelOwner].Length == 0 {
		if seen[pixelOwner] {
			return nil, fmt.Errorf("sprite %d: linked chain cycles", idx)
		}
		seen[pixelOwner] = true
		li := table.Sprites[pixelOwner].LinkedIndex
		if li < 0 || li >= len(table.Sprites) {
			return nil, fmt.Errorf("sprite %d: linked index %d out of range", pixelOwner, li)
		}
		pixelOwner = li
	}
	pe := table.Sprites[pixelOwner]
	pixelBytes := data[pe.Offset : pe.Offset+int64(pe.Length)]

	// Resolve the palette bank's real color bytes, following its own
	// LinkedIndex chain when it stores no color data of its own.
	pi := target.PaletteIndex
	if pi < 0 || pi >= len(table.Palettes) {
		return nil, fmt.Errorf("sprite %d: palette index %d out of range", idx, pi)
	}
	palOwner := pi
	seenP := map[int]bool{}
	for table.Palettes[palOwner].Length == 0 && len(table.Palettes) > 0 {
		if seenP[palOwner] {
			return nil, fmt.Errorf("palette %d: linked chain cycles", pi)
		}
		seenP[palOwner] = true
		li := table.Palettes[palOwner].LinkedIndex
		if li == 0 && palOwner == 0 {
			break
		}
		if li < 0 || li >= len(table.Palettes) {
			li = 0
		}
		if li == palOwner {
			break
		}
		palOwner = li
	}
	pal := table.Palettes[palOwner]
	var paletteColors []byte
	if pal.Length > 0 {
		paletteColors = data[pal.Offset : pal.Offset+int64(pal.Length)]
	} else {
		// Truly empty file-wide: shouldn't happen for a real fixture.
		paletteColors = make([]byte, pal.ColorCount*4)
	}

	writeSprites := []sff.V2WriteSprite{{
		Group: pe.Group, Image: pe.Image, Width: pe.Width, Height: pe.Height,
		AxisX: pe.AxisX, AxisY: pe.AxisY, Format: pe.Format, ColorDepth: pe.ColorDepth,
		PaletteIndex: 0, PixelData: pixelBytes,
	}}
	if pixelOwner != idx {
		writeSprites = append(writeSprites, sff.V2WriteSprite{
			Group: target.Group, Image: target.Image,
			PaletteIndex: 0, LinkedIndex: 0,
		})
	}

	var buf bytes.Buffer
	err = sff.SerializeV2(&buf, table.Header.Version, writeSprites, []sff.V2WritePalette{{
		Group: pal.Group, Number: pal.Number, ColorCount: pal.ColorCount, ColorData: paletteColors,
	}})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// readerAt adapts a byte slice to io.ReaderAt.
type readerAtBytes []byte

func (r readerAtBytes) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(r)) {
		return 0, fmt.Errorf("offset %d past end of data (len %d)", off, len(r))
	}
	n := copy(p, r[off:])
	if n < len(p) {
		return n, fmt.Errorf("short read at offset %d: got %d, want %d", off, n, len(p))
	}
	return n, nil
}

func readerAt(data []byte) readerAtBytes { return readerAtBytes(data) }
