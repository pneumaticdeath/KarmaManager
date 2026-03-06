//go:build js

package main

import (
	"encoding/base64"
	"fmt"
	"syscall/js"

	"fyne.io/fyne/v2"
)

func ShareGIF(gifPath string, window fyne.Window) {
	data, ok := wasmGIFCache[gifPath]
	if !ok {
		fmt.Println("ShareGIF: no data cached for", gifPath)
		return
	}
	b64 := base64.StdEncoding.EncodeToString(data)
	doc := js.Global().Get("document")
	a := doc.Call("createElement", "a")
	a.Set("href", "data:image/gif;base64,"+b64)
	a.Set("download", "animation.gif")
	doc.Get("body").Call("appendChild", a)
	a.Call("click")
	doc.Get("body").Call("removeChild", a)
}
