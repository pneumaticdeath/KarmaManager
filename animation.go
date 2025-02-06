package main

import (
	"errors"
	"fmt"
	// "image"
	"image/color"
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
var textSize float32 = 20.0
var glyphSpacing float32 = 1.0

type RuneGlyph struct {
	Letter           rune
	StartPos, EndPos fyne.Position
}

type Animation struct {
	Glyphs     []RuneGlyph
	Rows, Cols int
}

func NthRuneIndex(layout []RuneLayoutElement, r rune, n int) int {
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
	inputLayout, inputRows := MakeRuneLayout(inputLC, maxCols)
	anagramLayout, anagramRows := MakeRuneLayout(anagramLC, maxCols)

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

	surface            *fyne.Container
	scroll             *container.Scroll
	animations         []*fyne.Animation
	MoveDuration       time.Duration
	ColorCycleDuration time.Duration
	Icon               fyne.Resource
	Badge              string
	running            bool
}

func NewAnimationDisplay(icon fyne.Resource) *AnimationDisplay {
	surface := container.NewWithoutLayout()
	scroll := container.NewScroll(surface)
	scroll.Direction = container.ScrollNone

	ad := &AnimationDisplay{surface: surface, scroll: scroll, MoveDuration: 6 * time.Second, ColorCycleDuration: time.Second, Icon: icon, Badge: "made with KarmaManager"}
	ad.ExtendBaseWidget(ad)
	return ad
}

func (ad *AnimationDisplay) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(ad.scroll)
}

func (ad *AnimationDisplay) AnimateAnagram(input, anagram string) {
	ad.running = true
	dispSize := ad.surface.Size()
	maxCols := int(math.Floor(float64(dispSize.Width / (glyphSize.Width + glyphSpacing))))
	maxRows := int(math.Floor(float64(dispSize.Height / (glyphSize.Height + glyphSpacing))))

	icon := canvas.NewImageFromResource(ad.Icon)
	icon.SetMinSize(fyne.NewSize(64, 64))
	icon.FillMode = canvas.ImageFillContain
	badge := widget.NewLabel(ad.Badge)

	animation, err := NewAnimation(input, anagram, maxRows, maxCols)

	if err != nil {
		log.Println(err)
		ad.running = false
		return
	}

	ad.surface.RemoveAll()
	// Add icon and badging
	ad.surface.Add(icon)
	ad.surface.Add(badge)
	iconPos := fyne.NewPos(10, dispSize.Height-icon.MinSize().Height-10)
	icon.Move(iconPos)
	icon.Resize(icon.MinSize())
	badgePos := fyne.NewPos(20+icon.MinSize().Width, dispSize.Height-badge.MinSize().Height-10)
	badge.Move(badgePos)
	badge.Resize(badge.MinSize())

	purple := color.NRGBA{R: 192, B: 192, A: 255}

	numGlyphs := len(animation.Glyphs)
	animElements := make([]*canvas.Text, numGlyphs)
	ad.animations = make([]*fyne.Animation, numGlyphs)
	style := fyne.TextStyle{Monospace: true}
	for index, glyph := range animation.Glyphs {
		text := canvas.NewText(string(unicode.ToUpper(glyph.Letter)), theme.TextColor())
		text.TextStyle = style
		text.TextSize = textSize
		animElements[index] = text
		ad.surface.Add(text)
	}

	go func() {
		for ad.running {
			for index, glyph := range animation.Glyphs {
				text := animElements[index]
				anim := canvas.NewPositionAnimation(glyph.StartPos, glyph.EndPos, ad.MoveDuration, text.Move)
				anim.Start()
				ad.animations[index] = anim
			}

			time.Sleep(ad.MoveDuration)

			for index, text := range animElements {
				anim := canvas.NewColorRGBAAnimation(theme.TextColor(), purple, ad.ColorCycleDuration, func(newColor color.Color) {
					text.Color = newColor
					text.Refresh()
				})
				anim.AutoReverse = true
				anim.Start()
				ad.animations[index] = anim
			}

			time.Sleep(2 * ad.ColorCycleDuration)

			for index, glyph := range animation.Glyphs {
				text := animElements[index]
				anim := canvas.NewPositionAnimation(glyph.EndPos, glyph.StartPos, ad.MoveDuration, text.Move)
				anim.Start()
				ad.animations[index] = anim
			}

			time.Sleep(ad.MoveDuration + 500*time.Millisecond)

			if ad.running {
				fmt.Println("Animation starting over")
			} else {
				fmt.Println("Animation exiting")
			}
		}
	}()
}

/*
func (ad *AnimationDisplay) Start() {
	for _, anim := range ad.animations {
		anim.Start()
	}
	ad.running = true
}
*/

func (ad *AnimationDisplay) Stop() {
	for _, anim := range ad.animations {
		anim.Stop()
	}
	ad.running = false
}

func (ad *AnimationDisplay) Tapped(pe *fyne.PointEvent) {
	// nothing for now
}

func (ad *AnimationDisplay) Clear() {
	ad.animations = make([]*fyne.Animation, 0)
	ad.surface.RemoveAll()
	ad.surface.Refresh()
}
