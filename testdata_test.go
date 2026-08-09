package sff

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestTestdataFixtures_V1FilesParseAndExposeExpectedGroup is a smoke test
// for the trimmed real fixtures under testdata/files (see
// .vibe/backlog/done/023-vendor-real-sff-test-fixtures.md and
// testdata/README.md): each one must still parse as a valid .sff v1 file
// and expose a sprite in the group the fixture was built for. It does not
// assert decoded pixel content — that belongs to the fixture-driven
// scenario tests added later (item 028).
func TestTestdataFixtures_V1FilesParseAndExposeExpectedGroup(t *testing.T) {
	cases := []struct {
		file  string
		group int
	}{
		{"v1-basic.sff", 0},
		{"v1-last-sprite.sff", 10302},
		{"v1-zero-length-copy.sff", 0},
		{"v1-self-link.sff", 5040},
		{"v1-forward-link.sff", 5072},
		{"v1-invalid-size.sff", 10017},
		{"v1-external-palette-source.sff", 0},
		{"v1-same-palette-a.sff", 0},
		{"v1-same-palette-a-group5.sff", 5},
		{"v1-same-palette-b.sff", 0},
	}

	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			f := openTestdataFile(t, c.file)
			defer f.Close()

			table, err := ParseV1(f)
			if err != nil {
				t.Fatalf("ParseV1(%s): %v", c.file, err)
			}

			found := false
			for _, s := range table.Sprites {
				if s.Group == c.group {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s: no sprite in group %d", c.file, c.group)
			}
		})
	}
}

// TestTestdataFixtures_V2FilesParseAndExposeExpectedGroup mirrors
// TestTestdataFixtures_V1FilesParseAndExposeExpectedGroup for the v2
// fixtures.
func TestTestdataFixtures_V2FilesParseAndExposeExpectedGroup(t *testing.T) {
	cases := []struct {
		file  string
		group int
	}{
		{"v2-rle8.sff", 9000},
		{"v2-lz5.sff", 0},
		{"v2-png8.sff", 0},
		{"v2-png24.sff", 6053},
		{"v2-png32.sff", 9000},
		{"v2-zero-length-copy.sff", 186},
		{"v2-png8-forced-alpha.sff", 9000},
		{"v2-external-palette-source.sff", 0},
		{"v2-empty-palette-use-first.sff", 0},
		{"v2-empty-palette-use-previous.sff", 0},
		{"v2-empty-palette-use-previous2.sff", 0},
		{"v2-png8-b.sff", 9000},
		{"v2-loadmode1.sff", 3096},
	}

	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			f := openTestdataFile(t, c.file)
			defer f.Close()

			table, err := ParseV2(f)
			if err != nil {
				t.Fatalf("ParseV2(%s): %v", c.file, err)
			}

			found := false
			for _, s := range table.Sprites {
				if s.Group == c.group {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s: no sprite in group %d", c.file, c.group)
			}
		})
	}
}

// TestTestdataFixtures_ExpectedPNGsArePresent checks that every expected
// output PNG referenced by the future fixture-driven test suites (items
// 028/029) was vendored unmodified from the reference project.
func TestTestdataFixtures_ExpectedPNGsArePresent(t *testing.T) {
	var names []string
	for _, n := range []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10} {
		names = append(names, fmt.Sprintf("v1-sprite-%03d.png", n))
	}
	for _, n := range []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14} {
		names = append(names, fmt.Sprintf("v2-sprite-%03d.png", n))
	}

	for _, name := range names {
		path := filepath.Join("testdata", "sprites", name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected sprite PNG missing: %s: %v", path, err)
		}
	}
}

func openTestdataFile(t *testing.T, name string) *os.File {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", "files", name))
	if err != nil {
		t.Fatalf("opening testdata fixture %s: %v", name, err)
	}
	return f
}
