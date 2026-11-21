---
status: todo
---
# PCX decoding never hangs on truncated data

## Description
Found by the audit of 2026-11-21 (high, checked in the code). The lenient decoding of v0.10.0 reads the last RLE byte again past the end of the data, as Ikemen GO's `RlePcxDecode` does (image.go:1067-1096). When that last byte is a run marker with a count of 0 (`0xC0`), every later read is the same empty run and `for len(line) < lineLen` (`pcx.go:121`) never ends: `Load` on such a v1 `.sff` hangs, and freezes the WASM instance. Ikemen GO has the same flaw, so following it is not enough here.

## Acceptance Criteria
- [ ] A PCX sprite whose RLE data ends on `0xC0` before the image is full returns an error (or stops decoding) instead of hanging
- [ ] A truncated sprite that ends on another byte still decodes as Ikemen GO does (Yamcha's 4000,0)
- [ ] The fuzz target finds no hang on this shape; the case is a committed regression seed
