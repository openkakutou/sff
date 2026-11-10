---
status: todo
---
# Bound v1 linked-sprite chain resolution cost in Load

## Description
A v1 file whose sprites mostly have `Length == 0` makes `Load` re-resolve each sprite's whole backward chain (and re-decode the target PCX) per sprite, which is O(n²) for n up to `MaxTableEntries` (262144). A hostile file can therefore be slow (not memory-heavy). Memoize resolved images per index during `Load` (or equivalent) so each sprite is decoded at most once.

## Acceptance Criteria
- [ ] A v1 file with one real sprite followed by many zero-length sprites loads in linear time
- [ ] Decoded results unchanged on the real fixtures and the local corpus scan
- [ ] Regression test with a large zero-length chain

## Notes
Found while doing item 008 (fuzzing); out of its scope. Must stay compatible with both MUGEN and Ikemen GO.
