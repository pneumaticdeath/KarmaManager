package main

import (
	"sort"
	"strings"
	"unicode"
)

type ResultSet struct {
	mainDicts        []*Dictionary
	addedDicts       []*Dictionary
	privateDict      *Dictionary
	input            string
	normalizedInput  string
	included         []string
	excluded         []string
	wordCount        map[string]int
	resultCount      int
	mainDictIndex    int
	results          []string
	isDone           bool
	combinedDictName string
	progressCallback func(int, int)
	resultChan       <-chan string
}

func NewResultSet(mainDicts, addedDicts []*Dictionary, privateDict *Dictionary, mainDictIndex int) *ResultSet {
	rs := &ResultSet{mainDicts, addedDicts, privateDict, "", "", make([]string, 0), make([]string, 0), make(map[string]int), 0, mainDictIndex, make([]string, 0), true, "", nil, nil}

	rs.FindAnagrams("")
	return rs
}

func (rs *ResultSet) SetProgressCallback(cb func(int, int)) {
	rs.progressCallback = cb
}

func (rs *ResultSet) FindAnagrams(input string) {
	rs.input = input
	rs.normalizedInput = normalize(input)
	rs.Regenerate()
}

func (rs *ResultSet) Regenerate() {
	rs.resultCount = 0
	rs.wordCount = make(map[string]int)
	rs.results = make([]string, 0, 110)
	rs.isDone = false
	combinedDict := rs.CombineDicts()
	rs.resultChan = FindAnagrams(rs.input, rs.included, combinedDict)
	rs.combinedDictName = combinedDict.Name
	rs.FetchNext(100)
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

func (rs *ResultSet) FetchNext(count int) {
	if rs.isDone {
		return
	}

	fetchCount := 0

	for fetchCount < count && !rs.isDone {
		next, ok := <-rs.resultChan
		if ok {
			if normalize(next) != rs.normalizedInput {
				for _, word := range strings.Split(next, " ") {
					if word != "" {
						rs.wordCount[strings.ToLower(word)] += 1
					}
				}
				rs.results = append(rs.results, next)
				rs.resultCount += 1
				fetchCount += 1

				if rs.progressCallback != nil && fetchCount%10 == 0 {
					rs.progressCallback(fetchCount, count)
				}

			}
		} else {
			rs.isDone = true
		}
	}
	if rs.progressCallback != nil {
		rs.progressCallback(count, count)
	}
}

func (rs *ResultSet) IsDone() bool {
	return rs.isDone
}

func (rs *ResultSet) Count() int {
	return rs.resultCount
}

func (rs *ResultSet) GetAt(index int) (string, bool) {
	if index > rs.resultCount-10 {
		rs.FetchNext(index - rs.resultCount + 10)
	}
	if index >= rs.resultCount {
		return "", false
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
	return c[j].Count < c[i].Count
}

func (rs *ResultSet) TopNWords(n int) Counts {
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
