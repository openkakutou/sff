---
status: todo
depends_on: [005]
---
# Remaining Real-File Corpus Decode Gaps

## Description
Backlog item 005's real-file corpus scan (562 real character `.sff` files) found 31 remaining decode failures across 4 distinct shapes, deliberately left undiagnosed/unfixed by that item to keep its own scope to establishing the testing practice plus the two clear, well-understood bugs it did surface. Each shape, its real files, and its sprite/file counts are documented in `.vibe/fixture-sources.md`'s "Corpus compatibility scan results" section.

## Acceptance Criteria
- [ ] The `sff: pcx: unsupported color plane count 3, only 1 is supported` shape (9 sprites, 7 files) is either supported (a real, non-corrupt 3-plane/24-bit PCX decode path) or documented as a permanent, deliberate scope cut, mirroring the existing v2 linked-sprite gap
- [ ] The `reading pixel data: EOF` shape (16 sprites, 8 files, concentrated in one `Thor` file) is root-caused — either a genuine decoder bug (offset/length miscomputation for some v1 table shape) fixed with a trimmed real fixture in `testdata/`, or confirmed to be genuinely truncated/corrupt source files and documented as such
- [ ] The `SharedPalette: false` too-short-declared-length shape (3 sprites, 3 files) is root-caused and either fixed or documented as a known gap, distinct from the two shapes already fixed by item 005's `.vibe/decisions/015`
- [ ] The oversized-dimension shape (2 sprites: 17637×249 and 9979×24) is resolved: either `SpritePixelDimensionLimit` is revisited (e.g. made axis-aware, or raised) if these are legitimate sprites, or confirmed corrupt and left rejected
- [ ] Re-running the `SFF_CORPUS_DIR`-gated `TestCorpusCompat_RealSFFFiles_DecodeSuccessRate` scan afterward shows 0 undocumented failures (any remaining gap is an explicit, named exception in the test's classification, not a silent pass/fail toggle)

## Notes
Depends on item 005 for the corpus-scanning test harness (`corpus_compat_test.go`) and the local corpus itself (`SFF_CORPUS_DIR`, see `.vibe/fixture-sources.md` — machine-specific, never hardcoded). The exact failing (file, group, image) triples for each shape are listed in `.vibe/fixture-sources.md` as of 2026-08-23; re-run the scan first to confirm they still reproduce before investigating, since real files outside this repo's control (or a corpus refresh) could shift the exact list.
