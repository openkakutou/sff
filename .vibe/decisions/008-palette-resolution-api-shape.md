---
date: 2026-08-09
status: accepted
---
# Palette resolution is a separate, explicit-opt-in helper, not folded into `DecodePCX`/`DecodeV2Sprite`; `AlphaRule` is an explicit parameter

**Context:** `DecodePCX`/`DecodeV2Sprite` return indexed pixel data (one palette-index byte per pixel, via `PCXImage`/`V2Image`), not final RGBA colors. Turning indices into colors needs a second piece of data — the sprite's own embedded/linked palette, or a caller-supplied override (decision `010`) — and the two decoded-pixel families disagree on how index 0's alpha resolves: PCX and PNG8 force it fully transparent, RLE8/LZ5 use the palette's own literal alpha value (`palette.go:47-61`).

**Decision:** `Palette`, `ResolvePixels`, and the version-specific `ResolveV1Palette`/`ResolveV2Palette` (`palette.go`) live as their own API surface, called only when a consumer actually wants final on-screen colors — `DecodePCX`/`DecodeV2Sprite` never resolve colors internally. `ResolvePixels` takes an explicit `AlphaRule` parameter (`AlphaForceTransparentAtIndexZero` or `AlphaLiteral`, `palette.go:47-61`) rather than inferring it from an implicit source-format tag; callers that already know which format produced their pixel data (e.g. `resolveSpritePixelsV1`/`V2`, `resolve_sprite.go:74`, `115-121`) thread the right rule through explicitly.

**Reason:** A caller that only wants indexed pixel data — to inspect dimensions or compare raw index buffers, say — shouldn't pay palette I/O/decode cost or need to handle palette-resolution errors it doesn't care about. Keeping `AlphaRule` explicit keeps `ResolvePixels` itself simple and format-agnostic: it never needs to know about every current and future pixel format's alpha convention, since that one piece of format-specific knowledge stays with its caller, which already has it.

**Rejected alternatives:**
- *Fold palette resolution into `DecodePCX`/`DecodeV2Sprite` directly, returning RGBA colors* — rejected: forces every caller to pay palette I/O/decode cost and handle palette errors, even callers that only want raw indices/dimensions.
- *Infer `AlphaRule` inside `ResolvePixels` from an implicit source-format tag* — rejected: would couple a generic resolution function to every pixel format's own alpha convention, a piece of knowledge it doesn't otherwise need.
