//go:build ios

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework UIKit

#include <stdlib.h>
void shareGIFFile(const char *path);
*/
import "C"
import (
	"unsafe"

	"fyne.io/fyne/v2"
)

func ShareGIF(gifPath string, window fyne.Window) {
	cpath := C.CString(gifPath)
	defer C.free(unsafe.Pointer(cpath))
	C.shareGIFFile(cpath)
}
