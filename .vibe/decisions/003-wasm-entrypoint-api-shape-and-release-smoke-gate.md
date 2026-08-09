---
date: 2026-08-09
status: accepted
---
# WASM entrypoint mirrors character's global-function shape; release publishing is gated on a real smoke test of the built artifact

**Context:** Item 004 adds this repo's own `cmd/wasm/` WASM entrypoint and tagged-release pipeline, mirroring the sibling `character` repo's proven `cmd/wasm/` + `.github/workflows/release.yml` pattern (see `character`'s `.vibe/decisions/019-...` and `020-...`), since `stage` and lifebar apps consume `sff` directly and need a browser-ready build independent of `character`'s own.

**Decision:**
- The WASM module registers a single JS global, `OpenKakutouSff`, with two functions, mirroring `character`'s `OpenKakutouCharacter` global shape exactly (same never-throws/panic-recovery discipline, same one-of-two-fields-non-null result convention):
  - `load(sffBytes)` — parses via the existing `Load`, returns `{ spriteGroups: string|null, error: string|null }` (a JSON array of `SpriteGroup`). Metadata only, no pixel data — same split `character`'s `load`/`resolveSprites` pair already established.
  - `resolveSprites(sffBytes, requests, overrideBytes)` — batched, stateless pixel resolution via the existing `ResolveSpritePixels`/`DecodeExternalPalette`, identical contract to `character`'s own `resolveSprites` (which already delegates to these same `sff` functions internally): one `{ pixels, width, height, error }` per `[group, image]` request, a stable `"sprite not found: "` error prefix, an explicit invalid (including empty) `overrideBytes` always erroring rather than silently falling back.
- The release workflow (`.github/workflows/release.yml`) runs `go build ./...`/`go test ./...`/`go vet ./...` natively first, exactly like `character`'s, then builds `sff.wasm` (`GOOS=js GOARCH=wasm`) and copies the matching `wasm_exec.js` from the same pinned Go toolchain — but, deliberately deviating from `character`'s workflow, adds a step running `cmd/wasm/smoke.mjs` against the exact just-built `sff.wasm` bytes before the publish step, on a pinned Node.js version. `fail_on_unmatched_files: true` and job-scoped (not workflow-scoped) `contents: write` are kept as in `character`.

**Reason:** Reusing `character`'s exact global-function/result-shape conventions costs nothing (this repo's own `sff.ResolveSpritePixels`/`DecodeExternalPalette` are the very functions `character`'s wasm glue already calls) and keeps any future consumer's JS code shaped the same way regardless of which repo's WASM build it loads. The smoke-gate deviation closes a real gap identified during planning: `character`'s own release workflow builds and publishes its WASM artifact without ever running `smoke.mjs` against it, so a broken build (e.g. a panic-recovery regression at the JS boundary, or a `wasm_exec.js`/Go-version mismatch) could still be tagged and published as a permanent GitHub Release asset with no automated check catching it before publish.

**Rejected alternatives:**
- Copying `character`'s release workflow verbatim, without the smoke-test gate — rejected: mirroring an existing pattern is the default per this item's brief, but a pattern with a known, avoidable release-safety gap shouldn't be propagated unchanged into a second repo when the fix is a single added step.
- A stateful load-then-resolve handle (skip re-parsing `sffBytes` between `load` and `resolveSprites` calls) — rejected for the same reason `character`'s own decision 020 rejected it: no consumer usage pattern yet justifies a resource-lifecycle concern this module has nowhere else.
