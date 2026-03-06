// Package reorderlist provides a drag-to-reorder list widget for Fyne.
// Items can be grabbed by their drag handle (☰) and dragged vertically
// to change their order. Dragging on the content area (outside the handle)
// scrolls the list normally.
package reorderlist

import (
	"image/color"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// DragHandle is a small icon widget that accepts drag events and forwards
// them to the List. Only drags initiated on this widget trigger reordering;
// drags on the content area scroll the list instead.
type DragHandle struct {
	widget.BaseWidget
	icon   *widget.Icon
	onDrag func(dy float32)
	onEnd  func()
}

// minTouchSize is the minimum touch-target dimension recommended by Apple HIG
// and Material Design. The icon is centred inside this area so the visual
// size stays natural while the hit area is reliably tappable on mobile.
const minTouchSize float32 = 44

func newDragHandle(onDrag func(float32), onEnd func()) *DragHandle {
	h := &DragHandle{onDrag: onDrag, onEnd: onEnd}
	h.icon = widget.NewIcon(theme.MenuIcon())
	h.ExtendBaseWidget(h)
	return h
}

func (h *DragHandle) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewCenter(h.icon))
}

func (h *DragHandle) MinSize() fyne.Size {
	s := h.icon.MinSize()
	if s.Width < minTouchSize {
		s.Width = minTouchSize
	}
	if s.Height < minTouchSize {
		s.Height = minTouchSize
	}
	return s
}

func (h *DragHandle) Dragged(e *fyne.DragEvent) {
	if h.onDrag != nil {
		h.onDrag(e.Dragged.DY)
	}
}

func (h *DragHandle) DragEnd() {
	if h.onEnd != nil {
		h.onEnd()
	}
}

var _ fyne.Draggable = (*DragHandle)(nil)

// listRow is an internal composite: a highlight background rectangle stacked
// behind an HBox of [☰ handle | content widget].
type listRow struct {
	widget.BaseWidget
	bg  *canvas.Rectangle
	box *fyne.Container
}

func newListRow(handle *DragHandle, content fyne.CanvasObject) *listRow {
	r := &listRow{}
	r.bg = canvas.NewRectangle(color.Transparent)
	r.box = container.NewStack(r.bg, container.NewHBox(handle, content))
	r.ExtendBaseWidget(r)
	return r
}

func (r *listRow) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(r.box)
}

// List is a drag-to-reorder list widget. Grab the ☰ handle on any row and
// drag vertically to reorder. The content area scrolls normally.
type List struct {
	widget.BaseWidget

	// Items holds the current ordered item keys. Read after OnReorder fires.
	Items []string

	// MakeRow returns the content widget for a given item key. Called once
	// per item each time the list is built or rebuilt (after a drop or SetItems).
	MakeRow func(item string) fyne.CanvasObject

	// OnReorder is called after a successful drag-drop with the updated item slice.
	OnReorder func(newItems []string)

	rows    []*listRow
	vbox    *fyne.Container
	scroll  *container.Scroll
	surface *fyne.Container

	dragging bool
	dragFrom int
	dragTo   int
	accumY   float32
}

// New creates a drag-to-reorder list populated with items. makeRow is called
// once per item to produce its content widget. onReorder is called whenever
// the order changes.
func New(items []string, makeRow func(string) fyne.CanvasObject, onReorder func([]string)) *List {
	l := &List{
		Items:     make([]string, len(items)),
		MakeRow:   makeRow,
		OnReorder: onReorder,
	}
	copy(l.Items, items)
	l.vbox = container.NewVBox()
	l.scroll = container.NewVScroll(l.vbox)
	l.surface = container.NewBorder(nil, nil, nil, nil, l.scroll)
	l.ExtendBaseWidget(l)
	l.build()
	return l
}

func (l *List) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(l.surface)
}

// SetItems replaces the item list and rebuilds the widget. Existing scroll
// position is reset. Safe to call from the Fyne goroutine.
func (l *List) SetItems(items []string) {
	l.Items = make([]string, len(items))
	copy(l.Items, items)
	l.build()
	l.Refresh()
}

// build (re)creates all row widgets from l.Items. Must be called from the
// Fyne goroutine. Row indices are captured in closures and remain valid for
// the lifetime of a single drag gesture (we only rebuild on DragEnd).
func (l *List) build() {
	l.rows = make([]*listRow, len(l.Items))
	objects := make([]fyne.CanvasObject, len(l.Items))
	for i, item := range l.Items {
		idx := i // capture for closure
		content := l.MakeRow(item)
		handle := newDragHandle(
			func(dy float32) { l.onDrag(idx, dy) },
			func() { l.onDrop(idx) },
		)
		row := newListRow(handle, content)
		l.rows[idx] = row
		objects[idx] = row
	}
	l.vbox.Objects = objects
	l.vbox.Refresh()
}

// rowHeight returns the height of a single row, using MinSize as a proxy
// before layout has run.
func (l *List) rowHeight() float32 {
	if len(l.rows) > 0 {
		h := l.rows[0].MinSize().Height
		if h > 0 {
			return h
		}
	}
	return theme.TextSize() + theme.InnerPadding()*2
}

// onDrag accumulates vertical delta from drag events on a handle and
// updates dragTo and highlight colors.
func (l *List) onDrag(fromIdx int, dy float32) {
	if !l.dragging {
		l.dragging = true
		l.dragFrom = fromIdx
		l.accumY = 0
	}
	l.accumY += dy
	rowH := l.rowHeight()
	n := len(l.Items)
	target := l.dragFrom + int(math.Round(float64(l.accumY/rowH)))
	if target < 0 {
		target = 0
	}
	if target >= n {
		target = n - 1
	}
	l.dragTo = target
	l.updateHighlight()
}

// onDrop finalises the drag: moves Items[dragFrom] to Items[dragTo],
// rebuilds the widget, and fires OnReorder.
func (l *List) onDrop(_ int) {
	if !l.dragging {
		return
	}
	from, to := l.dragFrom, l.dragTo
	l.dragging = false
	l.accumY = 0

	if from != to {
		item := l.Items[from]

		// Remove the dragged item.
		without := make([]string, 0, len(l.Items)-1)
		for i, v := range l.Items {
			if i != from {
				without = append(without, v)
			}
		}

		// Insert at `to`. After the removal, `to` indexes correctly into the
		// new slice: when from < to the elements shift left, so without[:to]
		// naturally ends right before the intended landing slot; when from > to
		// the prefix is unchanged. Either way the item lands at index `to`.
		result := make([]string, 0, len(l.Items))
		result = append(result, without[:to]...)
		result = append(result, item)
		result = append(result, without[to:]...)
		l.Items = result
	}

	l.clearHighlight()
	l.build()
	l.vbox.Refresh()
	if l.OnReorder != nil {
		l.OnReorder(l.Items)
	}
}

// updateHighlight applies background colors: muted gray on the source row,
// primary accent on the target row, transparent elsewhere.
func (l *List) updateHighlight() {
	for i, row := range l.rows {
		switch {
		case i == l.dragFrom:
			row.bg.FillColor = color.NRGBA{R: 150, G: 150, B: 150, A: 80}
		case i == l.dragTo:
			row.bg.FillColor = withAlpha(theme.Color(theme.ColorNamePrimary), 150)
		default:
			row.bg.FillColor = color.Transparent
		}
		row.bg.Refresh()
	}
}

// clearHighlight resets all row backgrounds to transparent.
func (l *List) clearHighlight() {
	for _, row := range l.rows {
		row.bg.FillColor = color.Transparent
		row.bg.Refresh()
	}
}

// withAlpha returns c with its alpha channel replaced by a.
func withAlpha(c color.Color, a uint8) color.NRGBA {
	r, g, b, _ := c.RGBA()
	return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: a}
}
