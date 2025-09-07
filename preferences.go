package main

import (
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	inputPulseColorKey   = "io.patenaude.karmamanager.edit_color"
	anagramPulseColorKey = "io.patenaude.karmamanager.background_color"
	pulseDurationKey     = "io.patenaude.karmamanager.pulse_duration"
	moveDurationKey      = "io.patenaude.karmamanager.move_duration"
	pauseDurationKey     = "io.patenaude.karmamanager.pause_duration"
	showGuidedTourKey    = "io.patenaude.karmamanager.show_guided_tour"
)

var (
	defaultInputPulseColor   color.Color   = color.NRGBA{R: 0, G: 255, B: 0, A: 255}
	defaultAnagramPulseColor color.Color   = color.NRGBA{R: 192, G: 0, B: 192, A: 255}
	quickPulseDuration       time.Duration = 200 * time.Millisecond
	regularPulseDuration     time.Duration = 500 * time.Millisecond
	statelyPulseDuration     time.Duration = 800 * time.Millisecond
	quickMoveDuration        time.Duration = 1000 * time.Millisecond
	regularMoveDuration      time.Duration = 1500 * time.Millisecond
	statelyMoveDuration      time.Duration = 3000 * time.Millisecond
	quickPauseDuration       time.Duration = 1000 * time.Millisecond
	regularPauseDuration     time.Duration = 1500 * time.Millisecond
	statelyPauseDuration     time.Duration = 3000 * time.Millisecond
)

type ConfigT struct {
	app fyne.App
}

var Config ConfigT

func InitConfig(app fyne.App) {
	Config.app = app
}

func (c ConfigT) MoveDuration() time.Duration {
	return c.fetchDuration(moveDurationKey, regularMoveDuration)
}

func (c ConfigT) SetMoveDuration(value time.Duration) {
	c.setDuration(moveDurationKey, value)
}

func (c ConfigT) PulseDuration() time.Duration {
	return c.fetchDuration(pulseDurationKey, regularPulseDuration)
}

func (c ConfigT) SetPulseDuration(value time.Duration) {
	c.setDuration(pulseDurationKey, value)
}

func (c ConfigT) PauseDuration() time.Duration {
	return c.fetchDuration(pauseDurationKey, regularPauseDuration)
}

func (c ConfigT) SetPauseDuration(value time.Duration) {
	c.setDuration(pauseDurationKey, value)
}

func (c ConfigT) InputPulseColor() color.Color {
	return c.fetchColor(inputPulseColorKey, defaultInputPulseColor)
}

func (c ConfigT) SetInputPulseColor(clr color.Color) {
	c.setColor(inputPulseColorKey, clr)
}

func (c ConfigT) AnagramPulseColor() color.Color {
	return c.fetchColor(anagramPulseColorKey, defaultAnagramPulseColor)
}

func (c ConfigT) SetAnagramPulseColor(clr color.Color) {
	c.setColor(anagramPulseColorKey, clr)
}

func (c ConfigT) fetchColor(key string, def color.Color) color.Color {
	attr := c.app.Preferences().IntListWithFallback(key, make([]int, 0))
	if len(attr) != 4 {
		return def
	}
	// the values we get back at 16 bit scaled, but we neeed 8 bit values
	col := color.NRGBA{R: uint8(attr[0]), G: uint8(attr[1]), B: uint8(attr[2]), A: uint8(attr[3])}
	return col
}

func (c ConfigT) setColor(key string, clr color.Color) {
	r, g, b, a := clr.RGBA()
	attr := []int{int(r), int(g), int(b), int(a)}
	c.app.Preferences().SetIntList(key, attr)
}

func (c ConfigT) fetchDuration(key string, def time.Duration) time.Duration {
	ms := int64(c.app.Preferences().IntWithFallback(key, -1))
	if ms < 0 {
		return def
	} else {
		return time.Duration(ms) * time.Millisecond
	}
}

func (c ConfigT) setDuration(key string, value time.Duration) {
	c.app.Preferences().SetInt(key, int(value.Milliseconds()))
}

func (c ConfigT) ShowPreferencesDialog() {

	pulseDurationSelector := widget.NewSelect([]string{"quick", "regular", "stately"}, nil)
	pulseDuration := c.PulseDuration()
	if pulseDuration == quickPulseDuration {
		pulseDurationSelector.SetSelectedIndex(0)
	} else if pulseDuration == statelyPulseDuration {
		pulseDurationSelector.SetSelectedIndex(2)
	} else {
		pulseDurationSelector.SetSelectedIndex(1)
	}

	moveDurationSelector := widget.NewSelect([]string{"quick", "regular", "stately"}, nil)
	moveDuration := c.MoveDuration()
	if moveDuration == quickMoveDuration {
		moveDurationSelector.SetSelectedIndex(0)
	} else if moveDuration == statelyMoveDuration {
		moveDurationSelector.SetSelectedIndex(2)
	} else {
		moveDurationSelector.SetSelectedIndex(1)
	}

	pauseDurationSelector := widget.NewSelect([]string{"quick", "regular", "stately"}, nil)
	pauseDuration := c.PauseDuration()
	if pauseDuration == quickPauseDuration {
		pauseDurationSelector.SetSelectedIndex(0)
	} else if pauseDuration == statelyPauseDuration {
		pauseDurationSelector.SetSelectedIndex(2)
	} else {
		pauseDurationSelector.SetSelectedIndex(1)
	}

	inputPulseColorPickerButton := widget.NewButtonWithIcon("Input", theme.ColorPaletteIcon(), func() {
		picker := dialog.NewColorPicker("Input Highlight Color", "", func(clr color.Color) {
			c.SetInputPulseColor(clr)
		}, MainWindow)
		picker.Advanced = true
		picker.SetColor(c.InputPulseColor())
		picker.Show()
	})

	anagramPulseColorPickerButton := widget.NewButtonWithIcon("Anagram", theme.ColorPaletteIcon(), func() {
		picker := dialog.NewColorPicker("Anagram Highlight Color", "", func(clr color.Color) {
			c.SetAnagramPulseColor(clr)
		}, MainWindow)
		picker.Advanced = true
		picker.SetColor(c.AnagramPulseColor())
		picker.Show()
	})

	entries := []*widget.FormItem{
		widget.NewFormItem("Pause speed", pauseDurationSelector),
		widget.NewFormItem("Movement speed", moveDurationSelector),
		widget.NewFormItem("Color change speed", pulseDurationSelector),
		widget.NewFormItem("Input highlight color", inputPulseColorPickerButton),
		widget.NewFormItem("Anagram highlight color", anagramPulseColorPickerButton)}

	dialog.ShowForm("Animation Preferences", "Save", "Cancel", entries, func(save bool) {
		if save {
			if pauseDurationSelector.SelectedIndex() == 0 {
				c.SetPauseDuration(quickPauseDuration)
			} else if pauseDurationSelector.SelectedIndex() == 2 {
				c.SetPauseDuration(statelyPauseDuration)
			} else {
				c.SetPauseDuration(regularPauseDuration)
			}
			if moveDurationSelector.SelectedIndex() == 0 {
				c.SetMoveDuration(quickPauseDuration)
			} else if moveDurationSelector.SelectedIndex() == 2 {
				c.SetMoveDuration(statelyPauseDuration)
			} else {
				c.SetMoveDuration(regularPauseDuration)
			}
			if pulseDurationSelector.SelectedIndex() == 0 {
				c.SetPulseDuration(quickPulseDuration)
			} else if pulseDurationSelector.SelectedIndex() == 2 {
				c.SetPulseDuration(statelyPulseDuration)
			} else {
				c.SetPulseDuration(regularPulseDuration)
			}
		}
	}, MainWindow)
}
