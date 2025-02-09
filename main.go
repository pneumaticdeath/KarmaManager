package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	searchLimit = 200000
)

var MainWindow fyne.Window
var Icon fyne.Resource
var AppPreferences fyne.Preferences
var RebuildFavorites func()

func ShowInterestingWordsList(rs *ResultSet, n int, include func(string), exclude func(string), window fyne.Window) {
	rs.GetAt(50000) // just to get a little bit of data to work with
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
	d := dialog.NewCustom("Interesting words", "dismiss", topList, window)
	d.Resize(fyne.NewSize(400, 400))
	closeDialog = func() {
		d.Hide()
	}
	d.Show()
}

func main() {
	App := app.NewWithID("io.patenaude.karmamanager")
	MainWindow = App.NewWindow("Karma Manger")

	Icon = App.Metadata().Icon
	AppPreferences = App.Preferences()

	favorites := Favorites(App.Preferences())
	// favInputGroups := MakeGroupedFavorites(favorites)

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
				MainWindow.SetTitle(resultSet.CombinedDictName())
				return
			}
		}
		dialog.ShowError(errors.New("Can't find selected main dictionary"), MainWindow)
	})
	mainSelect.SetSelectedIndex(0)

	addedChecks := make([]fyne.CanvasObject, len(addedDicts))
	for i, ad := range addedDicts {
		enabled := &ad.Enabled // copy a pointer to an address
		check := widget.NewCheck(ad.Name, func(checked bool) {
			*enabled = checked
			reset()
			MainWindow.SetTitle(resultSet.CombinedDictName())
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

	inputClearButton := widget.NewButtonWithIcon("", theme.ContentClearIcon(), func() {
		inputdata.Set("")
		reset()
		reset_search()
	})

	progressBar := widget.NewProgressBar()
	progressBar.Min = 0.0
	progressBar.Max = 1.0
	pbCallback := func(current, goal int) {
		progressBar.SetValue(float64(current) / float64(goal))
		progressBar.Refresh()
	}
	resultSet.SetProgressCallback(pbCallback)

	input := container.NewBorder(nil, nil, nil, inputClearButton, inputEntry)
	inputBar := container.New(layout.NewAdaptiveGridLayout(2), input, progressBar)

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
		text, _ := resultSet.GetAt(index)
		label.Label.Text = fmt.Sprintf("%10d %s", index+1, text)
		object.Refresh()
	})

	lastSearchString := ""
	lastSearchIndex := -1
	searchError := widget.NewLabel("")
	searchbox := widget.NewEntry()
	searchbox.OnSubmitted = func(searchfor string) {
		searchError.Text = ""
		searchError.Refresh()
		if searchfor == "" {
			return
		}
		searchstring := strings.ToLower(searchfor)
		searchError.Text = fmt.Sprintf("Finding '%s'", searchstring)
		searchError.Refresh()
		i := 0
		if lastSearchString == searchstring {
			i = lastSearchIndex + 1
		} else {
			lastSearchString = searchstring
			lastSearchIndex = -1
		}
		count := 0
		for i < resultSet.Count() && i < searchLimit {
			text, _ := resultSet.GetAt(i)
			if strings.Index(strings.ToLower(text), searchstring) != -1 {
				count += 1
				// searchresultslist.Append(i)
				resultsDisplay.ScrollTo(i)
				resultsDisplay.Refresh()
				searchError.Text = fmt.Sprintf("Found at location %d", i+1)
				searchError.Refresh()
				lastSearchIndex = i
				return
			}
			i += 1
		}
		if count == 0 && resultSet.IsDone() && i >= resultSet.Count() {
			searchError.Text = "Not Found!"
			searchError.Refresh()
			lastSearchString = ""
			lastSearchIndex = -1
		} else if i >= searchLimit {
			if count == 0 {
				searchError.Text = fmt.Sprintf("Didn't find '%s' in the first %d results!", searchstring, searchLimit)
			} else {
				searchError.Text = fmt.Sprintf("Found %d instances in the first %d results!", count, searchLimit)
			}
			searchError.Refresh()
		}
	}
	searchbutton := widget.NewButton("Find", func() {
		searchbox.OnSubmitted(searchbox.Text)
	})

	searchcontainer := container.New(layout.NewGridLayout(3), searchbutton, searchbox, searchError)

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
		included, err := inclusiondata.Get()
		if err != nil {
			dialog.ShowError(err, MainWindow)
			return
		}
		includedphrases := strings.Split(included, "\n")
		resultSet.SetInclusions(includedphrases)
		resultSet.Regenerate()
		resultsDisplay.Refresh()
	}))
	inclusionclearbutton := widget.NewButtonWithIcon("", theme.ContentClearIcon(), func() {
		inclusiondata.Set("")
		// resultSet.SetInclusions([]string{})
		// resultSet.Regenerate()
		// resultsDisplay.Refresh()
	})

	exclusiondata := binding.NewString()
	exclusiondata.AddListener(binding.NewDataListener(func() {
		exclusions, _ := exclusiondata.Get()
		excludedwords := strings.Split(exclusions, " ")
		resultSet.SetExclusions(excludedwords)
		resultSet.Regenerate()
		resultsDisplay.Refresh()
	}))
	exclusionentry := widget.NewEntryWithData(exclusiondata)
	exclusionclearbutton := widget.NewButtonWithIcon("", theme.ContentClearIcon(), func() {
		exclusiondata.Set("")
		// resultSet.SetExclusions([]string{})
		// resultSet.Regenerate()
	})

	includeFunc := func(word string) {
		existing, _ := inclusiondata.Get()
		if existing == "" {
			inclusiondata.Set(word)
		} else {
			inclusiondata.Set(existing + " " + word)
		}
		resultsDisplay.Refresh()
	}

	excludeFunc := func(word string) {
		existing, _ := exclusiondata.Get()
		if existing == "" {
			exclusiondata.Set(word)
		} else {
			exclusiondata.Set(existing + " " + word)
		}
	}
	interestingbutton := widget.NewButton("Interesting words", func() {
		ShowInterestingWordsList(resultSet, 1000, includeFunc, excludeFunc, MainWindow)
	})
	exclusionlabel := container.New(layout.NewHBoxLayout(), widget.NewLabel("Excluded words"), exclusionclearbutton)
	bottomcontainer := container.New(layout.NewVBoxLayout(), exclusionlabel, exclusionentry)
	inclusionlabel := container.New(layout.NewHBoxLayout(), widget.NewLabel("Include phrases"), inclusionclearbutton, layout.NewSpacer(), interestingbutton)
	controlscontainer := container.NewBorder(inclusionlabel, bottomcontainer, nil, nil, inclusionentry)
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
				pulabel := widget.NewLabel("Copied to clipboard")
				pu := widget.NewPopUp(pulabel, MainWindow.Canvas())
				wsize := MainWindow.Canvas().Size()
				pu.Move(fyne.NewPos((wsize.Width)/2, (wsize.Height)/2))
				pu.Show()
				go func() {
					time.Sleep(time.Second)
					pu.Hide()
				}()
			})
			copyBothToCBMI := fyne.NewMenuItem("Copy input and anagram to clipboard", func() {
				MainWindow.Clipboard().SetContent(fmt.Sprintf("%s->%s", input, text))
				pulabel := widget.NewLabel("Copied to clipboard")
				pu := widget.NewPopUp(pulabel, MainWindow.Canvas())
				wsize := MainWindow.Canvas().Size()
				pu.Move(fyne.NewPos((wsize.Width)/2, (wsize.Height)/2))
				pu.Show()
				go func() {
					time.Sleep(time.Second)
					pu.Hide()
				}()
			})
			addToFavsMI := fyne.NewMenuItem("Add to favorites", func() {
				ShowEditor("Add to favorites", text, func(editted string) {
					newFav := FavoriteAnagram{resultSet.CombinedDictName(), input, editted}
					favorites = append(favorites, newFav)
					RebuildFavorites()
					SaveFavorites(favorites, App.Preferences())
					pulabel := widget.NewLabel("Added to favorites")
					pu := widget.NewPopUp(pulabel, MainWindow.Canvas())
					wsize := MainWindow.Canvas().Size()
					pu.Move(fyne.NewPos((wsize.Width)/2, (wsize.Height)/2))
					pu.Show()
					go func() {
						time.Sleep(time.Second)
						pu.Hide()
					}()
				}, MainWindow)
			})
			animateMI := fyne.NewMenuItem("Animate", func() {
				ad := NewAnimationDisplay(App.Metadata().Icon)
				input, _ := inputdata.Get()
				cd := dialog.NewCustom("Animated anagram...", "dismiss", ad, MainWindow)
				cd.Resize(fyne.NewSize(600, 400))
				cd.Show()
				ad.AnimateAnagram(input, text)
				cd.SetOnClosed(func() {
					ad.Stop()
				})
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
			pumenu := fyne.NewMenu("Pop up", copyAnagramToCBMI, copyBothToCBMI, addToFavsMI, animateMI, filterMI, excludeMI)
			widget.ShowPopUpMenuAtRelativePosition(pumenu, MainWindow.Canvas(), pe.Position, label)
		}
		obj.Refresh()
	}

	findContent := container.NewBorder(controlBar, searchcontainer, nil, nil, mainDisplay)
	// findContent = container.NewBorder(controlBar, progressBar, nil, nil, mainDisplay)

	// var refreshFavsList func()
	var selectTab func(int)

	sendToMainTabFunc := func(fav FavoriteAnagram) {
		reset()
		inputdata.Set(fav.Input)
		time.Sleep(50 * time.Millisecond)
		exclusiondata.Set("")
		inclusiondata.Set(fav.Anagram)
		selectTab(0)
		resultsDisplay.Refresh()
		inclusionentry.Refresh()
	}

	favsList := NewFavoritesList(&favorites, func(fav FavoriteAnagram) string {
		return fmt.Sprintf("%s -> %s", fav.Input, fav.Anagram)
	}, sendToMainTabFunc)

	// favsContent := container.New(layout.NewAdaptiveGridLayout(1), favsList)

	favsContent := favsList

	RebuildFavorites = func() {
		sort.Sort(favorites)
		favsList.Refresh()
	}

	content := container.NewAppTabs(container.NewTabItem("Find", findContent), container.NewTabItem("Favorites", favsContent))

	MainWindow.SetContent(content)

	selectTab = func(index int) {
		content.SelectTabIndex(index)
	}

	reset = func() {
		resultSet.Regenerate()
		searchError.Text = ""
		lastSearchIndex = -1
		lastSearchString = ""
		resultsDisplay.ScrollToTop()
		content.Refresh()
	}

	reset_search = func() {
		searchbox.Text = ""
		searchbox.Refresh()
		inclusiondata.Set("")
		exclusiondata.Set("")
	}

	MainWindow.Resize(fyne.NewSize(800, 600))
	MainWindow.ShowAndRun()
}
