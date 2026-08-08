# sff

A read/write Go library for MUGEN/Ikemen GO sprite sheet (`.sff`) files — v1 and v2 parsing, serialization, and palette resolution (including external `.act` palette overrides) — extracted out of [`character`](https://github.com/openkakutou/character) as a shared dependency of the [OpenKakutou](https://github.com/openkakutou) project. No rendering dependency; compiles to WebAssembly.

<!-- vibe:begin:features -->
This project is in early-stage development — no functionality yet, see the roadmap's decision `007-extract-sff-into-a-standalone-shared-repo` for the extraction scope.

Planned:

- Reading and writing `.sff` v1 sprite sheets (header, sprite table, PCX pixel data, sprite linking, palette sharing)
- Reading and writing `.sff` v2 sprite sheets (sprite/palette tables, raw/RLE8/LZ5/PNG pixel formats, sprite and palette linking)
- Resolving a sprite's palette into actual on-screen colors, including recoloring with an external `.act` palette file
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
No functionality is implemented yet — the parsing/serialization/palette API will be documented here as it lands. For now the package only exposes its version:

```go
package main

import (
	"fmt"

	"github.com/openkakutou/sff"
)

func main() {
	fmt.Println(sff.Version)
}
```
<!-- vibe:end:usage -->

<!-- vibe:begin:docs-index -->
No additional documentation yet.
<!-- vibe:end:docs-index -->
