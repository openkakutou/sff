---
date: 2026-08-09
status: accepted
---
# `decodeV2LZ5` reports malformed/truncated input as an error, unlike the reference decoders' silent clamp-and-continue behavior

**Context:** The reference decoders this package's LZ5 algorithm is ported from (JS/`sff-extractor` and Ikemen-GO's `Lz5Decode`) tolerate running out of input by clamping their read cursor to the last valid byte — silently reusing stale data — and let an over-long run stop writing once the output buffer is full without reporting it.

**Decision:** `decodeV2LZ5` (`v2_decoder.go:183-293`) — like `decodeV2RLE8` before it — treats both cases as a descriptive error instead: input exhausted before the declared pixel count is filled (the `next` helper, `v2_decoder.go:196-203`), and a run/back-reference that would overrun the declared image size (`v2_decoder.go:258-263`, `282-284`). This matches `decodeV2RLE8`'s own error-handling contract for the same "malformed compressed data" class of problem.

**Reason:** Silently reusing stale/adjacent memory or truncating output without signaling it turns a corrupt or adversarial input into a subtly wrong — not obviously broken — decoded image. An explicit error is safer for this package's callers, some of whom resolve untrusted, caller-supplied bytes via the WASM boundary (decision `013`), than reproducing the reference implementation's leniency bug-for-bug.

**Rejected alternatives:**
- *Replicate the reference decoders' clamp-and-continue behavior exactly, for maximum bug-compatibility* — rejected: no real fixture in this package's real-file corpus actually needs that leniency (LZ5 decodes correctly for every vendored real fixture without it), and propagating a known reference bug has a worse failure mode (silently wrong pixels) than a caught, reported error.
