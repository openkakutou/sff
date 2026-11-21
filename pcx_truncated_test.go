package sff

import (
	"encoding/binary"
	"testing"
	"time"
)

// truncatedPCX is a 2x1, 8 bpp, 1-plane PCX whose RLE data is the single
// byte rle.
func truncatedPCX(rle ...byte) []byte {
	data := make([]byte, pcxHeaderSize)
	data[2] = 1
	data[3] = 8
	binary.LittleEndian.PutUint16(data[8:10], 1)
	data[pcxPlanesOffset] = 1
	binary.LittleEndian.PutUint16(data[66:68], 2)
	return append(data, rle...)
}

func TestDecodePCX_DataEndingOnAnEmptyRun_ReturnsAnErrorInsteadOfHanging(t *testing.T) {
	done := make(chan error, 1)
	go func() {
		_, err := DecodePCX(truncatedPCX(0xC0))
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("DecodePCX of data ending on an empty run returned no error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("DecodePCX hangs on data ending on an empty run")
	}
}

func TestDecodePCX_DataEndingOnAPixel_RepeatsItAsIkemenGO(t *testing.T) {
	img, err := DecodePCX(truncatedPCX(7))
	if err != nil {
		t.Fatal(err)
	}
	if img.Pixels[0] != 7 || img.Pixels[1] != 7 {
		t.Fatalf("pixels = %v, want the last byte repeated (Ikemen GO image.go:1067-1096)", img.Pixels[:2])
	}
}

func TestDecodePCX_EmptyRunBeforeTheLastByte_StillDecodes(t *testing.T) {
	img, err := DecodePCX(truncatedPCX(0xC0, 0, 9))
	if err != nil {
		t.Fatal(err)
	}
	if img.Pixels[0] != 9 || img.Pixels[1] != 9 {
		t.Fatalf("pixels = %v, want 9 9", img.Pixels[:2])
	}
}
