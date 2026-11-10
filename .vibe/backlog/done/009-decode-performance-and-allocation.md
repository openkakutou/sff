---
status: done
---
# Benchmark and cut decode allocations on large sheets

## Description
Browsers decode large sprite sheets (thousands of sprites). Add benchmarks and remove avoidable allocation or repeated work in table parse, linking and palette resolution; record numbers.

## Acceptance Criteria
- [x] Benchmarks for full-sheet decode and single-sprite resolve with allocation counts
- [x] At least one measured improvement or documented no-op decision
- [x] WASM build time-to-first-sprite measured on a real large sheet
