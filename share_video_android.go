//go:build android

package main

/*
#cgo LDFLAGS: -landroid -llog
#include <stdlib.h>
void shareVideoViaJNI(uintptr_t env, uintptr_t ctx, const char *path);
*/
import "C"
import (
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
)

func ShareVideo(videoPath string, window fyne.Window) {
	nw, ok := window.(driver.NativeWindow)
	if !ok {
		return
	}
	nw.RunNative(func(ctx any) {
		cpath := C.CString(videoPath)
		defer C.free(unsafe.Pointer(cpath))
		switch ac := ctx.(type) {
		case *driver.AndroidContext:
			C.shareVideoViaJNI(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx), cpath)
		case *driver.AndroidWindowContext:
			C.shareVideoViaJNI(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx), cpath)
		}
	})
}
