package main

import (
	// "fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

type RuneLayoutElement struct {
	Rune     rune
	Row, Col int
}

func MakeRuneLayout(input string, maxColumns int) ([]RuneLayoutElement, int) {
	layout := make([]RuneLayoutElement, 0, len(input))
	words := strings.Split(input, " ")
	row := 0
	col := 0
	for _, word := range words {
		if word == "" {
			continue
		}

		// fmt.Println("Max Columns", maxColumns)
		remainingColumns := maxColumns - col
		for len(word) >= maxColumns {
			// fmt.Println("Remaining cols", remainingColumns)
			if remainingColumns > 1 {
				partial := word[:remainingColumns-1]
				word = word[remainingColumns-1:]
				i := 0
				for i < len(partial) {
					r := rune(partial[i])
					layout = append(layout, RuneLayoutElement{r, row, col + i})
					i += 1
				}
				layout = append(layout, RuneLayoutElement{'-', row, col + i})
			}
			row += 1
			col = 0
			remainingColumns = maxColumns
		}

		if len(word) > remainingColumns {
			row += 1
			col = 0
		}

		i := 0
		for i < len(word) {
			r := rune(word[i])
			layout = append(layout, RuneLayoutElement{r, row, col + i})
			i += 1
		}
		col += len(word) + 1 // the one is for the space after the word
		if col >= maxColumns {
			row += 1
			col = 0
		}
	}

	if col == 0 { // Edge case... we wrapped but didn't actually append any words
		return layout, row
	} else {
		return layout, row + 1
	}
}

func LayoutAndAnimateWordWidgets(wordWidgets []*WordWidget, padding, rowHeight float32, dispSize fyne.Size) {
	row := 0
	column := 0
	min_x := padding
	min_y := padding
	max_x := dispSize.Width - padding
	// max_y := dispSize.Height - padding
	x := min_x
	y := min_y

	for _, word := range wordWidgets {
		horizSpaceRemaining := max_x - x
		if word.MinSize().Width > horizSpaceRemaining && x > min_x {
			row += 1
			column = 0
			y += rowHeight
			x = min_x
		}

		pos := fyne.NewPos(x, y)
		// fmt.Printf("Moving glyph %d to %v\n", index, pos)
		// word.Move(pos)
		anim := canvas.NewPositionAnimation(word.Position(), pos, canvas.DurationStandard, word.Move)
		word.Resize(word.MinSize())
		word.Row = row
		word.Column = column
		anim.Start()

		x += word.MinSize().Width + padding

		column += 1

	}
}
