---
status: done
depends_on: [002]
---
# Implement V2 RLE5 Decode and Encode

## Description
`V2FormatRLE5` was the one v2 pixel format left unimplemented on both the decode and encode side (decisions `001`/`006`), deliberately, for lack of any real fixture — a scan of a ~562-file real-character corpus found zero sprites using it. The Product Owner asked to implement it anyway, without waiting for a real fixture to surface, accepting the resulting validation gap.

## Acceptance Criteria
- [x] `DecodeV2Sprite(V2FormatRLE5, ...)` decodes RLE5-encoded data into the correct row-major index buffer, ported from Ikemen-GO's `Rle5Decode` (`src/image.go`, fetched directly rather than reconstructed from memory)
- [x] Malformed/truncated RLE5 data returns a descriptive error instead of panicking or reading out of bounds, matching `decodeV2RLE8`/`decodeV2LZ5`'s own contract
- [x] `EncodeV2Sprite(V2FormatRLE5, ...)` encodes indexed pixel data into a valid RLE5 stream `decodeV2RLE5` decodes back correctly (semantic round trip), mirroring `encodeV2LZ5`'s "simplest correct" precedent
- [x] Round-trip (decode → encode → decode) is pixel-identical, covered by hand-built and synthetic tests
- [ ] A real-file corpus fixture validates the decoder/encoder — **not satisfied**: no real RLE5-encoded `.sff` v2 file is known to exist yet; see decision `014` for the accepted gap this leaves

## Notes
See `.vibe/decisions/014-v2-rle5-decode-and-encode-implemented-without-a-real-fixture.md` for the full rationale, what was and wasn't validated, and the explicit trade-off accepted by proceeding without a real fixture. Decisions `001` and `006` are amended with pointers to this supersession; their RLE8/LZ5/PNG reasoning is otherwise unchanged.

If a genuine RLE5-encoded character file ever turns up (the trigger condition this item previously wasn't waiting for), add it to `testdata/` and extend `TestEncodeV2Sprite_RLE8Format_RealFixtureRoundTrips`'s sibling pattern for RLE5 — closing the one remaining gap this item leaves.
