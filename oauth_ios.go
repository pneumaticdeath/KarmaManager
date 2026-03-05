//go:build ios

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework UIKit -framework SafariServices

#include <stdlib.h>
void openOAuthBrowser(const char *url);
void dismissOAuthBrowser();
*/
import "C"
import "unsafe"

import "fyne.io/fyne/v2"

func OpenOAuthBrowser(rawURL string, _ fyne.Window) {
	curl := C.CString(rawURL)
	defer C.free(unsafe.Pointer(curl))
	C.openOAuthBrowser(curl)
}

func DismissOAuthBrowser() { C.dismissOAuthBrowser() }
