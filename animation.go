package main

import (
	"errors"
	// "image"
	// "image/color"
	// "image/gif"
	"strings"
	// "unicode"

	"fyne.io/fyne/v2"
)

var glyphSize fyne.Size = fyne.NewSize(20, 20)
var glyphSpacing float32 = 4.0
var maxCols int = 30

type RuneGlyph struct {
	Letter                 rune
	StartingPos, EndingPos fyne.Position
}

type Animation struct {
	Glyphs     []RuneGlyph
	Rows, Cols int
	Total      int
}

type LayoutElement struct {
	Rune     rune
	Row, Col int
}

func MakeLayout(input string, maxColumns int) ([]LayoutElement, int) {
	layout := make([]LayoutElement, 0, len(input))
	words := strings.Split(input, " ")
	row := 0
	col := 0
	for _, word := range words {
		if word == "" {
			continue
		}

		remainingColumns := maxColumns - col
		for len(word) >= maxColumns {
			partial := word[:remainingColumns-1]
			word = word[remainingColumns-1:]
			i := 0
			for i < len(partial) {
				r := rune(partial[i])
				layout = append(layout, LayoutElement{r, row, col + i})
				i += 1
			}
			layout = append(layout, LayoutElement{'-', row, col + i})
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
			layout = append(layout, LayoutElement{r, row, col + i})
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

func NthRuneIndex(layout []LayoutElement, r rune, n int) int {
	index := 0
	foundCount := 0
	for index < len(layout) {
		if layout[index].Rune == r {
			foundCount += 1
			if foundCount == n {
				return index
			}
		}
		index += 1
	}
	return -1
}

func NewAnimation(input, anagram string, steps int) (*Animation, error) {
	inputRC := NewRuneCluster(input)
	anagramRC := NewRuneCluster(anagram)
	if !inputRC.Equals(anagramRC) {
		return nil, errors.New("input doesn't match anagram")
	}

	inputLC := strings.ToLower(input)
	anagramLC := strings.ToLower(anagram)
	inputLayout, inputRows := MakeLayout(inputLC, maxCols)
	anagramLayout, anagramRows := MakeLayout(anagramLC, maxCols)

	numGlyphs := max(len(inputLayout), len(anagramLayout))

	glyphs := make([]RuneGlyph, 0, numGlyphs)
	runeCounts := make(map[rune]int)
	glyphsUsed := make([]bool, len(anagramLayout))

	for _, element := range inputLayout {
		startPos := fyne.NewPos(float32(element.Col)*(glyphSize.Width+glyphSpacing), float32(element.Row)*(glyphSize.Height+glyphSpacing))
		runeCounts[element.Rune] += 1
		n := runeCounts[element.Rune]
		endPos := fyne.NewPos(-2*glyphSize.Width, -2*glyphSize.Height)
		endIndex := NthRuneIndex(anagramLayout, element.Rune, n)
		if endIndex >= 0 {
			glyphsUsed[endIndex] = true
			endPos.X = float32(anagramLayout[endIndex].Col) * (glyphSize.Width + glyphSpacing)
			endPos.Y = float32(anagramLayout[endIndex].Row) * (glyphSize.Height + glyphSpacing)
		}
		glyphs = append(glyphs, RuneGlyph{element.Rune, startPos, endPos})
	}

	for i, used := range glyphsUsed {
		if !used {
			endElement := anagramLayout[i]
			startPos := fyne.NewPos(-2*glyphSize.Width, -2*glyphSize.Height)
			endPos := fyne.NewPos(float32(endElement.Col)*(glyphSize.Width+glyphSpacing), float32(endElement.Row)*(glyphSize.Height+glyphSpacing))
			glyphs = append(glyphs, RuneGlyph{endElement.Rune, startPos, endPos})
		}
	}

	animation := Animation{glyphs, max(inputRows, anagramRows), maxCols, steps}
	return &animation, nil
}
