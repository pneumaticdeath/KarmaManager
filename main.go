package main

import (
	"errors"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

const searchLimit = 100000

func main() {
	App := app.NewWithID("io.patenaude.karmamanager")
	Window := App.NewWindow("Karma Manger")

	mainDicts, addedDicts, err := ReadDictionaries()
	if err != nil {
		panic(err)
	}

	var mainDictNames []string = make([]string, len(mainDicts))
	for i, d := range mainDicts {
		mainDictNames[i] = d.Name
	}

	resultSet := NewResultSet(mainDicts, addedDicts, 0)

	mainSelect := widget.NewSelect(mainDictNames, func(dictName string) {
		for i, n := range mainDictNames {
			if dictName == n {
				resultSet.SetMainIndex(i)
				resultSet.Regenerate()
				Window.SetTitle(resultSet.CombinedDictName())
				return
			}
		}
		dialog.ShowError(errors.New("Can't find selected main dictionary"), Window)
	})
	mainSelect.SetSelectedIndex(0)

	addedChecks := make([]fyne.CanvasObject, len(addedDicts))
	for i, ad := range addedDicts {
		enabled := &ad.Enabled // copy a pointer to an address
		check := widget.NewCheck(ad.Name, func(checked bool) {
			*enabled = checked
			resultSet.Regenerate()
			Window.SetTitle(resultSet.CombinedDictName())
		})
		check.Checked = ad.Enabled
		addedChecks[i] = check
	}
	addedDictsContainer := container.New(layout.NewHBoxLayout(), addedChecks...)

	inputEntry := widget.NewEntry()
	inputEntry.OnSubmitted = func(input string) {
		resultSet.FindAnagrams(input)
	}

	controlBar := container.New(layout.NewAdaptiveGridLayout(3), inputEntry, mainSelect, addedDictsContainer)

	resultsDisplay := widget.NewList(func() int { // list length
		return resultSet.Count()
	}, func() fyne.CanvasObject { // Make new entry
		return widget.NewLabel("Foo")
	}, func(index int, object fyne.CanvasObject) { // Update entry
		label, ok := object.(*widget.Label)
		if !ok {
			return
		}
		text, _ := resultSet.GetAt(index)
		label.Text = fmt.Sprintf("%10d %s", index+1, text)
		// fmt.Println(index, " ", text)
		object.Refresh()
	})

	lastSearchIndex := -1
	lastSearchText := ""
	searchError := widget.NewLabel("")
	searchbox := widget.NewEntry()
	searchbox.OnSubmitted = func(searchfor string) {
		searchError.Text = ""
		searchError.Refresh()
		if searchfor == "" {
			lastSearchIndex = -1
			return
		}
		searchstring := strings.ToLower(searchfor)
		searchError.Text = fmt.Sprintf("Searching for '%s'", searchstring)
		searchError.Refresh()
		i := 0
		if lastSearchText == searchstring {
			i = lastSearchIndex + 1
		} else {
			lastSearchText = searchstring
			lastSearchIndex = -1
		}
		for i < resultSet.Count() && i < searchLimit {
			text, _ := resultSet.GetAt(i)
			if strings.Index(strings.ToLower(text), searchstring) != -1 {
				resultsDisplay.ScrollTo(i)
				resultsDisplay.Refresh()
				searchError.Text = fmt.Sprintf("Found '%s' at %d", searchstring, i+1)
				searchError.Refresh()
				lastSearchIndex = i
				return
			}
			i += 1
		}
		if resultSet.IsDone() && i >= resultSet.Count() {
			if lastSearchIndex >= 0 {
				searchError.Text = fmt.Sprintf("'%s' not found after %d, starting over.",
					searchstring, lastSearchIndex+1)
				lastSearchIndex = -1
			} else {
				searchError.Text = "Not Found!"
			}
		} else if i >= searchLimit {
			searchError.Text = fmt.Sprintf("Didn't find '%s' in the first %d results!", searchstring, searchLimit)
			lastSearchIndex = i
		}
		searchError.Refresh()
	}

	advControls := container.New(layout.NewVBoxLayout(), widget.NewLabel("Search for:"), searchbox, searchError)

	mainDisplay := container.New(layout.NewAdaptiveGridLayout(2), resultsDisplay, advControls)

	content := container.NewBorder(controlBar, nil, nil, nil, mainDisplay)

	Window.SetContent(content)

	Window.Resize(fyne.NewSize(800, 600))
	Window.ShowAndRun()
}
