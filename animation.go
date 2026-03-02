package main

import (
	"errors"
	// "fmt"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"log"
	"math"
	"strings"
	"sync"
	"time"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var glyphSize fyne.Size = fyne.NewSize(15, 20)
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

	surface          *fyne.Container
	scroll           *container.Scroll
	Icon             fyne.Resource
	Badge            string
	CaptureCallback  func()
	CycleCallback    func()
	FinishedCallback func()
	running          bool
	setupComplete    bool
}

func NewAnimationDisplay(icon fyne.Resource) *AnimationDisplay {
	surface := container.NewWithoutLayout()
	scroll := container.NewScroll(surface)
	scroll.Direction = container.ScrollNone

	ad := &AnimationDisplay{surface: surface, scroll: scroll, Icon: icon, Badge: "made with KarmaManager"}
	ad.ExtendBaseWidget(ad)
	return ad
}

func (ad *AnimationDisplay) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(ad.scroll)
}

func (ad *AnimationDisplay) setupIconAndBadge(dispSize fyne.Size) {
	if ad.setupComplete {
		return
	}

	ad.setupComplete = true

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
}

func (ad *AnimationDisplay) AnimateAnagrams(input string, anagrams ...string) {
	ad.running = true
	dispSize := ad.surface.Size()
	maxCols := int(math.Floor(float64(dispSize.Width / (glyphSize.Width + glyphSpacing))))
	maxRows := int(math.Floor(float64(dispSize.Height / (glyphSize.Height + glyphSpacing))))

	ad.setupIconAndBadge(dispSize)

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
			anim := canvas.NewColorRGBAAnimation(theme.TextColor(), c, Config.PulseDuration(), func(newColor color.Color) {
				text.Color = newColor
				text.Refresh()
				if ad.CaptureCallback != nil {
					ad.CaptureCallback()
				}
				// fmt.Printf("Color of %s: %v\n", text.Text, newColor)
			})
			anim.Start()
		}

		time.Sleep(Config.PulseDuration())

		time.Sleep(Config.PauseDuration())

		for _, text := range animElements {
			anim := canvas.NewColorRGBAAnimation(c, theme.TextColor(), Config.PulseDuration(), func(newColor color.Color) {
				text.Color = newColor
				text.Refresh()
				if ad.CaptureCallback != nil {
					ad.CaptureCallback()
				}
				// fmt.Printf("Color of %s: %v\n", text.Text, newColor)
			})
			anim.Start()
		}

		time.Sleep(Config.PulseDuration())
	}

	go func() {
		for glyphIndex, glyph := range animation.Glyphs {
			text := animElements[glyphIndex]
			anim := canvas.NewPositionAnimation(fyne.NewPos(0, 0), glyph.StartPos, Config.MoveDuration(), func(pos fyne.Position) {
				text.Move(pos)
				if ad.CaptureCallback != nil {
					ad.CaptureCallback()
				}
			})
			anim.Start()
		}

		time.Sleep(Config.MoveDuration())

		colorPulse(Config.InputPulseColor())

		for ad.running {
			// Start to first pos
			for glyphIndex, glyph := range animation.Glyphs {
				text := animElements[glyphIndex]
				anim := canvas.NewPositionAnimation(glyph.StartPos, glyph.StepPos[0], Config.MoveDuration(), func(pos fyne.Position) {
					text.Move(pos)
					// fmt.Printf("Move %c to %v\n", glyph.Letter, pos)
					if ad.CaptureCallback != nil {
						ad.CaptureCallback()
					}
				})
				anim.Start()
			}

			time.Sleep(Config.MoveDuration())

			colorPulse(Config.AnagramPulseColor())

			// Now all the steps except the last one

			stepIndex := 0
			for stepIndex < len(anagrams)-1 {
				for glyphIndex, glyph := range animation.Glyphs {
					text := animElements[glyphIndex]
					anim := canvas.NewPositionAnimation(glyph.StepPos[stepIndex], glyph.StepPos[stepIndex+1], Config.MoveDuration(), func(pos fyne.Position) {
						text.Move(pos)
						if ad.CaptureCallback != nil {
							ad.CaptureCallback()
						}
						// fmt.Printf("Move %c to %v\n", glyph.Letter, pos)
					})
					anim.Start()
				}

				time.Sleep(Config.MoveDuration())

				colorPulse(Config.AnagramPulseColor())

				stepIndex += 1
			}

			for index, glyph := range animation.Glyphs {
				text := animElements[index]
				anim := canvas.NewPositionAnimation(glyph.StepPos[stepIndex], glyph.StartPos, Config.MoveDuration(), func(pos fyne.Position) {
					text.Move(pos)
					if ad.CaptureCallback != nil {
						ad.CaptureCallback()
					}
					// fmt.Printf("Move %c to %v\n", glyph.Letter, pos)
				})
				anim.Start()
			}

			time.Sleep(Config.MoveDuration())

			colorPulse(Config.InputPulseColor())
			// fmt.Println("Cycle completed")
			if ad.CycleCallback != nil {
				ad.CycleCallback()
			}
		}

		for glyphIndex, glyph := range animation.Glyphs {
			text := animElements[glyphIndex]
			anim := canvas.NewPositionAnimation(glyph.StartPos, fyne.NewPos(0, 0), Config.MoveDuration(), func(pos fyne.Position) {
				text.Move(pos)
				if ad.CaptureCallback != nil {
					ad.CaptureCallback()
				}
			})
			anim.Start()
		}

		time.Sleep(Config.MoveDuration())

		for _, obj := range animElements {
			ad.surface.Remove(obj)
		}

		if ad.FinishedCallback != nil {
			ad.FinishedCallback()
		}
	}()
}

func (ad *AnimationDisplay) Stop() {
	ad.running = false
}

func (ad *AnimationDisplay) Tapped(pe *fyne.PointEvent) {
	ad.Stop()
}

func (ad *AnimationDisplay) Clear() {
	ad.surface.RemoveAll()
	ad.surface.Refresh()
}

type GIFCaptureTool struct {
	Frames      []*image.Paletted
	Delays      []int
	mu          sync.Mutex
	lastCapture time.Time
	minInterval time.Duration
}

func NewGIFCaptureTool() *GIFCaptureTool {
	return &GIFCaptureTool{
		Frames:      make([]*image.Paletted, 0, 200),
		Delays:      make([]int, 0, 200),
		minInterval: 80 * time.Millisecond, // ~12fps
	}
}

func convertToPaletted(im image.Image) *image.Paletted {
	bounds := im.Bounds()
	pal := image.NewPaletted(bounds, palette.WebSafe)
	draw.FloydSteinberg.Draw(pal, bounds, im, image.Point{})
	return pal
}

// MakeCaptureCallback returns a rate-limited CaptureCallback that captures from
// the live window canvas at full resolution.
func (gct *GIFCaptureTool) MakeCaptureCallback(c fyne.Canvas) func() {
	return func() {
		gct.mu.Lock()
		defer gct.mu.Unlock()
		now := time.Now()
		if !gct.lastCapture.IsZero() && now.Sub(gct.lastCapture) < gct.minInterval {
			return
		}
		delay := 8 // default: 80ms in GIF centiseconds
		if !gct.lastCapture.IsZero() {
			delay = int(now.Sub(gct.lastCapture).Milliseconds() / 10)
			if delay < 1 {
				delay = 1
			}
			if delay > 500 {
				delay = 500
			}
		}
		im := c.Capture()
		gct.Frames = append(gct.Frames, convertToPaletted(im))
		gct.Delays = append(gct.Delays, delay)
		gct.lastCapture = now
	}
}

func (gct *GIFCaptureTool) GetGIF() *gif.GIF {
	gct.mu.Lock()
	defer gct.mu.Unlock()
	return &gif.GIF{Image: gct.Frames, Delay: gct.Delays, LoopCount: 0}
}
