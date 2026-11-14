---
date: 2026-11-14
status: accepted
---
# JASC `.pal` and indexed-PNG palettes are editing formats; `.pal` color order is chosen by the caller

**Context:** Backlog item 010 adds JASC `.pal` import/export and indexed-PNG palette import/export beside `.act`. Real community `.pal` files come in two orders: natural JASC order, first line = index 0 (`Nintendo/Luigi/color.pal`), and raw `.act` storage order, first line = index 255 (`King of Fighters/Raiden (XIII)/raiden01.pal`, a line-for-line text dump of `raiden01.act`). The file carries no marker telling them apart.

**Decision:** Neither engine loads these formats: Ikemen GO reads every `.def` `palN` entry through `readActPalette` regardless of extension (Ikemen-GO `src/char.go` `loadPalettes`, `src/image.go` `readActPalette`, commit `0f9911ad`), MUGEN 1.0's docs only describe `.act`, and all 3922 `palN` references in the local corpus are `.act`. So `.pal` and PNG palettes are editing/interchange formats only. `DecodeJASCPalette`/`EncodeJASCPalette` take a `PaletteOrder` (`PaletteOrderNatural` default, `PaletteOrderReversed` for `.act`-order dumps) instead of guessing. PNG palettes are read in natural `PLTE` order (verified 255–256/256 against the matching `.sff` palettes for Kirby HD, Mai, Ruby) and written as a 16×16 swatch where pixel *i* uses index *i*. Every decoder applies the `.act` alpha rule (index 0 transparent, others opaque) and pads a palette shorter than 256 colors with opaque black.

**Reason:** The order cannot be inferred reliably from content, and a silent wrong guess is worse than an explicit caller choice (e.g. an editor's "reverse" toggle).

**Rejected alternatives:**
- *Content heuristic (index 0 should be the background color)* — rejected: can be wrong silently.
- *Natural order only* — rejected: real `.act`-order `.pal` files would load with wrong colors and no way to fix them.
- *A `reversed bool` parameter* — rejected for a named type, readable at the call site.
