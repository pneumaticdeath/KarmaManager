package main

import (
	"errors"
	"fmt"
	"slices"
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

const (
	searchLimit  = 200000
	favoritesKey = "io.patenaude.karmamanager.favorites"
)

type FavoriteAnagram struct {
	Dictionaries, Input string
	Anagram             string
}

func encodeFavorite(fav FavoriteAnagram) string {
	return fmt.Sprintf("%s\n%s\n%s", fav.Dictionaries, fav.Input, fav.Anagram)
}

func decodeFavorite(s string) FavoriteAnagram {
	lines := strings.Split(s, "\n")
	return FavoriteAnagram{lines[0], lines[1], lines[2]}
}

func SaveFavorites(favorites []FavoriteAnagram, prefs fyne.Preferences) {
	strs := make([]string, len(favorites))
	for i, fav := range favorites {
		strs[i] = encodeFavorite(fav)
	}
	prefs.SetStringList(favoritesKey, strs)
}

func ShowFavoriteEditor(favs *[]FavoriteAnagram, index int, prefs fyne.Preferences, window fyne.Window) {
	fav := (*favs)[index]
	dictEntry := widget.NewEntry()
	dictEntry.Text = fav.Dictionaries
	inputEntry := widget.NewEntry()
	inputEntry.Text = fav.Input
	anagramEntry := widget.NewEntry()
	anagramEntry.Text = fav.Anagram
	deleteCheck := widget.NewCheck("Delete favorite", func(checked bool) {
		if checked {
			dictEntry.Disable()
			inputEntry.Disable()
			anagramEntry.Disable()
		} else {
			dictEntry.Enable()
			inputEntry.Enable()
			anagramEntry.Enable()
		}
	})
	items := []*widget.FormItem{
		widget.NewFormItem("Dictionaries", dictEntry),
		widget.NewFormItem("Input", inputEntry),
		widget.NewFormItem("Anagram", anagramEntry),
		widget.NewFormItem("DELETE", deleteCheck)}
	dialog.ShowForm("Edit Favorite", "Save", "Cancel", items, func(submitted bool) {
		if submitted {
			if deleteCheck.Checked {
				*favs = slices.Delete(*favs, index, index+1)
			} else if NewRuneCluster(inputEntry.Text).Equals(NewRuneCluster(anagramEntry.Text)) {
				fav.Dictionaries = dictEntry.Text
				fav.Input = inputEntry.Text
				fav.Anagram = anagramEntry.Text
				(*favs)[index] = fav
			} else {
				dialog.ShowError(errors.New("Input and Anagram no longer match"), window)
				return
			}
			SaveFavorites(*favs, prefs)
			window.Canvas().Refresh(window.Content())
		}
	}, window)
}

func Favorites(prefs fyne.Preferences) []FavoriteAnagram {
	strs := prefs.StringList(favoritesKey)
	favs := make([]FavoriteAnagram, len(strs))
	for i, s := range strs {
		favs[i] = decodeFavorite(s)
	}
	return favs
}

func main() {
	App := app.NewWithID("io.patenaude.karmamanager")
	Window := App.NewWindow("Karma Manger")

	favorites := Favorites(App.Preferences())

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

	resultsDisplay.UpdateItem = func(id widget.ListItemID, obj fyne.CanvasObject) {
		label, ok := obj.(*TapLabel)
		if !ok {
			return
		}
		text, _ := resultSet.GetAt(id)
		label.Label.Text = fmt.Sprintf("%10d %s", id+1, text)
		label.OnTapped = func(pe *fyne.PointEvent) {
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
			addToFavsMI := fyne.NewMenuItem("Add to favorites", func() {
				input, _ := inputdata.Get()
				newFav := FavoriteAnagram{resultSet.CombinedDictName(), input, text}
				favorites = append(favorites, newFav)
				SaveFavorites(favorites, App.Preferences())
				label := widget.NewLabel("Added to favorites")
				pu := widget.NewPopUp(label, Window.Canvas())
				wsize := Window.Canvas().Size()
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
			pumenu := fyne.NewMenu("Pop up", copyToCBMI, addToFavsMI, filterMI, excludeMI)
			widget.ShowPopUpMenuAtRelativePosition(pumenu, Window.Canvas(), pe.Position, label)
		}
		obj.Refresh()
	}

	findContent := container.NewBorder(controlBar, nil, nil, nil, mainDisplay)

	animationContent := NewAnimationDisplay()

	favsList := widget.NewList(func() int {
		return len(favorites)
	}, func() fyne.CanvasObject {
		return NewTapLabel("Fav")
	}, func(id widget.ListItemID, obj fyne.CanvasObject) {
		label, ok := obj.(*TapLabel)
		if !ok {
			return
		}
		label.Label.Text = fmt.Sprintf("%35s->%-35s", favorites[id].Input, favorites[id].Anagram)
		label.OnTapped = func(pe *fyne.PointEvent) {
			animateMI := fyne.NewMenuItem("Animate", func() {
				animationContent.AnimateAnagram(favorites[id].Input, favorites[id].Anagram)
			})
			editMI := fyne.NewMenuItem("Edit", func() {
				ShowFavoriteEditor(&favorites, id, App.Preferences(), Window)
			})
			pumenu := fyne.NewMenu("Pop up", animateMI, editMI)
			widget.ShowPopUpMenuAtRelativePosition(pumenu, Window.Canvas(), pe.Position, label)
		}

		label.Refresh()
	})

	favsContent := container.New(layout.NewAdaptiveGridLayout(2), favsList, animationContent)

	content := container.NewAppTabs(container.NewTabItem("Find", findContent), container.NewTabItem("Favorites", favsContent))

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
