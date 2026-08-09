package sff

import "testing"

func TestSprite_ZeroValue_AllFieldsZero(t *testing.T) {
	var s Sprite

	if s.Group != 0 {
		t.Errorf("expected zero-value Sprite to have Group 0, got %d", s.Group)
	}
	if s.Image != 0 {
		t.Errorf("expected zero-value Sprite to have Image 0, got %d", s.Image)
	}
	if s.Width != 0 {
		t.Errorf("expected zero-value Sprite to have Width 0, got %d", s.Width)
	}
	if s.Height != 0 {
		t.Errorf("expected zero-value Sprite to have Height 0, got %d", s.Height)
	}
	if s.AxisX != 0 || s.AxisY != 0 {
		t.Errorf("expected zero-value Sprite to have AxisX 0 and AxisY 0, got AxisX %d AxisY %d", s.AxisX, s.AxisY)
	}
	if s.Palette != 0 {
		t.Errorf("expected zero-value Sprite to have Palette 0, got %d", s.Palette)
	}
}

func TestSpriteGroup_ZeroValue_HasNoSprites(t *testing.T) {
	var g SpriteGroup

	if g.Index != 0 {
		t.Errorf("expected zero-value SpriteGroup to have Index 0, got %d", g.Index)
	}
	if g.Sprites != nil {
		t.Errorf("expected zero-value SpriteGroup to have nil Sprites, got %v", g.Sprites)
	}
	if len(g.Sprites) != 0 {
		t.Errorf("expected zero-value SpriteGroup to have 0 sprites, got %d", len(g.Sprites))
	}

	// Ranging over a nil Sprites slice must not panic.
	for range g.Sprites {
		t.Fatal("expected no iterations over a nil Sprites slice")
	}
}

func TestSprite_WithValues_PreservesAssignedFieldValues(t *testing.T) {
	s := Sprite{
		Group:   9,
		Image:   3,
		Width:   64,
		Height:  128,
		AxisX:   -20,
		AxisY:   -110,
		Palette: 2,
	}

	if s.Group != 9 {
		t.Errorf("expected Group 9, got %d", s.Group)
	}
	if s.Image != 3 {
		t.Errorf("expected Image 3, got %d", s.Image)
	}
	if s.Width != 64 || s.Height != 128 {
		t.Errorf("expected Width 64 and Height 128, got Width %d Height %d", s.Width, s.Height)
	}
	if s.AxisX != -20 || s.AxisY != -110 {
		t.Errorf("expected AxisX -20 and AxisY -110, got AxisX %d AxisY %d", s.AxisX, s.AxisY)
	}
	if s.Palette != 2 {
		t.Errorf("expected Palette 2, got %d", s.Palette)
	}
}

func TestSpriteGroup_WithSprites_KeepsIndexAndSpritesInOrder(t *testing.T) {
	g := SpriteGroup{
		Index: 9,
		Sprites: []Sprite{
			{Group: 9, Image: 0, Width: 64, Height: 128, AxisX: -20, AxisY: -110, Palette: 0},
			{Group: 9, Image: 1, Width: 64, Height: 128, AxisX: -20, AxisY: -110, Palette: 0},
		},
	}

	if g.Index != 9 {
		t.Errorf("expected Index 9, got %d", g.Index)
	}
	if len(g.Sprites) != 2 {
		t.Fatalf("expected 2 sprites, got %d", len(g.Sprites))
	}
	if g.Sprites[0].Image != 0 {
		t.Errorf("expected first sprite Image 0, got %d", g.Sprites[0].Image)
	}
	if g.Sprites[1].Image != 1 {
		t.Errorf("expected second sprite Image 1, got %d", g.Sprites[1].Image)
	}
}

func TestSpriteGroup_SpritesFromDifferentGroup_AreNotRejectedByTheType(t *testing.T) {
	// SpriteGroup is a plain data container: it does not itself enforce
	// that every Sprite.Group matches its own Index. That invariant, if
	// desired, is a parsing/construction concern for a later package, not
	// part of this pure-data model.
	g := SpriteGroup{
		Index: 1,
		Sprites: []Sprite{
			{Group: 2, Image: 0},
		},
	}

	if len(g.Sprites) != 1 {
		t.Fatalf("expected 1 sprite, got %d", len(g.Sprites))
	}
	if g.Sprites[0].Group != 2 {
		t.Errorf("expected sprite Group 2 to be stored as-is, got %d", g.Sprites[0].Group)
	}
}
