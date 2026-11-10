---
date: 2026-11-10
status: accepted
---
# v1 pixel-link resolution walks the chain without a visited set

**Context:** Backlog item 009 measured `Load` on large v1 sheets. `resolveV1Pixels` allocated a visited-index map sized to the whole sprite table on every call, so `Load` allocated memory quadratic in the sprite count (149 MB for a 2000-sprite sheet, see `docs/performance.md`). Decision 011's third bullet kept that map "for defense in depth".

**Decision:** This amends only the third bullet of decision 011, for pixel linking. `resolveV1Pixels` is an iterative walk with no visited set. Every step moves to a strictly smaller index (`i-1` for a zero-length sprite, a `LinkedIndex` strictly below `i` otherwise), so the walk always ends, and a cycle cannot be built. The linking rules themselves are unchanged. Palette inheritance is not affected.

**Reason:** Termination follows from the rules, so the map guarded a case that cannot occur. Removing it took v1 `Load` on a 2000-sprite sheet from 71.6 ms and 149 MB to 2.37 ms and 1.31 MB. A full scan of the local real-file corpus (562 files, 674,710 sprites) gave 0 new failures.

**Rejected alternatives:**
- *Keep the map:* it is pure cost for a case that cannot happen.
- *Replace it with a step counter capped at the table size:* still unreachable, and it adds a branch with no test that can trigger it.
