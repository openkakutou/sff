---
date: 2026-08-09
status: accepted
---
# `V1SpriteEntry` stays a distinct, low-level type from `Sprite` — it never carries Width/Height/Palette itself

**Context:** A `.sff` v1 file's on-disk sprite subheader (`v1.go`'s `V1SpriteEntry`) carries `Offset`/`Length`/`LinkedIndex`/`SharedPalette` — where a sprite's pixel data lives and how it's linked/palette-shared — but never its pixel dimensions or a resolved palette index: v1 simply doesn't declare width/height in the table, only implicitly in the PCX-encoded pixel data that follows. The shared, version-agnostic read-path model `Sprite` (`sprite.go`) always carries `Width`/`Height`/`Palette`, populated by whichever version's `Load` path fills it in. v2's own low-level type, `V2SpriteEntry`, already carries the same width/height/palette-index shape as `Sprite` (v2's table declares them on disk) — but is still kept distinct from `Sprite` too.

**Decision:** `V1SpriteEntry` models only what v1's on-disk table actually stores — never `Width`/`Height`/a resolved `Palette` index. `ParseV1` never decodes pixel data; it only reads the table. Turning a `V1SpriteEntry` into a `Sprite` — decoding PCX pixel data to recover `Width`/`Height` (`peekPCXDimensions`/`DecodePCX`) and deriving the palette-sharing counter (see decision `011`) — is `Load`'s job alone (`loadV1`, `load.go:99-129`), never the table type's or `ParseV1`'s.

**Reason:** Keeps `ParseV1` cheap and read-only-of-the-table — a caller that only wants sprite metadata for indexing/listing (`V1SpriteTable.Index`/`.Offset`) never pays a PCX-decode cost just to read the table. It also matches what's genuinely different between the two format versions: v2's table is self-describing enough that `V2SpriteEntry`→`Sprite` is a direct field copy (`loadV2`, `load.go:217-237`), while v1 genuinely requires decoding to know a sprite's dimensions — that version-specific difference stays contained to `Load`, not leaked into a data type meant to mirror what's on disk.

**Rejected alternatives:**
- *Add `Width`/`Height` fields to `V1SpriteEntry`, populated by having `ParseV1` itself decode PCX data* — rejected: makes every table read pay pixel-decode cost, even for callers who only want metadata, and blurs the table-vs-pixel-data boundary `ParseV1`/`DecodePCX` otherwise keep clean.
- *Merge `V1SpriteEntry` directly into `Sprite`, leaving `Width`/`Height` zero until `Load` fills them in* — rejected: a partially-populated public type is a footgun (a caller could reasonably assume `Sprite.Width` is always meaningful), and it erases the useful distinction between "what the table said" and "what `Load` derived".
