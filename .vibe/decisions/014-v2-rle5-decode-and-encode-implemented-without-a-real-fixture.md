---
date: 2026-08-09
status: accepted
---
# V2FormatRLE5 decode and encode are implemented, deliberately overriding decisions 001/006's "no real fixture, stay unimplemented" policy

**Context:** Decisions `001` and `006` both defer `V2FormatRLE5` (decode and encode) on the same grounds: a scan of a ~562-file real-character corpus found zero sprites using format code 3, and the reference project this package's other algorithms are cross-checked against (`sff-extractor`) has no RLE5 support either — so a decoder/encoder for this format would be exercised only by hand-built synthetic data, against this package's own "real-file compatibility over spec purity" constraint (`CLAUDE.md`). `character`'s own backlog (item 030, a placeholder never migrated into this repo) records the same conclusion and names a concrete, authoritative source to port from whenever the trigger condition (a real RLE5 file surfacing) occurs: `Rle5Decode` in `ikemen-engine/Ikemen-GO`'s `src/image.go`.

The Product Owner explicitly asked to implement RLE5 anyway, without waiting for that trigger, understanding the trade-off involved.

**Decision:** `decodeV2RLE5` (`v2_decoder.go`) and `encodeV2RLE5` (`v2_encoder.go`) are implemented and wired into `DecodeV2Sprite`/`EncodeV2Sprite`'s format switch, same as every other supported format.

- `decodeV2RLE5` is a line-by-line port of Ikemen-GO's `Rle5Decode` (fetched directly from `github.com/ikemen-engine/Ikemen-GO/develop/src/image.go` — not reconstructed from memory of the format, to avoid silently fabricating a plausible-but-wrong bitstream layout), restructured into this package's own idiom (explicit run-length loops instead of the reference's decrement-and-check style) and cross-checked by hand-tracing several worked examples against both the reference algorithm and this port before trusting the tests built from them. Like `decodeV2LZ5` (decision `009`), it reports malformed/truncated input as a descriptive error instead of the reference's silent clamp-and-continue behavior.
- `encodeV2RLE5` targets a semantic round trip through `DecodeV2Sprite` (decode → encode → decode is pixel-identical), not byte-exact reproduction of any real encoder's output — the same goal decision `001` already set for RLE8/LZ5. It always emits one run per block (never chains multiple runs via a block's "dl" field), the simplest correct strategy, mirroring `encodeV2LZ5`'s own "literal runs only" simplification.

**Validation gap this decision accepts:** unlike every other format this package handles, **RLE5 has never been validated against a single real on-disk `.sff` file** — there still isn't one in the known corpus. Its correctness rests entirely on: (a) the reference algorithm being ported verbatim rather than reimplemented from a written spec, and (b) round-trip tests (hand-built byte sequences traced against the reference by hand, plus `encodeV2RLE5`'s own output fed back through `decodeV2RLE5`). If a real RLE5-encoded sprite ever turns up (the trigger condition decisions `001`/`006`/`character` item 030 were waiting for), it should be added as a `testdata/` fixture and used to actually confirm this port's correctness — until then, "the reference algorithm was ported faithfully" is a weaker guarantee than this package's usual "validated against real, unmodified community files" standard (`CLAUDE.md` design constraint 4).

**Reason:** The Product Owner's call — proceeding gives `character`/`character-editor`/`stage` full v2 pixel-format coverage now rather than an indefinitely-deferred gap, at the accepted cost of one format lacking this package's usual real-file validation until a fixture surfaces.

**Rejected alternatives:**
- *Keep waiting for a real fixture, per decisions 001/006* — the default/previously-chosen path; not taken this time because the Product Owner asked to proceed anyway.
- *Reconstruct the RLE5 bit layout from memory/general format knowledge instead of fetching the actual reference source* — rejected: this package's whole value proposition is real-format compatibility, and guessing at a bitstream layout risks shipping code that looks plausible but silently produces wrong pixels for any real file that does eventually turn up, which is worse than the honest "unimplemented" error this replaces.
