---
status: todo
---
# Decode 3-Plane PCX Sprites

## Description
Six real community SFF files contain 3-plane (24-bit RGB) PCX sprites that sff rejects with "unsupported color plane count 3"; they are currently registered as a known gap (pcx3PlaneGapName). Decode them so those files load fully.

## Acceptance Criteria
- [ ] A 3-plane PCX sprite decodes to the correct RGB pixels, covered by a fixture
- [ ] The six affected real files load with no remaining gap, and the gap registration is removed
- [ ] RGB sprites behave in palette-related APIs without panicking (document how they interact with palettes)

## Notes
See .vibe/fixture-sources.md row for pcx3PlaneGapName. Decide whether the output stays indexed (quantize) or becomes RGBA, and document the choice. Compat is a MUST: MUGEN 1.0/1.1 and Ikemen GO, validated against real community files.
