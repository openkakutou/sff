# Fixture sources for `.sff` test data

Reference material and real-world files used to source or validate test
fixtures for this repo. Kept separate from the codebase index proper since
none of this is referenced by path from Go code — some of it is a local,
machine-specific resource unavailable in CI or on other machines.

Migrated from `character`'s own `.vibe/fixture-sources.md` (see this
repo's backlog item 001) and extended with item 005's own findings.

## `ikemen-launcher/sff-extractor` (JS reference project)

`github.com/ikemen-launcher/sff-extractor` — the user's own JS project,
extracting/decoding `.sff` sprite data. Used as the primary behavioral
reference for `sff`'s v1/v2 pixel decoding and palette-resolution logic
(RLE8, LZ5, PNG8's forced index-0 transparency, external `.act` palette
handling including its reversed-byte-order quirk). Its own `tests/files/`
and `tests/sprites/` are the source for the trimmed real fixtures vendored
into `testdata/` (see `testdata/README.md`).

Has no RLE5 decoder (`decodeSpriteBuffer.mjs` throws `TODO RLE5`) — not a
reference for that format, see below.

## `ikemen-engine/Ikemen-GO` (the real game engine)

`github.com/ikemen-engine/Ikemen-GO`, `src/image.go` — the actual game
engine `.sff` files are built to run in, written in Go. Used to
cross-validate `sff-extractor`'s decode logic (`Rle8Decode`/`Lz5Decode`
match `decodeRLE8.mjs`/`decodeLZ5.mjs` algorithmically) and as the primary
reference where `sff-extractor` has no implementation to port from —
notably `Rle5Decode`, the RLE5 pixel decoder, which stays unimplemented in
this repo (no known real fixture, see the decision recorded for that
scope).

## Local real-character corpus (not referenced from code)

`~/workspace/ikemen-quick-versus/chars/` on the machine this repo is
usually developed on: a real Ikemen GO frontend install with **562 real
character `.sff` files (~15GB)** across ~57 game franchises. Available
interactively for:
- Finding real fixtures for scenarios the bundled `sff-extractor` test
  files don't cover.
- Statistically validating format-code assumptions — e.g. a full scan
  found **zero** RLE5-encoded (format 3) sprites across all 562 files
  (55806 RLE8, 6163 LZ5, 52682+149+3154 PNG8/24/32), meaning RLE5 is
  apparently unused by real modern characters.

**This path must never be hardcoded into Go source, tests, or committed
config** — it only exists on this machine, not in CI or for other
contributors. Use it to *find and verify* candidate real fixtures, then
trim/vendor the result into `testdata/`, exactly as if it had been
sourced from any other one-off, non-reproducible location.

### `stand*.gif` — ground-truth palette-resolved renders

Many character folders in that same corpus also ship pre-rendered preview
GIFs of their "Stand" animation, one per selectable palette: `stand1.gif`
↔ `pal1` (the `[Files]` section's `pal1=...`/external `.act` entry in the
character's `.def`), `stand2.gif` ↔ `pal2`, and so on, up to however many
palettes the character defines (`pal.defaults` in `.def` lists which
palette slots exist). Confirmed against `Guilty Gear/Ky Kiske/`: 12
palette slots in its `.def`, 12 matching `standN.gif` files. Each is a
genuine multi-frame animated GIF (Ky Kiske's is 6 frames), not a static
portrait — it's the actual in-engine "Stand" animation loop rendered with
that palette applied.

Not every character ships the full set: `Guilty Gear/Baiken/` defines 12
palette slots but ships a single unnumbered `stand.gif` (presumably
rendered with `pal1`/the default) — treat the numbered correspondence as
"when present", not guaranteed universal coverage.

**Why this matters for `sff`:** these GIFs are an independent ground-truth
render of "this sprite, decoded, with this specific external palette
applied" — produced by an unrelated tool (whatever originally exported
these previews), not by anything in this repo's own dependency chain. That
makes them a stronger end-to-end check than the `sprites/*.png` fixtures
already vendored (which come from the same reference project, `sff-extractor`,
that the decoders are validated against): decode the character's "Stand"
animation's first sprite (or frame-matched sequence) via `ParseV1`/
`ParseV2` + `ResolveSpritePixels`, apply the corresponding `palN`'s `.act`
override, and compare against the matching GIF frame — pixel-for-pixel or
visually. Not yet acted on — a good candidate for a follow-up backlog
item once a character with a full `standN.gif` set is trimmed into
`testdata/`.

## Corpus compatibility scan results (backlog item 005, 2026-08-23)

`TestCorpusCompat_RealSFFFiles_DecodeSuccessRate` (`corpus_compat_test.go`,
gated on the `SFF_CORPUS_DIR` env var — never run by default) scanned the
full local corpus above: **562 files, 674,710 sprites**.

- **665,265 decoded successfully (98.6% of all sprites declared).**
- **9,414 sprites (1.4%)** hit the already-documented, accepted gap: v2
  sprites that share rather than own their pixel data (see
  `resolveSpritePixelsV2`'s own doc comment in `resolve_sprite.go`) — not
  counted as failures.
- **Excluding that gap, 665,265 / 665,296 attempted decodes succeeded
  (99.995%).**
- Two real bugs this scan found in `resolveV1Palette`'s inheritance rule
  were fixed as part of item 005 (closing 4,133 sprites' worth of
  failures) — see `.vibe/decisions/015-v1-zero-length-sprite-palette-inheritance-corrected.md`.

### Remaining 31 failures — documented, accepted gaps (not fixed by item 005)

Each is real, low-volume (31 / 674,710 = 0.0046%), and outside this
item's scope — triaged rather than silently ignored, tracked for
follow-up work:

| Shape | Count | Files | Notes |
|---|---|---|---|
| `sff: pcx: unsupported color plane count 3, only 1 is supported` | 9 | 7 files (`Green Lantern`, `Daredevil` ×2, `Donkey Kong SD` ×2, `Snorlax`, `Sailor Neptune`, `sf4omni`) | A 3-plane (24-bit RGB) PCX sprite — this decoder only supports the 1-plane (8-bit indexed) PCX MUGEN sprites normally use. Likely a portrait/special-purpose image exported as truecolor rather than indexed; needs its own decode path (or a documented permanent scope cut, mirroring the v2 linked-sprite gap) to resolve. |
| `reading pixel data: EOF` | 16 | 8 files (`Thor` alone accounts for 10) | The declared `[Offset, Offset+Length)` span reads past the actual file's end. Root cause not yet diagnosed — could be a real corrupted/truncated file in this corpus, or a genuine offset/length computation bug for a specific v1 table shape (all 16 are v1). Needs a minimal repro trimmed from one of these files to investigate further. |
| `sff: v1 palette: sprite N: declared length M is smaller than the 768-byte palette block it must contain` (`SharedPalette: false`) | 3 | `Darkstalkers/Anita`, `Doctor Strange`, `Donkey Kong SD` | Unlike the two shapes fixed in decision `015`, these sprites explicitly claim to own their palette (`SharedPalette` false) yet declare too little data for one — a genuine defect in these specific files (or a still-undiscovered third inheritance rule). Left as an error rather than guessed at. |
| `sff: declared sprite size WxH exceeds the 4096x4096 limit` | 2 | `Supergirl` (17637×249), `SatanZ2` (9979×24) | `SpritePixelDimensionLimit` (see `resolve_sprite.go`) is tuned for "typical" sprites, well under 1024×1024 per axis; these are extreme-aspect-ratio strips (very wide, short) that may be entirely legitimate (e.g. a filmstrip-style sheet) rather than corrupt. Worth revisiting whether the limit should be axis-aware or simply raised. |
| `sff: pcx: truncated RLE run at row 225: missing value byte` | 1 | `Dragon Ball/Yamcha` | A single real file with an apparently genuinely truncated/corrupt RLE stream — likely a legitimately damaged source file, not a decoder bug. |

Tracked as a follow-up backlog item rather than fixed here, to keep this
item's scope to establishing the practice plus the two clear, well-
understood bugs it surfaced.
