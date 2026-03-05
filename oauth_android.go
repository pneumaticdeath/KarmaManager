//go:build android

package main

/*
#cgo LDFLAGS: -landroid -llog
#include <stdlib.h>
void openOAuthBrowserJNI(uintptr_t env, uintptr_t ctx, const char *url);
*/
import "C"
import (
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
)

func OpenOAuthBrowser(rawURL string, window fyne.Window) {
	nw, ok := window.(driver.NativeWindow)
	if !ok {
		return
	}
	nw.RunNative(func(ctx any) {
		curl := C.CString(rawURL)
		defer C.free(unsafe.Pointer(curl))
		switch ac := ctx.(type) {
		case *driver.AndroidContext:
			C.openOAuthBrowserJNI(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx), curl)
		case *driver.AndroidWindowContext:
			C.openOAuthBrowserJNI(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx), curl)
		}
	})
}

func DismissOAuthBrowser() {} // Android: user dismisses the browser tab themselves
