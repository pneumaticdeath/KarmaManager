package main

import (
	"errors"
	// "image"
	// "image/color"
	// "image/gif"
	"log"
	"math"
	"strings"
	"time"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var glyphSize fyne.Size = fyne.NewSize(20, 20)
var glyphSpacing float32 = 1.0

type RuneGlyph struct {
	Letter           rune
	StartPos, EndPos fyne.Position
}

type Animation struct {
	Glyphs     []RuneGlyph
	Rows, Cols int
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

func NewAnimation(input, anagram string, maxRows, maxCols int) (*Animation, error) {
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

	animation := Animation{glyphs, max(inputRows, anagramRows), maxCols}
	return &animation, nil
}

type AnimationDisplay struct {
	widget.BaseWidget

	surface    *fyne.Container
	scroll     *container.Scroll
	animations []*fyne.Animation
	Duration   time.Duration
	Icon       fyne.Resource
	Badge      string
	running    bool
}

func NewAnimationDisplay(icon fyne.Resource) *AnimationDisplay {
	surface := container.NewWithoutLayout()
	scroll := container.NewScroll(surface)
	scroll.Direction = container.ScrollNone

	ad := &AnimationDisplay{surface: surface, scroll: scroll, Duration: 6*time.Second, Icon: icon, Badge: "made with KarmaManager"}
	ad.ExtendBaseWidget(ad)
	return ad
}

func (ad *AnimationDisplay) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(ad.scroll)
}

func (ad *AnimationDisplay) AnimateAnagram(input, anagram string) {
	dispSize := ad.surface.Size()
	maxCols := int(math.Floor(float64(dispSize.Width / (glyphSize.Width + glyphSpacing))))
	maxRows := int(math.Floor(float64(dispSize.Height / (glyphSize.Height + glyphSpacing))))

	// icon := canvas.NewImageFromResource(ad.Icon)
	// icon.SetMinSize(fyne.NewSize(32, 32))
	// icon.FillMode = canvas.ImageFillContain
	badge := widget.NewLabel(ad.Badge)

	animation, err := NewAnimation(input, anagram, maxRows, maxCols)

	if err != nil {
		log.Println(err)
		return
	}

	numGlyphs := len(animation.Glyphs)
	ad.animations = make([]*fyne.Animation, numGlyphs + 1)
	ad.surface.RemoveAll()
	style := fyne.TextStyle{Monospace: true}
	for index, glyph := range animation.Glyphs {
		text := canvas.NewText(string(unicode.ToUpper(glyph.Letter)), theme.TextColor())
		text.TextStyle = style
		text.TextSize = 20.0
		ad.surface.Add(text)
		anim := canvas.NewPositionAnimation(glyph.StartPos, glyph.EndPos, ad.Duration, text.Move)
		anim.AutoReverse = true
		anim.RepeatCount = fyne.AnimationRepeatForever
		anim.Start()
		ad.animations[index] = anim
	}
	// ad.surface.Add(icon)
	ad.surface.Add(badge)
	// iconStartPos := fyne.NewPos(dispSize.Width, dispSize.Height)
	// iconEndPos := fyne.NewPos(10, dispSize.Height - icon.MinSize().Height - 10)
	// iconAnim := canvas.NewPositionAnimation(iconStartPos, iconEndPos, ad.Duration, func(p fyne.Position) {
	// 	icon.Move(p)
	// 	ad.surface.Refresh()
	// })
	// iconAnim.AutoReverse = true
	// iconAnim.RepeatCount = fyne.AnimationRepeatForever
	// icon.Move(iconStartPos)
	// icon.Move(iconEndPos)
	// icon.Resize(icon.MinSize())
	badgeStartPos := fyne.NewPos(0-badge.MinSize().Width-10, dispSize.Height)
	// badgeEndPos := fyne.NewPos(10 + icon.MinSize().Width + 10, dispSize.Height - badge.MinSize().Height - 10)
	badgeEndPos := fyne.NewPos(10, dispSize.Height - badge.MinSize().Height - 10)
	//  badge.Move(badgeEndPos)
	// badge.Resize(badge.MinSize())
	badgeAnim := canvas.NewPositionAnimation(badgeStartPos, badgeEndPos, ad.Duration, func(p fyne.Position) {
		badge.Move(p)
	})
	badgeAnim.AutoReverse = true
	badgeAnim.RepeatCount = fyne.AnimationRepeatForever
	// iconAnim.Start()
	badgeAnim.Start()
	// ad.animations[numGlyphs] = iconAnim
	// ad.animations[numGlyphs+1] = iconAnim
	// ad.animations[numGlyphs+1] = badgeAnim
	ad.animations[numGlyphs] = badgeAnim

	ad.running = true
}

func (ad *AnimationDisplay) Start() {
	for _, anim := range ad.animations {
		anim.Start()
	}
	ad.running = true
}

func (ad *AnimationDisplay) Stop() {
	for _, anim := range ad.animations {
		anim.Stop()
	}
	ad.running = false
}

func (ad *AnimationDisplay) Tapped(pe *fyne.PointEvent) {
	if ad.running {
		ad.Stop()
	} else {
		ad.Start()
	}
}

func (ad *AnimationDisplay) Clear() {
	ad.animations = make([]*fyne.Animation, 0)
	ad.surface.RemoveAll()
	ad.surface.Refresh()
}
