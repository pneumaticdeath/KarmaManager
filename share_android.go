//go:build android

package main

/*
#cgo LDFLAGS: -landroid -llog
#include <stdlib.h>
void shareGIFViaJNI(uintptr_t env, uintptr_t ctx, const char *path);
*/
import "C"
import (
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
)

func ShareGIF(gifPath string, window fyne.Window) {
	nw, ok := window.(driver.NativeWindow)
	if !ok {
		return
	}
	nw.RunNative(func(ctx any) {
		cpath := C.CString(gifPath)
		defer C.free(unsafe.Pointer(cpath))
		switch ac := ctx.(type) {
		case *driver.AndroidContext:
			C.shareGIFViaJNI(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx), cpath)
		case *driver.AndroidWindowContext:
			C.shareGIFViaJNI(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx), cpath)
		}
	})
}
