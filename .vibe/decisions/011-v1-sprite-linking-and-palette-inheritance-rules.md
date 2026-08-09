---
date: 2026-08-09
status: accepted
---
# v1 sprite pixel-linking and palette-inheritance rules replicate the reference decoder's exact behavior, not a literal reading of the format

**Context:** v1's sprite table doesn't fully self-describe two things a consumer needs: which sprite a zero-length entry's pixel data actually comes from, and which palette a `SharedPalette == true` entry actually inherits. Both are governed by rules that are easy to get subtly wrong by reading the format spec loosely instead of verifying against real files and the reference decoder's actual behavior.

**Decision:**
- **Pixel linking** (`resolveV1Pixels`, `load.go:144-212`): a sprite with no pixel data of its own (`Length == 0`) always inherits the immediately preceding table entry's already-resolved image — its own stored `LinkedIndex` is not consulted in this case. A sprite that does carry its own pixel data (`Length > 0`) uses it as-is unless `LinkedIndex` is a genuine backward reference (strictly less than its own index, and not exactly `0`); a self- or forward-reference, or a `LinkedIndex` of exactly `0`, is treated as "no link" and falls back to the sprite's own data — `0` can never be a real link target through this field, a quirk of the on-disk format this package replicates rather than treats as a bug.
- **Palette inheritance** (`resolveV1Palette`, `palette.go:127-145`): a sprite that does not share (`SharedPalette == false`) decodes its own embedded palette block. One that does share inherits table index 0's own resolved palette when it is itself (Group 0, Image 0), or the immediately preceding sprite's resolved palette otherwise. Table index 0 always decodes its own block regardless of its own `SharedPalette` bit, since there is no earlier sprite for it to inherit from.
- Both rules guard against a cycle with an explicit `seen` map (`load.go:164-171`, `palette.go` uses plain recursion since every call strictly decreases the index there too), returning a descriptive error rather than recursing forever — kept for defense in depth even though, per `resolveV1Pixels`'s own analysis, every recursive call strictly decreases the index so a cycle should not actually be reachable.

**Reason:** Both rules were derived from, and are verified against, the reference decoder's actual behavior — not just a literal reading of the MUGEN format spec — matching `CLAUDE.md`'s "real-file compatibility over spec purity" constraint. Getting either rule wrong produces a plausible-looking but subtly incorrect image (wrong pixels or wrong colors) rather than an obvious crash — exactly the class of bug fixture-driven real-file testing (item 028) exists to catch.

**Rejected alternatives:**
- *Always consult a zero-length entry's own `LinkedIndex` field for pixel linking* — rejected: contradicts the reference decoder's actual (verified-against-real-files) behavior, which ignores it in this case in favor of "always the previous entry".
- *Treat `LinkedIndex` `0` as a normal, valid backward link to sprite 0* — rejected: the on-disk format has no way to distinguish "explicitly links to sprite 0" from "this field was never set" (both are the zero value), so treating a literal `0` as a real link risks misattributing unrelated sprites' pixel data.
