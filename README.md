# sff

A read/write Go library for MUGEN/Ikemen GO sprite sheet (`.sff`) files — v1 and v2 parsing, serialization, and palette resolution (including external `.act` palette overrides) — extracted out of [`character`](https://github.com/openkakutou/character) as a shared dependency of the [OpenKakutou](https://github.com/openkakutou) project. No rendering dependency; compiles to WebAssembly.

<!-- vibe:begin:features -->
- Reading and writing `.sff` v1 sprite sheets (header, sprite table, PCX pixel data, sprite linking, palette sharing)
- Reading and writing `.sff` v2 sprite sheets (sprite/palette tables, raw/RLE8/LZ5/PNG pixel formats, sprite and palette linking)
- Resolving a sprite's palette into actual on-screen colors, including recoloring with an external `.act` palette file
- Editing a palette's colors, with out-of-range values rejected instead of silently corrupted, and saving the edits back into either `.sff` file version or exporting them as a standalone `.act` file
- One-call resolution of a sprite's final on-screen pixels directly from a `.sff` file, by group/image reference
- Decoding/encoding of individual pixel formats used inside `.sff` files (PCX for v1, raw/RLE8/LZ5/PNG for v2)
- Validated against real, unmodified MUGEN/Ikemen GO community character files, not just hand-built test data

Planned:

- A WebAssembly build so web apps can decode sprites without a Go toolchain
<!-- vibe:end:features -->

<!-- vibe:begin:install -->
Requires [Go](https://go.dev/) 1.26 or later.

```sh
go get github.com/openkakutou/sff
```

Verify the install by importing the module in a Go file and running `go build`:

```go
import "github.com/openkakutou/sff"
```

To update to the latest version:

```sh
go get -u github.com/openkakutou/sff
```
<!-- vibe:end:install -->

<!-- vibe:begin:usage -->
Resolve a sprite's final on-screen pixels directly from a `.sff` file (version 1 or 2, auto-detected), given the sprite's `(group, image)` reference:

```go
package main

import (
	"fmt"
	"os"

	"github.com/openkakutou/sff"
)

func main() {
	f, err := os.Open("character.sff")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	// override is an optional external .act palette (nil uses the
	// sprite's own palette); pixels is a row-major []color.RGBA buffer.
	pixels, width, height, err := sff.ResolveSpritePixels(f, 0, 0, nil)
	if err != nil {
		panic(err)
	}
	fmt.Printf("decoded %dx%d sprite, %d pixels\n", width, height, len(pixels))
}
```

Lower-level entry points are also available for callers that need more control: `Load` (all sprite groups in a file, version-agnostic), `ParseV1`/`ParseV2` plus their pixel/palette decoders, and `SerializeV1`/`SerializeV2` for writing files back out.
<!-- vibe:end:usage -->

<!-- vibe:begin:docs-index -->
- [docs/api.md](docs/api.md) — public functions and types, organized by parsing, pixel decoding/encoding, palette resolution, and file writing
- [docs/architecture.md](docs/architecture.md) — how the read path, write path, and shared data model fit together internally
- [docs/testing.md](docs/testing.md) — test strategy, how to run the suite, and how the real-file fixture corpus works
<!-- vibe:end:docs-index -->
