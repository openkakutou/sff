---
date: 2026-09-02
status: accepted
---
# PCX 3-plane sprites and 4 confirmed-corrupt real files stay permanent, named gaps

**Context:** Backlog item 007's real-file corpus scan left 12 sprites (of 674,710) undiagnosed after the two real decoder fixes in decisions `016`/`017`, split into two distinct shapes:
1. **8 sprites, 6 files**: PCX pixel data declaring 3 color planes (24-bit truecolor) — this package's PCX decoder only ever supported the 1-plane (8-bit indexed) shape real MUGEN/Ikemen sprites normally use, matching the reference `sff-extractor` project this package's PCX decode logic is validated against.
2. **4 sprites, 4 files**: internally inconsistent declared data with no plausible systematic explanation — 3 sprites (`Anita`, `Doctor Strange`, `Donkey Kong SD`) declare `SharedPalette: false` with a `Length` too small to contain even the mandatory 768-byte palette block, confirmed (by comparing against the immediately following sprite's own subheader position) to be the file's true, self-consistent layout — genuinely too little space exists, not a misread. 1 sprite (`Yamcha`) has a PCX RLE stream that terminates mid-row with a missing value byte — a real, on-disk truncated/corrupt encoding, not a parsing bug.

**Decision:** Neither shape is implemented or worked around. Both are registered as explicit, named, individually-identified (by real file path + group/image) entries in `corpus_compat_test.go`'s `acceptedCorpusGaps`, so the corpus scan still passes loudly-verified rather than silently ignoring them — mirroring the existing `v2LinkedSpriteGapName` precedent (decision-equivalent gap, same test file). `ResolveSpritePixels`/`Load` continue to return a descriptive error for these exact sprites; every other sprite in the same files still decodes normally.

**Reason:** Both shapes are real, low-volume (12 / 674,710 = 0.0018%), and outside the value/risk trade-off of new decode logic:
- 3-plane PCX would need a materially different pixel model (direct RGB, no palette) than every other v1 sprite this package returns — the same category of scope cut already made for v2's linked-sprite pixel data (see `resolveSpritePixelsV2`'s own doc comment) and RLE5's real-fixture gap (decision `014`).
- The 4 confirmed-corrupt files have no consistent alternate layout rule to infer from — unlike decisions `011`/`015`'s v1 palette-inheritance fixes, which were each confirmed by a repeatable, consistent pattern across every affected sprite, these fail in ways with no shared structure to generalize from, and guessing at one would risk silently accepting genuinely malformed data elsewhere.

**Rejected alternatives:**
- **Implementing 3-plane PCX decode**: rejected for this item — would require extending `PCXImage`'s pixel model (or introducing a direct-color variant) to carry non-palette pixel data, a design question affecting every consumer of this package's v1 decode path, not a narrow bug fix; left for a future, deliberately-scoped item if real demand justifies it.
- **A generic "recover from any read/decode error, return a fallback" catch-all**: rejected — would silently mask genuinely new, undiagnosed failure shapes in the future the way the existing corpus test's "any undocumented failure fails loudly" design exists specifically to prevent.
