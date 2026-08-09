# Ubiquitous Language

## Sprite
A single image frame in a `.sff` sprite sheet: pixel dimensions, an axis (pivot) point for positioning, and a reference to the palette it is drawn with. Identified within a file by its `(group, image)` pair.
_Sources: `sprite.go`, `resolve_sprite.go`_

## Sprite group
A collection of sprites sharing the same group index — typically the frames of one stance or attack, addressed by their image index within the group.
_Sources: `sprite.go`, `load.go`_

## Axis (pivot) point
The offset from a sprite's top-left corner to the point used to position it on screen — e.g. where a character's feet or a hitbox's origin aligns.
**Do not confuse with:** the palette bank/table index also called "palette", an unrelated per-sprite reference.
_Sources: `sprite.go`_

## Palette
A 256-entry table of on-screen colors a sprite's indexed pixel data is resolved against. A `.sff` file's own palette can be entirely replaced by an external `.act` file at resolution time (see External palette override).
_Sources: `palette.go`_

## Palette sharing
A sprite that carries no palette of its own and instead reuses another sprite's already-resolved palette, rather than storing a duplicate 256-color block on disk — a real-file space-saving convention this package's resolution logic follows (both v1's explicit shared-palette flag and v2's linkable palette banks).
_Sources: `palette.go`, `v1.go`, `v2.go`_

## External palette override
Recoloring a sprite by substituting its resolved on-disk palette with one decoded from a standalone `.act` file, instead of the palette embedded in or referenced by the `.sff` file itself.
_Sources: `palette.go`, `resolve_sprite.go`_

## Sprite linking
A sprite entry that carries no pixel data of its own (zero declared length) and instead reuses an earlier sprite's already-decoded pixel data — another real-file space-saving convention, resolved positionally (the immediately preceding table entry) rather than always trusting the on-disk link field, per this package's fixture-driven corrections.
_Sources: `load.go`, `v1.go`, `v2_decoder.go`_

## Pixel encoding format
The on-disk compression/encoding scheme a sprite's pixel data uses. v1 sprites use PCX (run-length-encoded indexed color). v2 sprites use one of: raw (uncompressed indexed), RLE8 (run-length-encoded indexed), LZ5 (dictionary-compressed indexed), or PNG8/24/32 (indexed or direct-color, via standard PNG).
_Sources: `pcx.go`, `v2_decoder.go`_
