#!/usr/bin/env node
// smoke.mjs is a Node.js verification harness for the WASM entrypoint built
// from this directory (see main.go) — it exercises the module the same way
// a browser consumer would (fetch/instantiate the .wasm, call the exposed
// global functions, read back the result), without requiring an actual
// browser. It is not part of `go test` — syscall/js glue cannot run under
// the plain Go toolchain — and doubles as a minimal usage example for a JS
// consumer.
//
// Usage: node cmd/wasm/smoke.mjs [path/to/sff.wasm]
// (defaults to ./sff.wasm, relative to the repo root)

import { execSync } from "node:child_process";
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..", "..");
const wasmPath = path.resolve(process.argv[2] || path.join(repoRoot, "sff.wasm"));

const goroot = execSync("go env GOROOT").toString().trim();
const wasmExecPath = path.join(goroot, "lib", "wasm", "wasm_exec.js");

// wasm_exec.js defines a global `Go` constructor; importing it for its
// side effect is the same pattern used to load it in a browser <script> tag.
await import(`file://${wasmExecPath}`);

const go = new globalThis.Go();
const { instance } = await WebAssembly.instantiate(readFileSync(wasmPath), go.importObject);
go.run(instance); // does not return: keeps the Go runtime (and its registered functions) alive

function toUint8Array(relativePath) {
	return new Uint8Array(readFileSync(path.join(repoRoot, relativePath)));
}

function assert(condition, message) {
	if (!condition) {
		console.error(`FAIL: ${message}`);
		process.exitCode = 1;
	} else {
		console.log(`ok - ${message}`);
	}
}

// testdata/files/v1-basic.sff carries exactly one real sprite, at
// (group 0, image 0) — see testdata_test.go/resolve_sprite_test.go.
const sffBytes = toUint8Array("testdata/files/v1-basic.sff");

// --- load: nominal path ---
const okResult = globalThis.OpenKakutouSff.load(sffBytes);
assert(okResult.error === null, `load: nominal reports no error (got: ${okResult.error})`);
assert(typeof okResult.spriteGroups === "string", "load: nominal returns a spriteGroups JSON string");

const spriteGroups = JSON.parse(okResult.spriteGroups ?? "null");
assert(Array.isArray(spriteGroups) && spriteGroups.length === 1, `load: v1-basic.sff has 1 sprite group (got: ${spriteGroups?.length})`);
assert(spriteGroups[0]?.index === 0, `load: the group's index is 0 (got: ${spriteGroups[0]?.index})`);
assert(Array.isArray(spriteGroups[0]?.sprites) && spriteGroups[0].sprites.length === 1, "load: the group has 1 sprite");
assert(spriteGroups[0].sprites[0]?.group === 0 && spriteGroups[0].sprites[0]?.image === 0, "load: the sprite is (group 0, image 0)");
assert(spriteGroups[0].sprites[0]?.width > 0 && spriteGroups[0].sprites[0]?.height > 0, "load: the sprite reports positive dimensions");

// --- load: error path, malformed bytes ---
// Padded to at least 16 bytes (the signature+version peek size Load reads
// before it can even check the signature) so this actually exercises the
// "not a .sff file" branch, not a premature EOF on a too-short buffer.
const loadErrResult = globalThis.OpenKakutouSff.load(new TextEncoder().encode("garbage, not a real sff file"));
assert(loadErrResult.spriteGroups === null, "load: malformed bytes: spriteGroups is null");
assert(typeof loadErrResult.error === "string" && loadErrResult.error.length > 0, `load: malformed bytes: error is a non-empty string (got: ${loadErrResult.error})`);
assert(loadErrResult.error.includes("not a .sff file"), `load: malformed bytes: error identifies an unrecognized file (got: ${loadErrResult.error})`);

// --- load: error path, wrong argument count, must not crash the module ---
const loadArgCountResult = globalThis.OpenKakutouSff.load();
assert(loadArgCountResult.spriteGroups === null, "load: missing arguments: spriteGroups is null");
assert(typeof loadArgCountResult.error === "string" && loadArgCountResult.error.length > 0, "load: missing arguments: error is a non-empty string");

// The module must still respond correctly after an error — proves the
// earlier failures didn't leave the Go runtime in a broken state.
const afterLoadErrorResult = globalThis.OpenKakutouSff.load(sffBytes);
assert(afterLoadErrorResult.error === null, "load: module still works after a prior error");

// --- resolveSprites: nominal batch: one real sprite, one nonexistent (group, image) ---
const spritesResult = globalThis.OpenKakutouSff.resolveSprites(sffBytes, [[0, 0], [999, 999]], null);
assert(Array.isArray(spritesResult) && spritesResult.length === 2, "resolveSprites: returns one result per request");

const [found, notFound] = spritesResult;
assert(found.error === null, `resolveSprites: real sprite reports no error (got: ${found.error})`);
assert(found.pixels instanceof Uint8Array, "resolveSprites: real sprite returns a pixel buffer");
assert(found.pixels.length === found.width * found.height * 4, "resolveSprites: pixel buffer length is width*height*4 (RGBA)");
assert(found.width > 0 && found.height > 0, `resolveSprites: real sprite has positive dimensions (got: ${found.width}x${found.height})`);

assert(notFound.pixels === null, "resolveSprites: nonexistent sprite returns null pixels");
assert(notFound.width === 0 && notFound.height === 0, "resolveSprites: nonexistent sprite reports 0x0 dimensions");
assert(typeof notFound.error === "string" && notFound.error.startsWith("sprite not found: "), `resolveSprites: nonexistent sprite error is distinguishable (got: ${notFound.error})`);

// --- external palette override recolors the sprite ---
const actBytes = toUint8Array("testdata/files/cyclops-v1-palette1.act");
const overriddenResult = globalThis.OpenKakutouSff.resolveSprites(sffBytes, [[0, 0]], actBytes);
assert(overriddenResult[0].error === null, `resolveSprites: override reports no error (got: ${overriddenResult[0].error})`);
const differs = overriddenResult[0].pixels.some((b, i) => b !== found.pixels[i]);
assert(differs, "resolveSprites: external palette override changes the resolved colors");

// --- undefined and null overrideBytes are equivalent to "no override" ---
const undefinedOverrideResult = globalThis.OpenKakutouSff.resolveSprites(sffBytes, [[0, 0]], undefined);
const nullOverrideResult = globalThis.OpenKakutouSff.resolveSprites(sffBytes, [[0, 0]], null);
assert(
	undefinedOverrideResult[0].pixels.every((b, i) => b === nullOverrideResult[0].pixels[i]),
	"resolveSprites: undefined and null overrideBytes produce identical output",
);
assert(
	nullOverrideResult[0].pixels.every((b, i) => b === found.pixels[i]),
	"resolveSprites: no override matches the sprite's own palette",
);

// --- an explicitly empty overrideBytes is an error, not a silent fallback ---
const emptyOverrideResult = globalThis.OpenKakutouSff.resolveSprites(sffBytes, [[0, 0]], new Uint8Array(0));
assert(emptyOverrideResult[0].pixels === null, "resolveSprites: empty overrideBytes returns null pixels");
assert(typeof emptyOverrideResult[0].error === "string" && emptyOverrideResult[0].error.length > 0, "resolveSprites: empty overrideBytes reports an error");

// --- malformed sffBytes: no throw, every request in the batch reports an error ---
const malformedBatchResult = globalThis.OpenKakutouSff.resolveSprites(new TextEncoder().encode("garbage"), [[0, 0]], null);
assert(malformedBatchResult[0].pixels === null, "resolveSprites: malformed sffBytes returns null pixels");
assert(typeof malformedBatchResult[0].error === "string" && malformedBatchResult[0].error.length > 0, "resolveSprites: malformed sffBytes reports an error");

// The module must still respond correctly after resolveSprites errors too.
const afterResolveErrorResult = globalThis.OpenKakutouSff.load(sffBytes);
assert(afterResolveErrorResult.error === null, "module still works after a prior resolveSprites error");

if (process.exitCode) {
	console.error("\nsmoke test FAILED");
} else {
	console.log("\nsmoke test passed");
}
process.exit(process.exitCode ?? 0);
