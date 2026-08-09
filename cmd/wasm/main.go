//go:build js && wasm

// Command wasm is the WASM entrypoint for the sff library: thin syscall/js
// glue exposing this package's Load and ResolveSpritePixels to a browser
// (or any JS host) as global functions, so a consumer (stage, lifebar
// apps) can load and decode MUGEN/Ikemen .sff sprite sheets without a Go
// toolchain of its own — independent of character's own WASM build, which
// consumes this same package internally but publishes its own artifact.
//
// It carries no logic beyond argument conversion, calling into the root
// sff package, and marshaling results to JS — all real behavior lives in
// that package, which is unit-tested independently of this file (see
// .vibe/decisions/003-wasm-entrypoint-api-shape-and-release-smoke-gate.md,
// which mirrors the shape character's own cmd/wasm already established).
// This file's own behavior is instead verified by smoke.mjs, a Node.js
// script that loads the built module the way a real JS consumer would —
// syscall/js code cannot run under the plain `go test` toolchain.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/color"
	"syscall/js"

	sff "github.com/openkakutou/sff"
)

func main() {
	globalName := "OpenKakutouSff"
	js.Global().Set(globalName, js.ValueOf(map[string]any{
		"load":           js.FuncOf(load),
		"resolveSprites": js.FuncOf(resolveSprites),
	}))

	// Registering js.FuncOf callbacks does not keep the Go runtime alive on
	// its own; block forever so OpenKakutouSff.load/resolveSprites keep
	// working for the lifetime of the page.
	select {}
}

// load is OpenKakutouSff.load(sffBytes) as seen from JS: sffBytes is a
// Uint8Array (or any JS value js.CopyBytesToGo accepts) holding a .sff
// file's raw bytes. It always returns a JS object shaped
// { spriteGroups: string|null, error: string|null } — exactly one of the
// two fields is non-null — never throws and never lets an internal panic
// escape to the JS caller. spriteGroups carries sprite metadata only
// (dimensions, axis, palette index) — never decoded pixel/color data; see
// resolveSprites for that.
func load(this js.Value, args []js.Value) any {
	defer func() {
		// A panic here would otherwise propagate out of the js.Func
		// callback and tear down the whole page's WASM instance; recover
		// is this boundary's own responsibility, not Load's (which
		// already returns descriptive errors for every malformed-input
		// path it knows about).
		recover()
	}()

	if len(args) != 1 {
		return loadResult(nil, fmt.Errorf("OpenKakutouSff.load: expected 1 argument (sffBytes), got %d", len(args)))
	}

	sffBytes, err := bytesFromJS(args[0])
	if err != nil {
		return loadResult(nil, fmt.Errorf("OpenKakutouSff.load: sffBytes: %w", err))
	}

	groups, err := sff.Load(bytes.NewReader(sffBytes))
	if err != nil {
		return loadResult(nil, err)
	}
	if groups == nil {
		groups = []sff.SpriteGroup{}
	}

	data, err := json.Marshal(groups)
	if err != nil {
		return loadResult(nil, fmt.Errorf("OpenKakutouSff.load: encoding result as JSON: %w", err))
	}

	return loadResult(data, nil)
}

// bytesFromJS copies a JS Uint8Array-like value into a Go []byte via
// js.CopyBytesToGo, the standard syscall/js conversion. It returns a
// descriptive error instead of panicking if v is not a byte-array-like
// value (e.g. undefined, or missing a numeric "length").
func bytesFromJS(v js.Value) (b []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			b, err = nil, fmt.Errorf("expected a byte array, got %v (%v)", v, r)
		}
	}()

	length := v.Get("length").Int()
	buf := make([]byte, length)
	js.CopyBytesToGo(buf, v)
	return buf, nil
}

// loadResult builds this module's { spriteGroups, error } JS return
// shape. Exactly one field is ever non-null.
func loadResult(spriteGroupsJSON []byte, err error) map[string]any {
	if err != nil {
		return map[string]any{"spriteGroups": nil, "error": err.Error()}
	}
	return map[string]any{"spriteGroups": string(spriteGroupsJSON), "error": nil}
}

// resolveSprites is OpenKakutouSff.resolveSprites(sffBytes, requests,
// overrideBytes) as seen from JS: sffBytes is a Uint8Array holding a .sff
// file's raw bytes (transferred once for the whole call, not once per
// sprite); requests is an array of [group, image] number pairs;
// overrideBytes is an optional Uint8Array holding an external .act
// palette file — undefined or null means "use each sprite's own
// palette", any other value (including an empty Uint8Array) is decoded,
// and if invalid, reported as an error for every request in the batch
// rather than silently falling back. Returns one
// { pixels, width, height, error } object per request, in the same
// order — pixels is a flat, row-major RGBA byte buffer
// (width*height*4 bytes); on error, pixels/width/height are
// nil/0/0. Like load, this never throws and never leaves the module in a
// broken state after an error. See
// .vibe/decisions/003-wasm-entrypoint-api-shape-and-release-smoke-gate.md.
func resolveSprites(this js.Value, args []js.Value) any {
	defer func() {
		// See load's identical recover() — a panic here would otherwise
		// tear down the whole page's WASM instance.
		recover()
	}()

	if len(args) != 3 {
		return []any{spriteResult(nil, 0, 0, fmt.Errorf("OpenKakutouSff.resolveSprites: expected 3 arguments (sffBytes, requests, overrideBytes), got %d", len(args)))}
	}

	sffBytes, err := bytesFromJS(args[0])
	if err != nil {
		return []any{spriteResult(nil, 0, 0, fmt.Errorf("OpenKakutouSff.resolveSprites: sffBytes: %w", err))}
	}

	override, overrideErr := overridePaletteFromJS(args[2])

	n := args[1].Get("length").Int()
	r := bytes.NewReader(sffBytes)
	results := make([]any, n)
	for i := 0; i < n; i++ {
		if overrideErr != nil {
			results[i] = spriteResult(nil, 0, 0, fmt.Errorf("OpenKakutouSff.resolveSprites: overrideBytes: %w", overrideErr))
			continue
		}
		group, image, err := spriteRequestFromJS(args[1].Index(i))
		if err != nil {
			results[i] = spriteResult(nil, 0, 0, fmt.Errorf("OpenKakutouSff.resolveSprites: requests[%d]: %w", i, err))
			continue
		}
		pixels, width, height, err := sff.ResolveSpritePixels(r, group, image, override)
		results[i] = spriteResult(pixels, width, height, err)
	}
	return results
}

// overridePaletteFromJS decodes v as an external .act palette override, or
// returns (nil, nil) when v is JS undefined/null — the two values a
// caller uses to mean "no override, use the sprite's own palette". Any
// other value, including an empty Uint8Array, is decoded via
// sff.DecodeExternalPalette and its own validation (wrong size, etc.)
// surfaces as err, never a silent fallback to no override.
func overridePaletteFromJS(v js.Value) (*sff.Palette, error) {
	if v.IsUndefined() || v.IsNull() {
		return nil, nil
	}
	b, err := bytesFromJS(v)
	if err != nil {
		return nil, err
	}
	p, err := sff.DecodeExternalPalette(b)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// spriteRequestFromJS reads one [group, image] pair from requests[i] as
// seen from JS. Returns a descriptive error instead of panicking if v is
// not a length-2 array-like value.
func spriteRequestFromJS(v js.Value) (group, image int, err error) {
	defer func() {
		if r := recover(); r != nil {
			group, image, err = 0, 0, fmt.Errorf("expected a [group, image] pair, got %v (%v)", v, r)
		}
	}()

	if length := v.Get("length").Int(); length != 2 {
		return 0, 0, fmt.Errorf("expected a [group, image] pair (length 2), got length %d", length)
	}
	return v.Index(0).Int(), v.Index(1).Int(), nil
}

// spriteResult builds this module's { pixels, width, height, error } JS
// return shape for one resolved sprite. On error, pixels/width/height are
// explicitly nil/0/0 rather than left undefined.
func spriteResult(pixels []color.RGBA, width, height int, err error) map[string]any {
	if err != nil {
		return map[string]any{"pixels": nil, "width": 0, "height": 0, "error": err.Error()}
	}
	return map[string]any{"pixels": rgbaToJS(pixels), "width": width, "height": height, "error": nil}
}

// rgbaToJS flattens a row-major []color.RGBA buffer into a flat, straight-
// alpha JS Uint8Array (width*height*4 bytes: R, G, B, A per pixel) —
// directly usable as new ImageData(pixels, width, height) by a caller.
func rgbaToJS(pixels []color.RGBA) js.Value {
	buf := make([]byte, len(pixels)*4)
	for i, p := range pixels {
		buf[i*4], buf[i*4+1], buf[i*4+2], buf[i*4+3] = p.R, p.G, p.B, p.A
	}
	arr := js.Global().Get("Uint8Array").New(len(buf))
	js.CopyBytesToJS(arr, buf)
	return arr
}
