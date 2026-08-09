---
date: 2026-08-09
status: accepted
---
# `ResolveSpritePixels` (and its WASM binding) is a batched, stateless, per-call contract — no server-side handle/session

**Context:** `cmd/wasm`'s `resolveSprites` function (decision `003`) and this package's own `ResolveSpritePixels` (`resolve_sprite.go`) both need to decide how a JS caller resolves potentially many sprites' pixels from the same underlying `.sff` file bytes — e.g. a viewer rendering every frame of an animation, or every sprite in a sheet.

**Decision:** `ResolveSpritePixels` — and the WASM `resolveSprites` binding built on it — takes the full file bytes and a `(group, image)` request on every call, parsing/re-reading the file fresh each time, rather than exposing a stateful "open this file, get a handle, resolve N times against the handle" API. The WASM binding batches multiple sprite requests per call (one `{ pixels, width, height, error }` result per request, decision `003`), amortizing the cost of a single parse across many resolutions within one call, without requiring the caller to manage any resource lifecycle (open/close, handle validity) across calls.

**Reason:** No consumer usage pattern documented anywhere in this org yet justifies the added complexity — and the WASM-specific problem of a handle needing explicit release, since there is no garbage-collection visibility across the JS/Wasm boundary — of a stateful resource. A batched, stateless call is simpler to implement, simpler for a caller to use correctly (nothing to forget to close/release), and already captures the main benefit a stateful handle would provide, since within-call batching already amortizes the parse.

**Rejected alternatives:**
- *A stateful load-then-resolve handle, skipping re-parsing between calls* — rejected for the same reason decision `003` rejected it for the WASM binding specifically: no consumer usage pattern yet justifies a resource-lifecycle concern this module has nowhere else, and within-call batching already captures the main performance benefit without the lifecycle-management cost.
