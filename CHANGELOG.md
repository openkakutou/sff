All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

## [0.2.0] - 2026-08-09

### Added

- Sprites using the v2 RLE5 pixel compression format can now be loaded and saved, ported directly from Ikemen GO's own reference decoder. Unlike every other supported format, RLE5 has not been validated against a real character file — none using it is known to exist yet — so this support should be treated as unproven until one turns up.

## [0.1.0] - 2026-08-09

### Added

- Read and write support for MUGEN/Ikemen GO sprite sheet (`.sff`) files, versions 1 and 2, including color palette resolution and external `.act` palette overrides — migrated standalone from `character`, with no dependency on it.
- Sprites using the v2 RLE8 and LZ5 pixel compression formats can now be saved, not just loaded — completing round-trip support for both formats. The RLE5 format remains unsupported for saving (as for loading), since no real character file using it has ever been found.
- Palette colors can now be edited and saved back for both v1 and v2 sprite files, and exported as a standalone `.act` palette file compatible with other MUGEN/Ikemen GO tools. Editing a color with an out-of-range value (outside 0-255) is rejected with a descriptive error instead of being silently corrupted.
- Web apps can now load and decode `.sff` sprite sheets directly in the browser, without a Go toolchain, via a WebAssembly build. Every tagged release automatically publishes the WebAssembly module alongside its matching browser glue file as downloadable assets.

[Unreleased]: https://github.com/openkakutou/sff/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/openkakutou/sff/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/openkakutou/sff/releases/tag/v0.1.0
