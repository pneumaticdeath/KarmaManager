//go:build js

package main

import (
	"bytes"
	"image/gif"
)

// wasmGIFCache holds encoded GIF bytes keyed by the "path" used in WriteGIF.
// On WASM the real filesystem is unavailable, so we store data in memory.
var wasmGIFCache = map[string][]byte{}

func WriteGIF(g *gif.GIF, path string) error {
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		return err
	}
	wasmGIFCache[path] = buf.Bytes()
	return nil
}
