---
date: 2026-08-09
status: accepted
---
# `DecodeV2Sprite` covers Raw/RLE8/LZ5/PNG8/24/32; RLE5 stays unimplemented, explicit error over guessing

**Context:** A v2 sprite table entry's `Format` field can declare one of several pixel encodings — `V2FormatRaw`, `V2FormatRLE8`, `V2FormatLZ5`, the three PNG formats, and the real-but-effectively-unused `V2FormatRLE5`. Each has a genuinely different on-disk shape (`v2_decoder.go`'s `decodeV2Raw`/`decodeV2RLE8`/`decodeV2LZ5`/`decodeV2PNG`), dispatched from `DecodeV2Sprite`'s format switch (`v2_decoder.go:45-58`).

**Decision:** `DecodeV2Sprite` supports exactly `V2FormatRaw`, `V2FormatRLE8`, `V2FormatLZ5`, and the three PNG formats (delegating to the standard library's `image/png` for those). Any other format code — including `V2FormatRLE5` — returns a descriptive "unsupported pixel format" error rather than attempting a best-guess decode. Each decoder validates its own preconditions (color depth where meaningful, positive dimensions, declared-size-vs-actual-data-length) before touching the byte stream, and reports a descriptive error rather than panicking on malformed/truncated input.

**Reason:** A scan of a real, local corpus of ~562 `.sff` v2 files found Raw/RLE8/LZ5/PNG8/24/32 all used thousands of times each, and RLE5 used zero times — the reference decoder project this package's algorithms are ported from (`sff-extractor`) has no RLE5 support either. Shipping a decoder for a format with no known real-world fixture would be exercised only by hand-built synthetic data, against this package's own "real-file compatibility over spec purity" constraint (`CLAUDE.md`). An explicit unsupported-format error is safer than any guess at an unverifiable algorithm.

**Rejected alternatives:**
- *Implement RLE5 decode from the documented algorithm alone, with no real fixture to validate against* — rejected for the same reason the encode side already rejected it (see decision `001`): an unverified decoder is worse than an honest "unsupported format" error.

**Superseded (RLE5 portion only):** decision `014` implements `V2FormatRLE5` decode (and encode) anyway, at the Product Owner's explicit request, without waiting for the real fixture this decision treats as a prerequisite. The rest of this decision (Raw/RLE8/LZ5/PNG scope, per-decoder validation, error-over-panic contract) is unaffected.
