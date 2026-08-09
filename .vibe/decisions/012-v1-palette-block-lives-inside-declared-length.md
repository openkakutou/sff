---
date: 2026-08-09
status: accepted
---
# A v1 sprite's declared subheader `Length` includes its own trailing embedded palette block, not just its pixel data

**Context:** Verified against real, unmodified `.sff` v1 files: a sprite that owns its palette (`SharedPalette == false`) has its `Length` field cover both its pixel data *and* the `V1PaletteBlockSize`-byte (768) palette block that immediately follows it in the file — not, as a literal reading of "pixel data length" might suggest, just the pixel bytes, with the palette block appended after that `Length` span ends.

**Decision:** `ResolveV1Palette` reads an owning sprite's embedded palette block from the *last* `V1PaletteBlockSize` bytes of its own declared `[Offset, Offset+Length)` span, not a suffix starting right after it (`resolveV1Palette`, `palette.go:127-145`). `resolveV1Pixels` and `SerializeV1` symmetrically treat the real pixel-only byte count as `Length` minus `V1PaletteBlockSize` whenever `SharedPalette` is false (`load.go:192-202`, `v1_serializer.go:92-101`), returning a descriptive error if that would go negative — a corrupt or too-short declared `Length`.

**Reason:** This is a format quirk only discoverable by testing against real files, not the spec alone — a spec-only implementation would plausibly read `Length` as pixel-data-only and either read garbage as pixel data (running past the real end) or fail to locate the palette block at all. Matches `CLAUDE.md`'s "real-file compatibility over spec purity" constraint, same as decision `011`.

**Rejected alternatives:**
- *Model the palette block's location as immediately after a `Length` that covers pixel data only* — rejected: contradicted by real, unmodified `.sff` v1 files.
