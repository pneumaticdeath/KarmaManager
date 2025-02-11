package main

import (
	"errors"
	// "fmt"
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
	Letter   rune
	StartPos fyne.Position
	StepPos  []fyne.Position
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

func NthGlyphIndex(glyphs []RuneGlyph, r rune, n int) int {
	index := 0
	foundCount := 0
	for index < len(glyphs) {
		if glyphs[index].Letter == r {
			foundCount += 1
			if foundCount == n {
				return index
			}
		}
		index += 1
	}
	return -1
}

func NewAnimation(input string, anagrams []string, maxRows, maxCols int) (*Animation, error) {
	inputRC := NewRuneCluster(input)
	for _, anagram := range anagrams {
		anagramRC := NewRuneCluster(anagram)
		if !inputRC.Equals(anagramRC) {
			return nil, errors.New("input doesn't match anagram")
		}
	}

	inputLC := strings.ToLower(input)
	inputLayout, rows := MakeRuneLayout(inputLC, maxCols)
	numGlyphs := len(inputLayout)
	glyphs := make([]RuneGlyph, 0, numGlyphs)

	for _, element := range inputLayout {
		startPos := fyne.NewPos(float32(element.Col)*(glyphSize.Width+glyphSpacing), float32(element.Row)*(glyphSize.Height+glyphSpacing))
		glyphs = append(glyphs, RuneGlyph{element.Rune, startPos, make([]fyne.Position, len(anagrams))})
	}

	offscreenParking := fyne.NewPos(-2*glyphSize.Width, -2*glyphSize.Height)
	// anagramLayouts := make([][]RuneElement, len(anagrams))
	for index, anagram := range anagrams {
		anagramLC := strings.ToLower(anagram)
		anagramLayout, anagramRows := MakeRuneLayout(anagramLC, maxCols)
		if anagramRows > rows {
			rows = anagramRows
		}

		glyphsUsed := make([]bool, len(glyphs))
		runeCounts := make(map[rune]int)

		for _, element := range anagramLayout {
			runeCounts[element.Rune] += 1
			n := runeCounts[element.Rune]
			stepPos := fyne.NewPos(
				float32(element.Col)*(glyphSize.Width+glyphSpacing),
				float32(element.Row)*(glyphSize.Height+glyphSpacing))
			glyphIndex := NthGlyphIndex(glyphs, element.Rune, n)
			if glyphIndex >= 0 {
				glyphsUsed[glyphIndex] = true
				glyphs[glyphIndex].StepPos[index] = stepPos
			} else {
				glyphsUsed = append(glyphsUsed, true)
				newGlyph := RuneGlyph{element.Rune, offscreenParking, make([]fyne.Position, len(anagrams))}
				for i := 0; i < index; i += 1 {
					newGlyph.StepPos[i] = offscreenParking
				}
				newGlyph.StepPos[index] = stepPos
				glyphs = append(glyphs, newGlyph)
			}
		}
		for i, used := range glyphsUsed {
			if !used {
				glyphs[i].StepPos[index] = offscreenParking
			}
		}
	}

	animation := Animation{glyphs, rows, maxCols}
	return &animation, nil
}

type AnimationDisplay struct {
	widget.BaseWidget

	surface            *fyne.Container
	scroll             *container.Scroll
	MoveDuration       time.Duration
	ColorCycleDuration time.Duration
	PauseDuration      time.Duration
	Icon               fyne.Resource
	Badge              string
	running            bool
}

func NewAnimationDisplay(icon fyne.Resource) *AnimationDisplay {
	surface := container.NewWithoutLayout()
	scroll := container.NewScroll(surface)
	scroll.Direction = container.ScrollNone

	ad := &AnimationDisplay{surface: surface, scroll: scroll, MoveDuration: 3 * time.Second,
		ColorCycleDuration: 500 * time.Millisecond, PauseDuration: 1500 * time.Millisecond, Icon: icon,
		Badge: "made with KarmaManager"}
	ad.ExtendBaseWidget(ad)
	return ad
}

func (ad *AnimationDisplay) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(ad.scroll)
}

func (ad *AnimationDisplay) AnimateAnagrams(input string, anagrams ...string) {
	ad.running = true
	dispSize := ad.surface.Size()
	maxCols := int(math.Floor(float64(dispSize.Width / (glyphSize.Width + glyphSpacing))))
	maxRows := int(math.Floor(float64(dispSize.Height / (glyphSize.Height + glyphSpacing))))

	icon := canvas.NewImageFromResource(ad.Icon)
	icon.SetMinSize(fyne.NewSize(64, 64))
	icon.FillMode = canvas.ImageFillContain
	badge := widget.NewLabel(ad.Badge)

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

	style := fyne.TextStyle{Monospace: true}

	animation, err := NewAnimation(input, anagrams, maxRows, maxCols)
	if err != nil {
		log.Println(err)
		ad.running = false
		return
	}
	animElements := make([]*canvas.Text, len(animation.Glyphs))
	for index, glyph := range animation.Glyphs {
		text := canvas.NewText(string(unicode.ToUpper(glyph.Letter)), theme.TextColor())
		text.TextStyle = style
		text.TextSize = textSize
		animElements[index] = text
		ad.surface.Add(text)
	}

	colorPulse := func() {
		for _, text := range animElements {
			anim := canvas.NewColorRGBAAnimation(theme.TextColor(), purple, ad.ColorCycleDuration, func(newColor color.Color) {
				text.Color = newColor
				text.Refresh()
			})
			anim.Start()
		}

		time.Sleep(ad.ColorCycleDuration)

		time.Sleep(ad.PauseDuration)

		for _, text := range animElements {
			anim := canvas.NewColorRGBAAnimation(purple, theme.TextColor(), ad.ColorCycleDuration, func(newColor color.Color) {
				text.Color = newColor
				text.Refresh()
			})
			anim.Start()
		}

		time.Sleep(ad.ColorCycleDuration)
	}

	go func() {
		for ad.running {
			// Start to first pos
			for glyphIndex, glyph := range animation.Glyphs {
				text := animElements[glyphIndex]
				anim := canvas.NewPositionAnimation(glyph.StartPos, glyph.StepPos[0], ad.MoveDuration, text.Move)
				anim.Start()
			}

			time.Sleep(ad.MoveDuration)

			colorPulse()

			// Now all the steps except the last one

			stepIndex := 0
			for stepIndex < len(anagrams)-1 {
				for glyphIndex, glyph := range animation.Glyphs {
					text := animElements[glyphIndex]
					anim := canvas.NewPositionAnimation(glyph.StepPos[stepIndex], glyph.StepPos[stepIndex+1], ad.MoveDuration, text.Move)
					anim.Start()
				}

				time.Sleep(ad.MoveDuration)

				colorPulse()

				stepIndex += 1
			}

			for index, glyph := range animation.Glyphs {
				text := animElements[index]
				anim := canvas.NewPositionAnimation(glyph.StepPos[stepIndex], glyph.StartPos, ad.MoveDuration, text.Move)
				anim.Start()
			}

			time.Sleep(ad.MoveDuration + ad.PauseDuration)
		}
	}()
}

func (ad *AnimationDisplay) Stop() {
	ad.running = false
}

func (ad *AnimationDisplay) Tapped(pe *fyne.PointEvent) {
	// nothing for now
}

func (ad *AnimationDisplay) Clear() {
	ad.surface.RemoveAll()
	ad.surface.Refresh()
}
