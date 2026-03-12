package main

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type tourStep struct {
	title  string
	text   string
	tab    int    // 0=Find, 1=Favorites
	pos    string // "top", "center", "bottom"
	action func()
}

var tourSteps []tourStep

func initTourSteps(setInput func(string)) {
	tourSteps = []tourStep{
		{
			title: "Welcome to Karma Manager!",
			text:  "This quick tour will show you around.\nYou can skip it at any time.",
			tab:   0,
			pos:   "center",
		},
		{
			title: "Search for Anagrams",
			text:  "Type a word or phrase at the top.\nResults appear live as you type!",
			tab:   0,
			pos:   "center",
			action: func() {
				text := "Karma Manager"
				go func() {
					for i := range text {
						partial := text[:i+1]
						fyne.Do(func() { setInput(partial) })
						time.Sleep(80 * time.Millisecond)
					}
				}()
			},
		},
		{
			title: "Choose Dictionaries",
			text:  "Tap the dictionary button to pick which\nword lists to use. Larger dictionaries\ngive more results.",
			tab:   0,
			pos:   "center",
		},
		{
			title: "Browse Results",
			text:  "Tap any result to save it as a favorite,\ncopy it, or see an animation.",
			tab:   0,
			pos:   "center",
		},
		{
			title: "Interesting Words",
			text:  "Tap \"Interesting words\" to see the most\ncommon words across your results.\nGreat for finding words to include\nor exclude.",
			tab:   0,
			pos:   "center",
		},
		{
			title: "Filter Results",
			text:  "Use the Include and Exclude word lists\nto narrow down your results.",
			tab:   0,
			pos:   "center",
		},
		{
			title: "Your Favorites",
			text:  "Saved anagrams appear here, grouped\nby the original input phrase.",
			tab:   1,
			pos:   "center",
		},
		{
			title: "Favorites Actions",
			text:  "Search from a favorite, edit its words,\nor watch an animation of the anagram.",
			tab:   1,
			pos:   "bottom",
		},
		{
			title: "Import & Sync",
			text:  "Import anagrams via share links, or sign\nin to sync favorites across devices.",
			tab:   1,
			pos:   "bottom",
		},
	}
}

func ShowGuidedTour(selectTab func(int), setInput func(string), window fyne.Window) {
	initTourSteps(setInput)

	current := 0
	canvas := window.Canvas()

	titleLabel := widget.NewLabel("")
	titleLabel.TextStyle = fyne.TextStyle{Bold: true}
	titleLabel.Alignment = fyne.TextAlignCenter

	bodyLabel := widget.NewLabel("")
	bodyLabel.Alignment = fyne.TextAlignCenter

	stepLabel := widget.NewLabel("")
	stepLabel.Alignment = fyne.TextAlignCenter

	backBtn := widget.NewButtonWithIcon("", theme.NavigateBackIcon(), nil)
	nextBtn := widget.NewButtonWithIcon("", theme.NavigateNextIcon(), nil)
	skipBtn := widget.NewButton("Skip", nil)

	buttons := container.New(layout.NewHBoxLayout(), backBtn, layout.NewSpacer(), skipBtn, layout.NewSpacer(), nextBtn)
	content := container.New(layout.NewVBoxLayout(), titleLabel, bodyLabel, stepLabel, buttons)

	popup := widget.NewPopUp(content, canvas)

	var showStep func()
	showStep = func() {
		step := tourSteps[current]

		titleLabel.SetText(step.title)
		bodyLabel.SetText(step.text)
		stepLabel.SetText(fmt.Sprintf("%d of %d", current+1, len(tourSteps)))

		backBtn.Disable()
		if current > 0 {
			backBtn.Enable()
		}

		if current == len(tourSteps)-1 {
			nextBtn.SetText("Done")
			nextBtn.Icon = nil
		} else {
			nextBtn.SetText("")
			nextBtn.Icon = theme.NavigateNextIcon()
		}

		selectTab(step.tab)
		positionTooltip(popup, step.pos, canvas.Size())
		popup.Show()

		if step.action != nil {
			step.action()
		}
	}

	backBtn.OnTapped = func() {
		if current > 0 {
			current--
			showStep()
		}
	}

	nextBtn.OnTapped = func() {
		if current < len(tourSteps)-1 {
			current++
			showStep()
		} else {
			popup.Hide()
		}
	}

	skipBtn.OnTapped = func() {
		popup.Hide()
	}

	// Delay so the window is fully laid out (critical on mobile)
	go func() {
		time.Sleep(500 * time.Millisecond)
		fyne.Do(showStep)
	}()
}

func positionTooltip(popup *widget.PopUp, posHint string, canvasSize fyne.Size) {
	const fixedWidth float32 = 300

	popup.Resize(fyne.NewSize(fixedWidth, popup.MinSize().Height))

	x := (canvasSize.Width - fixedWidth) / 2
	if x < 0 {
		x = 0
	}

	var y float32
	switch posHint {
	case "top":
		y = canvasSize.Height * 0.12
	case "bottom":
		y = canvasSize.Height * 0.58
	default: // "center"
		y = (canvasSize.Height - popup.MinSize().Height) / 2
	}

	popup.Move(fyne.NewPos(x, y))
}
