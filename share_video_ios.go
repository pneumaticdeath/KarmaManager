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

// ShareVideo shares the video at path via the iOS share sheet.
// UIActivityViewController accepts any file type, so we reuse shareGIFFile.
func ShareVideo(videoPath string, window fyne.Window) {
	cpath := C.CString(videoPath)
	defer C.free(unsafe.Pointer(cpath))
	C.shareGIFFile(cpath)
}
