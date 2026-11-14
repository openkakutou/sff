---
status: todo
---
# Accept 772-byte Photoshop .act palettes

## Description
External `.act` palettes are only accepted at exactly 768 bytes. The local real-file corpus holds 33 `.act` files of 772 bytes (Photoshop's format with a 4-byte trailer, e.g. `01 00 00 00`), which are rejected today. Ikemen GO's `readActPalette` reads up to 768 bytes and ignores the rest, so these files work in the engine but not in this library.

## Acceptance Criteria
- [ ] A 772-byte `.act` with the Photoshop trailer decodes to the same palette as its first 768 bytes
- [ ] Validated against real 772-byte files from the corpus (vendored as a fixture)
- [ ] Other sizes still produce a clear error, and the accepted sizes and trailer handling are recorded against Ikemen GO's behavior

## Notes
Found while framing item 010 (out of its scope, decided by the Product Owner). Must stay compatible with both MUGEN and Ikemen GO.
