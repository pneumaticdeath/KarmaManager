package main

import (
	"errors"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
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

	reset := func() {
		resultSet.Regenerate()
	}

	reset_search := func() {
	}

	mainSelect := widget.NewSelect(mainDictNames, func(dictName string) {
		for i, n := range mainDictNames {
			if dictName == n {
				resultSet.SetMainIndex(i)
				reset()
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
			reset()
			Window.SetTitle(resultSet.CombinedDictName())
		})
		check.Checked = ad.Enabled
		addedChecks[i] = check
	}
	addedDictsContainer := container.New(layout.NewHBoxLayout(), addedChecks...)

	inputEntry := widget.NewEntry()
	inputEntry.OnSubmitted = func(input string) {
		reset()
		reset_search()
		resultSet.FindAnagrams(input)
	}

	inputSubmitButton := widget.NewButton("Find Anagrams", func() {
		inputEntry.OnSubmitted(inputEntry.Text)
	})

	controlBar := container.New(layout.NewAdaptiveGridLayout(4), inputEntry, inputSubmitButton,
		mainSelect, addedDictsContainer)

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

	searchresultslist := binding.NewIntList()

	searchError := widget.NewLabel("")
	searchbox := widget.NewEntry()
	searchbox.OnSubmitted = func(searchfor string) {
		searchresultslist.Set(make([]int, 0, 10))
		searchError.Text = ""
		searchError.Refresh()
		if searchfor == "" {
			return
		}
		searchstring := strings.ToLower(searchfor)
		searchError.Text = fmt.Sprintf("Filtering for '%s'", searchstring)
		searchError.Refresh()
		i := 0
		count := 0
		for i < resultSet.Count() && i < searchLimit {
			text, _ := resultSet.GetAt(i)
			if strings.Index(strings.ToLower(text), searchstring) != -1 {
				count += 1
				searchresultslist.Append(i)
				// resultsDisplay.ScrollTo(i)
				// resultsDisplay.Refresh()
				searchError.Text = fmt.Sprintf("Found %d matching anagrams.", count)
				searchError.Refresh()
			}
			i += 1
		}
		if count == 0 && resultSet.IsDone() && i >= resultSet.Count() {
			searchError.Text = "Not Found!"
			searchError.Refresh()
		} else if count == 0 && i >= searchLimit {
			searchError.Text = fmt.Sprintf("Didn't find '%s' in the first %d results!", searchstring, searchLimit)
			searchError.Refresh()
		}
	}
	searchbutton := widget.NewButton("Filter results", func() {
		searchbox.OnSubmitted(searchbox.Text)
	})
	searchresults := widget.NewListWithData(searchresultslist, func() fyne.CanvasObject {
		return widget.NewLabel("")
	}, func(item binding.DataItem, obj fyne.CanvasObject) {
		label, labelok := obj.(*widget.Label)
		if !labelok {
			dialog.ShowError(errors.New("Couldn't cast searchresult label"), Window)
			return
		}
		boundint, intok := item.(binding.Int)
		if !intok {
			dialog.ShowError(errors.New("Couldn't cast dataItem to Int"), Window)
			return
		}
		id, _ := boundint.Get()
		text, _ := resultSet.GetAt(id)
		label.Text = fmt.Sprintf("%10d %s", id+1, text)
		label.Refresh()
	})
	searchresults.OnSelected = func(id widget.ListItemID) {
		resultid, _ := searchresultslist.GetValue(id)
		resultsDisplay.ScrollTo(resultid)
		resultsDisplay.Refresh()
	}

	searchcontainer := container.New(layout.NewGridLayout(3), widget.NewLabel("Filter by:"), searchbox, searchbutton)
	searchcontrols := container.NewBorder(
		container.New(layout.NewVBoxLayout(), searchcontainer, searchError), nil, nil, nil, searchresults)

	mainDisplay := container.New(layout.NewAdaptiveGridLayout(2), resultsDisplay, searchcontrols)

	content := container.NewBorder(controlBar, nil, nil, nil, mainDisplay)

	Window.SetContent(content)

	reset = func() {
		resultSet.Regenerate()
		// searchbox.Text = ""
		searchError.Text = ""
		searchresultslist.Set(make([]int, 0))
		resultsDisplay.ScrollToTop()
		content.Refresh()
	}

	reset_search = func() {
		searchbox.Text = ""
		searchbox.Refresh()
	}

	Window.Resize(fyne.NewSize(800, 600))
	Window.ShowAndRun()
}
