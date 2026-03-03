//go:build !ios && !android

package main

import (
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
)

func ShareVideo(videoPath string, window fyne.Window) {
	fd := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
		if err != nil || uc == nil {
			return
		}
		defer uc.Close()
		data, err := os.ReadFile(videoPath)
		if err != nil {
			dialog.ShowError(err, window)
			return
		}
		if _, err = uc.Write(data); err != nil {
			dialog.ShowError(err, window)
		}
	}, window)
	fd.SetFileName("animation.mp4")
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".mp4"}))
	fd.Show()
}
