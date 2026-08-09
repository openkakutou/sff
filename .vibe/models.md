# Data models

## Sprite
| Field | Type | Notes |
|---|---|---|
| Group | int | Sprite group index this sprite belongs to |
| Image | int | Image index within its group |
| Width | int | Pixel width |
| Height | int | Pixel height |
| AxisX | int | Horizontal offset from top-left to axis (pivot) point |
| AxisY | int | Vertical offset from top-left to axis (pivot) point |
| Palette | int | Reference to the palette this sprite is drawn with; exact meaning is version-defined |
Defined in: `sprite.go`

## SpriteGroup
| Field | Type | Notes |
|---|---|---|
| Index | int | Sprite group index shared by every Sprite in Sprites |
| Sprites | []Sprite | Ordered collection of sprites in this group |
Defined in: `sprite.go`

## Palette
`[256]color.RGBA` — a resolved 256-entry color palette, shared between v1 and v2 once decoded. `EncodeV1Palette`/`EncodeV2Palette`/`EncodeExternalPalette` re-encode it back to each on-disk format (exact inverses of the corresponding `Decode*` functions). `(*Palette).SetColor(index, r, g, b, a int) error` is the validated way to edit a color from caller-supplied integers, rejecting an out-of-range index or component instead of silently wrapping.
Defined in: `palette.go`

## AlphaRule
Enum selecting how index 0 is treated when resolving indexed pixel data to RGBA: `AlphaForceTransparentAtIndexZero` (PCX/PNG8-decoded sprites) or `AlphaLiteral` (RLE8/LZ5-decoded sprites).
Defined in: `palette.go`

## PCXImage
Decoded v1 pixel buffer: index buffer + width/height, produced by `DecodePCX`/`ResolveV1Pixels`.
Defined in: `pcx.go`

## V2Image
Decoded v2 pixel buffer: pixel bytes + width/height + bytes-per-pixel (1 for indexed formats, 3/4 for direct-color PNG24/32), produced by `DecodeV2Sprite`.
Defined in: `v2_decoder.go`

## V1SpriteTable / V1Header / V1SpriteEntry
Low-level v1 read-path shapes as parsed directly from the file's binary layout (file offsets, on-disk sprite-linking/palette-sharing fields) — not the shared `Sprite` model, since a parser needs more than that model can hold.
Defined in: `v1.go`

## V2SpriteTable / V2Header / V2SpriteEntry / V2PaletteEntry
v2 equivalents of the above, plus a palette bank table (`V2PaletteEntry`) v1 has no equivalent of.
Defined in: `v2.go`

## V1WriteSprite / V2WriteSprite / V2WritePalette
Write-only input shapes for `SerializeV1`/`SerializeV2` — carry real bytes to embed (pixel data, palette bytes) rather than decoded/resolved data.
Defined in: `v1_serializer.go`, `v2_serializer.go`
