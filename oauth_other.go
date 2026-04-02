//go:build !ios && !android

package main

import (
	"net/url"

	"fyne.io/fyne/v2"
)

func OpenOAuthBrowser(rawURL string, _ fyne.Window) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return
	}
	_ = fyne.CurrentApp().OpenURL(parsed)
}

func DismissOAuthBrowser() {}
