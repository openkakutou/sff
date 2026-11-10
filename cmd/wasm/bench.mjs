#!/usr/bin/env node
// bench.mjs measures the WASM build's time-to-first-sprite on a real .sff
// file, the way a browser consumer would reach it: instantiate the module,
// load the sheet, then resolve its first sprite. Like smoke.mjs, it is not
// part of `go test`; numbers are recorded by hand in docs/performance.md.
//
// Usage: node cmd/wasm/bench.mjs path/to/sff.wasm path/to/sheet.sff
// The sheet path is always an argument: a local corpus path is never
// committed (see CLAUDE.md).

import { execSync } from "node:child_process";
import { readFileSync } from "node:fs";
import path from "node:path";
import { performance } from "node:perf_hooks";

const [wasmArg, sheetArg] = process.argv.slice(2);
if (!wasmArg || !sheetArg) {
	console.error("usage: node cmd/wasm/bench.mjs path/to/sff.wasm path/to/sheet.sff");
	process.exit(2);
}

const goroot = execSync("go env GOROOT").toString().trim();
await import(`file://${path.join(goroot, "lib", "wasm", "wasm_exec.js")}`);

const sheet = new Uint8Array(readFileSync(path.resolve(sheetArg)));

const t0 = performance.now();
const go = new globalThis.Go();
const { instance } = await WebAssembly.instantiate(readFileSync(path.resolve(wasmArg)), go.importObject);
go.run(instance); // does not return: keeps the Go runtime alive
const t1 = performance.now();

const loaded = globalThis.OpenKakutouSff.load(sheet);
const t2 = performance.now();
if (loaded.error !== null) {
	console.error(`load failed: ${loaded.error}`);
	process.exit(1);
}

const groups = JSON.parse(loaded.spriteGroups);
const spriteCount = groups.reduce((n, g) => n + g.sprites.length, 0);
if (spriteCount === 0) {
	console.error(`no sprites in ${sheetArg}`);
	process.exit(1);
}
const first = groups[0].sprites[0];
const [resolved] = globalThis.OpenKakutouSff.resolveSprites(sheet, [[first.group, first.image]], null);
const t3 = performance.now();
if (resolved.error !== null) {
	console.error(`resolveSprites failed: ${resolved.error}`);
	process.exit(1);
}

const ms = (a, b) => (b - a).toFixed(1);
console.log(`sheet: ${(sheet.length / 1048576).toFixed(1)} MiB, ${spriteCount} sprites`);
console.log(`instantiate: ${ms(t0, t1)} ms`);
console.log(`load: ${ms(t1, t2)} ms`);
console.log(`first sprite (${first.group},${first.image}) ${resolved.width}x${resolved.height}: ${ms(t2, t3)} ms`);
console.log(`time-to-first-sprite: ${ms(t0, t3)} ms`);
process.exit(0);
