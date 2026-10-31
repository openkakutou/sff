---
status: todo
---
# Sprite add, replace and remove with link repair

## Description
Editors manipulate sprite sheets sprite by sprite. Provide a core API to add a sprite, replace its pixels, and remove it, repairing links (linked sprites pointing at a removed one) and palette sharing, for v1 and v2.

## Acceptance Criteria
- [ ] Add/replace/remove for both versions with tests
- [ ] Removing a linked-to sprite relinks or reports its dependents
- [ ] Result re-parses and decodes identically elsewhere
