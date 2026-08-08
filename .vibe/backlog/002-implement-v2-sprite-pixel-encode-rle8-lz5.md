---
status: todo
depends_on: [001]
---
# Implement V2 Sprite Pixel Encode (RLE8, LZ5)

## Description
Only decode exists for `.sff` v2 pixel formats today (inherited from `character`). Sprite-saving in `character-editor` needs the encode side for RLE8 and LZ5. RLE5 encode stays deliberately deferred (no real-world fixture found across a 562-file corpus scanned in `character`'s history) — migrate that existing decision/note along with the code.

## Acceptance Criteria
- [ ] Round-trip (decode → encode → decode) is pixel-identical for RLE8 and LZ5
- [ ] RLE5 encode remains explicitly unimplemented with the same documented rationale
- [ ] A real-file corpus fixture validates the encoder, not just synthetic data

## Notes
Critical dependency for `character-editor`'s sprite-saving feature.
