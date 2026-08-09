---
date: 2026-08-09
status: accepted
---
# Palette resolution takes an optional `*Palette` override parameter rather than a separate resolve-with-override function

**Context:** `ResolveV1Palette`, `ResolveV2Palette`, and `ResolveSpritePixels` all need to support the "recolor" use case — a caller (e.g. an editor letting a user pick a different `.act` palette file) wants pixel resolution to use a caller-supplied `Palette` (typically decoded via `DecodeExternalPalette`) instead of whatever palette the sprite/table would normally resolve to.

**Decision:** Every resolution function that would otherwise need to look up a palette takes an `override *Palette` parameter (`palette.go:117`, `259`; `resolve_sprite.go:40`): `nil` means "resolve normally" (table lookup/inheritance chain as usual), non-nil means "use this `Palette` immediately, skip the table lookup" — checked first and short-circuiting (`palette.go:118-120`, `260-262`).

**Reason:** Keeps one function signature and one call site per concern, instead of doubling the exported surface with a `...WithOverride` twin of every resolver. A `nil` override in the common (no-recolor) case needs no wrapper/adapter, and a single non-nil override applies uniformly across v1, v2, and `ResolveSpritePixels` without each call site needing its own override-handling logic — cheap for a caller resolving many sprites/frames against the same replacement palette (e.g. an animation preview).

**Rejected alternatives:**
- *A separate `...WithOverride`-suffixed function per resolver* — rejected: doubles the exported surface for a single boolean-shaped behavior difference, and risks the two variants drifting (e.g. a bug fix landing in one but not the other).
