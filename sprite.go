// Package sff defines the pure-data model for MUGEN/Ikemen GO sprite (.sff)
// files: Sprite and SpriteGroup.
//
// This is the read-path surface — the stable vocabulary a library consumer
// (editor, engine) works with. It carries no binary decoding, file I/O, or
// write-only (format-preservation) logic; per CLAUDE.md's read/write
// separation constraint, that lives elsewhere so importing this data model
// alone never pulls in write-only dependencies.
//
// The model is version-agnostic: MUGEN's .sff format has two on-disk
// versions (v1 and v2) with different header/table layouts and pixel
// encodings, but both describe the same logical sprite metadata. No
// v1/v2-specific field lives here — each version's parser (added
// separately) is responsible for populating this shared shape. Sprite pixel
// data itself is also out of scope here; only the metadata needed to
// reference and place a sprite is modeled.
package sff

// Sprite is a single MUGEN/Ikemen sprite's metadata: which group and image
// index identify it, its pixel dimensions, the offset from its top-left
// corner to its axis (pivot) point, and which palette it uses.
type Sprite struct {
	// Group is the sprite group index this sprite belongs to.
	Group int `json:"group"`
	// Image is this sprite's image index within its Group.
	Image int `json:"image"`
	// Width is the sprite's width in pixels.
	Width int `json:"width"`
	// Height is the sprite's height in pixels.
	Height int `json:"height"`
	// AxisX is the horizontal offset from the sprite's top-left corner to
	// its axis (pivot) point, used when positioning the sprite.
	AxisX int `json:"axisX"`
	// AxisY is the vertical offset from the sprite's top-left corner to
	// its axis (pivot) point, used when positioning the sprite.
	AxisY int `json:"axisY"`
	// Palette is a reference to the palette this sprite is drawn with. Its
	// exact meaning (e.g. a shared palette table index) is defined by the
	// .sff version that populates it.
	Palette int `json:"palette"`
}

// SpriteGroup is a collection of Sprites that share the same group index —
// e.g. the frames of a single stance or attack, addressed by their Image
// index within the group.
type SpriteGroup struct {
	// Index is the sprite group index shared by every Sprite in Sprites.
	Index int `json:"index"`
	// Sprites is the ordered collection of sprites belonging to this group.
	Sprites []Sprite `json:"sprites"`
}
