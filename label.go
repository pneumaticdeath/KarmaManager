package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

type TapLabel struct {
	widget.BaseWidget

	Label    *widget.Label
	OnTapped func(*fyne.PointEvent)
}

func NewTapLabel(text string) *TapLabel {
	tl := &TapLabel{}
	tl.Label = widget.NewLabel(text)
	tl.ExtendBaseWidget(tl)
	return tl
}

func (tl *TapLabel) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(tl.Label)
}

func (tl *TapLabel) Tapped(pe *fyne.PointEvent) {
	if tl.OnTapped != nil {
		tl.OnTapped(pe)
	}
}
