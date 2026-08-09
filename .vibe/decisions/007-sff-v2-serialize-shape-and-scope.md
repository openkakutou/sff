---
date: 2026-08-09
status: accepted
---
# `SerializeV2` always writes into the literal data section, only the header fields `ParseV2` itself understands

**Context:** A `.sff` v2 file's on-disk header has a legacy v1-compatibility region and both a "literal" and a "translated" data section, distinguished by a per-entry on-disk flag bit — a feature of the original engine. This package's own `V2SpriteEntry`/`V2PaletteEntry` (`v2.go`) never model that flag as a distinct concept beyond always resolving `Offset` against the literal section; `ParseV2` never reads or exposes a translated-vs-literal distinction as data.

**Decision:** `SerializeV2` (`v2_serializer.go`) only ever writes into the file's literal data section — the on-disk translated-data flag bit is always written as `0` (`writeV2SpriteEntry`, `v2_serializer.go:187-210`) — and only fills the header bytes `ParseV2` actually reads (`writeV2Header`, `v2_serializer.go:164-185`); the legacy v1-compatibility header region is left zeroed.

**Reason:** Since this package's own read path doesn't model or expose the translated-vs-literal distinction, a round trip through `Serialize`→`Parse` never needs it — writing a translated-data-section option this package can't verify by reading it back would be speculative and unverifiable.

**Rejected alternatives:**
- *Model the translated-data section as a genuine write option* — rejected: no real consumer need, and no way to validate it round-trips correctly since `ParseV2` doesn't distinguish it on read either.
