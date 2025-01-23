package main

type ResultSet struct {
	mainDicts        []*Dictionary
	addedDicts       []*Dictionary
	input            string
	resultCount      int
	mainDictIndex    int
	results          []string
	isDone           bool
	combinedDictName string
	resultChan       <-chan string
}

func NewResultSet(mainDicts, addedDicts []*Dictionary, mainDictIndex int) *ResultSet {
	rs := &ResultSet{mainDicts, addedDicts, "", 0, mainDictIndex, make([]string, 0), true, "", nil}

	rs.FindAnagrams("")
	return rs
}

func (rs *ResultSet) FindAnagrams(input string) {
	rs.input = input
	rs.Regenerate()
}

func (rs *ResultSet) Regenerate() {
	rs.resultCount = 0
	rs.results = make([]string, 0, 110)
	rs.isDone = false
	combinedDict := rs.CombineDicts()
	rs.resultChan = FindAnagrams(rs.input, combinedDict)
	rs.combinedDictName = combinedDict.Name
	rs.FetchNext(100)
}

func (rs *ResultSet) CombineDicts() *Dictionary {
	dicts := make([]*Dictionary, 0, len(rs.addedDicts)+1)
	dicts = append(dicts, rs.mainDicts[rs.mainDictIndex])
	for _, d := range rs.addedDicts {
		if d.Enabled {
			dicts = append(dicts, d)
		}
	}

	return MergeDictionaries(dicts...)
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

	for count > 0 && !rs.isDone {
		next, ok := <-rs.resultChan
		if ok {
			rs.results = append(rs.results, next)
			rs.resultCount += 1
		} else {
			rs.isDone = true
			return
		}
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
