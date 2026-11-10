---
status: todo
---
# v1 Load reports a palette index that disagrees with the resolved palette

## Description
`loadV1` numbers `Sprite.Palette` with its own counter that only looks at `SharedPalette`. `resolveV1Palette` uses a different rule (decisions 011, 015): a zero-length entry inherits the previous sprite's palette whatever its `SharedPalette` bit, and a shared (Group 0, Image 0) entry inherits table index 0's palette. On those real-file shapes, `Load` (and the WASM `load`) gives a sprite a new palette number while it is drawn with the previous sprite's palette. A consumer grouping sprites by `Palette` gets wrong groups.

## Acceptance Criteria
- [ ] `Sprite.Palette` from `Load` follows the same inheritance rule as `ResolveV1Palette`, written once
- [ ] Test with a zero-length sprite not marked `SharedPalette`, and with a shared (0,0) sprite at an index above 0
- [ ] Behaviour checked against the reference decoders (MUGEN and Ikemen GO) and the local corpus scan

## Notes
Found by the review of item 009 (pre-existing, not introduced by it). Must stay compatible with both MUGEN and Ikemen GO.
