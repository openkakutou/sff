All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

## [0.9.0] - 2026-11-21

### Added
- `ResolveCharacterSpritePixels`: a character's sprite with its selected .act palette, as Ikemen GO draws it: in an SFF v1 sheet the .act replaces the first palette (sprite 0's and every sprite sharing it), sprites with their own palette keep theirs.

## [0.8.1] - 2026-11-17

### Fixed

- External palettes saved by Photoshop (`.act` files of 772 bytes, used by several real characters) are now accepted instead of rejected; they load with the same colors as in Ikemen GO.

## [0.8.0] - 2026-11-16

### Added

- Classic (v1) sheets containing truecolor sprites now load completely: those sprites appear in their real colors, with their solid background color made transparent, instead of making the whole sheet fail to load.

## [0.7.0] - 2026-11-15

### Added

- Editors can now add a sprite to a sheet, replace a sprite's image, or remove a sprite, for both classic (v1) and newer (v2) sheets; removing a sprite that other sprites reuse keeps those sprites looking exactly the same, and an unused v2 palette can be removed too.

## [0.6.0] - 2026-11-14

### Added

- Palettes can now be imported from and exported to JASC .pal files (in standard order, or in the reversed order some community files use) and indexed PNG images, in addition to .act files; damaged or unsupported files are refused with a clear error.

## [0.5.2] - 2026-11-12

### Fixed

- Classic (v1) sprite sheets in which many sprites reuse an earlier sprite's image no longer take a very long time to load: each image is now read once instead of once per sprite reusing it, so a crafted sheet of 100,000 such sprites loads in a few hundredths of a second instead of about 18 seconds.

## [0.5.1] - 2026-11-11

### Fixed

- Classic (v1) sprite sheets now report the right palette for every sprite: sprites that borrow another sprite's palette without being flagged as such, and flagged sprites of the first group and image, were numbered with a different palette than the one they are drawn with, which grouped sprites by palette wrongly.

## [0.5.0] - 2026-11-10

### Added

- Large classic (v1) sprite sheets now load with far less memory and noticeably faster — up to 30 times faster on a sheet of 2,000 sprites, and up to a second sooner in the browser on real 100 MB character sheets — with exactly the same sprites as before. Load and display costs are now measured and documented, with commands to reproduce them.

## [0.4.0] - 2026-11-10

### Added

- Corrupt or malicious sprite files can no longer make an app exhaust its memory: every declared image size, data length and sprite count is now checked against a safe limit and rejected with a clear error, while every real character file keeps loading as before. The readers are now covered by automated fuzz testing.

## [0.3.1] - 2026-09-02

### Fixed

- Fixed two more real-file decoding bugs found by the large real-file compatibility scan: a character sheet's last sprite could fail to load entirely when it reused an earlier sprite's palette, and legitimate extreme-aspect-ratio sprites (very wide and short, or very tall and narrow) were rejected as implausibly large. The handful of remaining, individually-confirmed corrupt or unsupported real files are now clearly and permanently documented rather than silently retried.

## [0.3.0] - 2026-08-24

### Added

- Decoding is now validated against a large corpus of real MUGEN/Ikemen GO character sprite sheets (99.995% of attempted sprites decode correctly), and two real-file decoding bugs this uncovered are fixed: certain sprites that reuse an earlier sprite's palette no longer fail to load.

## [0.2.0] - 2026-08-09

### Added

- Sprites using the v2 RLE5 pixel compression format can now be loaded and saved, ported directly from Ikemen GO's own reference decoder. Unlike every other supported format, RLE5 has not been validated against a real character file — none using it is known to exist yet — so this support should be treated as unproven until one turns up.

## [0.1.0] - 2026-08-09

### Added

- Read and write support for MUGEN/Ikemen GO sprite sheet (`.sff`) files, versions 1 and 2, including color palette resolution and external `.act` palette overrides — migrated standalone from `character`, with no dependency on it.
- Sprites using the v2 RLE8 and LZ5 pixel compression formats can now be saved, not just loaded — completing round-trip support for both formats. The RLE5 format remains unsupported for saving (as for loading), since no real character file using it has ever been found.
- Palette colors can now be edited and saved back for both v1 and v2 sprite files, and exported as a standalone `.act` palette file compatible with other MUGEN/Ikemen GO tools. Editing a color with an out-of-range value (outside 0-255) is rejected with a descriptive error instead of being silently corrupted.
- Web apps can now load and decode `.sff` sprite sheets directly in the browser, without a Go toolchain, via a WebAssembly build. Every tagged release automatically publishes the WebAssembly module alongside its matching browser glue file as downloadable assets.

[Unreleased]: https://github.com/openkakutou/sff/compare/v0.8.1...HEAD
[0.8.1]: https://github.com/openkakutou/sff/compare/v0.8.0...v0.8.1
[0.8.0]: https://github.com/openkakutou/sff/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/openkakutou/sff/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/openkakutou/sff/compare/v0.5.2...v0.6.0
[0.5.2]: https://github.com/openkakutou/sff/compare/v0.5.1...v0.5.2
[0.5.1]: https://github.com/openkakutou/sff/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/openkakutou/sff/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/openkakutou/sff/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/openkakutou/sff/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/openkakutou/sff/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/openkakutou/sff/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/openkakutou/sff/releases/tag/v0.1.0
