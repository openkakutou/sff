---
date: 2026-08-09
status: accepted
---
# Palette write-path API shape: validated setter + symmetric Encode* functions

**Context:** Backlog item 003 asks for palette color editing plus re-encoding
for both v1 and v2, and exporting a palette as a standalone `.act` file. The
existing code only has one direction: `DecodeV1Palette`, `DecodeV2Palette`,
and `DecodeExternalPalette` turn on-disk bytes into a `Palette`. There was no
inverse, and no validated way to change a color in a `Palette`.

**Decision:**
1. Add `EncodeV1Palette(Palette) []byte`, `EncodeV2Palette(Palette, colorCount int) ([]byte, error)`,
   and `EncodeExternalPalette(Palette) []byte` as the exact inverses of
   `DecodeV1Palette`, `DecodeV2Palette`, and `DecodeExternalPalette`
   respectively, mirroring the existing `Decode*`/`Encode*` pairing already
   used for pixel data (`DecodePCX`/`EncodePCX`, `DecodeV2Sprite`/`EncodeV2Sprite`).
2. Add `Palette.SetColor(index, r, g, b, a int) error`, a validated setter
   that rejects an out-of-range index or an out-of-range (outside `[0,255]`)
   color component with a descriptive error, instead of the silent
   wraparound a direct `uint8(v)` conversion would produce on a value like
   `300` or `-1`.

**Reason:** `Palette` is `[256]color.RGBA`, whose fields are already
`uint8` — a plain Go assignment through that type can never hold an
out-of-range value, so the "reject out-of-range, don't silently clamp or
corrupt" acceptance criterion only has teeth at a boundary where values
first arrive as wider integers (a caller-facing edit API, and eventually
the WASM/JS bridge in item 004, where JSON/JS numbers aren't `uint8`-typed).
`SetColor` is that boundary. Keeping `Encode*` as pure, always-succeeding
transforms of an already-valid `Palette` (no validation inside them) keeps
the validation responsibility in exactly one place instead of duplicated
across every encode path.

**Rejected alternatives:**
- Validating inside `EncodeV1Palette`/`EncodeV2Palette` themselves: rejected
  because a `Palette`'s `color.RGBA` fields cannot be out of range by
  construction — there is nothing left to validate there, and doing so
  anyway would duplicate `SetColor`'s job at the wrong layer.
- Exposing a raw `func NewColor(r,g,b,a int) (color.RGBA, error)` instead of
  a `Palette` method: rejected because it separates the range check from
  the index check, letting a caller validate the color but still assign it
  at an out-of-range index without an error.
