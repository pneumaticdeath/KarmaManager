package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

const searchLimit = 1000000

type FavoriteAnagrams struct {
	Dictionaries, Input string
	Anagrams            []string
}

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

	inputdata := binding.NewString()
	inputEntry := widget.NewEntryWithData(inputdata)
	inputEntry.OnSubmitted = func(input string) {
		reset()
		reset_search()
		resultSet.FindAnagrams(input)
	}
	inputdata.AddListener(binding.NewDataListener(func() {
		inputEntry.OnSubmitted(inputEntry.Text)
	}))

	inputClearButton := widget.NewButton("Clear input", func() {
		inputdata.Set("")
		resultSet.FindAnagrams("")
		reset()
		reset_search()
	})

	inputBar := container.New(layout.NewAdaptiveGridLayout(2), inputEntry, inputClearButton)
	dictionaryBar := container.New(layout.NewAdaptiveGridLayout(2), mainSelect, addedDictsContainer)

	controlBar := container.New(layout.NewVBoxLayout(), inputBar, dictionaryBar)

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
		} else if i >= searchLimit {
			if count == 0 {
				searchError.Text = fmt.Sprintf("Didn't find '%s' in the first %d results!", searchstring, searchLimit)
			} else {
				searchError.Text = fmt.Sprintf("Found %d instances in the first %d results!", count, searchLimit)
			}
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
	searchcontrols := container.NewBorder(container.New(layout.NewVBoxLayout(), searchcontainer, searchError), nil, nil, nil, searchresults)

	inclusiondata := binding.NewString()
	inclusiondata.Set("")
	inclusionentry := widget.NewEntryWithData(inclusiondata)
	inclusionentry.MultiLine = true
	inclusionentry.Validator = func(input string) error {
		rc := NewRuneCluster(inputEntry.Text)
		phrases := strings.Split(input, "\n")
		for index, phrase := range phrases {
			phraseRC := NewRuneCluster(phrase)
			if !phraseRC.SubSetOf(rc) {
				return errors.New(fmt.Sprintf("Line %d not a subset of the input", index+1))
			}
		}
		return nil
	}
	inclusiondata.AddListener(binding.NewDataListener(func() {
		included, _ := inclusiondata.Get()
		includedphrases := strings.Split(included, "\n")
		resultSet.SetInclusions(includedphrases)
		resultSet.Regenerate()
		resultsDisplay.Refresh()
	}))

	exclusiondata := binding.NewString()
	exclusiondata.AddListener(binding.NewDataListener(func() {
		exclusions, _ := exclusiondata.Get()
		excludedwords := strings.Split(exclusions, " ")
		resultSet.SetExclusions(excludedwords)
		resultSet.Regenerate()
		resultsDisplay.Refresh()
	}))
	exclusionentry := widget.NewEntryWithData(exclusiondata)
	exclusioncontainer := container.New(layout.NewVBoxLayout(), widget.NewLabel("Excluded words"), exclusionentry)
	advancedcontainer := container.NewBorder(widget.NewLabel("Include phrases"), exclusioncontainer, nil, nil, inclusionentry)
	controltabs := container.NewAppTabs(container.NewTabItem("Filter", searchcontrols), container.NewTabItem("Advanced", advancedcontainer))
	mainDisplay := container.New(layout.NewAdaptiveGridLayout(2), resultsDisplay, controltabs)

	resultsDisplay.OnSelected = func(id widget.ListItemID) {
		text, _ := resultSet.GetAt(id)
		copyToCBMI := fyne.NewMenuItem("Copy to clipboard", func() {
			Window.Clipboard().SetContent(text)
			label := widget.NewLabel("Copied to clipboard")
			pu := widget.NewPopUp(label, Window.Canvas())
			wsize := Window.Canvas().Size()
			// lsize := label.Size()
			pu.Move(fyne.NewPos((wsize.Width)/2, (wsize.Height)/2))
			pu.Show()
			go func() {
				time.Sleep(3 * time.Second)
				pu.Hide()
			}()
		})
		words := strings.Split(text, " ")
		filterMIs := make([]*fyne.MenuItem, len(words))
		excludeMIs := make([]*fyne.MenuItem, len(words))
		for index, word := range words {
			filterMIs[index] = fyne.NewMenuItem(word, func() {
				searchbox.Text = word
				searchbox.Refresh()
				searchbox.OnSubmitted(searchbox.Text)
			})
			excludeMIs[index] = fyne.NewMenuItem(word, func() {
				existing, _ := exclusiondata.Get()
				if existing == "" {
					exclusiondata.Set(word)
				} else {
					exclusiondata.Set(existing + " " + word)
				}
			})
		}
		filtermenu := fyne.NewMenu("Filter by", filterMIs...)
		exclusionmenu := fyne.NewMenu("Exclude", excludeMIs...)
		filterMI := fyne.NewMenuItem("Filter by", nil)
		filterMI.ChildMenu = filtermenu
		excludeMI := fyne.NewMenuItem("Exclude", nil)
		excludeMI.ChildMenu = exclusionmenu
		pumenu := fyne.NewMenu("Pop up", copyToCBMI, filterMI, excludeMI)
		rdsize := resultsDisplay.Size()
		widget.ShowPopUpMenuAtRelativePosition(pumenu, Window.Canvas(), fyne.NewPos(rdsize.Width/3, rdsize.Height/3), resultsDisplay)
		go func() {
			time.Sleep(15 * time.Second)
			resultsDisplay.UnselectAll()
		}()
	}

	content := container.NewBorder(controlBar, nil, nil, nil, mainDisplay)

	Window.SetContent(content)

	reset = func() {
		resultSet.Regenerate()
		searchError.Text = ""
		searchresultslist.Set(make([]int, 0))
		resultsDisplay.ScrollToTop()
		content.Refresh()
	}

	reset_search = func() {
		searchbox.Text = ""
		searchbox.Refresh()
		inclusiondata.Set("")
		exclusiondata.Set("")
	}

	Window.Resize(fyne.NewSize(800, 600))
	Window.ShowAndRun()
}
