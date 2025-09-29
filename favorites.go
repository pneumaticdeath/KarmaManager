package main

import (
	"fmt"
	"log"
	// "image/gif"
	"slices"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
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
	oldInput := fav.Input
	ShowEditor("Edit input phrase", fav.Input, func(newInput string) {
		if newInput != oldInput {
			for f_index, f := range *favs {
				if f.Input == oldInput {
					f.Input = newInput
					(*favs)[f_index] = f
				}
			}
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

type FavoritesAccList struct {
	widget.BaseWidget

	surface            *fyne.Container
	buttonbar          *fyne.Container
	listSize           int
	list               *widget.List
	MultiAnimateButton *widget.Button
	// SendToMainButton   *widget.Button
	baseSlice *FavoritesSlice
}

func NewFavoritesAccList(title string, baseList, this *FavoritesSlice, sendToMain func(string)) *FavoritesAccList {
	fal := &FavoritesAccList{}
	// fal.title = title
	fal.baseSlice = baseList

	fal.listSize = len(*this)

	fal.MultiAnimateButton = widget.NewButton("Animate many", func() {
		if len(*this) > 1 {
			// fmt.Printf("Making multi anim dialog for %s\n", title)
			anagrams := make([]string, len(*this))
			for index, fav := range *this {
				anagrams[index] = fav.Anagram
			}
			ShowMultiAnagramPicker("Animate which anagrams", "animate", "cancel", "shuffle", anagrams, func(chosen []string) {
				if len(chosen) > 0 {
					ShowAnimation("Animated anagrams...", title, chosen, MainWindow)
				}
			}, MainWindow)

		}
	})

	sendToMainButton := widget.NewButton("Send to Find tab", func() {
		sendToMain(title)
	})

	editInputButton := widget.NewButton("Edit input", func() {
		if len(*this) > 0 {
			ShowFavoriteInputEditor(fal.baseSlice, fal.findGlobalID((*this)[0]), AppPreferences, RebuildFavorites, MainWindow)
		} else {
			log.Panicln("FavoritesAccList: trying to edit input on a 0 length list")
		}
	})

	fal.buttonbar = container.New(layout.NewGridLayout(3), fal.MultiAnimateButton, sendToMainButton, editInputButton)

	fal.list = widget.NewList(func() int {
		return len(*this)
	}, func() fyne.CanvasObject {
		return NewTapLabel("Fav")
	}, func(id widget.ListItemID, obj fyne.CanvasObject) {
		label, ok := obj.(*TapLabel)
		if !ok {
			return
		}
		label.Label.Text = (*this)[id].Anagram
		label.Label.Alignment = fyne.TextAlignCenter
		label.OnTapped = func(pe *fyne.PointEvent) {
			copyAnagramMI := fyne.NewMenuItem("Copy anagram to clipboard", func() {
				MainWindow.Clipboard().SetContent((*this)[id].Anagram)
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
			copyBothMI := fyne.NewMenuItem("Copy input and anagram to clipboard", func() {
				MainWindow.Clipboard().SetContent(fmt.Sprintf("%s ↔️ %s", (*this)[id].Input, (*this)[id].Anagram))
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
				anagrams := []string{(*this)[id].Anagram}
				ShowAnimation("Animated anagram...", (*this)[id].Input, anagrams, MainWindow)
			})
			sendToMainMI := fyne.NewMenuItem("Send anagram to Find tab", func() {
				sendToMain((*this)[id].Anagram)
			})
			editMI := fyne.NewMenuItem("Edit", func() {
				globalID := fal.findGlobalID((*this)[id])
				ShowFavoriteAnagramEditor(fal.baseSlice, globalID, AppPreferences, RebuildFavorites, MainWindow)
			})
			deleteMI := fyne.NewMenuItem("Delete", func() {
				globalID := fal.findGlobalID((*this)[id])
				ShowDeleteFavConfirm(fal.baseSlice, globalID, AppPreferences, RebuildFavorites, MainWindow)
			})
			pumenu := fyne.NewMenu("Pop up", copyAnagramMI, copyBothMI, animateMI, sendToMainMI, editMI, deleteMI)
			widget.ShowPopUpMenuAtRelativePosition(pumenu, MainWindow.Canvas(), pe.Position, label)
		}

		label.Refresh()
	})

	fal.surface = container.NewBorder(fal.buttonbar, nil, nil, nil, fal.list)

	fal.ExtendBaseWidget(fal)

	return fal
}

func (fal *FavoritesAccList) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(fal.surface)
}

func (fal *FavoritesAccList) MinSize() fyne.Size {
	bbsize := fal.buttonbar.MinSize()
	lsize := fal.list.MinSize()
	length := min(fal.listSize, 5)
	return fyne.NewSize(max(bbsize.Width, lsize.Width), bbsize.Height+float32(length)*(lsize.Height+5.0))
}

func (fal *FavoritesAccList) findGlobalID(selectedFavorite FavoriteAnagram) int {
	for globalID, globalFavorite := range *(fal.baseSlice) {
		if selectedFavorite.Dictionaries == globalFavorite.Dictionaries && selectedFavorite.Input == globalFavorite.Input && selectedFavorite.Anagram == globalFavorite.Anagram {
			return globalID
		}
	}
	fmt.Println("Couldn't find global id")
	return -1
}

type FavoritesDisplay struct {
	widget.BaseWidget

	baseList      *FavoritesSlice
	groupedList   GroupedFavorites
	selectedInput string
	accordion     *widget.Accordion
	surface       *fyne.Container
	sendToMain    func(string)
}

func NewFavoritesDisplay(list *FavoritesSlice, sendToMain func(string)) *FavoritesDisplay {
	fd := &FavoritesDisplay{}

	fd.baseList = list
	fd.sendToMain = sendToMain
	fd.RegenGroups()

	preferencesButton := widget.NewButtonWithIcon("Animation Settings", theme.SettingsIcon(), Config.ShowPreferencesDialog)

	fd.surface = container.NewBorder(preferencesButton, nil, nil, nil, container.NewVScroll(fd.accordion))

	fd.ExtendBaseWidget(fd)

	return fd
}

func (fd *FavoritesDisplay) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(fd.surface)
}

func (fd *FavoritesDisplay) RegenGroups() {
	fd.groupedList = MakeGroupedFavorites(*fd.baseList)

	inputs := make([]string, 0, len(fd.groupedList))
	for input, _ := range fd.groupedList {
		inputs = append(inputs, input)
	}
	sort.Slice(inputs, func(i, j int) bool {
		return strings.ToLower(inputs[i]) < strings.ToLower(inputs[j])
	})

	opened := make(map[string]bool)
	if fd.accordion != nil {
		for _, ai := range fd.accordion.Items {
			title := ai.Title
			li := strings.LastIndex(title, " (")
			if li >= 0 {
				opened[title[:li]] = ai.Open
			} else {
				opened[title] = ai.Open
			}
		}
	}

	accordionItemList := make([]*(widget.AccordionItem), 0, len(fd.groupedList))
	for _, input := range inputs {
		fav := fd.groupedList[input]
		fal := NewFavoritesAccList(input, fd.baseList, &fav, fd.sendToMain)
		if len(fd.groupedList[input]) > 1 {
			fal.MultiAnimateButton.Enable()
		} else {
			fal.MultiAnimateButton.Disable()
		}

		ai := widget.NewAccordionItem(fmt.Sprintf("%s (%d)", input, len(fd.groupedList[input])), fal)
		ai.Open = opened[input]
		accordionItemList = append(accordionItemList, ai)
	}

	if fd.accordion == nil {
		fd.accordion = widget.NewAccordion(accordionItemList...)
		fd.accordion.MultiOpen = false
	} else {
		fd.accordion.Items = accordionItemList
		fd.accordion.Refresh()
	}

	// fd.surface.Refresh()
}
