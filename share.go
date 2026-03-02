//go:build !ios && !android

package main

import (
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
)

func ShareGIF(gifPath string, window fyne.Window) {
	fd := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
		if err != nil || uc == nil {
			return
		}
		defer uc.Close()
		data, err := os.ReadFile(gifPath)
		if err != nil {
			dialog.ShowError(err, window)
			return
		}
		if _, err = uc.Write(data); err != nil {
			dialog.ShowError(err, window)
		}
	}, window)
	fd.SetFileName("animation.gif")
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".gif"}))
	fd.Show()
}
