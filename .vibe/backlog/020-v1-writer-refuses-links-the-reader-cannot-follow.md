---
status: todo
---
# The v1 writer refuses links the reader cannot follow

## Description
Found by the audit of 2026-11-21 (medium). For a sprite with no pixel data, `SerializeV1` (`v1_serializer.go:79`) and `AddV1Sprite` (`v1_edit.go:114`) only check that `LinkedIndex` is in range. The reader (decision 011) gives a zero-length shared sprite the previous entry's pixels whatever its link, and reads a link of 0 or a forward link as no link. A written file then reads back other pixels, or fails with "pcx data too short".

## Acceptance Criteria
- [ ] Writing a link the reader would read differently is refused with a descriptive error, or the target's pixels are written as a copy (as `RemoveV1Sprite` does)
- [ ] Write → read returns the requested pixels for every accepted link
