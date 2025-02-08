package main

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

const favoritesKey = "io.patenaude.karmamanager.favorites"

type FavoriteAnagram struct {
	Dictionaries, Input string
	Anagram             string
}

type GroupedFavorites map[string][]FavoriteAnagram

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
	d.Resize(fyne.NewSize(600, 400))
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

func MakeGroupedFavorites(favs []FavoriteAnagram) map[string][]FavoriteAnagram {
	groups := make(map[string][]FavoriteAnagram)

	for _, fav := range favs {
		input := fav.Input
		_, present := groups[input]
		if !present {
			groups[input] = make([]FavoriteAnagram, 0, 3)
		}

		groups[input] = append(groups[input], fav)
	}

	return groups
}

func NewFavoritesList(list []FavoriteAnagram, labelFunc func(FavoriteAnagram) string, sendToMainTab func(FavoriteAnagram)) *widget.List {
	lobj := widget.NewList(func() int {
		return len(list)
	}, func() fyne.CanvasObject {
		return NewTapLabel("Fav")
	}, func(id widget.ListItemID, obj fyne.CanvasObject) {
		label, ok := obj.(*TapLabel)
		if !ok {
			return
		}
		label.Label.Text = labelFunc(list[id])
		label.Label.Alignment = fyne.TextAlignCenter
		label.OnTapped = func(pe *fyne.PointEvent) {
			copyAnagramToCBMI := fyne.NewMenuItem("Copy anagram to clipboard", func() {
				MainWindow.Clipboard().SetContent(list[id].Anagram)
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
				MainWindow.Clipboard().SetContent(fmt.Sprintf("%s->%s", list[id].Input, list[id].Anagram))
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
			animateMI := fyne.NewMenuItem("Animate", func() {
				ad := NewAnimationDisplay(Icon)
				cd := dialog.NewCustom("Animated anagram...", "dismiss", ad, MainWindow)
				cd.Resize(fyne.NewSize(600, 300))
				cd.Show()
				ad.AnimateAnagram(list[id].Input, list[id].Anagram)
				cd.SetOnClosed(func() {
					ad.Stop()
				})
			})
			sendToMainMI := fyne.NewMenuItem("Send to main input tab", func() {
				sendToMainTab(list[id])
			})
			editAnagramMI := fyne.NewMenuItem("Edit Anagram", func() {
				ShowFavoriteAnagramEditor(&list, id, AppPreferences, RebuildFavorites, MainWindow)
			})
			editInputMI := fyne.NewMenuItem("Edit Input", func() {
				ShowFavoriteInputEditor(&list, id, AppPreferences, RebuildFavorites, MainWindow)
			})
			deleteMI := fyne.NewMenuItem("Delete", func() {
				ShowDeleteFavConfirm(&list, id, AppPreferences, RebuildFavorites, MainWindow)
			})
			pumenu := fyne.NewMenu("Pop up", copyAnagramToCBMI, copyBothToCBMI, animateMI, sendToMainMI, editInputMI, editAnagramMI, deleteMI)
			widget.ShowPopUpMenuAtRelativePosition(pumenu, MainWindow.Canvas(), pe.Position, label)
		}

		label.Refresh()
	})

	return lobj
}
