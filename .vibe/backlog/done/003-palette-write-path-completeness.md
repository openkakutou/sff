---
status: done
depends_on: [001]
---
# Palette Write-Path Completeness (Edit + Re-Encode v1/v2, Export `.act`)

## Description
Support editing palette colors and re-encoding both v1 (embedded PCX palette) and v2 palette formats, plus exporting a palette as a standalone `.act` file.

## Acceptance Criteria
- [x] An edited palette re-encodes correctly for both v1 and v2 sprites
- [x] `.act` export round-trips with `.act` import
- [x] An out-of-range color value is rejected with a descriptive error, not silently clamped or corrupted

## Notes
Cross-repo dependency for `character-editor`'s Palette Editor item.
