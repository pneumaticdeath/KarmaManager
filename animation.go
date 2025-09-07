package main

import (
	"errors"
	// "fmt"
	// "image"
	// "image/draw"
	"image/color"
	// "image/color/palette"
	// "image/gif"
	"log"
	"math"
	"strings"
	"time"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	// "fyne.io/fyne/v2/driver/software"
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
	CaptureCallback    func()
	CycleCallback      func()
	running            bool
}

func NewAnimationDisplay(icon fyne.Resource) *AnimationDisplay {
	surface := container.NewWithoutLayout()
	scroll := container.NewScroll(surface)
	scroll.Direction = container.ScrollNone

	ad := &AnimationDisplay{surface: surface, scroll: scroll, MoveDuration: 1000 * time.Millisecond,
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
	green := color.NRGBA{G: 192, A: 255}

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

	colorPulse := func(c color.Color) {
		for _, text := range animElements {
			anim := canvas.NewColorRGBAAnimation(theme.TextColor(), c, ad.ColorCycleDuration, func(newColor color.Color) {
				text.Color = newColor
				text.Refresh()
				if ad.CaptureCallback != nil {
					ad.CaptureCallback()
				}
				// fmt.Printf("Color of %s: %v\n", text.Text, newColor)
			})
			anim.Start()
		}

		time.Sleep(ad.ColorCycleDuration)

		time.Sleep(ad.PauseDuration)

		for _, text := range animElements {
			anim := canvas.NewColorRGBAAnimation(c, theme.TextColor(), ad.ColorCycleDuration, func(newColor color.Color) {
				text.Color = newColor
				text.Refresh()
				if ad.CaptureCallback != nil {
					ad.CaptureCallback()
				}
				// fmt.Printf("Color of %s: %v\n", text.Text, newColor)
			})
			anim.Start()
		}

		time.Sleep(ad.ColorCycleDuration)
	}

	go func() {
		for glyphIndex, glyph := range animation.Glyphs {
			text := animElements[glyphIndex]
			anim := canvas.NewPositionAnimation(fyne.NewPos(0, 0), glyph.StartPos, ad.MoveDuration, func(pos fyne.Position) {
				text.Move(pos)
				if ad.CaptureCallback != nil {
					ad.CaptureCallback()
				}
			})
			anim.Start()
		}

		time.Sleep(ad.MoveDuration)

		colorPulse(green)

		for ad.running {
			// Start to first pos
			for glyphIndex, glyph := range animation.Glyphs {
				text := animElements[glyphIndex]
				anim := canvas.NewPositionAnimation(glyph.StartPos, glyph.StepPos[0], ad.MoveDuration, func(pos fyne.Position) {
					text.Move(pos)
					// fmt.Printf("Move %c to %v\n", glyph.Letter, pos)
					if ad.CaptureCallback != nil {
						ad.CaptureCallback()
					}
				})
				anim.Start()
			}

			time.Sleep(ad.MoveDuration)

			colorPulse(purple)

			// Now all the steps except the last one

			stepIndex := 0
			for stepIndex < len(anagrams)-1 {
				for glyphIndex, glyph := range animation.Glyphs {
					text := animElements[glyphIndex]
					anim := canvas.NewPositionAnimation(glyph.StepPos[stepIndex], glyph.StepPos[stepIndex+1], ad.MoveDuration, func(pos fyne.Position) {
						text.Move(pos)
						if ad.CaptureCallback != nil {
							ad.CaptureCallback()
						}
						// fmt.Printf("Move %c to %v\n", glyph.Letter, pos)
					})
					anim.Start()
				}

				time.Sleep(ad.MoveDuration)

				colorPulse(purple)

				stepIndex += 1
			}

			for index, glyph := range animation.Glyphs {
				text := animElements[index]
				anim := canvas.NewPositionAnimation(glyph.StepPos[stepIndex], glyph.StartPos, ad.MoveDuration, func(pos fyne.Position) {
					text.Move(pos)
					if ad.CaptureCallback != nil {
						ad.CaptureCallback()
					}
					// fmt.Printf("Move %c to %v\n", glyph.Letter, pos)
				})
				anim.Start()
			}

			time.Sleep(ad.MoveDuration)

			colorPulse(green)
			// fmt.Println("Cycle completed")
			if ad.CycleCallback != nil {
				ad.CycleCallback()
			}
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

/*
func convertToPaletted(im image.Image) *image.Paletted {
	bounds := im.Bounds()
	pal := image.NewPaletted(bounds, palette.WebSafe)
	draw.Draw(pal, bounds, im, image.Point{}, draw.Src)

	return pal
}

type GIFCaptureTool struct {
	Frames      []*image.Paletted
	Delays      []int
	Running     bool
	lastCapture time.Time
}

func (gct *GIFCaptureTool) Done() bool {
	return !gct.Running
}

func (gct *GIFCaptureTool) GetGIF() *gif.GIF {
	return  &gif.GIF{Image: gct.Frames, Delay: gct.Delays, LoopCount: 0}
}

func MakeAnimatedGIF(complete func()) (*AnimationDisplay, *GIFCaptureTool) {
	gct := &GIFCaptureTool{}
	gct.Running = true
	ad := NewAnimationDisplay(Icon)
	// gct.ad.MoveDuration = 600*time.Millisecond
	// gct.ad.ColorCycleDuration = 100*time.Millisecond
	// gct.ad.PauseDuration = 300*time.Millisecond
	ad.CycleCallback = func() {
		ad.Stop()
		gct.Running = false
		complete()
	}
	gct.Frames = make([]*image.Paletted, 0, 200)
	gct.Delays = make([]int, 0, 200)
	ad.CaptureCallback = func() {
		delay := min(50,max(1, int(time.Since(gct.lastCapture).Milliseconds())/10))
		im := software.Render(ad, fyne.CurrentApp().Settings().Theme())
		gct.Frames = append(gct.Frames, convertToPaletted(im))
		gct.Delays = append(gct.Delays, delay)
		gct.lastCapture = time.Now()
	}

	// ad.SetMinSize(fyne.NewSize(600, 350))
	// ad.Resize(ad.MinSize())
	// gct.lastCapture = time.Now()
	return ad, gct
}
*/
