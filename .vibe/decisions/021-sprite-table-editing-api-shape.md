---
date: 2026-11-15
status: accepted
---
# Sprite add/replace/remove operate on the write slices and repair links instead of reporting them

**Context:** Editors need to add a sprite, replace its pixels and remove it (backlog item 011) without breaking links (a linked sprite pointing at a removed one) or palette sharing, for v1 and v2, and the result must re-parse and decode identically elsewhere. 🔒 It must stay compatible with MUGEN 1.0/1.1 and Ikemen GO, so the rules are the ones the readers apply (decisions 011, 012, 015, 017), validated against real fixtures under `testdata/`.

**Decision:**
- The API is `AddV1Sprite`/`ReplaceV1Sprite`/`RemoveV1Sprite` over `[]V1WriteSprite`, and `AddV2Sprite`/`ReplaceV2Sprite`/`RemoveV2Sprite`/`AddV2Palette`/`RemoveV2Palette` over `[]V2WriteSprite` + `[]V2WritePalette` (`v1_edit.go`, `v2_edit.go`). Each returns new slices, never mutating the input's elements; byte slices (`PixelData`, `Palette`, `ColorData`) are shared by reference, not copied. Errors are `sff: v1:`/`sff: v2:` prefixed.
- Add appends (so every existing index stays valid), rejects a duplicate key and what `SerializeV1`/`SerializeV2` reject (v1 own palette of 768 bytes unless shared, link out of range); v2 also rejects a palette index outside the bank table.
- Replace swaps pixel data (v1 optionally its own palette, v2 also dimensions/format) and un-links a linked sprite. Dependents of a replaced owner keep following it.
- Remove repairs: the first dependent inherits the removed owner's pixel data (v2: with its format, color depth and dimensions), the other dependents relink to it, a removed linked sprite sends its dependents to its own target, and all indices are renumbered. v1 goes further because readers resolve a zero-length shared-palette sprite from the previous table entry and a shared palette from the previous sprite, and cannot express a link to table index 0: after the structural repair each sprite's resolved pixels and palette are recomputed with the readers' own rules (`v1PixelOwner`, `v1PaletteSource`) and compared to the pre-removal ones; a sprite whose result changed first gets its resolved palette as its own block, then, if still different, a copy of its resolved pixel data. Nothing is ever left silently different.
- `RemoveV2Palette` rejects a bank some sprite still draws with (listing the sprites) rather than guessing a replacement; unused banks are repaired like sprites and every `PaletteIndex` is renumbered.

**Reason:** The write slices are what `SerializeV1`/`SerializeV2` consume, so an editor holds them anyway and the edit is one pure function away from a serializable result. Repairing keeps the sheet valid and decoded output identical, which is the acceptance criterion; the v1 verification step makes that independent of the quirks of the on-disk linking rules instead of re-deriving each case by hand.

**Rejected alternatives:**
- *Report dependents instead of repairing:* pushes the same relinking logic into every editor and leaves a broken intermediate sheet.
- *Operate on parsed tables (`V1SpriteTable`/`V2SpriteTable`):* those carry `Offset`/`Length` facts of a source file that become meaningless after an edit (decision 005/007), and pixel bytes live outside them, so an edit would need the reader and the file too.

**Unverified / divergences:** no MUGEN-vs-Ikemen GO divergence is introduced: v1 reuses the readers' verified rules. For v2, how an engine decodes a linked sprite whose own width/height differ from its target's is not verified against a real file (no real v2 linked fixture exists, see item 029); the inherited fields are therefore taken from the removed owner, the only combination known to decode.
