All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Added

- Read and write support for MUGEN/Ikemen GO sprite sheet (`.sff`) files, versions 1 and 2, including color palette resolution and external `.act` palette overrides — migrated standalone from `character`, with no dependency on it.
- Sprites using the v2 RLE8 and LZ5 pixel compression formats can now be saved, not just loaded — completing round-trip support for both formats. The RLE5 format remains unsupported for saving (as for loading), since no real character file using it has ever been found.
