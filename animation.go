package main

import (
	"errors"
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
	"fyne.io/fyne/v2/driver/software"
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

func layoutCenterOffset(layout []RuneLayoutElement, dispSize fyne.Size) fyne.Position {
	maxCol, maxRow := 0, 0
	for _, e := range layout {
		if e.Col > maxCol {
			maxCol = e.Col
		}
		if e.Row > maxRow {
			maxRow = e.Row
		}
	}
	textWidth := float32(maxCol+1) * (glyphSize.Width + glyphSpacing)
	textHeight := float32(maxRow+1) * (glyphSize.Height + glyphSpacing)
	return fyne.NewPos(
		(dispSize.Width-textWidth)/2,
		(dispSize.Height-textHeight)/2,
	)
}

func NewAnimation(input string, anagrams []string, dispSize fyne.Size) (*Animation, error) {
	maxCols := int(math.Floor(float64(dispSize.Width / (glyphSize.Width + glyphSpacing))))

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

	inputOffset := layoutCenterOffset(inputLayout, dispSize)
	for _, element := range inputLayout {
		startPos := fyne.NewPos(
			inputOffset.X+float32(element.Col)*(glyphSize.Width+glyphSpacing),
			inputOffset.Y+float32(element.Row)*(glyphSize.Height+glyphSpacing),
		)
		glyphs = append(glyphs, RuneGlyph{element.Rune, startPos, make([]fyne.Position, len(anagrams))})
	}

	offscreenParking := fyne.NewPos(-2*glyphSize.Width, -2*glyphSize.Height)
	for index, anagram := range anagrams {
		anagramLC := strings.ToLower(anagram)
		anagramLayout, anagramRows := MakeRuneLayout(anagramLC, maxCols)
		if anagramRows > rows {
			rows = anagramRows
		}

		anagramOffset := layoutCenterOffset(anagramLayout, dispSize)
		glyphsUsed := make([]bool, len(glyphs))
		runeCounts := make(map[rune]int)

		for _, element := range anagramLayout {
			runeCounts[element.Rune] += 1
			n := runeCounts[element.Rune]
			stepPos := fyne.NewPos(
				anagramOffset.X+float32(element.Col)*(glyphSize.Width+glyphSpacing),
				anagramOffset.Y+float32(element.Row)*(glyphSize.Height+glyphSpacing),
			)
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
	iconImage        *canvas.Image
	badgeLabel       *widget.Label
	pendingInput     string
	pendingAnagrams  []string
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

func (ad *AnimationDisplay) setupIconAndBadge() {
	if ad.setupComplete {
		return
	}

	ad.setupComplete = true

	ad.iconImage = canvas.NewImageFromResource(ad.Icon)
	ad.iconImage.SetMinSize(fyne.NewSize(64, 64))
	ad.iconImage.FillMode = canvas.ImageFillContain
	ad.badgeLabel = widget.NewLabel(ad.Badge)

	ad.surface.RemoveAll()
	ad.surface.Add(ad.iconImage)
	ad.surface.Add(ad.badgeLabel)
	// Attempt immediate placement; Resize() will correct it once layout is applied.
	ad.positionIconBadge(ad.surface.Size())
}

func (ad *AnimationDisplay) positionIconBadge(dispSize fyne.Size) {
	if ad.iconImage == nil || dispSize.Width == 0 || dispSize.Height == 0 {
		return
	}
	iconPos := fyne.NewPos(10, dispSize.Height-ad.iconImage.MinSize().Height-10)
	ad.iconImage.Move(iconPos)
	ad.iconImage.Resize(ad.iconImage.MinSize())
	badgePos := fyne.NewPos(20+ad.iconImage.MinSize().Width, dispSize.Height-ad.badgeLabel.MinSize().Height-10)
	ad.badgeLabel.Move(badgePos)
	ad.badgeLabel.Resize(ad.badgeLabel.MinSize())
}

func (ad *AnimationDisplay) Resize(size fyne.Size) {
	ad.BaseWidget.Resize(size)
	ad.positionIconBadge(size)
	if ad.pendingInput != "" && size.Width > 0 && size.Height > 0 {
		input := ad.pendingInput
		anagrams := ad.pendingAnagrams
		ad.pendingInput = ""
		ad.pendingAnagrams = nil
		ad.startAnimation(input, anagrams, size)
	}
}

func (ad *AnimationDisplay) AnimateAnagrams(input string, anagrams ...string) {
	ad.running = true
	ad.setupIconAndBadge()

	// Use the widget's own size (set by BaseWidget.Resize during layout).
	// On platforms where layout is async (iOS), this may be zero on first call;
	// Resize() will pick up the pending animation once the real size arrives.
	size := ad.Size()
	if size.Width > 0 && size.Height > 0 {
		ad.startAnimation(input, anagrams, size)
	} else {
		ad.pendingInput = input
		ad.pendingAnagrams = anagrams
	}
}

func (ad *AnimationDisplay) startAnimation(input string, anagrams []string, dispSize fyne.Size) {
	style := fyne.TextStyle{Monospace: true}

	animation, err := NewAnimation(input, anagrams, dispSize)
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

	// moveGlyphs drives all glyph positions from a single animation so every
	// glyph is guaranteed to update on the same tick with no stagger.
	// It uses a dummy (0,0)→(1,0) animation and treats pos.X as a progress
	// value t∈[0,1] to linearly interpolate each glyph's from/to positions.
	moveGlyphs := func(fromPos, toPos func(i int) fyne.Position) {
		n := len(animation.Glyphs)
		froms := make([]fyne.Position, n)
		tos := make([]fyne.Position, n)
		for i := range animation.Glyphs {
			froms[i] = fromPos(i)
			tos[i] = toPos(i)
		}
		anim := canvas.NewPositionAnimation(
			fyne.NewPos(0, 0), fyne.NewPos(1, 0),
			Config.MoveDuration(),
			func(pos fyne.Position) {
				t := pos.X
				for i, text := range animElements {
					text.Move(fyne.NewPos(
						froms[i].X+t*(tos[i].X-froms[i].X),
						froms[i].Y+t*(tos[i].Y-froms[i].Y),
					))
				}
				if ad.CaptureCallback != nil {
					ad.CaptureCallback()
				}
			},
		)
		anim.Start()
		time.Sleep(Config.MoveDuration())
	}

	// colorPulse drives all glyph colors from a single animation for the
	// same reason — one ticker callback, no per-glyph stagger.
	colorPulse := func(c color.Color) {
		anim := canvas.NewColorRGBAAnimation(theme.TextColor(), c, Config.PulseDuration(), func(newColor color.Color) {
			for _, text := range animElements {
				text.Color = newColor
				text.Refresh()
			}
			if ad.CaptureCallback != nil {
				ad.CaptureCallback()
			}
		})
		anim.Start()
		time.Sleep(Config.PulseDuration())
		time.Sleep(Config.PauseDuration())

		anim2 := canvas.NewColorRGBAAnimation(c, theme.TextColor(), Config.PulseDuration(), func(newColor color.Color) {
			for _, text := range animElements {
				text.Color = newColor
				text.Refresh()
			}
			if ad.CaptureCallback != nil {
				ad.CaptureCallback()
			}
		})
		anim2.Start()
		time.Sleep(Config.PulseDuration())
	}

	go func() {
		for i, glyph := range animation.Glyphs {
			animElements[i].Move(glyph.StartPos)
		}
		if ad.CaptureCallback != nil {
			ad.CaptureCallback()
		}

		for ad.running {
			moveGlyphs(
				func(i int) fyne.Position { return animation.Glyphs[i].StartPos },
				func(i int) fyne.Position { return animation.Glyphs[i].StepPos[0] },
			)

			colorPulse(Config.AnagramPulseColor())

			stepIndex := 0
			for stepIndex < len(anagrams)-1 {
				si := stepIndex
				moveGlyphs(
					func(i int) fyne.Position { return animation.Glyphs[i].StepPos[si] },
					func(i int) fyne.Position { return animation.Glyphs[i].StepPos[si+1] },
				)
				colorPulse(Config.AnagramPulseColor())
				stepIndex++
			}

			si := stepIndex
			moveGlyphs(
				func(i int) fyne.Position { return animation.Glyphs[i].StepPos[si] },
				func(i int) fyne.Position { return animation.Glyphs[i].StartPos },
			)

			colorPulse(Config.InputPulseColor())
			if ad.CycleCallback != nil {
				ad.CycleCallback()
			}
		}

		// Allow any in-flight animation ticks to complete before tearing down,
		// so capture callbacks see the final frames of the last pulse.
		time.Sleep(100 * time.Millisecond)

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

type rawFrame struct {
	im    image.Image
	delay int
}

type GIFCaptureTool struct {
	rawFrames   []rawFrame
	mu          sync.Mutex
	lastCapture time.Time
	minInterval time.Duration
}

func NewGIFCaptureTool() *GIFCaptureTool {
	return &GIFCaptureTool{
		rawFrames:   make([]rawFrame, 0, 200),
		minInterval: 80 * time.Millisecond, // ~12fps
	}
}

func convertToPaletted(im image.Image) *image.Paletted {
	bounds := im.Bounds()
	pal := image.NewPaletted(bounds, palette.WebSafe)
	draw.FloydSteinberg.Draw(pal, bounds, im, image.Point{})
	return pal
}

// MakeCaptureCallback returns a rate-limited CaptureCallback that renders only
// the animation surface to an off-screen software canvas. This avoids capturing
// the whole window and keeps the heavy palette conversion out of the hot path.
func (gct *GIFCaptureTool) MakeCaptureCallback(ad *AnimationDisplay) func() {
	return func() {
		gct.mu.Lock()
		now := time.Now()
		if !gct.lastCapture.IsZero() && now.Sub(gct.lastCapture) < gct.minInterval {
			gct.mu.Unlock()
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
		gct.lastCapture = now
		gct.mu.Unlock()

		// Render just the animation surface at its current size.
		surfaceSize := ad.surface.Size()
		offC := software.NewCanvas()
		offC.SetPadded(false)
		offC.Resize(surfaceSize)
		offC.SetContent(ad.surface)
		im := offC.Capture()
		// Restore surface state in case SetContent resized/moved it.
		ad.surface.Resize(surfaceSize)
		ad.surface.Move(fyne.NewPos(0, 0))

		gct.mu.Lock()
		gct.rawFrames = append(gct.rawFrames, rawFrame{im: im, delay: delay})
		gct.mu.Unlock()
	}
}

// GetGIF converts the captured raw frames to a GIF. This is intentionally
// deferred until after the animation completes so dithering doesn't block
// the animation goroutine.
func (gct *GIFCaptureTool) GetGIF() *gif.GIF {
	gct.mu.Lock()
	defer gct.mu.Unlock()
	frames := make([]*image.Paletted, len(gct.rawFrames))
	delays := make([]int, len(gct.rawFrames))
	for i, rf := range gct.rawFrames {
		frames[i] = convertToPaletted(rf.im)
		delays[i] = rf.delay
	}
	return &gif.GIF{Image: frames, Delay: delays, LoopCount: 0}
}
