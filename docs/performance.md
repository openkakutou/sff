# Performance

Decode cost on large sprite sheets, measured for backlog item 009. Numbers are from one machine (Intel N97, linux/amd64, Go 1.26.1, Node 22.13.0) and only meaningful relative to each other.

## How to measure

Go benchmarks (time and allocations), on 2000-sprite synthetic v1/v2 sheets built in memory:

```sh
go test -run '^$' -bench . -benchmem .
```

`BenchmarkLoad_RealFile` runs `Load` on a real file of your choice and is skipped unless `SFF_BENCH_FILE` points at one:

```sh
SFF_BENCH_FILE=/path/to/sheet.sff go test -run '^$' -bench RealFile -benchmem .
```

WASM time-to-first-sprite (instantiate the module, `load` the sheet, `resolveSprites` its first sprite), the path a browser consumer takes:

```sh
GOOS=js GOARCH=wasm go build -o sff.wasm ./cmd/wasm
node cmd/wasm/bench.mjs sff.wasm /path/to/sheet.sff
```

No local sheet path is committed: both take it as an argument or environment variable.

## Change made: no per-sprite table-sized allocation in v1 linking

Resolving a v1 sprite's pixels used to allocate a visited-index map sized to the whole sprite table, once per sprite, so `Load` on a v1 sheet allocated memory quadratic in its sprite count. Each link step strictly decreases the sprite index, so the walk now needs no visited set (decision 019, amending decision 011's cycle guard). Linking rules and results are unchanged: a full scan of the local real-file corpus (562 files, 674,710 sprites) found 0 new failures.

| Benchmark (synthetic, 2000 sprites) | Before | After |
|---|---|---|
| `Load`, v1 | 71.6 ms, 149 MB, 26305 allocs | 2.37 ms, 1.31 MB, 8301 allocs |
| `Load`, v2 | 0.91 ms, 0.88 MB, 304 allocs | unchanged |
| `ResolveSpritePixels`, v1 | 1.67 ms, 443 KB, 2022 allocs | 1.62 ms, 369 KB, 2013 allocs |
| `ResolveSpritePixels`, v2 | 0.34 ms, 452 KB, 16 allocs | unchanged |

WASM time-to-first-sprite on real community character sheets:

| Sheet | Before | After |
|---|---|---|
| v1, 106.7 MiB, 4913 sprites | 5255 ms (load 5104 ms) | 4186 ms (load 3994 ms) |
| v1, 146.4 MiB, 4617 sprites | 5514 ms (load 5288 ms) | 5215 ms (load 4891 ms) |
| v2, 104.3 MiB, 3594 sprites | 385 ms (load 226 ms) | 388 ms (load 200 ms), unchanged within noise |

Native `Load` on the 146.4 MiB v1 sheet after the change: 1.57 s, 471 MB, 25403 allocs per call.

## Remaining cost: v1 `Load` decodes every image

A v1 sprite table carries no width/height, so `Load` fully decodes each sprite's PCX image only to read its size, and decodes the target again for every sprite linked to it. This now dominates v1 load time (about 4–5 s in WASM on the real sheets above, versus about 0.2 s for a v2 sheet of similar size). Reading the size from the PCX header alone would remove most of it, but would also stop `Load` from reporting a sprite whose pixel data is corrupt. That trade-off is backlog item 015; the linked-sprite re-decode is backlog item 014.
