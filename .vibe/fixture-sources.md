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
visually. Not yet acted on; a good candidate scenario for backlog item 005
(fixture-driven compatibility testing) or a follow-up item once a
character with a full `standN.gif` set is trimmed into `testdata/`.
