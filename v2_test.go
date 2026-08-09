package sff

import (
	"bytes"
	"strings"
	"testing"
)

// v2PaletteSpec describes one synthetic .sff v2 palette table entry used to
// build test fixtures.
type v2PaletteSpec struct {
	group, number, colorCount, linkedIndex uint16
	ofs, length                            uint32
}

// v2SpriteSpec describes one synthetic .sff v2 sprite table entry used to
// build test fixtures.
type v2SpriteSpec struct {
	group, image, width, height uint16
	axisX, axisY                int16
	linkedIndex                 uint16
	format, colorDepth          byte
	ofs, length                 uint32
	paletteIndex                uint16
	flag                        uint16
}

// buildV2File assembles a byte-exact synthetic .sff v2 file: the 64-byte
// header portion ParseV2 reads, immediately followed by the palette table
// then the sprite table. Pixel/palette data itself is never written: ParseV2
// only locates it (offset + length), it does not read it.
func buildV2File(t *testing.T, verhi byte, spriteCount, paletteCount uint32, lofs, tofs uint32, palettes []v2PaletteSpec, sprites []v2SpriteSpec) []byte {
	t.Helper()

	var buf bytes.Buffer

	buf.WriteString("ElecbyteSpr\x00") // signature, 12 bytes
	buf.Write([]byte{0, 1, 0, verhi})  // version: verlo3, verlo2, verlo1, verhi
	writeU32(&buf, 0)                  // compatibility version, unused
	for i := 0; i < 4; i++ {
		writeU32(&buf, 0) // legacy v1-only fields, unused in v2
	}

	paletteTableOffset := uint32(v2HeaderSize)
	spriteTableOffset := paletteTableOffset + uint32(len(palettes))*v2PaletteEntrySize

	writeU32(&buf, spriteTableOffset)  // first sprite header offset
	writeU32(&buf, spriteCount)        // declared sprite count
	writeU32(&buf, paletteTableOffset) // first palette header offset
	writeU32(&buf, paletteCount)       // declared palette count
	writeU32(&buf, lofs)               // literal data section base offset
	writeU32(&buf, 0)                  // unused
	writeU32(&buf, tofs)               // translated data section base offset

	if buf.Len() != v2HeaderSize {
		t.Fatalf("test setup: header is %d bytes, want %d", buf.Len(), v2HeaderSize)
	}

	for _, p := range palettes {
		writeU16(&buf, p.group)
		writeU16(&buf, p.number)
		writeU16(&buf, p.colorCount)
		writeU16(&buf, p.linkedIndex)
		writeU32(&buf, p.ofs)
		writeU32(&buf, p.length)
	}

	for _, s := range sprites {
		writeU16(&buf, s.group)
		writeU16(&buf, s.image)
		writeU16(&buf, s.width)
		writeU16(&buf, s.height)
		writeU16(&buf, uint16(s.axisX))
		writeU16(&buf, uint16(s.axisY))
		writeU16(&buf, s.linkedIndex)
		buf.WriteByte(s.format)
		buf.WriteByte(s.colorDepth)
		writeU32(&buf, s.ofs)
		writeU32(&buf, s.length)
		writeU16(&buf, s.paletteIndex)
		writeU16(&buf, s.flag)
	}

	return buf.Bytes()
}

func TestParseV2_ValidHeader_ReturnsVersionSpriteAndPaletteCounts(t *testing.T) {
	data := buildV2File(t, 2, 2, 1, 100, 200,
		[]v2PaletteSpec{{group: 0, number: 0, colorCount: 16, ofs: 0, length: 64}},
		[]v2SpriteSpec{
			{group: 0, image: 0, width: 10, height: 20, ofs: 0, length: 5},
			{group: 0, image: 1, width: 10, height: 20, ofs: 5, length: 5},
		},
	)

	table, err := ParseV2(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ParseV2: unexpected error: %v", err)
	}

	if table.Header.Version != [4]byte{0, 1, 0, 2} {
		t.Errorf("expected version {0,1,0,2}, got %v", table.Header.Version)
	}
	if table.Header.SpriteCount != 2 {
		t.Errorf("expected SpriteCount 2, got %d", table.Header.SpriteCount)
	}
	if table.Header.PaletteCount != 1 {
		t.Errorf("expected PaletteCount 1, got %d", table.Header.PaletteCount)
	}
}

func TestParseV2_SpriteEntries_ResolveOffsetAgainstLiteralOrTranslatedSection(t *testing.T) {
	data := buildV2File(t, 2, 2, 0, 1000, 5000,
		nil,
		[]v2SpriteSpec{
			// flag bit0 == 0: pixel data lives in the literal section (lofs-relative).
			{group: 0, image: 0, width: 32, height: 48, axisX: 5, axisY: -3, format: 2, colorDepth: 8, ofs: 10, length: 40, paletteIndex: 1, flag: 0},
			// flag bit0 == 1: pixel data lives in the translated section (tofs-relative).
			{group: 1, image: 2, width: 16, height: 16, ofs: 20, length: 60, paletteIndex: 0, flag: 1},
		},
	)

	table, err := ParseV2(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ParseV2: unexpected error: %v", err)
	}

	if len(table.Sprites) != 2 {
		t.Fatalf("expected 2 sprites, got %d", len(table.Sprites))
	}

	first := table.Sprites[0]
	if first.Group != 0 || first.Image != 0 {
		t.Errorf("expected first sprite (0,0), got (%d,%d)", first.Group, first.Image)
	}
	if first.Width != 32 || first.Height != 48 {
		t.Errorf("expected first sprite size (32,48), got (%d,%d)", first.Width, first.Height)
	}
	if first.AxisX != 5 || first.AxisY != -3 {
		t.Errorf("expected first sprite axis (5,-3), got (%d,%d)", first.AxisX, first.AxisY)
	}
	if first.Format != 2 {
		t.Errorf("expected first sprite Format 2, got %d", first.Format)
	}
	if first.ColorDepth != 8 {
		t.Errorf("expected first sprite ColorDepth 8, got %d", first.ColorDepth)
	}
	if first.PaletteIndex != 1 {
		t.Errorf("expected first sprite PaletteIndex 1, got %d", first.PaletteIndex)
	}
	wantFirstOffset := int64(1000 + 10) // literal section base + ofs
	if first.Offset != wantFirstOffset {
		t.Errorf("expected first sprite Offset %d, got %d", wantFirstOffset, first.Offset)
	}

	second := table.Sprites[1]
	wantSecondOffset := int64(5000 + 20) // translated section base + ofs
	if second.Offset != wantSecondOffset {
		t.Errorf("expected second sprite Offset %d, got %d", wantSecondOffset, second.Offset)
	}

	offset, ok := table.Offset(1, 2)
	if !ok {
		t.Fatalf("expected Offset(1, 2) to resolve")
	}
	if offset != wantSecondOffset {
		t.Errorf("expected Offset(1, 2) = %d, got %d", wantSecondOffset, offset)
	}

	if _, ok := table.Offset(9, 9); ok {
		t.Errorf("expected Offset(9, 9) to not resolve for a nonexistent sprite")
	}
}

func TestParseV2_LinkedSprite_PreservesLinkedIndexAndZeroLength(t *testing.T) {
	data := buildV2File(t, 2, 2, 0, 100, 200,
		nil,
		[]v2SpriteSpec{
			{group: 0, image: 0, width: 8, height: 8, ofs: 0, length: 10},
			{group: 0, image: 1, width: 8, height: 8, linkedIndex: 0, length: 0},
		},
	)

	table, err := ParseV2(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ParseV2: unexpected error: %v", err)
	}

	linked := table.Sprites[1]
	if linked.Length != 0 {
		t.Errorf("expected linked sprite Length 0, got %d", linked.Length)
	}
	if linked.LinkedIndex != 0 {
		t.Errorf("expected linked sprite LinkedIndex 0, got %d", linked.LinkedIndex)
	}
}

func TestParseV2_PaletteEntries_ResolveOffsetAndPreserveLinkedBank(t *testing.T) {
	data := buildV2File(t, 2, 0, 2, 500, 900,
		[]v2PaletteSpec{
			{group: 0, number: 0, colorCount: 16, ofs: 0, length: 64},
			{group: 0, number: 1, colorCount: 16, linkedIndex: 0, ofs: 0, length: 0},
		},
		nil,
	)

	table, err := ParseV2(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ParseV2: unexpected error: %v", err)
	}

	if len(table.Palettes) != 2 {
		t.Fatalf("expected 2 palettes, got %d", len(table.Palettes))
	}

	first := table.Palettes[0]
	if first.ColorCount != 16 {
		t.Errorf("expected first palette ColorCount 16, got %d", first.ColorCount)
	}
	wantOffset := int64(500) // literal section base + ofs(0)
	if first.Offset != wantOffset {
		t.Errorf("expected first palette Offset %d, got %d", wantOffset, first.Offset)
	}

	linked := table.Palettes[1]
	if linked.Length != 0 {
		t.Errorf("expected linked palette Length 0, got %d", linked.Length)
	}
	if linked.LinkedIndex != 0 {
		t.Errorf("expected linked palette LinkedIndex 0, got %d", linked.LinkedIndex)
	}

	offset, ok := table.PaletteOffset(0, 0)
	if !ok {
		t.Fatalf("expected PaletteOffset(0, 0) to resolve")
	}
	if offset != wantOffset {
		t.Errorf("expected PaletteOffset(0, 0) = %d, got %d", wantOffset, offset)
	}

	if _, ok := table.PaletteOffset(9, 9); ok {
		t.Errorf("expected PaletteOffset(9, 9) to not resolve for a nonexistent palette bank")
	}
}

func TestParseV2_ZeroSpritesAndPalettes_ReturnsEmptyTableWithoutError(t *testing.T) {
	data := buildV2File(t, 2, 0, 0, 0, 0, nil, nil)

	table, err := ParseV2(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ParseV2: unexpected error: %v", err)
	}

	if len(table.Sprites) != 0 {
		t.Errorf("expected 0 sprites, got %d", len(table.Sprites))
	}
	if len(table.Palettes) != 0 {
		t.Errorf("expected 0 palettes, got %d", len(table.Palettes))
	}
	if _, ok := table.Offset(0, 0); ok {
		t.Errorf("expected Offset lookup on an empty table to never resolve")
	}
}

func TestParseV2_TruncatedHeader_ReturnsErrorNotPanic(t *testing.T) {
	data := buildV2File(t, 2, 0, 0, 0, 0, nil, nil)

	_, err := ParseV2(bytes.NewReader(data[:40])) // header alone is 64 bytes

	if err == nil {
		t.Fatalf("expected an error for a truncated header, got nil")
	}
}

func TestParseV2_WrongSignature_ReturnsDescriptiveError(t *testing.T) {
	data := buildV2File(t, 2, 0, 0, 0, 0, nil, nil)
	copy(data[0:12], "NotASffFile\x00")

	_, err := ParseV2(bytes.NewReader(data))

	if err == nil {
		t.Fatalf("expected an error for a wrong signature, got nil")
	}
	if !strings.Contains(err.Error(), "signature") {
		t.Errorf("expected error to mention the signature, got: %v", err)
	}
}

func TestParseV2_V1VersionByte_ReturnsDescriptiveError(t *testing.T) {
	// A well-formed v1 file (verhi == 1) handed to ParseV2 should be
	// rejected with a message pointing at the version, not silently
	// misparsed as v2.
	data := buildV2File(t, 1, 0, 0, 0, 0, nil, nil)

	_, err := ParseV2(bytes.NewReader(data))

	if err == nil {
		t.Fatalf("expected an error for a non-v2 version byte, got nil")
	}
	if !strings.Contains(err.Error(), "version") {
		t.Errorf("expected error to mention the version, got: %v", err)
	}
}

func TestParseV2_SpriteTableTruncatedBeforeDeclaredCount_ReturnsErrorNotPanic(t *testing.T) {
	// Header declares 2 sprites but the file is cut off partway through the
	// second sprite table entry.
	data := buildV2File(t, 2, 2, 0, 0, 0, nil, []v2SpriteSpec{
		{group: 0, image: 0, width: 8, height: 8, length: 10},
		{group: 0, image: 1, width: 8, height: 8, length: 10},
	})
	truncated := data[:len(data)-5]

	_, err := ParseV2(bytes.NewReader(truncated))

	if err == nil {
		t.Fatalf("expected an error when the sprite table is truncated before the declared count, got nil")
	}
}

func TestParseV2_PaletteTableTruncatedBeforeDeclaredCount_ReturnsErrorNotPanic(t *testing.T) {
	// Header declares 2 palettes but only 1 entry's worth of data follows.
	data := buildV2File(t, 2, 0, 2, 0, 0, []v2PaletteSpec{
		{group: 0, number: 0, colorCount: 16, ofs: 0, length: 64},
	}, nil)

	_, err := ParseV2(bytes.NewReader(data))

	if err == nil {
		t.Fatalf("expected an error when the palette table is truncated before the declared count, got nil")
	}
}
