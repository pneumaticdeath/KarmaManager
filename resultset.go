package main

import (
	// "fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	max_cached_resultsetstates = 25
)

type RSState struct {
	input            string
	normalizedInput  string
	included         []string
	excluded         []string
	wordCount        map[string]int
	resultCount      int
	results          []string
	isDone           bool
	combinedDict     *Dictionary
	combinedDictName string
	resultChan       <-chan string
	lastUsed         time.Time
}

func NewRSState() *RSState {
	state := &RSState{}
	state.included = make([]string, 0)
	state.excluded = make([]string, 0)
	state.wordCount = make(map[string]int)
	state.results = make([]string, 0, 25)
	state.lastUsed = time.Now()

	return state
}

type ResultSet struct {
	mainDicts            []*Dictionary
	addedDicts           []*Dictionary
	privateDict          *Dictionary
	state                *RSState
	cached               []*RSState
	mainDictIndex        int
	fetchLock            sync.Mutex
	fetchTarget          int
	inFetch              bool
	abortFlag            bool
	progressCallback     func(int, int)
	refreshCallback      func()
	workingStartCallback func()
	workingStopCallback  func()
}

func NewResultSet(mainDicts, addedDicts []*Dictionary, privateDict *Dictionary, mainDictIndex int) *ResultSet {
	rs := &ResultSet{mainDicts, addedDicts, privateDict, NewRSState(), make([]*RSState, 0), mainDictIndex, sync.Mutex{}, 0, false, false, nil, nil, nil, nil}

	rs.RebuildDictionaries()
	rs.FindAnagrams("")
	return rs
}

func (rs *ResultSet) SetProgressCallback(cb func(int, int)) {
	rs.progressCallback = cb
}

func (rs *ResultSet) SetRefreshCallback(cb func()) {
	rs.refreshCallback = cb
}

func (rs *ResultSet) SetWorkingStartCallback(cb func()) {
	rs.workingStartCallback = cb
}

func (rs *ResultSet) SetWorkingStopCallback(cb func()) {
	rs.workingStopCallback = cb
}

func (rs *ResultSet) FindAnagrams(input string) {
	rs.setState(input, make([]string, 0), make([]string, 0), rs.state.combinedDictName)
}

func cmpStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for index, a_element := range a {
		if a_element != b[index] {
			return false
		}
	}

	return true
}

func (rs *ResultSet) setState(input string, included, excluded []string, combDictName string) {
	var state *RSState

	rs.Abort()

	for _, cachedState := range rs.cached {
		if cachedState.input == input && cachedState.combinedDictName == combDictName && cmpStringSlices(cachedState.included, included) && cmpStringSlices(cachedState.excluded, excluded) {
			log.Println("Using cached RSState ", cachedState.input)
			cachedState.lastUsed = time.Now()
			rs.state = cachedState
			if rs.refreshCallback != nil {
				rs.refreshCallback()
			}
			return
		}
	}

	log.Println("Building new state for " + input)

	state = NewRSState()
	state.input = input
	state.normalizedInput = Normalize(input)
	state.included = included
	state.excluded = excluded
	state.combinedDictName = combDictName
	state.combinedDict = rs.GetDict(combDictName, excluded)

	for _, ex := range state.excluded {
		log.Println("constructed exlcusion: ", ex)
	}

	rs.cached = append(rs.cached, state)
	rs.trimCache()

	rs.state = state
	rs.Regenerate()
}

func (rs *ResultSet) GetDict(name string, exclusions []string) *Dictionary {
	log.Println("Building new dict for " + name)

	newDict := rs.CombineDicts(exclusions)

	if newDict == nil {
		log.Panicln("Got null dictionary from CombineDicts")
	}

	if newDict.Name != name {
		log.Panicf("Warning: generated dict name %s != passed dict name %s\n", newDict.Name, name)
	}

	return newDict
}

func (rs *ResultSet) trimCache() {
	sort.Slice(rs.cached, func(i, j int) bool {
		return rs.cached[j].lastUsed.Before(rs.cached[i].lastUsed)
	})

	log.Printf("RSState cache is now %d elements\n", len(rs.cached))

	if len(rs.cached) > max_cached_resultsetstates {
		log.Println("trimming cache")
		rs.cached = rs.cached[:max_cached_resultsetstates]
	}
}

func (rs *ResultSet) Abort() {
	if rs.inFetch {
		log.Println("Aborting in-process FetchTo()")
		rs.abortFlag = true
		for rs.abortFlag {
			time.Sleep(time.Millisecond * 100)
		}
	}
}

func (rs *ResultSet) RebuildDictionaries() {
	name := rs.MakeCombinedDictName()
	rs.setState(rs.state.input, rs.state.included, rs.state.excluded, name)
}

func (rs *ResultSet) MakeCombinedDictName() string {
	names := make([]string, 0, len(rs.addedDicts)+2)

	names = append(names, rs.mainDicts[rs.mainDictIndex].Name)
	for _, ad := range rs.addedDicts {
		if ad.Enabled {
			names = append(names, ad.Name)
		}
	}
	if rs.privateDict.Enabled {
		names = append(names, "Private")
	}

	return strings.Join(names, " + ")
}

func (rs *ResultSet) Regenerate() {
	rs.state.resultCount = 0
	rs.fetchTarget = 0
	rs.state.wordCount = make(map[string]int)
	rs.state.results = make([]string, 0, 110)
	rs.state.isDone = false
	go func() {
		rs.Abort()
		rs.state.resultChan = FindAnagrams(rs.state.input, rs.state.included, rs.state.combinedDict)
		time.Sleep(time.Millisecond)
		rs.FetchTo(25)
		if rs.refreshCallback != nil {
			rs.refreshCallback()
		}
	}()
}

func (rs *ResultSet) CombineDicts(excluded []string) *Dictionary {
	dicts := make([]*Dictionary, 0, len(rs.addedDicts)+2)
	dicts = append(dicts, rs.mainDicts[rs.mainDictIndex])
	for _, d := range rs.addedDicts {
		if d.Enabled {
			dicts = append(dicts, d)
		}
	}
	if rs.privateDict.Enabled {
		dicts = append(dicts, rs.privateDict)
	}

	return MergeDictionaries(excluded, dicts...)
}

func (rs *ResultSet) CombinedDictName() string {
	return rs.state.combinedDictName
}

func (rs *ResultSet) SetMainIndex(index int) {
	rs.mainDictIndex = index
	rs.RebuildDictionaries()
}

func (rs *ResultSet) FetchTo(target int) {
	if rs.state.isDone {
		return
	}

	if rs.fetchTarget < target {
		log.Printf("New fetch target %d\n", target)
		rs.fetchTarget = target
	}

	lockSuccess := rs.fetchLock.TryLock()
	if !lockSuccess {
		log.Println("Tried to FetchTo() while already locked")
		return
	}
	log.Println("Acquired lock")
	rs.inFetch = true
	defer func() {
		rs.inFetch = false
		rs.fetchLock.Unlock()
		log.Printf("Released lock at %d\n", rs.state.resultCount)
	}()

	if rs.workingStartCallback != nil {
		rs.workingStartCallback()
	}

	if rs.progressCallback != nil {
		rs.progressCallback(rs.state.resultCount, rs.fetchTarget)
	}

	for !rs.state.isDone && rs.state.resultCount < rs.fetchTarget {
		next, ok := <-rs.state.resultChan
		if rs.abortFlag {
			log.Println("FetchTo() aborted")
			rs.abortFlag = false
			break
		}
		if ok {
			// log.Println("Got anagram ", next)
			if Normalize(next) != rs.state.normalizedInput {
				for _, word := range strings.Split(next, " ") {
					if word != "" {
						rs.state.wordCount[word] += 1
					}
				}
				rs.state.results = append(rs.state.results, next)
				rs.state.resultCount += 1

				if rs.progressCallback != nil && rs.state.resultCount%2 == 0 {
					rs.progressCallback(rs.state.resultCount, rs.fetchTarget)
				}
			}
		} else {
			rs.fetchTarget = rs.state.resultCount
			rs.state.isDone = true
		}
	}
	if rs.progressCallback != nil {
		rs.progressCallback(rs.state.resultCount, rs.fetchTarget)
	}

	if rs.workingStopCallback != nil {
		rs.workingStopCallback()
	}
}

func (rs *ResultSet) IsDone() bool {
	return rs.state.isDone
}

func (rs *ResultSet) Count() int {
	return rs.state.resultCount
}

func (rs *ResultSet) IsEmpty() bool {
	return rs.state.resultCount == 0 && rs.state.isDone
}

func (rs *ResultSet) GetAt(index int) (string, bool) {
	// log.Printf("Getting item at %d\n", index)
	if index > rs.state.resultCount-10 {
		go func() {
			rs.FetchTo(index + 10)
		}()
		for !rs.state.isDone && index >= rs.state.resultCount {
			// log.Println("GetAt waiting...")
			time.Sleep(time.Millisecond)
		}
	}

	if index < rs.state.resultCount {
		return rs.state.results[index], true
	} else {
		return "", false
	}
}

func (rs *ResultSet) SetInclusions(phrases []string) {
	rs.setState(rs.state.input, phrases, rs.state.excluded, rs.state.combinedDictName)
}

func (rs *ResultSet) SetExclusions(words []string) {
	rs.setState(rs.state.input, rs.state.included, words, rs.state.combinedDictName)
}

type WordCount struct {
	Word  string
	Count int
}

type Counts []WordCount

func (c Counts) Len() int {
	return len(c)
}

func (c Counts) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

func (c Counts) Less(i, j int) bool {
	return len(c[j].Word)*c[j].Count < len(c[i].Word)*c[i].Count
}

func (rs *ResultSet) TopNWords(n int) Counts {
	rs.fetchLock.Lock()
	defer rs.fetchLock.Unlock()
	words := make(Counts, 0, len(rs.state.wordCount))
	for w, c := range rs.state.wordCount {
		words = append(words, WordCount{w, c})
	}

	sort.Sort(words)

	if len(words) > n {
		return words[:n]
	} else {
		return words
	}
}

func Normalize(str string) string {
	b := strings.Builder{}
	for _, c := range strings.Trim(str, " ") {
		r := rune(c)
		if unicode.IsSpace(r) || unicode.IsLetter(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}

	var words sort.StringSlice = strings.Split(b.String(), " ")
	words.Sort()

	return strings.Join(words, " ")
}
