---
status: done
---
# Extract & Migrate Existing `sff` Parse/Serialize/Palette Code From `character`

## Description
`character`'s `sff/` package (v1 and v2 `.sff` parsing/serialization, palette resolution including `.act` overrides) is the starting point for this repo — move that code here, adapt the package/module path, and ensure this repo's own test suite (using `character`'s existing fixtures/testdata as a starting corpus) passes standalone with no dependency on `character`.

## Acceptance Criteria
- [x] Existing v1/v2 parse+serialize+palette functionality is reproduced here with equivalent test coverage
- [x] No dependency on the `character` module
- [x] `character`'s own migration item (035) can then depend on this being tagged/released

## Notes
This is the cross-repo blocker for `character`'s backlog item 035 (Migrate To Depend On Standalone sff Repo).
