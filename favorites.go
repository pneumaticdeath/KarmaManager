package main

import (
	"fmt"
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
)

const favoritesKey = "io.patenaude.karmamanager.favorites"

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

