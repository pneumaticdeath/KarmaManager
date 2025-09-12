package main

import (
	"fmt"
	// "image/gif"
	"slices"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

const favoritesKey = "io.patenaude.karmamanager.favorites"

type FavoriteAnagram struct {
	Dictionaries, Input string
	Anagram             string
}

type FavoritesSlice []FavoriteAnagram

type GroupedFavorites map[string]FavoritesSlice

func (fs FavoritesSlice) Len() int {
	return len(fs)
}

func (fs FavoritesSlice) Swap(i, j int) {
	fs[i], fs[j] = fs[j], fs[i]
}

func (fs FavoritesSlice) Less(i, j int) bool {
	if strings.ToLower(fs[i].Input) == strings.ToLower(fs[j].Input) {
		return strings.ToLower(fs[i].Anagram) < strings.ToLower(fs[j].Anagram)
	}
	return strings.ToLower(fs[i].Input) < strings.ToLower(fs[j].Input)
}

func encodeFavorite(fav FavoriteAnagram) string {
	return fmt.Sprintf("%s\n%s\n%s", fav.Dictionaries, fav.Input, fav.Anagram)
}

func decodeFavorite(s string) FavoriteAnagram {
	lines := strings.Split(s, "\n")
	return FavoriteAnagram{lines[0], lines[1], lines[2]}
}

func Favorites(prefs fyne.Preferences) FavoritesSlice {
	strs := prefs.StringList(favoritesKey)
	favs := make(FavoritesSlice, len(strs))
	for i, s := range strs {
		favs[i] = decodeFavorite(s)
	}
	sort.Sort(favs)
	return favs
}

func SaveFavorites(favorites FavoritesSlice, prefs fyne.Preferences) {
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
	d.Resize(fyne.NewSize(600, 400))
	ef.Initialize()
	d.Show()
}

func ShowFavoriteAnagramEditor(favs *FavoritesSlice, index int, prefs fyne.Preferences, refresh func(), window fyne.Window) {
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

func ShowFavoriteInputEditor(favs *FavoritesSlice, index int, prefs fyne.Preferences, refresh func(), window fyne.Window) {
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

func ShowDeleteFavConfirm(favs *FavoritesSlice, id int, prefs fyne.Preferences, refresh func(), window fyne.Window) {
	dialog.ShowConfirm("Really delete?", fmt.Sprintf("Really delete \"%s\"?", (*favs)[id].Anagram), func(confirmed bool) {
		if confirmed {
			*favs = slices.Delete(*favs, id, id+1)
			refresh()
			SaveFavorites(*favs, prefs)
		}
	}, window)
}

func MakeGroupedFavorites(favs FavoritesSlice) GroupedFavorites {
	groups := make(map[string]FavoritesSlice)

	for _, fav := range favs {
		input := fav.Input
		_, present := groups[input]
		if !present {
			groups[input] = make(FavoritesSlice, 0, 3)
		}

		groups[input] = append(groups[input], fav)
	}

	return groups
}

type FavoritesList struct {
	widget.BaseWidget

	baseList    *FavoritesSlice
	GroupedList GroupedFavorites
	selectedInput       string
	surface             *fyne.Container
	labelFunc           func(FavoriteAnagram) string
	sendToMainTab       func(string)
	inputSelectBar      *fyne.Container
	multiAnimateButton  *widget.Button
	sendToMainTabButton *widget.Button
	listOfInputs        *widget.Select
	AnagramDisplay      *widget.List
}

func NewFavoritesList(list *FavoritesSlice, labelFunc func(FavoriteAnagram) string, sendToMainTab func(string)) *FavoritesList {
	fl := &FavoritesList{}

	fl.baseList = list
	fl.labelFunc = labelFunc
	fl.sendToMainTab = sendToMainTab

	refresh := func() {}

	fl.listOfInputs = widget.NewSelect([]string{}, func(selected string) {
		fl.selectedInput = selected
		fmt.Printf("Selected \"%s\" (%d elements)\n", selected, len(fl.GroupedList[fl.selectedInput]))
		if len(fl.GroupedList[fl.selectedInput]) > 1 {
			fl.multiAnimateButton.Enable()
		} else {
			fl.multiAnimateButton.Disable()
		}

		go func() {
			time.Sleep(10 * time.Millisecond)
			fyne.Do(fl.AnagramDisplay.Refresh)
		}()
		refresh()
	})

	fl.multiAnimateButton = widget.NewButton("Animate multiple", func() {
		if len(fl.GroupedList[fl.selectedInput]) > 1 {
			anagrams := make([]string, len(fl.GroupedList[fl.selectedInput]))
			for index, fav := range fl.GroupedList[fl.selectedInput] {
				anagrams[index] = fav.Anagram
			}
			ShowMultiAnagramPicker("Animate which anagrams", "animate", "cancel", "shuffle", anagrams, func(chosen []string) {
				if len(chosen) > 0 {
					ShowAnimation("Animated anagrams...", fl.selectedInput, chosen, MainWindow)
				}
			}, MainWindow)

		}
	})
	fl.multiAnimateButton.Disable()

	fl.sendToMainTabButton = widget.NewButton("Send to main", func() {
		fl.sendToMainTab(fl.selectedInput)
	})

	fl.inputSelectBar = container.New(layout.NewHBoxLayout(), fl.listOfInputs, fl.multiAnimateButton, fl.sendToMainTabButton)

	fl.RegenGroups()
	fl.MakeAnagramList()

	fl.surface = container.NewBorder(fl.inputSelectBar, nil, nil, nil, fl.AnagramDisplay)

	refresh = func() {
		// fl.AnagramDisplay.Refresh()
		// don't know why the above doesn't do it....
		fl.surface.Refresh()
	}

	fl.ExtendBaseWidget(fl)

	return fl
}

func (fl *FavoritesList) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(fl.surface)
}

func (fl *FavoritesList) Refresh() {
	fl.surface.Refresh()
}

func (fl *FavoritesList) RegenGroups() {
	fl.GroupedList = MakeGroupedFavorites(*fl.baseList)

	inputList := make([]string, 0, len(fl.GroupedList))

	for input, _ := range fl.GroupedList {
		inputList = append(inputList, input)
	}

	sort.Strings(inputList)

	if len(fl.GroupedList[fl.selectedInput]) > 1 {
		fl.multiAnimateButton.Enable()
	} else {
		fl.multiAnimateButton.Disable()
	}

	fl.listOfInputs.Options = inputList
	fl.listOfInputs.Refresh()
	fl.inputSelectBar.Refresh()
}

func (fl *FavoritesList) findGlobalID(selectedListID int) int {
	selectedFavorite := fl.GroupedList[fl.selectedInput][selectedListID]
	for globalID, globalFavorite := range *(fl.baseList) {
		if selectedFavorite.Dictionaries == globalFavorite.Dictionaries && selectedFavorite.Input == globalFavorite.Input && selectedFavorite.Anagram == globalFavorite.Anagram {
			return globalID
		}
	}
	fmt.Println("Couldn't find global id")
	return -1
}

func (fl *FavoritesList) MakeAnagramList() {
	fl.AnagramDisplay = widget.NewList(func() int {
		return len(fl.GroupedList[fl.selectedInput])
	}, func() fyne.CanvasObject {
		return NewTapLabel("Fav")
	}, func(id widget.ListItemID, obj fyne.CanvasObject) {
		label, ok := obj.(*TapLabel)
		if !ok {
			return
		}
		label.Label.Text = fl.labelFunc(fl.GroupedList[fl.selectedInput][id])
		label.Label.Alignment = fyne.TextAlignCenter
		label.OnTapped = func(pe *fyne.PointEvent) {
			copyAnagramToCBMI := fyne.NewMenuItem("Copy anagram to clipboard", func() {
				MainWindow.Clipboard().SetContent(fl.GroupedList[fl.selectedInput][id].Anagram)
				pulabel := widget.NewLabel("Copied to clipboard")
				pu := widget.NewPopUp(pulabel, MainWindow.Canvas())
				wsize := MainWindow.Canvas().Size()
				pu.Move(fyne.NewPos((wsize.Width)/2, (wsize.Height)/2))
				pu.Show()
				go func() {
					time.Sleep(time.Second)
					fyne.Do(pu.Hide)
				}()
			})
			copyBothToCBMI := fyne.NewMenuItem("Copy input and anagram to clipboard", func() {
				MainWindow.Clipboard().SetContent(fmt.Sprintf("%s ↔️ %s", fl.GroupedList[fl.selectedInput][id].Input, fl.GroupedList[fl.selectedInput][id].Anagram))
				pulabel := widget.NewLabel("Copied to clipboard")
				pu := widget.NewPopUp(pulabel, MainWindow.Canvas())
				wsize := MainWindow.Canvas().Size()
				pu.Move(fyne.NewPos((wsize.Width)/2, (wsize.Height)/2))
				pu.Show()
				go func() {
					time.Sleep(time.Second)
					fyne.Do(pu.Hide)
				}()
			})
			animateMI := fyne.NewMenuItem("Animate", func() {
				anagrams := []string{fl.GroupedList[fl.selectedInput][id].Anagram}
				ShowAnimation("Animated anagram...", fl.GroupedList[fl.selectedInput][id].Input, anagrams, MainWindow)
			})
			/*
				multiAnimationMI := fyne.NewMenuItem(fmt.Sprintf("Animate multiple with input \"%s\"", fl.GroupedList[fl.selectedInput][id].Input), func() {
					anagrams := make([]string, len(groups[fl.GroupedList[fl.selectedInput][id].Input]))
					for index, fav := range groups[fl.GroupedList[fl.selectedInput][id].Input] {
						anagrams[index] = fav.Anagram
					}
					ShowMultiAnagramPicker("Animate which anagrams", "animate", "cancel", "shuffle", anagrams, func(chosen []string) {
						if len(chosen) > 0 {
							ShowAnimation("Animated anagrams...", fl.GroupedList[fl.selectedInput][id].Input, chosen, MainWindow)
						}
					}, MainWindow)
				})
			*/
			sendToMainMI := fyne.NewMenuItem("Send anagram to main input tab", func() {
				fl.sendToMainTab(fl.GroupedList[fl.selectedInput][id].Anagram)
			})
			editAnagramMI := fyne.NewMenuItem("Edit Anagram", func() {
				globalID := fl.findGlobalID(id)
				ShowFavoriteAnagramEditor(fl.baseList, globalID, AppPreferences, RebuildFavorites, MainWindow)
				// fl.AnagramDisplay.Refresh()
			})
			editInputMI := fyne.NewMenuItem("Edit Input", func() {
				globalID := fl.findGlobalID(id)
				ShowFavoriteInputEditor(fl.baseList, globalID, AppPreferences, RebuildFavorites, MainWindow)
				fl.RegenGroups()
				// fl.AnagramDisplay.Refresh()
			})
			deleteMI := fyne.NewMenuItem("Delete", func() {
				globalID := fl.findGlobalID(id)
				ShowDeleteFavConfirm(fl.baseList, globalID, AppPreferences, RebuildFavorites, MainWindow)
				// fl.AnagramDisplay.Refresh()
			})
			pumenu := fyne.NewMenu("Pop up", copyAnagramToCBMI, copyBothToCBMI, animateMI, sendToMainMI, editInputMI, editAnagramMI, deleteMI)
			widget.ShowPopUpMenuAtRelativePosition(pumenu, MainWindow.Canvas(), pe.Position, label)
		}

		label.Refresh()
	})
	fl.AnagramDisplay.Refresh()
	/* } else {
		fmt.Printf("Can't build list for input \"%s\"\n", fl.SelectedInput)
		// blank list?
		fl.AnagramDisplay = widget.NewList(func() int { return 0 }, func() fyne.CanvasObject { return NewTapLabel("Foo") }, func(_ widget.ListItemID, _ fyne.CanvasObject) {})
	} */

}
