package main

import (
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
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

const oauthRedirectURI = "https://karmamanager-sync.fly.dev/oauth/callback"

var searchtimeout time.Duration = time.Second
var searchlimit int = 100000
var MainWindow fyne.Window
var Icon fyne.Resource
var AppPreferences fyne.Preferences
var RebuildFavorites = func() {} // no-op until main() sets the real implementation
var favorites FavoritesSlice

func ShowPrivateDictSettings(private *Dictionary, saveCallback func(),  window fyne.Window) {
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
		if saveCallback != nil {
			saveCallback()
		}
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
	cd.Resize(fyne.NewSize(400, 600))
	cd.Show()

	var gifButton, videoButton *widget.Button
	var cachedGCT *GIFCaptureTool
	var cachedGIFPath, cachedMP4Path string

	// runCapture runs an off-screen animation to populate cachedGCT, then calls
	// onCaptured. If cachedGCT is already set, it skips straight to onCaptured.
	// Both export buttons are disabled for the duration.
	runCapture := func(onCaptured func(*GIFCaptureTool)) {
		gifButton.Disable()
		videoButton.Disable()
		if cachedGCT != nil {
			go func() {
				onCaptured(cachedGCT)
				fyne.Do(func() {
					gifButton.Enable()
					videoButton.Enable()
				})
			}()
			return
		}
		// Snapshot the surface size now, while the dialog is laid out.
		captureSize := ad.surface.Size()
		progressBar := widget.NewProgressBarInfinite()
		progressDialog := dialog.NewCustomWithoutButtons("Rendering…", progressBar, MainWindow)
		progressDialog.Show()
		go func() {
			adCapture := NewAnimationDisplay(Icon)
			adCapture.surface.Resize(captureSize)
			adCapture.Resize(captureSize)
			gct := NewGIFCaptureTool()
			adCapture.CaptureCallback = gct.MakeCaptureCallback(adCapture)
			adCapture.CycleCallback = func() { adCapture.Stop() }
			done := make(chan struct{})
			adCapture.FinishedCallback = func() { close(done) }
			adCapture.AnimateAnagrams(startPhrase, anagrams...)
			<-done
			cachedGCT = gct
			fyne.Do(progressDialog.Hide)
			onCaptured(gct)
			fyne.Do(func() {
				gifButton.Enable()
				videoButton.Enable()
			})
		}()
	}

	gifButton = widget.NewButton("Save as GIF", func() {
		if cachedGIFPath != "" {
			ShareGIF(cachedGIFPath, MainWindow)
			return
		}
		runCapture(func(gct *GIFCaptureTool) {
			g := gct.GetGIF()
			tmpPath := os.TempDir() + "/karmamanager_anim.gif"
			err := WriteGIF(g, tmpPath)
			fyne.Do(func() {
				if err != nil {
					dialog.ShowError(err, MainWindow)
				} else {
					cachedGIFPath = tmpPath
					ShareGIF(tmpPath, MainWindow)
				}
			})
		})
	})

	videoButton = widget.NewButton("Save as Video", func() {
		if cachedMP4Path != "" {
			ShareVideo(cachedMP4Path, MainWindow)
			return
		}
		runCapture(func(gct *GIFCaptureTool) {
			frames, delays := gct.GetRawFrames()
			tmpPath := os.TempDir() + "/karmamanager_anim.mp4"
			err := WriteMP4(frames, delays, tmpPath)
			fyne.Do(func() {
				if err != nil {
					dialog.ShowError(err, MainWindow)
				} else {
					cachedMP4Path = tmpPath
					ShareVideo(tmpPath, MainWindow)
				}
			})
		})
	})

	cd.SetButtons([]fyne.CanvasObject{
		widget.NewButton("Dismiss", func() { cd.Hide() }),
		gifButton,
		videoButton,
	})

	ad.FinishedCallback = func() {
		fyne.Do(cd.Hide)
	}
	ad.AnimateAnagrams(startPhrase, anagrams...)
	cd.SetOnClosed(func() {
		ad.Stop()
	})
}

func ShowMultiPicker(title, submitlabel, dismisslabel, shufflelabel string, choices []string, callback func([]string), window fyne.Window) {
	anaChecks := make([]bool, len(choices))
	for index := range choices {
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
		check.Text = UnmarkSpaces(choices[id])
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
		chosen := make([]string, 0, len(choices))
		for index, check := range anaChecks {
			if check {
				chosen = append(chosen, choices[index])
			}
		}
		callback(chosen)
	})
	submitbutton.Importance = widget.HighImportance
	shufflebutton := widget.NewButton(shufflelabel, func() {
		rand.Shuffle(len(choices), func(i, j int) {
			choices[i], choices[j] = choices[j], choices[i]
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
	rs.GetAt(searchlimit - 10) // just to get a little bit of data to work with
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
		label.Label.Text = fmt.Sprintf("%s %d", UnmarkSpaces(topN[id].Word), topN[id].Count)
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

func ShowAccountDialog(window fyne.Window) {
	if SyncSvc != nil && SyncSvc.IsAuthenticated() {
		showSignedInDialog(window)
	} else {
		showSignInDialog(window)
	}
}

func showSignedInDialog(window fyne.Window) {
	emailLabel := widget.NewLabel("Signed in as: " + SyncSvc.UserEmail())
	emailLabel.Wrapping = fyne.TextWrapWord

	syncNowButton := widget.NewButton("Sync Now", nil)
	signOutButton := widget.NewButton("Sign Out", nil)
	deleteAccountButton := widget.NewButton("Delete Account", nil)
	deleteAccountButton.Importance = widget.DangerImportance

	var d *dialog.CustomDialog
	syncNowButton.OnTapped = func() {
		syncNowButton.Disable()
		go func() {
			if err := SyncSvc.FullSync(&favorites); err != nil {
				fyne.Do(func() { dialog.ShowError(err, window) })
			} else {
				fyne.Do(func() { ShowPopUpMessage("Sync complete", time.Second, window) })
			}
			fyne.Do(syncNowButton.Enable)
		}()
	}
	signOutButton.OnTapped = func() {
		SyncSvc.SignOut()
		d.Hide()
		ShowPopUpMessage("Signed out", time.Second, window)
	}
	deleteAccountButton.OnTapped = func() {
		dialog.ShowConfirm(
			"Delete Account",
			"This will permanently delete your account and all synced favorites from the server. Your local favorites are not affected. This cannot be undone.",
			func(confirmed bool) {
				if !confirmed {
					return
				}
				deleteAccountButton.Disable()
				go func() {
					err := SyncSvc.DeleteAccount()
					fyne.Do(func() {
						if err != nil {
							dialog.ShowError(err, window)
							deleteAccountButton.Enable()
							return
						}
						d.Hide()
						ShowPopUpMessage("Account deleted", time.Second, window)
					})
				}()
			},
			window,
		)
	}

	content := container.NewVBox(
		emailLabel,
		widget.NewSeparator(),
		syncNowButton,
		signOutButton,
		widget.NewSeparator(),
		deleteAccountButton,
	)
	d = dialog.NewCustomWithoutButtons("Sync Account", content, window)
	d.SetButtons([]fyne.CanvasObject{
		widget.NewButton("Close", func() { d.Hide() }),
	})
	d.Show()
}

func showSignInDialog(window fyne.Window) {
	emailEntry := widget.NewEntry()
	emailEntry.SetPlaceHolder("your@email.com")

	codeEntry := widget.NewEntry()
	codeEntry.SetPlaceHolder("6-digit code")
	codeEntry.Hide()

	statusLabel := widget.NewLabel("")
	statusLabel.Wrapping = fyne.TextWrapWord

	var otpID string
	var sendButton, verifyButton *widget.Button
	var d *dialog.CustomDialog

	// OAuth section — buttons are added asynchronously when providers are fetched.
	oauthButtons := container.NewVBox()
	orLabel := widget.NewLabel("— or —")
	orLabel.Alignment = fyne.TextAlignCenter
	orLabel.Hide()

	// setAllButtonsEnabled enables or disables every interactive element in the dialog.
	setAllButtonsEnabled := func(enabled bool) {
		for _, obj := range oauthButtons.Objects {
			if btn, ok := obj.(*widget.Button); ok {
				if enabled {
					btn.Enable()
				} else {
					btn.Disable()
				}
			}
		}
		if enabled {
			sendButton.Enable()
		} else {
			sendButton.Disable()
		}
	}

	sendButton = widget.NewButton("Send Code", func() {
		email := strings.TrimSpace(emailEntry.Text)
		if email == "" {
			statusLabel.SetText("Please enter your email address.")
			return
		}
		sendButton.Disable()
		statusLabel.SetText("Sending…")
		go func() {
			id, err := SyncSvc.RequestOTP(email)
			fyne.Do(func() {
				if err != nil {
					statusLabel.SetText("Error: " + err.Error())
					sendButton.Enable()
					return
				}
				otpID = id
				emailEntry.Hide()
				sendButton.Hide()
				codeEntry.Show()
				verifyButton.Show()
				statusLabel.SetText("Check your email for a 6-digit code.")
			})
		}()
	})

	verifyButton = widget.NewButton("Verify", func() {
		code := strings.TrimSpace(codeEntry.Text)
		if code == "" {
			statusLabel.SetText("Please enter the code from your email.")
			return
		}
		verifyButton.Disable()
		statusLabel.SetText("Verifying…")
		go func() {
			err := SyncSvc.AuthWithOTP(otpID, code)
			fyne.Do(func() {
				if err != nil {
					statusLabel.SetText("Error: " + err.Error())
					verifyButton.Enable()
					return
				}
				d.Hide()
				ShowPopUpMessage("Signed in!", time.Second, window)
				go func() {
					if err := SyncSvc.FullSync(&favorites); err != nil {
						log.Println("Post-login sync failed:", err)
					}
				}()
			})
		}()
	})
	verifyButton.Hide()

	// Async-fetch OAuth providers and populate buttons when they arrive.
	go func() {
		if SyncSvc == nil {
			return
		}
		providers, err := SyncSvc.FetchOAuthProviders()
		if err != nil {
			log.Println("FetchOAuthProviders:", err)
			fyne.Do(func() { statusLabel.SetText("Could not load sign-in options: " + err.Error()) })
			return
		}
		if len(providers) == 0 {
			return
		}
		fyne.Do(func() {
			for _, p := range providers {
				p := p // capture loop variable
				btn := widget.NewButton("Sign in with "+p.DisplayName, func() {
					setAllButtonsEnabled(false)
					statusLabel.SetText("Opening browser…")
					codeVerifier := generateCodeVerifier()
					codeChallenge := generateCodeChallenge(codeVerifier)
					authURL := buildOAuthURL(p.AuthURL, oauthRedirectURI, codeChallenge)
					OpenOAuthBrowser(authURL, window)
					go func() {
						code, err := SyncSvc.PollOAuthCode(p.State, 120*time.Second)
						if err != nil {
							DismissOAuthBrowser()
							fyne.Do(func() {
								statusLabel.SetText("Error: " + err.Error())
								setAllButtonsEnabled(true)
							})
							return
						}
						DismissOAuthBrowser()
						err = SyncSvc.AuthWithOAuth2(p.Name, code, codeVerifier, oauthRedirectURI)
						if err != nil {
							fyne.Do(func() {
								statusLabel.SetText("Error: " + err.Error())
								setAllButtonsEnabled(true)
							})
							return
						}
						fyne.Do(func() {
							d.Hide()
							ShowPopUpMessage("Signed in!", time.Second, window)
							go func() {
								if err := SyncSvc.FullSync(&favorites); err != nil {
									log.Println("Post-login sync failed:", err)
								}
							}()
						})
					}()
				})
				oauthButtons.Add(btn)
			}
			orLabel.Show()
			oauthButtons.Refresh()
		})
	}()

	content := container.NewVBox(oauthButtons, orLabel, emailEntry, codeEntry, statusLabel)
	d = dialog.NewCustom("Sign In to Sync", "Cancel", content, window)
	d.SetButtons([]fyne.CanvasObject{
		widget.NewButton("Cancel", func() { d.Hide() }),
		sendButton,
		verifyButton,
	})
	d.Show()
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
	App := app.NewWithID("io.patenaude.karmamanager")
	MainWindow = App.NewWindow("Karma Manger")

	InitConfig(App)

	Icon = App.Metadata().Icon
	AppPreferences = App.Preferences()
	SyncSvc = NewSyncClient(AppPreferences)

	favorites = Favorites(App.Preferences())
	// Deduplicate local favorites at load time — no network needed.
	// Handles duplicates that accumulated before or between syncs.
	if deduped := localDedupFavorites(favorites); len(deduped) < len(favorites) {
		favorites = deduped
		SaveFavorites(favorites, AppPreferences)
	}
	if SyncSvc.IsAuthenticated() {
		go func() {
			if err := SyncSvc.FullSync(&favorites); err != nil {
				log.Println("Auto-sync failed:", err)
			}
		}()
	}

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

	addedChecks := make([]fyne.CanvasObject, len(addedDicts)+2)
	for i, ad := range addedDicts {
		enabled := &ad.Enabled // copy a pointer to an address
		check := widget.NewCheck(ad.Name, func(checked bool) {
			*enabled = checked
			resultSet.RebuildDictionaries()
			MainWindow.SetTitle(resultSet.CombinedDictName())
			saveDictSelections()
		})
		ad.Enabled = false
		for _, name := range remainingDicts {
			if ad.Name == name {
				// log.Printf("Enabling added dictionary %s\n", name)
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
		resultSet.RebuildDictionaries()
		MainWindow.SetTitle(resultSet.CombinedDictName())
		saveDictSelections()
	})
	privateCheck.Checked = privateDict.Enabled
	addedChecks[len(addedDicts)] = privateCheck
	privateDictSettingsButton := widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), func() {
		ShowPrivateDictSettings(privateDict, resultSet.DumpCache, MainWindow)
	})
	addedChecks[len(addedDicts)+1] = privateDictSettingsButton
	addedDictsContainer := container.New(layout.NewHBoxLayout(), addedChecks...)

	mainSelect := widget.NewSelect(mainDictNames, func(dictName string) {
		for i, n := range mainDictNames {
			if dictName == n {
				selectedMainIndex = i
				resultSet.SetMainIndex(i)
				MainWindow.SetTitle(resultSet.CombinedDictName())
				saveDictSelections()
				return
			}
		}
		dialog.ShowError(errors.New("Can't find selected main dictionary"), MainWindow)
	})
	mainSelect.SetSelectedIndex(selectedMainIndex)

	inputdata := binding.NewString()
	inputEntry := widget.NewEntryWithData(inputdata)
	inputEntry.SetPlaceHolder("What are we anagramming?")
	inputdata.AddListener(binding.NewDataListener(func() {
		if inputEntry.OnSubmitted != nil {
			inputEntry.OnSubmitted(inputEntry.Text)
		}
	}))

	inputClearButton := widget.NewButtonWithIcon("", theme.ContentClearIcon(), func() {
		inputdata.Set("")
		time.Sleep(50 * time.Millisecond)
		reset_search()
	})

	progressBar := widget.NewProgressBar()
	progressBar.Min = 0.0
	progressBar.Max = 1.0
	pbCallback := func(current, goal int) {
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
		})
	}
	wbStopCallback := func() {
		fyne.Do(func() {
			workingBar.Stop()
			workingBar.Hide()
			interestingButton.Enable()
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
		if resultSet.IsEmpty() {
			return 1 // So we get the "No results" label if the list is empty
		}
		return resultSet.Count()
	}, func() fyne.CanvasObject { // Make new entry
		return NewTapLabel("Foo")
	}, func(index int, object fyne.CanvasObject) { // Update entry
		return // to be replaced later
	})
	inputEntry.OnSubmitted = func(input string) {
		reset_search()
		resultSet.FindAnagrams(input)
	}

	inclusionwords := NewWordList([]string{})
	SetInclusions := func() {
		includestring := strings.Join(inclusionwords.Words, " ")
		if includestring != "" {
			resultSet.SetInclusions([]string{includestring})
		} else {
			resultSet.SetInclusions([]string{})
		}
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
		text, text_ok := resultSet.GetAt(id)
		if text_ok {
			label.Label.Text = fmt.Sprintf("%10d %s", id+1, UnmarkSpaces(text))
			label.Label.TextStyle = fyne.TextStyle{Italic: false}
			label.OnTapped = func(pe *fyne.PointEvent) {
				input, _ := inputdata.Get()
				copyAnagramToCBMI := fyne.NewMenuItem("Copy anagram to clipboard", func() {
					MainWindow.Clipboard().SetContent(UnmarkSpaces(text))
					ShowPopUpMessage("Copied to clipboard", time.Second, MainWindow)
				})
				copyBothToCBMI := fyne.NewMenuItem("Copy input and anagram to clipboard", func() {
					MainWindow.Clipboard().SetContent(fmt.Sprintf("%s ↔️ %s", input, UnmarkSpaces(text)))
					ShowPopUpMessage("Copied to clipboard", time.Second, MainWindow)
				})
				addToFavsMI := fyne.NewMenuItem("Add to favorites", func() {
					// first check to see if this is a duplicate
					newInputNormalized := Normalize(input)
					newAnagramNormalized := Normalize(text)
					for _, existing := range favorites {
						if newInputNormalized == Normalize(existing.Input) && newAnagramNormalized == Normalize(existing.Anagram) {
							// log.Printf("Detected duplicate with \"%s\"\n", existing.Anagram)
							dialog.ShowConfirm("Duplicate detected", fmt.Sprintf("Looks similar to \"%s\".  Add anyway?", existing.Anagram), func(addAnyway bool) {
								if addAnyway {
									ShowEditor("Add to favorites", text, func(editted string) {
										newFav := FavoriteAnagram{resultSet.CombinedDictName(), strings.TrimSpace(input), editted, newUUID()}
										favorites = append(favorites, newFav)
										RebuildFavorites()
										SaveFavorites(favorites, App.Preferences())
										ShowPopUpMessage("Added to favorites", time.Second, MainWindow)
										if SyncSvc.IsAuthenticated() {
											go SyncSvc.Push(newFav)
										}
									}, MainWindow)
								} else {
									ShowPopUpMessage("Not adding duplicate", time.Second, MainWindow)
								}
							}, MainWindow)
							return
						}
					}
					ShowEditor("Add to favorites", text, func(editted string) {
						// log.Println("No duplicate detected")
						newFav := FavoriteAnagram{resultSet.CombinedDictName(), strings.TrimSpace(input), editted, newUUID()}
						favorites = append(favorites, newFav)
						RebuildFavorites()
						SaveFavorites(favorites, App.Preferences())
						ShowPopUpMessage("Added to favorites", time.Second, MainWindow)
						if SyncSvc.IsAuthenticated() {
							go SyncSvc.Push(newFav)
						}
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
					includeMIs[index] = fyne.NewMenuItem(UnmarkSpaces(word), func() {
						includeFunc(word)
					})
					excludeMIs[index] = fyne.NewMenuItem(UnmarkSpaces(word), func() {
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
		} else {
			label.Label.Text = "       No results!"
			label.Label.TextStyle = fyne.TextStyle{Italic: true}
		}
		obj.Refresh()
	}

	findContent := container.NewBorder(controlBar, nil, nil, nil, mainDisplay)

	var selectTab func(int)

	sendToMainTabFunc := func(input string) {
		inputdata.Set(input)
		time.Sleep(50 * time.Millisecond)
		selectTab(0)
		resultsDisplay.Refresh()
	}

	favsList := NewFavoritesDisplay(&favorites, sendToMainTabFunc)

	favsContent := favsList

	RebuildFavorites = func() {
		sort.Sort(favorites)
		favsList.RegenGroups()
		favsList.Refresh()
	}

	iconImage := canvas.NewImageFromResource(Icon)
	iconImage.SetMinSize(fyne.NewSize(128, 128))
	iconImage.FillMode = canvas.ImageFillContain

	aboutContent := container.New(layout.NewVBoxLayout(),
		layout.NewSpacer(),
		iconImage,
		container.New(layout.NewHBoxLayout(), layout.NewSpacer(), widget.NewLabel(App.Metadata().Name), layout.NewSpacer()),
		container.New(layout.NewHBoxLayout(), layout.NewSpacer(), widget.NewLabel("Copyright 2025, 2026"), layout.NewSpacer()),
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

	resultSet.SetRefreshCallback(func() {
		fyne.Do(resultsDisplay.Refresh)
		fyne.Do(resultsDisplay.ScrollToTop)
	})

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
