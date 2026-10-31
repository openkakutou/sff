---
status: todo
---
# Fuzz the sff readers

## Description
Add native Go fuzz targets for v1 and v2 parsing and every pixel decoder (PCX, RLE8, RLE5, LZ5, PNG), with size caps against decompression bombs.

## Acceptance Criteria
- [ ] Fuzz targets cover both containers and all decoders
- [ ] Declared dimensions/lengths are bounded; oversize returns an error
- [ ] Any finding fixed with a regression seed
