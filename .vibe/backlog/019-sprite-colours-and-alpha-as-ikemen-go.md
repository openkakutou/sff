---
status: todo
---
# Sprite colours and alpha as Ikemen GO resolves them

## Description
Found by the audit of 2026-11-21 (medium, not yet re-checked against Ikemen GO line by line):
- PNG24/PNG32 pixels go through `color.RGBAModel` (`v2_decoder.go:498`), which premultiplies, while `V2Image`, `docs/wasm.md:81-83` and the WASM output promise straight alpha. Ikemen GO decodes to NRGBA (image.go:1451-1460): semi-transparent edges come out darker here.
- `DecodeV2Palette` keeps the on-disk alpha byte (`palette.go:277`). For an SFF v2.0.0.0 sheet (verlo2 = 0) Ikemen GO forces index 0 transparent and every other index opaque (image.go:2345-2351).
- RLE5 falls to `AlphaForceTransparentAtIndexZero` while RLE8 and LZ5 use `AlphaLiteral` (`resolve_sprite.go:179`); Ikemen GO treats formats 2, 3 and 4 alike (image.go:1421-1438).
- A palette bank whose length is not a multiple of 4, or holds more than 256 colours, is an error (`palette.go:267-271`); Ikemen GO reads `min(len/4, 256)` colours (image.go:2307-2329).

🔒 MUST: Ikemen GO behaviour (roadmap decision `038`).

## Acceptance Criteria
- [ ] Each rule above is checked against the Ikemen GO source and, where it differs, follows it, with a test per case
- [ ] Docs and WASM output state the alpha that is actually returned
