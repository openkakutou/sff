---
date: 2026-11-17
status: accepted
---
# 772-byte Photoshop `.act` palettes are accepted, other non-768 sizes stay rejected

**Context:** Backlog item 017. The local real-file corpus holds 6006 `.act` files of 768 bytes and 33 of 772 bytes (all Guilty Gear characters, e.g. `eddie03.act`). The 772-byte files carry Photoshop's 4-byte trailer: a big-endian color count (`01 00` = 256 on every one) and a transparent index (`00 00`, `00 c8`, `00 ff`…). `DecodeExternalPalette` rejected them.

**Decision:** `DecodeExternalPalette` accepts exactly 768 bytes, or exactly 772 bytes, and in that case ignores the trailer entirely: the palette is decoded from the first 768 bytes, so it is the same as for those 768 bytes. Every other size is still an error, and the error message names both accepted sizes.

| Engine | Behavior on a non-768 `.act` | Source |
|---|---|---|
| Ikemen GO | Reads up to 768 bytes with `io.ReadFull` and ignores the rest. A shorter file is accepted: missing colors stay at zero. | `ikemen-engine/Ikemen-GO` `src/image.go` `readActPalette` (commit `54d4148`) |
| MUGEN 1.0/1.1 | **Unverified.** No source available here, and no MUGEN binary was run. | — |
| `sff` (this repo) | 768 or 772 accepted, trailer ignored. Every other size rejected. | this decision |

**Reason:** 772 is the only non-768 size seen in the real corpus, and it is a known, named format: Photoshop's. Ikemen GO loads these files, so rejecting them was a real incompatibility. The trailer's transparent index is ignored, as Ikemen GO ignores it: index 0 stays the transparent color, the same convention as for a 768-byte file.

**Rejected alternatives:**
- *Accept any size like Ikemen GO (truncate or zero-pad)* — rejected: no real file needs it, and a silently padded palette from a truncated or wrong file is worse than a clear error. Item 017's criteria ask for other sizes to keep a clear error.
- *Honor the trailer's transparent index* — rejected: neither engine does, so an `.act` would render differently here than in Ikemen GO.
- *Check the trailer's color count (`0x0100`)* — rejected: Ikemen GO does not check it. Every corpus file has `0x0100` anyway, so the check would add a rejection path no real file needs.
