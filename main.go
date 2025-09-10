package main

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var searchtimeout time.Duration = time.Second
var searchlimit int = 50000
var MainWindow fyne.Window
var Icon fyne.Resource
var AppPreferences fyne.Preferences
var RebuildFavorites func()

func ShowPrivateDictSettings(private *Dictionary, window fyne.Window) {
	wl := NewWordList(private.Words)
	d := dialog.NewCustom("Private words", "submit", wl, window)
	d.Resize(fyne.NewSize(300, 500))
	addbutton := widget.NewButton("Add", func() {
		wl.ShowAddWord("Add word", "Add", "Cancel", nil, window)
	})
	savebutton := widget.NewButton("Save", func() {
		d.Hide()
		private.Words = wl.Words
		SavePrivateDictionary(private, AppPreferences)
	})
	savebutton.Importance = widget.HighImportance
	dismissbutton := widget.NewButton("Cancel", func() {
		d.Hide()
	})

	buttons := []fyne.CanvasObject{dismissbutton, addbutton, savebutton}
	d.SetButtons(buttons)
	d.Show()
}

func ShowAnimation(title, startPhrase string, anagrams []string, window fyne.Window) {
	ad := NewAnimationDisplay(Icon)
	cd := dialog.NewCustom(title, "dismiss", ad, MainWindow)
	cd.Resize(fyne.NewSize(600, 400))
	cd.Show()
	ad.AnimateAnagrams(startPhrase, anagrams...)
	cd.SetOnClosed(func() {
		ad.Stop()
	})
}

func ShowMultiAnagramPicker(title, submitlabel, dismisslabel, shufflelabel string, anagrams []string, callback func([]string), window fyne.Window) {
	anaChecks := make([]bool, len(anagrams))
	for index := range anagrams {
		anaChecks[index] = true
	}
	chooseList := widget.NewList(func() int {
		return len(anaChecks)
	}, func() fyne.CanvasObject {
		return widget.NewCheck("***picker***", nil)
	}, func(id widget.ListItemID, obj fyne.CanvasObject) {
		check, ok := obj.(*widget.Check)
		if !ok {
			return
		}
		check.Text = anagrams[id]
		check.Checked = anaChecks[id]
		check.OnChanged = func(checked bool) {
			anaChecks[id] = checked
		}
		check.Refresh()
	})

	d := dialog.NewCustom(title, submitlabel, chooseList, window)
	d.Resize(fyne.NewSize(300, 500))
	submitbutton := widget.NewButton(submitlabel, func() {
		d.Hide()
		chosen := make([]string, 0, len(anagrams))
		for index, check := range anaChecks {
			if check {
				chosen = append(chosen, anagrams[index])
			}
		}
		callback(chosen)
	})
	submitbutton.Importance = widget.HighImportance
	shufflebutton := widget.NewButton(shufflelabel, func() {
		rand.Shuffle(len(anagrams), func(i, j int) {
			anagrams[i], anagrams[j] = anagrams[j], anagrams[i]
			anaChecks[i], anaChecks[j] = anaChecks[j], anaChecks[i]
		})
		d.Refresh()
	})
	dismissbutton := widget.NewButton(dismisslabel, func() {
		d.Hide()
	})
	buttons := []fyne.CanvasObject{shufflebutton, dismissbutton, submitbutton}
	d.SetButtons(buttons)
	d.Show()
}

func ShowInterestingWordsList(rs *ResultSet, n int, include func(string), exclude func(string), window fyne.Window) {
	// fmt.Printf("Trying to get item at %d\n", searchlimit)
	rs.GetAt(searchlimit) // just to get a little bit of data to work with
	// fmt.Println("Found it")
	topN := rs.TopNWords(n)
	var closeDialog func()
	topList := widget.NewList(func() int {
		return len(topN)
	}, func() fyne.CanvasObject {
		return NewTapLabel("TopN")
	}, func(id widget.ListItemID, obj fyne.CanvasObject) {
		label, ok := obj.(*TapLabel)
		if !ok {
			return
		}
		label.Label.Text = fmt.Sprintf("%s %d", topN[id].Word, topN[id].Count)
		label.OnTapped = func(pe *fyne.PointEvent) {
			includeMI := fyne.NewMenuItem("Include", func() {
				include(topN[id].Word)
				closeDialog()
			})
			excludeMI := fyne.NewMenuItem("Exclude", func() {
				exclude(topN[id].Word)
				closeDialog()
			})
			pumenu := fyne.NewMenu("Pop up", includeMI, excludeMI)
			widget.ShowPopUpMenuAtRelativePosition(pumenu, window.Canvas(), pe.Position, label)
		}

		label.Refresh()
	})
	d := dialog.NewCustom(fmt.Sprintf("Interesting words in %d results", rs.Count()), "dismiss", topList, window)
	d.Resize(fyne.NewSize(400, 400))
	closeDialog = func() {
		fyne.Do(d.Hide)
	}
	fyne.Do(d.Show)
}

func ShowPopUpMessage(message string, duration time.Duration, window fyne.Window) {
	pulabel := widget.NewLabel(message)
	pu := widget.NewPopUp(pulabel, window.Canvas())
	pusize := pu.MinSize()
	wsize := window.Canvas().Size()
	pu.Move(fyne.NewPos((wsize.Width-pusize.Width)/2, (wsize.Height-pusize.Height)/2))
	pu.Show()
	go func() {
		time.Sleep(duration)
		fyne.Do(pu.Hide)
	}()
}

func main() {
	App := app.NewWithID("io.patenaude.karmamanager")
	MainWindow = App.NewWindow("Karma Manger")

	InitConfig(App)

	Icon = App.Metadata().Icon
	AppPreferences = App.Preferences()

	favorites := Favorites(App.Preferences())

	mainDicts, addedDicts, err := ReadDictionaries()
	if err != nil {
		panic(err)
	}

	privateDict := GetPrivateDictionary(AppPreferences)

	selectedDicts := GetDictionarySelections(AppPreferences)
	mainSelected := ""
	var remainingDicts []string
	if len(selectedDicts) > 0 {
		mainSelected = selectedDicts[0]
		remainingDicts = selectedDicts[1:]
	}
	if len(remainingDicts) > 0 && remainingDicts[len(remainingDicts)-1] == "Private" {
		privateDict.Enabled = true
		remainingDicts = remainingDicts[:len(remainingDicts)-1]
	} else {
		privateDict.Enabled = false
	}

	var mainDictNames []string = make([]string, len(mainDicts))
	for i, d := range mainDicts {
		mainDictNames[i] = d.Name
	}

	resultSet := NewResultSet(mainDicts, addedDicts, privateDict, 0)

	reset := func() {
		resultSet.Regenerate(nil)
	}

	reset_search := func() {
	}

	selectedMainIndex := 0
	for i, n := range mainDictNames {
		if mainSelected == n {
			selectedMainIndex = i
			break
		}
	}
	resultSet.SetMainIndex(selectedMainIndex)

	saveDictSelections := func() {
		dicts := make([]string, 0, len(addedDicts)+2)
		dicts = append(dicts, mainDictNames[selectedMainIndex])
		for _, ad := range addedDicts {
			if ad.Enabled {
				dicts = append(dicts, ad.Name)
			}
		}
		if privateDict.Enabled {
			dicts = append(dicts, "Private")
		}
		SaveDictionarySelections(dicts, AppPreferences)
	}

	mainSelect := widget.NewSelect(mainDictNames, func(dictName string) {
		for i, n := range mainDictNames {
			if dictName == n {
				selectedMainIndex = i
				resultSet.SetMainIndex(i)
				reset()
				MainWindow.SetTitle(resultSet.CombinedDictName())
				saveDictSelections()
				return
			}
		}
		dialog.ShowError(errors.New("Can't find selected main dictionary"), MainWindow)
	})
	mainSelect.SetSelectedIndex(selectedMainIndex)

	addedChecks := make([]fyne.CanvasObject, len(addedDicts)+2)
	for i, ad := range addedDicts {
		enabled := &ad.Enabled // copy a pointer to an address
		check := widget.NewCheck(ad.Name, func(checked bool) {
			*enabled = checked
			reset()
			MainWindow.SetTitle(resultSet.CombinedDictName())
			saveDictSelections()
		})
		ad.Enabled = false
		for _, name := range remainingDicts {
			if ad.Name == name {
				ad.Enabled = true
				break
			}
		}
		check.Checked = ad.Enabled
		addedChecks[i] = check
	}
	privateEnabled := &privateDict.Enabled
	privateCheck := widget.NewCheck(privateDict.Name, func(checked bool) {
		*privateEnabled = checked
		reset()
		MainWindow.SetTitle(resultSet.CombinedDictName())
		saveDictSelections()
	})
	privateCheck.Checked = privateDict.Enabled
	addedChecks[len(addedDicts)] = privateCheck
	privateDictSettingsButton := widget.NewButtonWithIcon("", theme.SettingsIcon(), func() {
		ShowPrivateDictSettings(privateDict, MainWindow)
	})
	addedChecks[len(addedDicts)+1] = privateDictSettingsButton
	addedDictsContainer := container.New(layout.NewHBoxLayout(), addedChecks...)

	inputdata := binding.NewString()
	inputEntry := widget.NewEntryWithData(inputdata)
	inputdata.AddListener(binding.NewDataListener(func() {
		if inputEntry.OnSubmitted != nil {
			inputEntry.OnSubmitted(inputEntry.Text)
		}
	}))

	inputClearButton := widget.NewButtonWithIcon("", theme.ContentClearIcon(), func() {
		inputdata.Set("")
		time.Sleep(50 * time.Millisecond)
		reset_search()
		// reset()
	})

	progressBar := widget.NewProgressBar()
	progressBar.Min = 0.0
	progressBar.Max = 1.0
	pbCallback := func(current, goal int) {
		// fmt.Printf("Progress %d of %d\n", current, goal)
		fyne.Do(func() {
			progressBar.SetValue(float64(current) / float64(goal))
			progressBar.Refresh()
		})
	}
	resultSet.SetProgressCallback(pbCallback)

	interestingButton := widget.NewButton("Interesting words", nil)

	workingBar := widget.NewActivity()
	wbStartCallback := func() {
		fyne.Do(func() {
			interestingButton.Disable()
			workingBar.Show()
			workingBar.Start()
			// workingBar.Refresh()
		})
	}
	wbStopCallback := func() {
		fyne.Do(func() {
			workingBar.Stop()
			workingBar.Hide()
			interestingButton.Enable()
			// workingBar.Refresh()
		})
	}
	resultSet.SetWorkingStartCallback(wbStartCallback)
	resultSet.SetWorkingStopCallback(wbStopCallback)

	interestBar := container.New(layout.NewGridLayout(2), interestingButton, progressBar)
	rightSideBar := container.NewBorder(nil, nil, nil, workingBar, interestBar)
	workingBar.Stop()
	workingBar.Hide()

	inputField := container.NewBorder(nil, nil, nil, inputClearButton, inputEntry)
	inputBar := container.New(layout.NewAdaptiveGridLayout(2), inputField, rightSideBar)

	dictionaryBar := container.New(layout.NewAdaptiveGridLayout(2), mainSelect, addedDictsContainer)

	controlBar := container.New(layout.NewVBoxLayout(), inputBar, dictionaryBar)

	resultsDisplay := widget.NewList(func() int { // list length
		return resultSet.Count()
	}, func() fyne.CanvasObject { // Make new entry
		return NewTapLabel("Foo")
	}, func(index int, object fyne.CanvasObject) { // Update entry
		label, ok := object.(*TapLabel)
		if !ok {
			return
		}
		go func() {
			text, _ := resultSet.GetAt(index)
			label.Label.Text = fmt.Sprintf("%10d %s", index+1, text)
			fyne.Do(object.Refresh)
		}()
	})
	inputEntry.OnSubmitted = func(input string) {
		reset_search()
		// reset()
		resultSet.FindAnagrams(input, func() {
			fyne.Do(resultsDisplay.Refresh)
		})
	}

	inclusionwords := NewWordList([]string{})
	SetInclusions := func() {
		includestring := strings.Join(inclusionwords.Words, " ")
		resultSet.SetInclusions([]string{includestring})
		resultSet.Regenerate(func() {
			fyne.Do(resultsDisplay.Refresh)
		})
	}
	inclusionwords.OnDelete = SetInclusions
	inclusionaddbutton := widget.NewButtonWithIcon("", theme.ContentAddIcon(), func() {
		inclusionwords.ShowAddWord("Include word", "Include", "Cancel", SetInclusions, MainWindow)
	})
	inclusionClearButton := widget.NewButtonWithIcon("", theme.ContentClearIcon(), func() {})
	inclusionClearButton.OnTapped = func() {
		inclusionwords.Clear()
		SetInclusions()
	}

	exclusionwords := NewWordList([]string{})
	SetExclusions := func() {
		resultSet.SetExclusions(exclusionwords.Words)
		resultSet.Regenerate(func() {
			fyne.Do(resultsDisplay.Refresh)
		})
	}
	exclusionwords.OnDelete = func() {
		SetExclusions()
	}
	exclusionaddbutton := widget.NewButtonWithIcon("", theme.ContentAddIcon(), func() {
		exclusionwords.ShowAddWord("Exclude what?", "Exclude", "Cancel", func() {
			SetExclusions()
		}, MainWindow)
	})
	exclusionClearButton := widget.NewButtonWithIcon("", theme.ContentClearIcon(), func() {})
	exclusionClearButton.OnTapped = func() {
		exclusionwords.Clear()
		SetExclusions()
	}

	includeFunc := func(word string) {
		inclusionwords.Words = append(inclusionwords.Words, word)
		inclusionwords.Refresh()
		SetInclusions()
	}

	excludeFunc := func(word string) {
		exclusionwords.Words = append(exclusionwords.Words, word)
		exclusionwords.Refresh()
		SetExclusions()
	}

	interestingButton.OnTapped = func() {
		go func() {
			ShowInterestingWordsList(resultSet, 1000, includeFunc, excludeFunc, MainWindow)
		}()
	}
	exclusionlabel := container.New(layout.NewHBoxLayout(), widget.NewLabel("Exclude"), exclusionaddbutton, exclusionClearButton)
	exclusioncontainer := container.NewBorder(exclusionlabel, nil, nil, nil, exclusionwords)
	inclusionlabel := container.New(layout.NewHBoxLayout(), widget.NewLabel("Include"), inclusionaddbutton, inclusionClearButton)
	inclusioncontainer := container.NewBorder(inclusionlabel, nil, nil, nil, inclusionwords)
	controlscontainer := container.New(layout.NewGridLayout(2), inclusioncontainer, exclusioncontainer)
	mainDisplay := container.New(layout.NewAdaptiveGridLayout(2), resultsDisplay, controlscontainer)

	resultsDisplay.UpdateItem = func(id widget.ListItemID, obj fyne.CanvasObject) {
		label, ok := obj.(*TapLabel)
		if !ok {
			return
		}
		text, _ := resultSet.GetAt(id)
		label.Label.Text = fmt.Sprintf("%10d %s", id+1, text)
		label.OnTapped = func(pe *fyne.PointEvent) {
			input, _ := inputdata.Get()
			copyAnagramToCBMI := fyne.NewMenuItem("Copy anagram to clipboard", func() {
				MainWindow.Clipboard().SetContent(text)
				ShowPopUpMessage("Copied to clipboard", time.Second, MainWindow)
			})
			copyBothToCBMI := fyne.NewMenuItem("Copy input and anagram to clipboard", func() {
				MainWindow.Clipboard().SetContent(fmt.Sprintf("%s ↔️ %s", input, text))
				ShowPopUpMessage("Copied to clipboard", time.Second, MainWindow)
			})
			addToFavsMI := fyne.NewMenuItem("Add to favorites", func() {
				ShowEditor("Add to favorites", text, func(editted string) {
					newFav := FavoriteAnagram{resultSet.CombinedDictName(), strings.TrimSpace(input), editted}
					favorites = append(favorites, newFav)
					RebuildFavorites()
					SaveFavorites(favorites, App.Preferences())
					ShowPopUpMessage("Added to favorites", time.Second, MainWindow)
				}, MainWindow)
			})
			animateMI := fyne.NewMenuItem("Animate", func() {
				input, _ = inputdata.Get()
				ShowAnimation("Animate anagram...", input, []string{text}, MainWindow)
			})
			words := strings.Split(text, " ")
			includeMIs := make([]*fyne.MenuItem, len(words))
			excludeMIs := make([]*fyne.MenuItem, len(words))
			for index, word := range words {
				includeMIs[index] = fyne.NewMenuItem(word, func() {
					includeFunc(word)
				})
				excludeMIs[index] = fyne.NewMenuItem(word, func() {
					excludeFunc(word)
				})
			}
			includemenu := fyne.NewMenu("Include", includeMIs...)
			exclusionmenu := fyne.NewMenu("Exclude", excludeMIs...)
			includeMI := fyne.NewMenuItem("Include", nil)
			includeMI.ChildMenu = includemenu
			excludeMI := fyne.NewMenuItem("Exclude", nil)
			excludeMI.ChildMenu = exclusionmenu
			var pumenu *fyne.Menu
			pumenu = fyne.NewMenu("Pop up", copyAnagramToCBMI, copyBothToCBMI, addToFavsMI, animateMI, includeMI, excludeMI)
			widget.ShowPopUpMenuAtRelativePosition(pumenu, MainWindow.Canvas(), pe.Position, label)
		}
		obj.Refresh()
	}

	findContent := container.NewBorder(controlBar, nil, nil, nil, mainDisplay)

	var selectTab func(int)

	sendToMainTabFunc := func(fav FavoriteAnagram) {
		inputdata.Set(fav.Input)
		// reset()
		// reset_search()
		time.Sleep(50 * time.Millisecond)
		selectTab(0)
		// inclusionwords.Clear()
		resultsDisplay.Refresh()
	}

	favsList := NewFavoritesList(&favorites, func(fav FavoriteAnagram) string {
		return fmt.Sprintf("%s ↔️ %s", fav.Input, fav.Anagram)
	}, sendToMainTabFunc)

	// favsContent := container.New(layout.NewAdaptiveGridLayout(1), favsList)

	favsContent := favsList

	RebuildFavorites = func() {
		sort.Sort(favorites)
		favsList.RegenGroups()
		favsList.MakeAnagramList()
		favsList.Refresh()
	}

	iconImage := canvas.NewImageFromResource(Icon)
	iconImage.SetMinSize(fyne.NewSize(128, 128))
	iconImage.FillMode = canvas.ImageFillContain

	preferencesButton := widget.NewButtonWithIcon("", theme.SettingsIcon(), Config.ShowPreferencesDialog)

	aboutContent := container.New(layout.NewVBoxLayout(),
		container.New(layout.NewHBoxLayout(), layout.NewSpacer(), preferencesButton),
		layout.NewSpacer(),
		iconImage,
		container.New(layout.NewHBoxLayout(), layout.NewSpacer(), widget.NewLabel(App.Metadata().Name), layout.NewSpacer()),
		container.New(layout.NewHBoxLayout(), layout.NewSpacer(), widget.NewLabel("Copyright 2025"), layout.NewSpacer()),
		container.New(layout.NewHBoxLayout(), layout.NewSpacer(), widget.NewLabel("by Mitch Patenaude"), layout.NewSpacer()),
		container.New(layout.NewHBoxLayout(), layout.NewSpacer(), widget.NewLabel(fmt.Sprintf("Version %s (build %d)", App.Metadata().Version, App.Metadata().Build)), layout.NewSpacer()),
		container.New(layout.NewHBoxLayout(), layout.NewSpacer(), widget.NewLabel("with thanks to the 80's Mac shareware of the same name"), layout.NewSpacer()),
		layout.NewSpacer())

	content := container.NewAppTabs(
		container.NewTabItem("Find", findContent),
		container.NewTabItem("Favorites", favsContent),
		container.NewTabItem("About", aboutContent))

	MainWindow.SetContent(content)

	selectTab = func(index int) {
		content.SelectTabIndex(index)
	}

	reset = func() {
		resultSet.Regenerate(func() {
			fyne.Do(resultsDisplay.ScrollToTop)
			fyne.Do(resultsDisplay.Refresh)
		})
		content.Refresh()
	}

	reset_search = func() {
		// not using SetInclusions() or SetExclusions because they will refresh other fields
		inclusionwords.Clear()
		resultSet.SetInclusions([]string{})
		exclusionwords.Clear()
		resultSet.SetExclusions([]string{})
	}

	MainWindow.Resize(fyne.NewSize(800, 600))
	MainWindow.ShowAndRun()
}
