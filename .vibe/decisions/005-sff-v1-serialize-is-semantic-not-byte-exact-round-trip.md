---
date: 2026-08-09
status: accepted
---
# `SerializeV1`/`EncodePCX` target a semantic round trip, not byte-exact reproduction of an original file

**Context:** `.sff` v1's binary layout — specifically PCX scanline RLE encoding — has more than one valid on-disk representation of the same pixels: a real MUGEN-authored file's exact byte layout reflects whatever run-length choices its own original authoring tool made, which this package has no way (and no need) to reproduce.

**Decision:** `SerializeV1` (`v1_serializer.go`) and `EncodePCX` (`pcx_encoder.go`) always produce a fresh, valid, freshly-encoded layout — never an attempt at byte-for-byte reproduction of any original file. `EncodePCX` specifically always emits RLE run-length units (`encodeRun`, `pcx_encoder.go:63-80`), even for a run of a single pixel, rather than ever emitting a bare literal byte — one code path, unambiguous to decode, at the cost of occasionally being a few bytes larger than a hand-optimized original encoder's output for the same pixels. Round-trip correctness is verified semantically (parse → serialize → re-parse → re-decode → compare pixels), never by comparing raw bytes against an original file.

**Reason:** `.sff` is binary, not diff-friendly text the way `stage`'s `.def` or `character`'s `.air`/`.cns` are — there's no Git-diff-noise reason to preserve an original file's exact bytes the way those formats' `Document` types do. Always-run-length-encoding keeps `EncodePCX`'s correctness trivial to reason about (one path: `encodeRun`) instead of adding a second literal-byte path purely for a marginal size win that no acceptance criterion asks for.

**Rejected alternatives:**
- *Attempt byte-exact reproduction of an original file's own RLE choices* — rejected: there's no way to know which of several valid encodings an original authoring tool chose without reimplementing that tool's own encoder heuristics — an intractable, unnecessary goal for a semantic round trip.
- *Emit bare literal bytes for singleton runs* (never marker-encode a run of length 1) — rejected: introduces a second encoding path, and requires extra care to never do so for a byte whose own bit pattern would misparse as a run marker on decode (top two bits `0b01`), for no round-trip-correctness benefit over always marker-encoding.
