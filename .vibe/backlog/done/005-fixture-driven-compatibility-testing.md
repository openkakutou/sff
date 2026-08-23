---
status: done
depends_on: [001]
---
# Fixture-Driven Compatibility Testing (MUGEN + Ikemen GO)

## Description
Extend the real-file compatibility testing practice `character` already established for `.sff` (its own history references a corpus of 562 real `.sff` v2 files) to this standalone repo, explicitly covering both MUGEN 1.0/1.1 and Ikemen GO sprite files.

## Acceptance Criteria
- [ ] A documented, gitignored-path fixture corpus (same rule as `character`'s `.vibe/fixture-sources.md`: never hardcode the local corpus path into source/tests/committed config) is used to validate decode success rate
- [ ] Any real-file parse failures are triaged and either fixed or explicitly documented as known gaps, not silently ignored

## Notes
Mirrors `character`'s `.vibe/fixture-sources.md` practice, now migrated to
this repo's own `.vibe/fixture-sources.md` — read that file for the exact
convention to replicate.

That file's "`stand*.gif` — ground-truth palette-resolved renders" section
describes a concrete, independent-of-`sff-extractor` validation scenario
worth covering here: real characters' `standN.gif` preview animations are
ground-truth renders of a specific sprite decoded with a specific external
`.act` palette (`palN`) applied, produced by a tool outside this repo's own
reference chain. Comparing `ResolveSpritePixels` output against a
GIF-decoded frame (for a character with a full numbered `standN.gif` set,
trimmed into `testdata/`) would be a stronger end-to-end check than the
existing `sprites/*.png` fixtures, which share a reference project with the
decoders being validated.
