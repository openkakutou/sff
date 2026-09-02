---
status: in_progress
depends_on: [005]
---
# Remaining Real-File Corpus Decode Gaps

## Description
Backlog item 005's real-file corpus scan (562 real character `.sff` files) found 31 remaining decode failures across 4 distinct shapes, deliberately left undiagnosed/unfixed by that item to keep its own scope to establishing the testing practice plus the two clear, well-understood bugs it did surface. Each shape, its real files, and its sprite/file counts are documented in `.vibe/fixture-sources.md`'s "Corpus compatibility scan results" section.

## Acceptance Criteria
- [x] The `sff: pcx: unsupported color plane count 3, only 1 is supported` shape (8 sprites, 6 files — 9/7 in the original 2026-08-23 count, corrected on re-scan) is documented as a permanent, deliberate scope cut, mirroring the existing v2 linked-sprite gap — see `.vibe/decisions/018`
- [x] The `reading pixel data: EOF` shape (17 sprites, 7 files — 16/8 in the original count, corrected on re-scan) is root-caused: a genuine decoder bug (a v1 file's last sprite owning real pixel data can overstate its own declared `Length` by 768 bytes when `SharedPalette` is `true`), fixed with a trimmed real fixture in `testdata/` — see `.vibe/decisions/017`
- [x] The `SharedPalette: false` too-short-declared-length shape (3 sprites, 3 files) is root-caused (confirmed, per each file's own consistent neighboring-subheader layout, to genuinely lack room for the mandatory palette block — not a decoder bug or an undiscovered inheritance rule) and documented as a known, permanent gap — see `.vibe/decisions/018`
- [x] The oversized-dimension shape (2 sprites: 17637×249 and 9979×24) is resolved: `SpritePixelDimensionLimit` is now axis-aware (area-based, not per-axis) — both are legitimate sprites — see `.vibe/decisions/016`
- [x] Re-running the `SFF_CORPUS_DIR`-gated `TestCorpusCompat_RealSFFFiles_DecodeSuccessRate` scan afterward shows 0 undocumented failures (confirmed: 665,284 decoded, 9,414 + 8 + 4 sprites across 3 named, explicit gaps, 0 undocumented — the 1 remaining shape from the original count, `truncated RLE run`, is folded into the same confirmed-corrupt-file gap as the `SharedPalette: false` shape above, both diagnosed as genuinely inconsistent real source data)

## Notes
Depends on item 005 for the corpus-scanning test harness (`corpus_compat_test.go`) and the local corpus itself (`SFF_CORPUS_DIR`, see `.vibe/fixture-sources.md` — machine-specific, never hardcoded). The exact failing (file, group, image) triples for each shape are listed in `.vibe/fixture-sources.md` as of 2026-08-23; re-run the scan first to confirm they still reproduce before investigating, since real files outside this repo's control (or a corpus refresh) could shift the exact list.
