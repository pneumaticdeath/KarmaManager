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

func Favorites(prefs fyne.Preferences) []FavoriteAnagram {
	strs := prefs.StringList(favoritesKey)
	favs := make([]FavoriteAnagram, len(strs))
	for i, s := range strs {
		favs[i] = decodeFavorite(s)
	}
	return favs
}

func SaveFavorites(favorites []FavoriteAnagram, prefs fyne.Preferences) {
	strs := make([]string, len(favorites))
	for i, fav := range favorites {
		strs[i] = encodeFavorite(fav)
	}
	prefs.SetStringList(favoritesKey, strs)
}

func ShowEditor(title, text string, submit func(string), window fyne.Window) {
	words := strings.Split(text, " ")
	ef := NewEditField(words, window)
	d := dialog.NewCustomConfirm(title, "Save", "Cancel", ef, func(submitted bool) {
		if submitted {
			submit(strings.Join(ef.Words, " "))
		}
	}, window)
	d.Resize(fyne.NewSize(400, 400))
	ef.Initialize()
	d.Show()
}

func ShowFavoriteAnagramEditor(favs *[]FavoriteAnagram, index int, prefs fyne.Preferences, refresh func(), window fyne.Window) {
	fav := (*favs)[index]
	ShowEditor("Edit anagram", fav.Anagram, func(newAnagram string) {
		if fav.Anagram != newAnagram {
			fav.Anagram = newAnagram
			(*favs)[index] = fav
			if refresh != nil {
				refresh()
			}
			SaveFavorites(*favs, prefs)
		}
	}, window)
}

func ShowFavoriteInputEditor(favs *[]FavoriteAnagram, index int, prefs fyne.Preferences, refresh func(), window fyne.Window) {
	fav := (*favs)[index]
	ShowEditor("Edit input phrase", fav.Input, func(newInput string) {
		if fav.Input != newInput {
			fav.Input = newInput
			(*favs)[index] = fav
			if refresh != nil {
				refresh()
			}
			SaveFavorites(*favs, prefs)
		}
	}, window)
}

func ShowDeleteFavConfirm(favs *[]FavoriteAnagram, id int, prefs fyne.Preferences, refresh func(), window fyne.Window) {
	dialog.ShowConfirm("Really delete?", "Are you sure you want to delete this favorite?", func(confirmed bool) {
		if confirmed {
			*favs = slices.Delete(*favs, id, id+1)
			refresh()
			SaveFavorites(*favs, prefs)
		}
	}, window)
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
		included, _ := inclusiondata.Get()
		includedphrases := strings.Split(included, "\n")
		resultSet.SetInclusions(includedphrases)
		resultSet.Regenerate()
		// resultsDisplay.Refresh()
	}))

	exclusiondata := binding.NewString()
	exclusiondata.AddListener(binding.NewDataListener(func() {
		exclusions, _ := exclusiondata.Get()
		excludedwords := strings.Split(exclusions, " ")
		resultSet.SetExclusions(excludedwords)
		resultSet.Regenerate()
		// resultsDisplay.Refresh()
	}))
	exclusionentry := widget.NewEntryWithData(exclusiondata)
	bottomcontainer := container.New(layout.NewVBoxLayout(), widget.NewLabel("Excluded words"), exclusionentry)
	controlscontainer := container.NewBorder(widget.NewLabel("Include phrases"), bottomcontainer, nil, nil, inclusionentry)
	mainDisplay := container.New(layout.NewAdaptiveGridLayout(2), resultsDisplay, controlscontainer)

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
				pulabel := widget.NewLabel("Copied to clipboard")
				pu := widget.NewPopUp(pulabel, Window.Canvas())
				wsize := Window.Canvas().Size()
				pu.Move(fyne.NewPos((wsize.Width)/2, (wsize.Height)/2))
				pu.Show()
				go func() {
					time.Sleep(2 * time.Second)
					pu.Hide()
				}()
			})
			addToFavsMI := fyne.NewMenuItem("Add to favorites", func() {
				input, _ := inputdata.Get()
				newFav := FavoriteAnagram{resultSet.CombinedDictName(), input, text}
				favorites = append(favorites, newFav)
				SaveFavorites(favorites, App.Preferences())
				pulabel := widget.NewLabel("Added to favorites")
				pu := widget.NewPopUp(pulabel, Window.Canvas())
				wsize := Window.Canvas().Size()
				pu.Move(fyne.NewPos((wsize.Width)/2, (wsize.Height)/2))
				pu.Show()
				go func() {
					time.Sleep(2 * time.Second)
					pu.Hide()
				}()
			})
			animateMI := fyne.NewMenuItem("Animate", func() {
				ad := NewAnimationDisplay(App.Metadata().Icon)
				input, _ := inputdata.Get()
				cd := dialog.NewCustom("Animated anagram...", "dismiss", ad, Window)
				cd.Resize(fyne.NewSize(500, 400))
				cd.Show()
				ad.AnimateAnagram(input, text)
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
			pumenu := fyne.NewMenu("Pop up", copyToCBMI, addToFavsMI, animateMI, filterMI, excludeMI)
			widget.ShowPopUpMenuAtRelativePosition(pumenu, Window.Canvas(), pe.Position, label)
		}
		obj.Refresh()
	}

	findContent := container.NewBorder(controlBar, searchcontainer, nil, nil, mainDisplay)

	var refreshFavsList func()

	favsList := widget.NewList(func() int {
		return len(favorites)
	}, func() fyne.CanvasObject {
		return NewTapLabel("Fav")
	}, func(id widget.ListItemID, obj fyne.CanvasObject) {
		label, ok := obj.(*TapLabel)
		if !ok {
			return
		}
		label.Label.Text = fmt.Sprintf("%s->%s", favorites[id].Input, favorites[id].Anagram)
		label.Label.Alignment = fyne.TextAlignCenter
		label.OnTapped = func(pe *fyne.PointEvent) {
			animateMI := fyne.NewMenuItem("Animate", func() {
				ad := NewAnimationDisplay(App.Metadata().Icon)
				cd := dialog.NewCustom("Animated anagram...", "dismiss", ad, Window)
				cd.Resize(fyne.NewSize(400, 300))
				cd.Show()
				ad.AnimateAnagram(favorites[id].Input, favorites[id].Anagram)
			})
			editAnagramMI := fyne.NewMenuItem("Edit Anagram", func() {
				ShowFavoriteAnagramEditor(&favorites, id, App.Preferences(), refreshFavsList, Window)
			})
			editInputMI := fyne.NewMenuItem("Edit Input", func() {
				ShowFavoriteInputEditor(&favorites, id, App.Preferences(), refreshFavsList, Window)
			})
			deleteMI := fyne.NewMenuItem("Delete", func() {
				ShowDeleteFavConfirm(&favorites, id, App.Preferences(), refreshFavsList, Window)
			})
			pumenu := fyne.NewMenu("Pop up", animateMI, editInputMI, editAnagramMI, deleteMI)
			widget.ShowPopUpMenuAtRelativePosition(pumenu, Window.Canvas(), pe.Position, label)
		}

		label.Refresh()
	})

	refreshFavsList = func() {
		favsList.Refresh()
	}

	favsContent := container.New(layout.NewAdaptiveGridLayout(1), favsList)

	content := container.NewAppTabs(container.NewTabItem("Find", findContent), container.NewTabItem("Favorites", favsContent))

	Window.SetContent(content)

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

	Window.Resize(fyne.NewSize(800, 600))
	Window.ShowAndRun()
}
