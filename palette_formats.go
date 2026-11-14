package sff

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strconv"
	"strings"
)

// PaletteOrder selects how the entries of a JASC .pal file map to palette
// indices. MUGEN and Ikemen GO load neither .pal nor PNG palettes; these are
// editing formats, so the caller chooses the order.
type PaletteOrder int

const (
	// PaletteOrderNatural (the zero value): the file's first entry is palette
	// index 0, entry j is index j.
	PaletteOrderNatural PaletteOrder = iota
	// PaletteOrderReversed: the file lists entries in raw .act storage order,
	// so the first entry of a 256-entry file is index 255. For a file of
	// N < 256 entries, entry j maps to index N-1-j.
	PaletteOrderReversed
)

func jascError(format string, args ...any) error {
	return fmt.Errorf("sff: jasc palette: "+format, args...)
}

// DecodeJASCPalette decodes a JASC-PAL ("JASC-PAL", "0100", count, then count
// "R G B" lines) file into a Palette. Lines end in LF with an optional CR.
// A count in [1,256] is accepted; missing indices are opaque black. As for
// .act files, every entry is opaque except index 0, which is transparent.
func DecodeJASCPalette(data []byte, order PaletteOrder) (Palette, error) {
	lines := strings.Split(string(data), "\n")
	for i := range lines {
		lines[i] = strings.TrimSuffix(lines[i], "\r")
	}
	// Trailing blank lines are end-of-file, not entries.
	for len(lines) > 1 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if lines[0] != "JASC-PAL" {
		return Palette{}, jascError("line 1: expected header %q, got %q", "JASC-PAL", lines[0])
	}
	if len(lines) < 2 || lines[1] != "0100" {
		return Palette{}, jascError("line 2: expected version %q", "0100")
	}
	if len(lines) < 3 {
		return Palette{}, jascError("line 3: missing color count")
	}
	count, err := strconv.Atoi(lines[2])
	if err != nil || count < 1 || count > 256 {
		return Palette{}, jascError("line 3: color count %q is not an integer in [1, 256]", lines[2])
	}

	var p Palette
	for i := range p {
		p[i] = color.RGBA{A: 255}
	}
	for j := 0; j < count; j++ {
		lineNo := 4 + j
		if lineNo > len(lines) {
			return Palette{}, jascError("expected %d color entries, found %d", count, j)
		}
		fields := strings.Fields(lines[lineNo-1])
		if len(fields) != 3 {
			return Palette{}, jascError("line %d: expected 3 components, got %d", lineNo, len(fields))
		}
		var rgb [3]byte
		for k, f := range fields {
			v, err := strconv.Atoi(f)
			if err != nil || v < 0 || v > 255 {
				return Palette{}, jascError("line %d: component %q is not an integer in [0, 255]", lineNo, f)
			}
			rgb[k] = byte(v)
		}
		idx := j
		if order == PaletteOrderReversed {
			idx = count - 1 - j
		}
		p[idx] = color.RGBA{R: rgb[0], G: rgb[1], B: rgb[2], A: 255}
	}
	for i := 4 + count - 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "" {
			return Palette{}, jascError("line %d: unexpected content after %d color entries", i+1, count)
		}
	}
	p[0].A = 0
	return p, nil
}

// EncodeJASCPalette encodes p as a 256-entry JASC-PAL file with CRLF line
// endings. In PaletteOrderReversed the first entry is index 255 (.act order).
// Alpha is ignored.
func EncodeJASCPalette(p Palette, order PaletteOrder) []byte {
	var b bytes.Buffer
	b.WriteString("JASC-PAL\r\n0100\r\n256\r\n")
	for j := 0; j < 256; j++ {
		idx := j
		if order == PaletteOrderReversed {
			idx = 255 - j
		}
		c := p[idx]
		fmt.Fprintf(&b, "%d %d %d\r\n", c.R, c.G, c.B)
	}
	return b.Bytes()
}

// DecodePNGPalette extracts the PLTE palette of an indexed PNG into a
// Palette (natural order). Missing entries are opaque black; tRNS values are
// ignored: every entry is opaque except index 0, which is transparent.
func DecodePNGPalette(data []byte) (Palette, error) {
	// Bound the declared size before png.Decode allocates the pixel buffer from IHDR.
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return Palette{}, fmt.Errorf("sff: png palette: %w", err)
	}
	if _, ok := cfg.ColorModel.(color.Palette); !ok {
		return Palette{}, fmt.Errorf("sff: png palette: not an indexed (palette) PNG")
	}
	if err := checkSpriteDimensions(cfg.Width, cfg.Height); err != nil {
		return Palette{}, fmt.Errorf("sff: png palette: %w", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return Palette{}, fmt.Errorf("sff: png palette: %w", err)
	}
	pi, ok := img.(*image.Paletted)
	if !ok {
		return Palette{}, fmt.Errorf("sff: png palette: not an indexed (palette) PNG")
	}
	var p Palette
	for i := range p {
		p[i] = color.RGBA{A: 255}
		if i < len(pi.Palette) {
			// NRGBA keeps the RGB of tRNS-transparent entries (RGBA() would premultiply it away).
			c := color.NRGBAModel.Convert(pi.Palette[i]).(color.NRGBA)
			p[i] = color.RGBA{R: c.R, G: c.G, B: c.B, A: 255}
		}
	}
	p[0].A = 0
	return p, nil
}

// EncodePNGPalette encodes p as a 16x16 indexed PNG swatch image where pixel
// i (row-major) uses palette index i. Index 0 is marked transparent (tRNS).
func EncodePNGPalette(p Palette) ([]byte, error) {
	pal := make(color.Palette, 256)
	img := image.NewPaletted(image.Rect(0, 0, 16, 16), pal)
	for i := range p {
		a := uint8(255)
		if i == 0 {
			a = 0
		}
		pal[i] = color.NRGBA{R: p[i].R, G: p[i].G, B: p[i].B, A: a}
		img.Pix[i] = uint8(i)
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		return nil, fmt.Errorf("sff: png palette: %w", err)
	}
	return b.Bytes(), nil
}
