# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## About

Karma Manager is a cross-platform anagram finder built in Go using the [Fyne](https://fyne.io/) GUI framework. It is primarily intended as a mobile app but also supports desktop and web targets.

## Commands

```bash
# Run tests
go test ./...

# Run a single test
go test -run TestName ./...

# Build and run locally
go run .

# Package for all desktop platforms (Windows, Linux, macOS)
./package_desktop.sh

# Release for macOS App Store
./release_desktop.sh

# Release for Android and iOS
./release_mobile.sh

# Build and deploy web (WASM)
./release_web.sh
```

Release scripts require a `.env` file with credentials (Android keystore, iOS certificates, web deploy target). `package_desktop.sh` uses `fyne-cross` for Windows/Linux and `fyne package` directly for macOS.

## Architecture

All code lives in a single `main` package. The core pipeline is:

**Input → RuneCluster → Dictionary filter → Recursive anagram search → ResultSet → UI**

### Key files

- **anagram.go** — Recursive backtracking anagram algorithm. `FindAnagrams` pre-filters a dictionary to words whose runes are a subset of the input, then `findTuples` recurses to find all multi-word combinations. Results stream via a channel.
- **cluster.go** — `RuneCluster` is `map[rune]int` (character frequency map). Subset checks, subtraction, and equality drive all anagram matching.
- **dictionary.go** — Loads dictionaries from embedded JSON files (`/json/`). Three main dictionaries (Full/Standard/Basic) and three optional ones (Names/Places/Offensive). Handles private user-defined word lists stored in Fyne preferences.
- **resultset.go** — Wraps the search state. Caches up to 25 past searches (LRU), manages inclusion/exclusion filters, and exposes lazy result fetching (`GetAt`, `FetchTo`) with mutex-based concurrency. UI callbacks for progress, refresh, and working state are set here.
- **favorites.go** — `FavoriteAnagram{Dictionaries, Input, Anagram}` stored in preferences. Grouped into `GroupedFavorites` (map keyed by Input) for accordion UI display.
- **main.go** — All UI layout and wiring. Sets up tabs (Find / Favorites / About), dictionary selector, inclusion/exclusion word lists, results list with context menus, and the "Interesting words" analysis dialog.
- **animation.go** — Letter-by-letter animation that morphs one phrase into its anagram. `AnimationDisplay` is a Fyne widget; `RuneGlyph` tracks per-character paths.
- **editor.go** — Draggable word-tile editor for adjusting favorite anagrams before saving.
- **preferences.go** — Animation colors and timing preferences (pulse/move/pause durations), stored via Fyne preferences API.

### Data flow for a search

1. User types in input field → `inputEntry.OnSubmitted` fires
2. `reset_search()` clears inclusions/exclusions
3. `resultSet.FindAnagrams(input)` — builds a `RuneCluster`, filters the combined dictionary, launches `findTuples` goroutine streaming results into an `RSState`
4. `RSState` is cached in `ResultSet`; results are lazy-loaded on demand via `GetAt`
5. UI list calls `resultsDisplay.UpdateItem` → `resultSet.GetAt(id)` → triggers `FetchTo` if needed
6. `SetRefreshCallback` fires `resultsDisplay.Refresh` when new results arrive

### Dictionary structure

Embedded in the binary via `//go:embed`. `main-dicts.json` and `added-dicts.json` reference JSON word-list files. At runtime, dictionaries are combined into a single deduplicated `combinedDict` on the `ResultSet`.

### Preferences keys

- `favorites` — newline-delimited encoded favorite anagrams
- `private-dictionary` — user-added words
- `dictionary-selections` — last-used main dict + enabled optional dicts (trailing `"Private"` enables private dict)
- Animation settings: color RGB values, duration timings
