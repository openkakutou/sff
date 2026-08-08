---
status: todo
depends_on: [001]
---
# WASM Entrypoint And Release Pipeline

## Description
This repo needs its own WASM build and tagged-release pipeline (mirroring `character`'s `cmd/wasm/` + `.github/workflows/release.yml` pattern), since `stage` and lifebar apps consume it directly, independent of `character`'s own WASM build.

## Acceptance Criteria
- [ ] `GOOS=js GOARCH=wasm` build produces a working `sff.wasm` + `wasm_exec.js`
- [ ] A Node-based smoke test verifies it loads and decodes a real fixture sprite
- [ ] A GitHub Actions workflow publishes both artifacts on every tag, mirroring `character`'s release workflow

## Notes
None.
