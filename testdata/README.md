# testdata

Real test fixtures used by the `.sff` fixture-driven test suites. Migrated
as-is from `character`'s own `sff/testdata` (originally vendored under
`character`'s backlog item 023, and extended by its items 028/029) — see
this repo's `.vibe/backlog/done/001-extract-and-migrate-sff-code-from-character.md`.

## Source

Downloaded from `github.com/ikemen-launcher/sff-extractor`, commit
`2d4af64d26441bf4d692bb479275d64b11869678`:

- `tests/files/*.sff` / `*.act` → trimmed into `files/*.sff` (see below)
- `tests/sprites/*.png` → copied unmodified into `sprites/*.png`

## `sprites/*.png`

Unmodified. These are the reference project's own JS-generated
expected-output PNGs — ground truth for pixel comparisons in items 028/029.

## `files/*.sff`

**Trimmed, not byte-identical to upstream.** A real character `.sff` file
carries every sprite in the character (hundreds of animation frames); the
full set of source files used to build these fixtures totals ~329MB, which
is not vendored into this repository. Each file here was produced from a
real source file by `gen/main.go`, a standalone regeneration tool (not part
of the `sff` package's public API): it locates the exact sprite a scenario
needs (via `ParseV1`/`ParseV2`, selecting the group/nth-in-group the same
way the reference project's own tests do), copies that sprite's real
pixel/palette bytes verbatim, and writes a minimal valid `.sff` containing
just those bytes plus whatever else the same scenario's underlying quirk
depends on (a zero-length "copy" sprite plus its real pixel-owning
ancestor, or a palette-sharing sprite plus its real palette-owning
ancestor, kept as two real entries with their genuine relationship
preserved rather than flattened away).

No pixel or palette *content* is invented — every embedded byte is copied
from the real upstream file. Only the surrounding container (header,
sprite/palette table, which other sprites are physically present) is
authored by the trimming tool, to keep the fixture small. Item 028's v1
scenarios needing an inherited sprite (a zero-length "copy" target and/or a
palette-sharing target) collapse to at most one donor entry immediately
before the target — real Length-versus-palette-block layout (see
`.vibe/decisions/018-v1-palette-block-lives-inside-declared-length.md`) and
the corrected linking rule (`.vibe/decisions/017-...md`) both resolve a
predecessor positionally, never via the on-disk `LinkedIndex` field, once a
sprite has no pixel data of its own.

To regenerate (e.g. after adding a new scenario to `gen/main.go`):

```
SRC_DIR=/path/to/sff-extractor/tests/files go run ./testdata/gen
```

## `files/*.act`

Unmodified, standalone external-palette fixtures (`greenarrow-v1-palette1.act`,
`cyclops-v1-palette1.act`), 768 raw bytes each — small enough to vendor
as-is, unlike the multi-megabyte `.sff` source files. Used by item 028's
"index == linked index" and "external palette" scenarios.

### Known caveat: `v2-loadmode1.sff`

`kazuki-v2.sff`'s "on-demand" (`loadMode = 1`) sprite (item 029) is trimmed
using `ParseV2`'s current offset resolution, which does not yet understand
on-demand addressing (see item 029's notes). This fixture may need
regenerating once `ParseV2` is extended to support it.
