package sff

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sffCorpusDirEnv names the environment variable a caller sets to point at
// a local, machine-specific corpus of real .sff files to scan — see
// .vibe/fixture-sources.md's "Local real-character corpus" section. Never
// hardcode that path itself here: the corpus scan below is entirely
// skipped when this variable is unset, so this package's normal test run
// (CI included) never depends on it.
const sffCorpusDirEnv = "SFF_CORPUS_DIR"

// v2LinkedSpriteGapName labels this package's one already-documented,
// accepted v2 decode gap (see resolveSpritePixelsV2's own doc comment in
// resolve_sprite.go): a v2 sprite that shares rather than owns its pixel
// data has no validated decode path yet. scanFileSprites classifies a
// zero-Length v2 entry as this gap rather than a failure needing triage.
const v2LinkedSpriteGapName = "v2 linked/shared sprite pixel data"

// scanFileSprites parses f (path is used only for error messages) once and
// attempts to decode every sprite it declares, mirroring the same
// decode-then-resolve-palette logic ResolveSpritePixels applies per
// sprite. Unlike calling that self-contained, one-sprite-at-a-time
// function in a loop — which would re-parse the same sprite table from
// scratch on every call, prohibitively slow for a real file with
// thousands of sprites — this parses the table exactly once and reuses it
// for every entry.
//
// total is every sprite declared in the file; knownGaps counts entries
// that hit v2LinkedSpriteGapName; failures describes every other decode
// error, one line per sprite.
func scanFileSprites(f io.ReaderAt, path string) (total int, knownGaps map[string]int, failures []string, err error) {
	isV2, err := detectVersion(f)
	if err != nil {
		return 0, nil, nil, err
	}
	knownGaps = make(map[string]int)

	if isV2 {
		table, err := ParseV2(f)
		if err != nil {
			return 0, nil, nil, err
		}
		for _, e := range table.Sprites {
			total++
			if e.Length == 0 {
				knownGaps[v2LinkedSpriteGapName]++
				continue
			}
			if err := decodeV2SpriteEntry(table, f, e); err != nil {
				failures = append(failures, fmt.Sprintf("%s (group %d, image %d): %v", path, e.Group, e.Image, err))
			}
		}
		return total, knownGaps, failures, nil
	}

	table, err := ParseV1(f)
	if err != nil {
		return 0, nil, nil, err
	}
	for i, e := range table.Sprites {
		total++
		if _, err := resolveV1Pixels(table, f, i, nil); err != nil {
			failures = append(failures, fmt.Sprintf("%s (group %d, image %d): %v", path, e.Group, e.Image, err))
			continue
		}
		if _, err := ResolveV1Palette(table, f, i, nil); err != nil {
			failures = append(failures, fmt.Sprintf("%s (group %d, image %d): palette: %v", path, e.Group, e.Image, err))
		}
	}
	return total, knownGaps, failures, nil
}

// decodeV2SpriteEntry decodes one already-parsed v2 sprite table entry's
// pixel data and, for an indexed (palette-based) format, resolves its
// palette too — the same two steps resolveSpritePixelsV2 performs for a
// single (group, image) lookup, applied directly to an entry from a table
// already in hand instead of re-parsing to find it.
func decodeV2SpriteEntry(table *V2SpriteTable, r io.ReaderAt, e V2SpriteEntry) error {
	if err := checkSpriteDimensions(e.Width, e.Height); err != nil {
		return err
	}
	raw := make([]byte, e.Length)
	if _, err := r.ReadAt(raw, e.Offset); err != nil {
		return fmt.Errorf("reading sprite pixel data: %w", err)
	}
	img, err := DecodeV2Sprite(e.Format, e.Width, e.Height, e.ColorDepth, raw)
	if err != nil {
		return fmt.Errorf("decoding sprite pixel data: %w", err)
	}
	if img.BytesPerPixel != 1 {
		return nil // direct-color (PNG24/PNG32): no palette to resolve
	}
	if _, err := ResolveV2Palette(table, r, e.PaletteIndex, nil); err != nil {
		return fmt.Errorf("resolving sprite palette: %w", err)
	}
	return nil
}

func TestScanFileSprites_V1Fixture_DecodesEverySpriteWithNoFailures(t *testing.T) {
	f := openTestdataFile(t, "v1-basic.sff")
	defer f.Close()

	total, knownGaps, failures, err := scanFileSprites(f, "v1-basic.sff")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total == 0 {
		t.Fatal("expected at least one sprite to be scanned")
	}
	if len(failures) != 0 {
		t.Errorf("expected no decode failures, got %v", failures)
	}
	if len(knownGaps) != 0 {
		t.Errorf("expected no known-gap entries for a v1 file (that gap is v2-only), got %v", knownGaps)
	}
}

func TestScanFileSprites_V2LinkedSpriteFixture_ClassifiesItAsAKnownGap(t *testing.T) {
	f := openTestdataFile(t, "v2-zero-length-copy.sff")
	defer f.Close()

	total, knownGaps, failures, err := scanFileSprites(f, "v2-zero-length-copy.sff")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total == 0 {
		t.Fatal("expected at least one sprite to be scanned")
	}
	if len(failures) != 0 {
		t.Errorf("expected the linked sprite to be classified as a known gap, not a failure, got %v", failures)
	}
	if knownGaps[v2LinkedSpriteGapName] != 1 {
		t.Errorf("expected exactly 1 sprite classified under %q, got %d (all gaps: %v)", v2LinkedSpriteGapName, knownGaps[v2LinkedSpriteGapName], knownGaps)
	}
}

func TestScanFileSprites_MalformedFile_ReturnsError(t *testing.T) {
	f := openTestdataFile(t, "v1-invalid-size.sff")
	defer f.Close()

	// v1-invalid-size.sff parses successfully as a table (its malformed
	// field is a sprite's declared size, not the table itself) but must
	// still surface as an error from scanFileSprites: exercised via
	// resolveV1Pixels's v1MaxPixelCount guard producing a synthetic 1x1
	// fallback rather than an error — so this fixture alone doesn't force
	// the file-level error path. Use a corrupt/truncated reader instead to
	// exercise it directly.
	_, _, _, err := scanFileSprites(&truncatedReaderAt{}, "truncated")
	if err == nil {
		t.Fatal("expected an error for a file whose table cannot even be parsed")
	}
}

// truncatedReaderAt is an io.ReaderAt that always reports a short read,
// used to force ParseV1/ParseV2/detectVersion's own error path without
// needing a dedicated on-disk fixture for it.
type truncatedReaderAt struct{}

func (truncatedReaderAt) ReadAt(_ []byte, _ int64) (int, error) {
	return 0, fmt.Errorf("simulated truncated read")
}

// TestCorpusCompat_RealSFFFiles_DecodeSuccessRate is the fixture-driven
// compatibility scan backlog item 005 asks for: every real, unmodified
// .sff file under SFF_CORPUS_DIR is loaded and every sprite it declares is
// decoded through the same decode+palette-resolve logic a consumer
// (character, stage, lifebar-editor) relies on. Any failure that isn't
// the one already-documented v2LinkedSpriteGapName fails the test loudly
// instead of being silently ignored, so a corpus file that hits an
// undocumented gap is caught here rather than shipping quietly broken.
//
// Skipped by default: this depends on a local, machine-specific corpus
// (see .vibe/fixture-sources.md) that is never vendored into this repo or
// available in CI.
func TestCorpusCompat_RealSFFFiles_DecodeSuccessRate(t *testing.T) {
	corpusDir := os.Getenv(sffCorpusDirEnv)
	if corpusDir == "" {
		t.Skipf("%s not set — skipping real-file corpus compatibility scan (see .vibe/fixture-sources.md)", sffCorpusDirEnv)
	}

	var files []string
	err := filepath.WalkDir(corpusDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(path), ".sff") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking corpus directory %q: %v", corpusDir, err)
	}
	if len(files) == 0 {
		t.Fatalf("no .sff files found under %q", corpusDir)
	}

	var totalSprites, decoded int
	knownGapCounts := make(map[string]int)
	var loadFailures []string
	var decodeFailures []string

	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			loadFailures = append(loadFailures, fmt.Sprintf("%s: opening: %v", path, err))
			continue
		}

		n, fileGapCounts, fileFailures, err := scanFileSprites(f, path)
		f.Close()
		if err != nil {
			loadFailures = append(loadFailures, fmt.Sprintf("%s: %v", path, err))
			continue
		}
		totalSprites += n
		gaps := 0
		for gap, c := range fileGapCounts {
			knownGapCounts[gap] += c
			gaps += c
		}
		decoded += n - len(fileFailures) - gaps
		decodeFailures = append(decodeFailures, fileFailures...)
	}

	successRate := 0.0
	if totalSprites > 0 {
		successRate = 100 * float64(decoded) / float64(totalSprites)
	}
	t.Logf("corpus scan: %d files under %s, %d sprites — %d decoded (%.1f%%), %d failures", len(files), corpusDir, totalSprites, decoded, successRate, len(decodeFailures))
	for gap, n := range knownGapCounts {
		t.Logf("  %d sprite(s) hit the known, already-documented gap %q", n, gap)
	}

	if len(loadFailures) > 0 {
		max := len(loadFailures)
		if max > 20 {
			max = 20
		}
		t.Errorf("%d file(s) failed to load:\n%s", len(loadFailures), strings.Join(loadFailures[:max], "\n"))
	}
	if len(decodeFailures) > 0 {
		max := len(decodeFailures)
		if max > 20 {
			max = 20
		}
		t.Errorf("%d sprite(s) failed to decode with an undocumented error (showing up to %d):\n%s", len(decodeFailures), max, strings.Join(decodeFailures[:max], "\n"))
	}
}
