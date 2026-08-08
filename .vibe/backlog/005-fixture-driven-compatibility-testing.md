---
status: todo
depends_on: [001]
---
# Fixture-Driven Compatibility Testing (MUGEN + Ikemen GO)

## Description
Extend the real-file compatibility testing practice `character` already established for `.sff` (its own history references a corpus of 562 real `.sff` v2 files) to this standalone repo, explicitly covering both MUGEN 1.0/1.1 and Ikemen GO sprite files.

## Acceptance Criteria
- [ ] A documented, gitignored-path fixture corpus (same rule as `character`'s `.vibe/fixture-sources.md`: never hardcode the local corpus path into source/tests/committed config) is used to validate decode success rate
- [ ] Any real-file parse failures are triaged and either fixed or explicitly documented as known gaps, not silently ignored

## Notes
Mirrors `character`'s `.vibe/fixture-sources.md` practice — read that file for the exact convention to replicate.
