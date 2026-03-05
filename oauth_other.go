//go:build !ios && !android

package main

import "fyne.io/fyne/v2"

func OpenOAuthBrowser(_ string, _ fyne.Window) {}
func DismissOAuthBrowser()                     {}
