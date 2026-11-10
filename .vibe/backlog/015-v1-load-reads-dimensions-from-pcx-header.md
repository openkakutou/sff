---
status: todo
---
# Read v1 sprite dimensions from the PCX header in Load

## Description
A v1 sprite table carries no width/height, so `Load` fully decodes every sprite's PCX image only to read its size. Measured in item 009 (`docs/performance.md`), this dominates v1 load time: about 4–5 s in the WASM build on real 100–150 MB sheets, against about 0.2 s for a v2 sheet of similar size. Reading the size from the PCX header alone would remove most of it. The trade-off: `Load` would no longer report a sprite whose pixel data is corrupt. That error would only surface when the sprite is resolved. Decide that trade-off in an ADR first.

## Acceptance Criteria
- [ ] ADR recording whether `Load` may skip pixel decoding and how corrupt pixel data is then reported
- [ ] v1 `Load` time measured before and after (Go benchmarks and `cmd/wasm/bench.mjs` on a real large sheet), numbers in `docs/performance.md`
- [ ] Dimensions reported by `Load` unchanged on the real fixtures and the local corpus scan

## Notes
Found while doing item 009, out of its scope. Related to item 014 (linked-sprite re-decode). Must stay compatible with both MUGEN and Ikemen GO.
