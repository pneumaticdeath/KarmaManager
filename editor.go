package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var fontSize float32 = 20.0
var padding float32 = 20.0

type WordWidget struct {
	widget.BaseWidget

	Word       string
	Text       *canvas.Text
	box        *fyne.Container
	outline    *canvas.Rectangle
	dragged    bool
	startPos   fyne.Position
	xOff, yOff float32
}

func NewWordWidget(word string) *WordWidget {
	ww := &WordWidget{Word: word, Text: canvas.NewText(word, theme.TextColor())}

	ww.box = container.NewWithoutLayout()
	red := color.NRGBA{R: 255, G: 0, B: 0, A: 255}
	ww.outline = canvas.NewRectangle(color.Transparent)
	ww.outline.StrokeColor = red
	ww.outline.StrokeWidth = 3.0
	ww.outline.CornerRadius = 5.0
	ww.outline.Resize(fyne.NewSize(ww.Text.MinSize().Width+10, ww.Text.MinSize().Height+10))
	ww.box.Add(ww.outline)
	ww.box.Add(ww.Text)
	ww.Text.Move(fyne.NewPos(5, 5))

	ww.ExtendBaseWidget(ww)
	return ww
}

func (ww *WordWidget) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(ww.box)
}

func (ww *WordWidget) Dragged(e *fyne.DragEvent) {
	if !ww.dragged {
		ww.dragged = true
		ww.startPos = ww.Position()
		ww.xOff = 0.0
		ww.yOff = 0.0
	}

	dx, dy := e.Dragged.Components()
	ww.xOff += dx
	ww.yOff += dy

	ww.Move(fyne.NewPos(ww.startPos.X+ww.xOff, ww.startPos.Y+ww.yOff))
}

func (ww *WordWidget) DragEnd() {
	ww.dragged = false
}

func (ww *WordWidget) MinSize() fyne.Size {
	return fyne.NewSize(ww.Text.MinSize().Width+10, ww.Text.MinSize().Height+10)
}

var _ fyne.Draggable = (*WordWidget)(nil)

type EditField struct {
	widget.BaseWidget

	Words            []string
	MaxRows, MaxCols int
	surface          *fyne.Container
	widgets          []*WordWidget
}

func NewEditField(words []string) *EditField {
	ef := &EditField{Words: words}

	ef.surface = container.NewWithoutLayout()
	// ef.scroll = container.NewScroll(ef.surface)
	// ef.scroll.Direction = container.ScrollNone

	ef.ExtendBaseWidget(ef)
	return ef
}

func (ef *EditField) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(ef.surface)
	// return widget.NewSimpleRenderer(ef.scroll)
}

func (ef *EditField) Draw() {
	var maxheight float32

	ef.widgets = make([]*WordWidget, len(ef.Words))
	for index, word := range ef.Words {
		widget := NewWordWidget(word)
		ef.surface.Add(widget)
		if widget.MinSize().Height > maxheight {
			maxheight = widget.MinSize().Height
		}
		ef.widgets[index] = widget
	}
	LayoutWordGlyphs(ef.widgets, padding, maxheight+padding, ef.surface.Size())

	ef.surface.Refresh()
}
