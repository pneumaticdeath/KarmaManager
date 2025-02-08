package main

import (
	"errors"
	"fmt"
	"image/color"
	"math"
	"slices"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var fontSize float32 = 20.0
var padding float32 = 20.0

type WordWidget struct {
	widget.BaseWidget

	Index       int
	Text        *canvas.Text
	Row, Column int
	box         *fyne.Container
	outline     *canvas.Rectangle
	dragged     bool
	startPos    fyne.Position
	xOff, yOff  float32
	dropfunc    func(int, fyne.Position)
	OnTapped    func()
}

func NewWordWidget(index int, word string, drop func(int, fyne.Position), tap func()) *WordWidget {
	ww := &WordWidget{Index: index, dropfunc: drop, OnTapped: tap}

	ww.Text = canvas.NewText(word, theme.TextColor())
	ww.Text.TextStyle = fyne.TextStyle{Monospace: true}
	ww.Text.TextSize = fontSize

	ww.Row, ww.Column = -1, -1

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
	if ww.dropfunc != nil {
		ww.dropfunc(ww.Index, ww.startPos)
	}
}

func (ww *WordWidget) MinSize() fyne.Size {
	return fyne.NewSize(ww.Text.MinSize().Width+10, ww.Text.MinSize().Height+10)
}

func (ww *WordWidget) Tapped(e *fyne.PointEvent) {
	if ww.OnTapped != nil {
		ww.OnTapped()
	}
}

var _ fyne.Draggable = (*WordWidget)(nil)
var _ fyne.Tappable = (*WordWidget)(nil)

type EditField struct {
	widget.BaseWidget

	Words      []string
	wordheight float32
	surface    *fyne.Container
	widgets    []*WordWidget
	window     fyne.Window
}

func NewEditField(words []string, window fyne.Window) *EditField {
	ef := &EditField{}

	ef.Words = make([]string, 0, len(words))
	for _, word := range words {
		if word == "" {
			continue
		}
		ef.Words = append(ef.Words, word)
	}
	ef.window = window

	ef.surface = container.NewWithoutLayout()

	ef.ExtendBaseWidget(ef)
	return ef
}

func (ef *EditField) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(ef.surface)
}

func (ef *EditField) Initialize() {
	ef.widgets = make([]*WordWidget, len(ef.Words))

	ef.surface.RemoveAll()
	for index, word := range ef.Words {
		widget := NewWordWidget(index, word, ef.DropCallback, func() {
			ef.ShowWordEdit(index)
		})

		ef.surface.Add(widget)
		if widget.MinSize().Height > ef.wordheight {
			ef.wordheight = widget.MinSize().Height
		}
		ef.widgets[index] = widget
	}
	LayoutWordWidgets(ef.widgets, padding, ef.wordheight+padding, ef.surface.Size())

	ef.surface.Refresh()
}

func (ef *EditField) ShowWordEdit(index int) {
	entry := widget.NewEntry()
	entry.Text = ef.Words[index]
	entry.Validator = func(word string) error {
		if !NewRuneCluster(word).Equals(NewRuneCluster(ef.Words[index])) {
			return errors.New("Not equivalent")
		}
		return nil
	}
	items := []*widget.FormItem{widget.NewFormItem("", entry)}
	d := dialog.NewForm(fmt.Sprintf("Edit %s", ef.Words[index]), "Save", "Cancel", items, func(submitted bool) {
		if submitted {
			word := entry.Text
			if word != ef.Words[index] {
				ef.Words[index] = word
				ef.Initialize()
			}
		}
	}, ef.window)
	d.Resize(fyne.NewSize(250, 250))
	d.Show()
}

func (ef *EditField) DropCallback(index int, initialPos fyne.Position) {
	currentPos := ef.widgets[index].Position()

	newRow := int(math.Floor(float64((currentPos.Y-padding)/(ef.wordheight+padding) + 0.5)))
	// oldRow := ef.widgets[index].Row

	targetX := ef.widgets[index].Position().X + ef.widgets[index].Size().Width/2
	// targetY := ef.widgets[index].Position().Y + ef.widgets[index].Size().Height/2

	targetIndex := len(ef.widgets)
	for i, ww := range ef.widgets {
		var widgetPos fyne.Position
		if i == index {
			widgetPos = initialPos
		} else {
			widgetPos = ww.Position()
		}
		centerX := widgetPos.X + ww.Size().Width/2
		// centerY := widgetPos.Y + ww.Size().Height/2
		if ww.Row == newRow && centerX > targetX || ww.Row > newRow {
			targetIndex = i
			break
		}
	}
	// reinitialize := false
	if targetIndex >= len(ef.widgets) && index != len(ef.widgets)-1 {
		ef.Words = append(ef.Words, ef.Words[index])
		ef.Words = slices.Delete(ef.Words, index, index+1)
		// reinitialize = true
	} else if targetIndex > index {
		ef.Words = slices.Insert(ef.Words, targetIndex, ef.Words[index])
		ef.Words = slices.Delete(ef.Words, index, index+1)
		// reinitialize = true
	} else if targetIndex < index {
		word := ef.Words[index]
		ef.Words = slices.Delete(ef.Words, index, index+1)
		ef.Words = slices.Insert(ef.Words, targetIndex, word)
		// reinitialize = true
	}

	ef.Initialize()
}
