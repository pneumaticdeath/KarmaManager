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
	// SelectedInput  string
	selectedList   FavoritesSlice
	surface        *fyne.Container
	labelFunc      func(FavoriteAnagram) string
	sendToMainTab  func(FavoriteAnagram)
	inputSelectBar *fyne.Container
	listOfInputs   *widget.Select
	AnagramDisplay *widget.List
	source         *FavoritesSlice
}

func NewFavoritesList(list *FavoritesSlice, labelFunc func(FavoriteAnagram) string, sendToMainTab func(FavoriteAnagram)) *FavoritesList {
	fl := &FavoritesList{}

	fl.baseList = list
	fl.labelFunc = labelFunc
	fl.sendToMainTab = sendToMainTab

	fl.RegenGroups()
	fl.MakeAnagramList()

	fl.surface = container.NewBorder(fl.inputSelectBar, nil, nil, nil, fl.AnagramDisplay)

	fl.ExtendBaseWidget(fl)

	return fl
}

func (fl *FavoritesList) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(fl.surface)
}

/* 
func (fl *FavoritesList) Refresh() {
	fl.surface.Refresh()
}
*/

func (fl *FavoritesList) RegenGroups() {
	fl.GroupedList = MakeGroupedFavorites(*fl.baseList)

	inputList := make([]string, 0, len(fl.GroupedList))

	for input, _ := range fl.GroupedList {
		inputList = append(inputList, input)
	}

	sort.Strings(inputList)

	// fl.MakeAnagramList()

	fl.listOfInputs = widget.NewSelect(inputList, func(selected string) {
		fl.selectedList = fl.GroupedList[selected]
		fmt.Printf("Selected \"%s\" (%d elements)\n", selected, len(fl.selectedList))
		go func() {
			time.Sleep(10 * time.Millisecond)
			fyne.Do(fl.AnagramDisplay.Refresh)
		}()
	})

	fl.inputSelectBar = container.New(layout.NewHBoxLayout(), fl.listOfInputs)

}

func (fl *FavoritesList) findGlobalID(selectedListID int) int {
	selectedFavorite := fl.selectedList[selectedListID]
	for globalID, globalFavorite := range *(fl.baseList) {
		if selectedFavorite.Dictionaries == globalFavorite.Dictionaries && selectedFavorite.Input == globalFavorite.Input && selectedFavorite.Anagram == globalFavorite.Anagram {
			return globalID
		}
	}
	fmt.Println("Couldn't find global id")
	return -1
}

func (fl *FavoritesList) MakeAnagramList() {
	// fmt.Printf("Building list of %d elements for input \"%s\"\n", len(fl.selectedList), fl.SelectedInput)
	fl.AnagramDisplay = widget.NewList(func() int {
		return len(fl.selectedList)
	}, func() fyne.CanvasObject {
		return NewTapLabel("Fav")
	}, func(id widget.ListItemID, obj fyne.CanvasObject) {
		label, ok := obj.(*TapLabel)
		if !ok {
			return
		}
		label.Label.Text = fl.labelFunc(fl.selectedList[id])
		label.Label.Alignment = fyne.TextAlignCenter
		label.OnTapped = func(pe *fyne.PointEvent) {
			copyAnagramToCBMI := fyne.NewMenuItem("Copy anagram to clipboard", func() {
				MainWindow.Clipboard().SetContent(fl.selectedList[id].Anagram)
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
				MainWindow.Clipboard().SetContent(fmt.Sprintf("%s ↔️ %s", fl.selectedList[id].Input, fl.selectedList[id].Anagram))
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
				anagrams := []string{fl.selectedList[id].Anagram}
				ShowAnimation("Animated anagram...", fl.selectedList[id].Input, anagrams, MainWindow)
			})
			/*
				multiAnimationMI := fyne.NewMenuItem(fmt.Sprintf("Animate multiple with input \"%s\"", fl.selectedList[id].Input), func() {
					anagrams := make([]string, len(groups[fl.selectedList[id].Input]))
					for index, fav := range groups[fl.selectedList[id].Input] {
						anagrams[index] = fav.Anagram
					}
					ShowMultiAnagramPicker("Animate which anagrams", "animate", "cancel", "shuffle", anagrams, func(chosen []string) {
						if len(chosen) > 0 {
							ShowAnimation("Animated anagrams...", fl.selectedList[id].Input, chosen, MainWindow)
						}
					}, MainWindow)
				})
			*/
			sendToMainMI := fyne.NewMenuItem("Send to main input tab", func() {
				fl.sendToMainTab(fl.selectedList[id])
			})
			editAnagramMI := fyne.NewMenuItem("Edit Anagram", func() {
				globalID := fl.findGlobalID(id)
				ShowFavoriteAnagramEditor(fl.baseList, globalID, AppPreferences, RebuildFavorites, MainWindow)
				// fl.AnagramDisplay.Refresh()
			})
			editInputMI := fyne.NewMenuItem("Edit Input", func() {
				globalID := fl.findGlobalID(id)
				ShowFavoriteInputEditor(fl.baseList, globalID, AppPreferences, RebuildFavorites, MainWindow)
				// fl.RegenGroups()
				// fl.AnagramDisplay.Refresh()
			})
			deleteMI := fyne.NewMenuItem("Delete", func() {
				globalID := fl.findGlobalID(id)
				ShowDeleteFavConfirm(fl.baseList, globalID, AppPreferences, RebuildFavorites, MainWindow)
				fl.AnagramDisplay.Refresh()
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
