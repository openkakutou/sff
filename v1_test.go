package sff

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

// v1SubfileSpec describes one synthetic .sff v1 sprite subfile used to
// build test fixtures: the 32-byte subheader fields plus how many bytes of
// (arbitrary, undecoded) pixel data follow it.
type v1SubfileSpec struct {
	length        uint32
	axisX         int16
	axisY         int16
	group         uint16
	image         uint16
	linkedIndex   uint16
	sharedPalette bool
	pixelDataLen  int
}

// buildV1File assembles a byte-exact synthetic .sff v1 file: a 512-byte
// header followed by one 32-byte subheader + pixel data blob per spec,
// linked in file order via each subheader's "next subfile" offset.
func buildV1File(t *testing.T, groupCount, imageCount uint32, sharedPalette bool, specs []v1SubfileSpec) []byte {
	t.Helper()

	var buf bytes.Buffer

	buf.WriteString("ElecbyteSpr\x00") // signature, 12 bytes
	buf.Write([]byte{1, 0, 0, 1})      // version bytes
	writeU32(&buf, groupCount)         // group count
	writeU32(&buf, imageCount)         // image count
	firstOffset := uint32(headerSize)
	if len(specs) == 0 {
		firstOffset = 0
	}
	writeU32(&buf, firstOffset) // offset to first subfile
	writeU32(&buf, 0)           // subfile header length (unused by v1 readers)
	if sharedPalette {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}
	buf.Write(make([]byte, 3))   // reserved
	buf.Write(make([]byte, 476)) // comment padding, up to 512 total

	if buf.Len() != headerSize {
		t.Fatalf("test setup: header is %d bytes, want %d", buf.Len(), headerSize)
	}

	// Compute each subfile's absolute offset up front so "next subfile"
	// fields can be filled in.
	offsets := make([]uint32, len(specs))
	cur := uint32(headerSize)
	for i, s := range specs {
		offsets[i] = cur
		cur += subheaderSize + uint32(s.pixelDataLen)
	}

	for i, s := range specs {
		next := uint32(0)
		if i+1 < len(specs) {
			next = offsets[i+1]
		}
		writeU32(&buf, next)
		writeU32(&buf, s.length)
		writeU16(&buf, uint16(s.axisX))
		writeU16(&buf, uint16(s.axisY))
		writeU16(&buf, s.group)
		writeU16(&buf, s.image)
		writeU16(&buf, s.linkedIndex)
		if s.sharedPalette {
			buf.WriteByte(1)
		} else {
			buf.WriteByte(0)
		}
		buf.Write(make([]byte, 13)) // subheader padding

		buf.Write(bytes.Repeat([]byte{0xAB}, s.pixelDataLen))
	}

	return buf.Bytes()
}

func writeU32(buf *bytes.Buffer, v uint32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	buf.Write(b[:])
}

func writeU16(buf *bytes.Buffer, v uint16) {
	var b [2]byte
	binary.LittleEndian.PutUint16(b[:], v)
	buf.Write(b[:])
}

func TestParseV1_ValidHeader_ReturnsGroupImageCountsVersionAndPaletteType(t *testing.T) {
	data := buildV1File(t, 3, 2, true, []v1SubfileSpec{
		{length: 10, axisX: 5, axisY: -3, group: 0, image: 0, pixelDataLen: 10},
		{length: 20, axisX: 0, axisY: 0, group: 0, image: 1, pixelDataLen: 20},
	})

	table, err := ParseV1(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ParseV1: unexpected error: %v", err)
	}

	if table.Header.Version != [4]byte{1, 0, 0, 1} {
		t.Errorf("expected version {1,0,0,1}, got %v", table.Header.Version)
	}
	if table.Header.GroupCount != 3 {
		t.Errorf("expected GroupCount 3, got %d", table.Header.GroupCount)
	}
	if table.Header.ImageCount != 2 {
		t.Errorf("expected ImageCount 2, got %d", table.Header.ImageCount)
	}
	if !table.Header.SharedPalette {
		t.Errorf("expected SharedPalette true")
	}
}

func TestParseV1_SpriteIndexTable_ResolvesGroupImagePairsToOffsets(t *testing.T) {
	data := buildV1File(t, 1, 2, false, []v1SubfileSpec{
		{length: 10, axisX: 5, axisY: -3, group: 0, image: 0, pixelDataLen: 10},
		{length: 20, axisX: 1, axisY: 2, group: 0, image: 1, pixelDataLen: 20},
	})

	table, err := ParseV1(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ParseV1: unexpected error: %v", err)
	}

	if len(table.Sprites) != 2 {
		t.Fatalf("expected 2 sprites, got %d", len(table.Sprites))
	}

	first := table.Sprites[0]
	if first.Group != 0 || first.Image != 0 {
		t.Errorf("expected first sprite (0,0), got (%d,%d)", first.Group, first.Image)
	}
	if first.AxisX != 5 || first.AxisY != -3 {
		t.Errorf("expected first sprite axis (5,-3), got (%d,%d)", first.AxisX, first.AxisY)
	}
	if first.Length != 10 {
		t.Errorf("expected first sprite Length 10, got %d", first.Length)
	}
	wantFirstOffset := int64(headerSize + subheaderSize)
	if first.Offset != wantFirstOffset {
		t.Errorf("expected first sprite Offset %d, got %d", wantFirstOffset, first.Offset)
	}

	second := table.Sprites[1]
	if second.Group != 0 || second.Image != 1 {
		t.Errorf("expected second sprite (0,1), got (%d,%d)", second.Group, second.Image)
	}
	wantSecondOffset := wantFirstOffset + 10 + subheaderSize
	if second.Offset != wantSecondOffset {
		t.Errorf("expected second sprite Offset %d, got %d", wantSecondOffset, second.Offset)
	}

	offset, ok := table.Offset(0, 1)
	if !ok {
		t.Fatalf("expected Offset(0, 1) to resolve")
	}
	if offset != wantSecondOffset {
		t.Errorf("expected Offset(0, 1) = %d, got %d", wantSecondOffset, offset)
	}

	if _, ok := table.Offset(9, 9); ok {
		t.Errorf("expected Offset(9, 9) to not resolve for a nonexistent sprite")
	}
}

func TestParseV1_ZeroSprites_ReturnsEmptyTableWithoutError(t *testing.T) {
	data := buildV1File(t, 0, 0, false, nil)

	table, err := ParseV1(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ParseV1: unexpected error: %v", err)
	}

	if len(table.Sprites) != 0 {
		t.Errorf("expected 0 sprites, got %d", len(table.Sprites))
	}

	if _, ok := table.Offset(0, 0); ok {
		t.Errorf("expected Offset lookup on an empty table to never resolve")
	}
}

func TestParseV1_LinkedSprite_PreservesLinkedIndexAndZeroLength(t *testing.T) {
	data := buildV1File(t, 1, 2, true, []v1SubfileSpec{
		{length: 10, group: 0, image: 0, pixelDataLen: 10},
		{length: 0, group: 0, image: 1, linkedIndex: 0, sharedPalette: true, pixelDataLen: 0},
	})

	table, err := ParseV1(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ParseV1: unexpected error: %v", err)
	}

	linked := table.Sprites[1]
	if linked.Length != 0 {
		t.Errorf("expected linked sprite Length 0, got %d", linked.Length)
	}
	if linked.LinkedIndex != 0 {
		t.Errorf("expected linked sprite LinkedIndex 0, got %d", linked.LinkedIndex)
	}
	if !linked.SharedPalette {
		t.Errorf("expected linked sprite SharedPalette true")
	}
}

func TestParseV1_TruncatedHeader_ReturnsErrorNotPanic(t *testing.T) {
	data := buildV1File(t, 1, 1, false, []v1SubfileSpec{
		{length: 10, group: 0, image: 0, pixelDataLen: 10},
	})

	_, err := ParseV1(bytes.NewReader(data[:100])) // header alone is 512 bytes

	if err == nil {
		t.Fatalf("expected an error for a truncated header, got nil")
	}
}

func TestParseV1_WrongSignature_ReturnsDescriptiveError(t *testing.T) {
	data := buildV1File(t, 1, 1, false, []v1SubfileSpec{
		{length: 10, group: 0, image: 0, pixelDataLen: 10},
	})
	copy(data[0:12], "NotASffFile\x00")

	_, err := ParseV1(bytes.NewReader(data))

	if err == nil {
		t.Fatalf("expected an error for a wrong signature, got nil")
	}
	if !strings.Contains(err.Error(), "signature") {
		t.Errorf("expected error to mention the signature, got: %v", err)
	}
}

func TestParseV1_ChainEndsBeforeDeclaredImageCount_ReturnsError(t *testing.T) {
	// Header declares 3 images but the subfile chain only has 1 entry
	// (its "next subfile" offset is 0, ending the chain early).
	data := buildV1File(t, 1, 3, false, []v1SubfileSpec{
		{length: 10, group: 0, image: 0, pixelDataLen: 10},
	})

	_, err := ParseV1(bytes.NewReader(data))

	if err == nil {
		t.Fatalf("expected an error when the subfile chain ends before the declared image count, got nil")
	}
}

func TestParseV1_SubheaderOffsetPastEndOfFile_ReturnsErrorNotPanic(t *testing.T) {
	data := buildV1File(t, 1, 2, false, []v1SubfileSpec{
		{length: 10, group: 0, image: 0, pixelDataLen: 10},
	})
	// Drop the trailing bytes of the file so the second subfile's offset
	// (referenced by the first subheader's "next subfile" field) points
	// past the end of the available data.
	truncated := data[:len(data)-5]

	_, err := ParseV1(bytes.NewReader(truncated))

	if err == nil {
		t.Fatalf("expected an error when a subheader offset is past the end of file, got nil")
	}
}
