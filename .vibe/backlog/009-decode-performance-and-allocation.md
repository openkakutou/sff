---
status: todo
---
# Benchmark and cut decode allocations on large sheets

## Description
Browsers decode large sprite sheets (thousands of sprites). Add benchmarks and remove avoidable allocation or repeated work in table parse, linking and palette resolution; record numbers.

## Acceptance Criteria
- [ ] Benchmarks for full-sheet decode and single-sprite resolve with allocation counts
- [ ] At least one measured improvement or documented no-op decision
- [ ] WASM build time-to-first-sprite measured on a real large sheet
