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
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const favoritesKey = "io.patenaude.karmamanager.favorites"

type FavoriteAnagram struct {
	Dictionaries, Input string
	Anagram             string
	ID                  string // client-generated UUID, stable sync key
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
	if fav.ID == "" {
		fav.ID = newUUID()
	}
	return fmt.Sprintf("%s\n%s\n%s\n%s", fav.Dictionaries, fav.Input, fav.Anagram, fav.ID)
}

func decodeFavorite(s string) FavoriteAnagram {
	lines := strings.Split(s, "\n")
	fav := FavoriteAnagram{Dictionaries: lines[0], Input: lines[1], Anagram: lines[2]}
	if len(lines) > 3 && lines[3] != "" {
		fav.ID = lines[3]
	} else {
		fav.ID = newUUID()
	}
	return fav
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

// localDedupFavorites returns a new slice with content duplicates removed,
// keeping the first occurrence by normalized (input, anagram) pair.
func localDedupFavorites(favs FavoritesSlice) FavoritesSlice {
	type contentKey struct{ input, anagram string }
	seen := make(map[contentKey]bool, len(favs))
	out := make(FavoritesSlice, 0, len(favs))
	for _, fav := range favs {
		k := contentKey{Normalize(fav.Input), Normalize(fav.Anagram)}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, fav)
	}
	return out
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
			if SyncSvc != nil && SyncSvc.IsAuthenticated() {
				go SyncSvc.Push(fav)
			}
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
					if SyncSvc != nil && SyncSvc.IsAuthenticated() {
						go SyncSvc.Push(f)
					}
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
	if id < 0 || id >= len(*favs) {
		return
	}
	fav := (*favs)[id]
	dialog.ShowConfirm("Really delete?", fmt.Sprintf("Really delete \"%s\"?", fav.Anagram), func(confirmed bool) {
		if confirmed {
			clientID := fav.ID
			*favs = slices.Delete(*favs, id, id+1)
			refresh()
			SaveFavorites(*favs, prefs)
			if SyncSvc != nil && SyncSvc.IsAuthenticated() {
				go SyncSvc.DeleteRemote(clientID)
			}
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

// findGlobalFavID returns the index of fav in baseList, preferring ID match.
func findGlobalFavID(baseList *FavoritesSlice, fav FavoriteAnagram) int {
	if fav.ID != "" {
		for i, f := range *baseList {
			if f.ID == fav.ID {
				return i
			}
		}
	}
	for i, f := range *baseList {
		if fav.Dictionaries == f.Dictionaries && fav.Input == f.Input && fav.Anagram == f.Anagram {
			return i
		}
	}
	return -1
}

// --- Flat virtualized favorites list ---

type favRowKind int

const (
	favRowHeader  favRowKind = iota
	favRowAnagram            // nolint:deadcode,varcheck
)

type favFlatRow struct {
	kind  favRowKind
	input string          // group key (both kinds)
	fav   FavoriteAnagram // anagram rows only
	count int             // header rows only
}

type FavoritesDisplay struct {
	widget.BaseWidget

	baseList     *FavoritesSlice
	groupedList  GroupedFavorites
	openGroups   map[string]bool
	sortedInputs []string
	flatRows     []favFlatRow
	list         *widget.List
	surface      *fyne.Container
	sendToMain   func(string)
}

func (fd *FavoritesDisplay) buildFlatRows() {
	fd.flatRows = fd.flatRows[:0]
	for _, input := range fd.sortedInputs {
		group := fd.groupedList[input]
		fd.flatRows = append(fd.flatRows, favFlatRow{kind: favRowHeader, input: input, count: len(group)})
		if fd.openGroups[input] {
			for _, fav := range group {
				fd.flatRows = append(fd.flatRows, favFlatRow{kind: favRowAnagram, input: input, fav: fav})
			}
		}
	}
}

func (fd *FavoritesDisplay) toggleGroup(input string) {
	fd.openGroups[input] = !fd.openGroups[input]
	fd.buildFlatRows()
	fd.list.Refresh()
}

func (fd *FavoritesDisplay) RegenGroups() {
	fd.groupedList = MakeGroupedFavorites(*fd.baseList)
	inputs := make([]string, 0, len(fd.groupedList))
	for input := range fd.groupedList {
		inputs = append(inputs, input)
	}
	sort.Slice(inputs, func(i, j int) bool {
		return strings.ToLower(inputs[i]) < strings.ToLower(inputs[j])
	})
	fd.sortedInputs = inputs
	// Prune open state for groups that no longer exist.
	for input := range fd.openGroups {
		if _, ok := fd.groupedList[input]; !ok {
			delete(fd.openGroups, input)
		}
	}
	fd.buildFlatRows()
	if fd.list != nil {
		fd.list.Refresh()
	}
}

func (fd *FavoritesDisplay) createListItem() fyne.CanvasObject {
	toggleBtn := widget.NewButton("▶", nil)
	inputLabel := NewTapLabel("input")
	inputLabel.Label.TextStyle = fyne.TextStyle{Bold: true}
	sendBtn := widget.NewButtonWithIcon("", theme.SearchIcon(), nil)
	editBtn := widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), nil)
	animBtn := widget.NewButtonWithIcon("", theme.MediaPlayIcon(), nil)
	headerCont := container.NewHBox(toggleBtn, inputLabel, layout.NewSpacer(), sendBtn, editBtn, animBtn)

	anagramLabel := NewTapLabel("anagram")
	anagramLabel.Label.Alignment = fyne.TextAlignCenter
	anagramCont := container.NewPadded(anagramLabel)

	return container.NewStack(headerCont, anagramCont)
}

func (fd *FavoritesDisplay) updateListItem(id widget.ListItemID, obj fyne.CanvasObject) {
	stack, ok := obj.(*fyne.Container)
	if !ok || id >= len(fd.flatRows) {
		return
	}
	row := fd.flatRows[id]
	headerCont := stack.Objects[0].(*fyne.Container)
	anagramCont := stack.Objects[1].(*fyne.Container)

	if row.kind == favRowHeader {
		anagramCont.Hide()
		headerCont.Show()

		toggleBtn := headerCont.Objects[0].(*widget.Button)
		inputLabel := headerCont.Objects[1].(*TapLabel)
		// Objects[2] is the spacer — skip
		sendBtn := headerCont.Objects[3].(*widget.Button)
		editBtn := headerCont.Objects[4].(*widget.Button)
		animBtn := headerCont.Objects[5].(*widget.Button)

		input := row.input
		if fd.openGroups[input] {
			toggleBtn.SetText("▼")
		} else {
			toggleBtn.SetText("▶")
		}
		toggleBtn.OnTapped = func() { fd.toggleGroup(input) }
		inputLabel.Label.SetText(fmt.Sprintf("%s (%d)", input, row.count))
		inputLabel.OnTapped = func(_ *fyne.PointEvent) { fd.toggleGroup(input) }

		sendBtn.OnTapped = func() { fd.sendToMain(input) }

		editBtn.OnTapped = func() {
			group := fd.groupedList[input]
			if len(group) > 0 {
				globalID := findGlobalFavID(fd.baseList, group[0])
				ShowFavoriteInputEditor(fd.baseList, globalID, AppPreferences, RebuildFavorites, MainWindow)
			}
		}

		if row.count > 1 {
			animBtn.Enable()
		} else {
			animBtn.Disable()
		}
		animBtn.OnTapped = func() {
			group := fd.groupedList[input]
			anagrams := make([]string, len(group))
			for i, fav := range group {
				anagrams[i] = fav.Anagram
			}
			ShowMultiPicker("Animate which anagrams", "animate", "cancel", "shuffle", anagrams, func(chosen []string) {
				if len(chosen) > 0 {
					ShowAnimation("Animated anagrams...", input, chosen, MainWindow)
				}
			}, MainWindow)
		}
	} else {
		headerCont.Hide()
		anagramCont.Show()

		anagramLabel := anagramCont.Objects[0].(*TapLabel)
		fav := row.fav
		anagramLabel.Label.Text = UnmarkSpaces(fav.Anagram)
		anagramLabel.Label.Refresh()
		anagramLabel.OnTapped = func(pe *fyne.PointEvent) {
			copyAnagramMI := fyne.NewMenuItem("Copy anagram to clipboard", func() {
				MainWindow.Clipboard().SetContent(UnmarkSpaces(fav.Anagram))
				ShowPopUpMessage("Copied to clipboard", time.Second, MainWindow)
			})
			copyBothMI := fyne.NewMenuItem("Copy input and anagram to clipboard", func() {
				MainWindow.Clipboard().SetContent(fmt.Sprintf("%s ↔️ %s", fav.Input, UnmarkSpaces(fav.Anagram)))
				ShowPopUpMessage("Copied to clipboard", time.Second, MainWindow)
			})
			animateMI := fyne.NewMenuItem("Animate", func() {
				ShowAnimation("Animated anagram...", fav.Input, []string{fav.Anagram}, MainWindow)
			})
			sendToMainMI := fyne.NewMenuItem("Send anagram to Find tab", func() {
				fd.sendToMain(fav.Anagram)
			})
			editMI := fyne.NewMenuItem("Edit", func() {
				globalID := findGlobalFavID(fd.baseList, fav)
				ShowFavoriteAnagramEditor(fd.baseList, globalID, AppPreferences, RebuildFavorites, MainWindow)
			})
			deleteMI := fyne.NewMenuItem("Delete", func() {
				globalID := findGlobalFavID(fd.baseList, fav)
				ShowDeleteFavConfirm(fd.baseList, globalID, AppPreferences, RebuildFavorites, MainWindow)
			})
			shareLinkMI := fyne.NewMenuItem("Share link", func() {
				if SyncSvc == nil || !SyncSvc.IsAuthenticated() {
					dialog.ShowInformation("Account required", "Sign in via the Sync button in Favorites to share anagrams.", MainWindow)
					return
				}
				go func() {
					shareURL, err := SyncSvc.GenerateShareURL(fav.ID)
					fyne.Do(func() {
						if err != nil {
							dialog.ShowError(err, MainWindow)
							return
						}
						MainWindow.Clipboard().SetContent(shareURL)
						ShowPopUpMessage("Link copied!", time.Second, MainWindow)
					})
				}()
			})
			pumenu := fyne.NewMenu("Pop up", copyAnagramMI, copyBothMI, animateMI, sendToMainMI, editMI, deleteMI, shareLinkMI)
			widget.ShowPopUpMenuAtRelativePosition(pumenu, MainWindow.Canvas(), pe.Position, anagramLabel)
		}
		anagramLabel.Refresh()
	}
}

// ShowImportFromLinkDialog lets the user paste a share URL and imports the
// referenced favorite into their local collection.
func ShowImportFromLinkDialog(favs *FavoritesSlice, prefs fyne.Preferences, refresh func(), window fyne.Window) {
	urlEntry := widget.NewEntry()
	urlEntry.SetPlaceHolder("Paste share link here…")
	// Pre-populate from clipboard when it looks like a share URL.
	if cb := window.Clipboard().Content(); strings.Contains(cb, "/share/") {
		urlEntry.SetText(cb)
	}

	items := []*widget.FormItem{widget.NewFormItem("Share link", urlEntry)}
	dialog.ShowForm("Import from link", "Import", "Cancel", items, func(submitted bool) {
		if !submitted {
			return
		}
		go func() {
			fav, err := FetchSharedFavorite(urlEntry.Text)
			fyne.Do(func() {
				if err != nil {
					dialog.ShowError(err, window)
					return
				}
				// Duplicate check.
				normIn := Normalize(fav.Input)
				normAn := Normalize(fav.Anagram)
				for _, existing := range *favs {
					if Normalize(existing.Input) == normIn && Normalize(existing.Anagram) == normAn {
						dialog.ShowInformation("Already in favorites",
							fmt.Sprintf("%q is already in your favorites.", UnmarkSpaces(fav.Anagram)), window)
						return
					}
				}
				// Confirm and save.
				msg := fmt.Sprintf("Import \"%s\" → \"%s\"?", fav.Input, UnmarkSpaces(fav.Anagram))
				dialog.ShowConfirm("Import favorite", msg, func(confirmed bool) {
					if !confirmed {
						return
					}
					*favs = append(*favs, fav)
					refresh()
					SaveFavorites(*favs, prefs)
					ShowPopUpMessage("Imported!", time.Second, window)
					if SyncSvc != nil && SyncSvc.IsAuthenticated() {
						go SyncSvc.Push(fav)
					}
				}, window)
			})
		}()
	}, window)
}

func NewFavoritesDisplay(list *FavoritesSlice, sendToMain func(string)) *FavoritesDisplay {
	fd := &FavoritesDisplay{
		baseList:   list,
		sendToMain: sendToMain,
		openGroups: make(map[string]bool),
	}
	fd.RegenGroups()

	fd.list = widget.NewList(
		func() int { return len(fd.flatRows) },
		fd.createListItem,
		fd.updateListItem,
	)

	preferencesButton := widget.NewButtonWithIcon("Animation Settings", theme.SettingsIcon(), Config.ShowPreferencesDialog)
	importButton := widget.NewButtonWithIcon("Import", theme.DownloadIcon(), func() {
		ShowImportFromLinkDialog(fd.baseList, AppPreferences, RebuildFavorites, MainWindow)
	})
	syncButton := widget.NewButtonWithIcon("Sync", theme.UploadIcon(), func() { ShowAccountDialog(MainWindow) })
	buttons := container.New(layout.NewGridLayout(3), preferencesButton, importButton, syncButton)
	fd.surface = container.NewBorder(buttons, nil, nil, nil, fd.list)
	fd.ExtendBaseWidget(fd)
	return fd
}

func (fd *FavoritesDisplay) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(fd.surface)
}
