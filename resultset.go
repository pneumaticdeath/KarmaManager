package main

import (
	// "fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

type ResultSet struct {
	mainDicts            []*Dictionary
	addedDicts           []*Dictionary
	privateDict          *Dictionary
	input                string
	normalizedInput      string
	included             []string
	excluded             []string
	wordCount            map[string]int
	resultCount          int
	mainDictIndex        int
	results              []string
	isDone               bool
	fetchLock            sync.Mutex
	fetchTarget          int
	inFetch              bool
	abortFlag            bool
	combinedDictName     string
	progressCallback     func(int, int)
	workingStartCallback func()
	workingStopCallback  func()
	resultChan           <-chan string
}

func NewResultSet(mainDicts, addedDicts []*Dictionary, privateDict *Dictionary, mainDictIndex int) *ResultSet {
	rs := &ResultSet{mainDicts, addedDicts, privateDict, "", "", make([]string, 0), make([]string, 0), make(map[string]int), 0, mainDictIndex, make([]string, 0), true, sync.Mutex{}, 0, false, false, "", nil, nil, nil, nil}

	rs.FindAnagrams("", nil)
	return rs
}

func (rs *ResultSet) SetProgressCallback(cb func(int, int)) {
	rs.progressCallback = cb
}

func (rs *ResultSet) SetWorkingStartCallback(cb func()) {
	rs.workingStartCallback = cb
}

func (rs *ResultSet) SetWorkingStopCallback(cb func()) {
	rs.workingStopCallback = cb
}

func (rs *ResultSet) FindAnagrams(input string, refreshCallback func()) {
	rs.input = input
	rs.normalizedInput = normalize(input)
	rs.Regenerate(refreshCallback)
}

func (rs *ResultSet) Abort() {
	if rs.inFetch {
		// fmt.Println("Aborting in-process FetchTo()")
		rs.abortFlag = true
		for rs.abortFlag {
			time.Sleep(time.Millisecond * 100)
		}
	}
}

func (rs *ResultSet) Regenerate(refreshCallback func()) {
	rs.resultCount = 0
	rs.fetchTarget = 0
	rs.wordCount = make(map[string]int)
	rs.results = make([]string, 0, 110)
	rs.isDone = false
	combinedDict := rs.CombineDicts()
	go func() {
		rs.Abort()
		rs.resultChan = FindAnagrams(rs.input, rs.included, combinedDict)
		rs.combinedDictName = combinedDict.Name
		rs.FetchTo(25)
		if refreshCallback != nil {
			refreshCallback()
		}
	}()
}

func (rs *ResultSet) CombineDicts() *Dictionary {
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

	return MergeDictionaries(rs.excluded, dicts...)
}

func (rs *ResultSet) CombinedDictName() string {
	return rs.combinedDictName
}

func (rs *ResultSet) SetMainIndex(index int) {
	rs.mainDictIndex = index
}

func (rs *ResultSet) FetchTo(target int) {
	if rs.isDone {
		return
	}

	if rs.fetchTarget < target {
		// fmt.Printf("New fetch target %d\n", target)
		rs.fetchTarget = target
	}

	lockSuccess := rs.fetchLock.TryLock()
	if !lockSuccess {
		// fmt.Println("Tried to FetchTo() while already locked")
		return
	}
	// fmt.Println("Acquired lock")
	rs.inFetch = true

	if rs.workingStartCallback != nil {
		rs.workingStartCallback()
	}

	if rs.progressCallback != nil {
		rs.progressCallback(rs.resultCount, rs.fetchTarget)
	}

	for !rs.isDone && rs.resultCount < rs.fetchTarget {
		next, ok := <-rs.resultChan
		if rs.abortFlag {
			// fmt.Println("FetchTo() aborted")
			rs.abortFlag = false
			break
		}
		if ok {
			if normalize(next) != rs.normalizedInput {
				for _, word := range strings.Split(next, " ") {
					if word != "" {
						rs.wordCount[word] += 1
					}
				}
				rs.results = append(rs.results, next)
				rs.resultCount += 1

				if rs.progressCallback != nil && rs.resultCount%2 == 0 {
					rs.progressCallback(rs.resultCount, rs.fetchTarget)
				}
			}
		} else {
			rs.fetchTarget = rs.resultCount
			rs.isDone = true
		}
	}
	if rs.progressCallback != nil {
		rs.progressCallback(rs.resultCount, rs.fetchTarget)
	}

	if rs.workingStopCallback != nil {
		rs.workingStopCallback()
	}

	rs.inFetch = false
	rs.fetchLock.Unlock()
	// fmt.Printf("Released lock at %d\n", rs.resultCount)
}

func (rs *ResultSet) IsDone() bool {
	return rs.isDone
}

func (rs *ResultSet) Count() int {
	return rs.resultCount
}

func (rs *ResultSet) GetAt(index int) (string, bool) {
	// fmt.Printf("Getting item at %d\n", index)
	if index > rs.resultCount-10 {
		go func() {
			rs.FetchTo(index + 10)
		}()
		for !rs.isDone && index >= rs.resultCount {
			time.Sleep(time.Millisecond)
		}

		if index >= rs.resultCount {
			return "", false
		}
	}
	return rs.results[index], true
}

func (rs *ResultSet) SetInclusions(phrases []string) {
	rs.included = phrases
}

func (rs *ResultSet) SetExclusions(words []string) {
	rs.excluded = words
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
	words := make(Counts, 0, len(rs.wordCount))
	for w, c := range rs.wordCount {
		words = append(words, WordCount{w, c})
	}

	sort.Sort(words)

	if len(words) > n {
		return words[:n]
	} else {
		return words
	}
}

func normalize(str string) string {
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
