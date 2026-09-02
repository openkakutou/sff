---
date: 2026-09-02
status: accepted
---
# `SpritePixelDimensionLimit` bounds total pixel area, not either axis independently

**Context:** Backlog item 007's real-file corpus scan (562 files, 674,710 sprites) found two legitimate real sprites `checkSpriteDimensions` rejected outright: `Supergirl` at 17637x249 and `SatanZ2` at 9979x24 — extreme-aspect-ratio "filmstrip" sprites, not corrupt data. The existing check rejected any sprite with either axis beyond `SpritePixelDimensionLimit` (4096), which both exceed on one axis despite a tiny total pixel count (4,391,613 and 239,496 respectively — both far under the area a 4096x4096 sprite already implies).

**Decision:** `checkSpriteDimensions` now rejects a sprite only when `width * height` exceeds `SpritePixelDimensionLimit * SpritePixelDimensionLimit` (16,777,216), computed in `int64` to avoid overflow. Both axes are still implicitly bounded by the same total (a sprite cannot have one axis exceed the area limit on its own, since the other axis is always ≥ 1), but neither axis is checked independently anymore.

**Reason:** `SpritePixelDimensionLimit`'s own documented purpose is bounding the pixel buffer `ResolveSpritePixels` allocates from untrusted (WASM-boundary) input — a memory-safety guard, not a shape assumption. Total pixel area is exactly what determines that buffer's size; an area-based bound preserves the identical worst-case allocation ceiling while accepting real, legitimate sprite shapes a per-axis check has no principled reason to reject.

**Rejected alternatives:**
- **Raising the per-axis limit** (e.g. to 20000): rejected — would also raise the worst-case allocation ceiling for a square sprite (20000x20000 vastly exceeds 4096x4096's own footprint), a real regression in the guard's actual purpose, to accommodate a shape problem that isn't about raw axis size at all.
- **A separate, higher per-axis limit specifically for the smaller-of-two-axes case**: rejected as needless complexity — the area-based check already expresses exactly the real constraint (total buffer size) in one simple comparison.
