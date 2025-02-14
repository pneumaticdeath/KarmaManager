package main

import (
	"errors"
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type WordListWidget struct {
	widget.BaseWidget

	Text         *canvas.Text
	box          *fyne.Container
	DeleteButton *widget.Button
	OnDelete     func()
}

func NewWordListWidget(text string, delCB func()) *WordListWidget {
	wlw := &WordListWidget{}

	wlw.Text = canvas.NewText(text, theme.TextColor())
	// wlw.Text.TextStyle = fyne.TextStyle{Monospace: true}
	// wlw.Text.TextSize = fontSize

	wlw.OnDelete = delCB

	wlw.DeleteButton = widget.NewButtonWithIcon("", theme.ContentClearIcon(), func() {
		if wlw.OnDelete != nil {
			wlw.OnDelete()
		}
	})

	wlw.box = container.NewWithoutLayout()
	wlw.box.Add(wlw.Text)
	wlw.box.Add(wlw.DeleteButton)
	textsize := wlw.Text.MinSize()
	buttonsize := wlw.DeleteButton.MinSize()
	wlw.DeleteButton.Resize(buttonsize)
	wlw.Text.Resize(textsize)

	if textsize.Height > buttonsize.Height {
		wlw.DeleteButton.Move(fyne.NewPos(textsize.Width+padding, (textsize.Height-buttonsize.Height)/2.0))
	} else {
		wlw.Text.Move(fyne.NewPos(0, (buttonsize.Height-textsize.Height)/2))
		wlw.DeleteButton.Move(fyne.NewPos(textsize.Width+padding, 0))
	}

	wlw.ExtendBaseWidget(wlw)

	return wlw
}

func (wlw *WordListWidget) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(wlw.box)
}

func (wlw *WordListWidget) SetText(text string) {
	wlw.Text.Text = text
}

func (wlw *WordListWidget) MinSize() fyne.Size {
	textsize := wlw.Text.MinSize()
	buttonsize := wlw.DeleteButton.MinSize()
	return fyne.NewSize(textsize.Width+buttonsize.Width+padding, max(textsize.Height, buttonsize.Height))
}

type WordList struct {
	widget.BaseWidget

	list  *widget.List
	Words []string
}

func NewWordList(words []string) *WordList {
	wl := &WordList{}

	wl.Words = make([]string, len(words))
	copy(wl.Words, words)
	wl.list = widget.NewList(func() int {
		return len(wl.Words)
	}, func() fyne.CanvasObject {
		return NewWordListWidget("WordListWidget", nil)
	}, func(id widget.ListItemID, obj fyne.CanvasObject) {
		wlw, ok := obj.(*WordListWidget)
		if !ok {
			return
		}

		wlw.SetText(wl.Words[id])
		wlw.OnDelete = func() {
			wl.Words = slices.Delete(wl.Words, id, id+1)
			wl.list.Refresh()
		}
	})

	wl.ExtendBaseWidget(wl)

	return wl
}

func (wl *WordList) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(wl.list)
}

func (wl *WordList) ShowAddWord(title, submit, dismiss string, window fyne.Window) {
	wordEntry := widget.NewEntry()
	wordEntry.Validator = func(word string) error {
		if strings.Index(word, " ") != -1 {
			return errors.New("Must not contain spaces")
		}
		return nil
	}
	items := []*widget.FormItem{widget.NewFormItem("", wordEntry)}
	d := dialog.NewForm(title, submit, dismiss, items, func(submitted bool) {
		if submitted {
			wl.Words = append(wl.Words, wordEntry.Text)
			wl.list.Refresh()
		}
	}, window)
	d.Resize(fyne.NewSize(250, 250))
	d.Show()
}
