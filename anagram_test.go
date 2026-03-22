package main

import (
	"sort"
	"testing"
)

// --- Helpers ---

// collectAll drains the channel and returns all results as a sorted slice.
func collectAll(ch <-chan string) []string {
	var results []string
	for r := range ch {
		results = append(results, r)
	}
	sort.Strings(results)
	return results
}

// collectN reads up to n results from the channel, then drains the rest.
func collectN(ch <-chan string, n int) []string {
	var results []string
	for r := range ch {
		if len(results) < n {
			results = append(results, r)
		}
	}
	return results
}

// smallDict is a small hand-crafted dictionary for correctness tests.
var smallDict = &Dictionary{
	Name:  "small",
	Words: []string{"pneumatic", "death", "hated", "foobar"},
}

// mediumDict has enough words to exercise pruning and multi-word anagrams.
var mediumDict = &Dictionary{
	Name: "medium",
	Words: []string{
		"act", "acts", "art", "arts", "ate", "cat", "cats", "ear", "ears",
		"east", "eat", "eats", "era", "eras", "rat", "rats", "rest", "sat",
		"sea", "seat", "set", "star", "stare", "state", "taste", "tea", "teas",
		"tear", "tears", "test", "the", "trace", "crate", "crest", "race",
		"rate", "rates", "scar", "scare", "care", "cares", "acre", "acres",
		"a", "i",
	},
}

// --- Correctness Tests ---

func TestAnagrams(t *testing.T) {
	testDict := NewDictionary("testing")
	testDict.Words = []string{"pneumatic", "death", "hated", "foobar"}

	ad := NewAnnotatedDict(testDict)

	if len(ad) != 4 {
		t.Error("Didn't annotate the whole test dictionary")
	}

	results := FindAnagrams("Mitch Patenaude", make([]string, 0), testDict)

	result1, ok1 := <-results
	result2, ok2 := <-results

	if !ok1 {
		t.Error("Didn't produce any results")
		return
	} else if result1 != "pneumatic death" && result1 != "pneumatic hated" {
		t.Error("Got unexpected result " + result1)
	}

	if !ok2 {
		t.Error("Didn't find all possibilities")
	} else if result2 != "pneumatic hated" && result2 != "pneumatic death" {
		t.Error("Got unexpected second result " + result2)
	}

	if result1 == result2 {
		t.Error("Repeated result " + result1)
	}

	result3, ok3 := <-results
	if ok3 {
		t.Error("Got too many results")
	}
	if result3 != "" {
		t.Error("Got result on closed channel " + result3) // not possible
	}

	noresults := FindAnagrams("Quixotic", make([]string, 0), testDict)
	unresult, ok4 := <-noresults
	if ok4 {
		t.Error("Got anagram that shouldn't exist" + unresult)
	}
}

func TestAnagramsAllValid(t *testing.T) {
	// Every result must use exactly the same letters as the input.
	input := "Mitch Patenaude"
	inputRC := NewRuneCluster(input)

	results := collectAll(FindAnagrams(input, nil, smallDict))

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	for _, r := range results {
		rc := NewRuneCluster(r)
		if !rc.Equals(inputRC) {
			t.Errorf("Result %q doesn't use the same letters as %q", r, input)
		}
	}
}

func TestAnagramsNoDuplicates(t *testing.T) {
	input := "star eats"
	results := collectAll(FindAnagrams(input, nil, mediumDict))

	seen := make(map[string]bool)
	for _, r := range results {
		if seen[r] {
			t.Errorf("Duplicate result: %q", r)
		}
		seen[r] = true
	}
}

func TestAnagramsEmptyInput(t *testing.T) {
	results := collectAll(FindAnagrams("", nil, smallDict))
	if len(results) != 0 {
		t.Errorf("Expected 0 results for empty input, got %d", len(results))
	}

	results = collectAll(FindAnagrams("   ", nil, smallDict))
	if len(results) != 0 {
		t.Errorf("Expected 0 results for whitespace input, got %d", len(results))
	}
}

func TestAnagramsNoMatch(t *testing.T) {
	results := collectAll(FindAnagrams("zzzzz", nil, mediumDict))
	if len(results) != 0 {
		t.Errorf("Expected 0 results for unmatchable input, got %d", len(results))
	}
}

func TestAnagramsSingleWord(t *testing.T) {
	// "trace" and "crate" are anagrams of each other
	dict := &Dictionary{Name: "test", Words: []string{"trace", "crate", "react"}}
	results := collectAll(FindAnagrams("trace", nil, dict))

	// Should find "crate" and "react" but not "trace" itself (same normalized form)
	for _, r := range results {
		rc := NewRuneCluster(r)
		inputRC := NewRuneCluster("trace")
		if !rc.Equals(inputRC) {
			t.Errorf("Result %q doesn't match input letters", r)
		}
	}
}

func TestAnagramsWithInclusion(t *testing.T) {
	input := "Mitch Patenaude"
	include := []string{"death"}

	results := collectAll(FindAnagrams(input, include, smallDict))

	if len(results) != 1 {
		t.Fatalf("Expected 1 result with inclusion 'death', got %d: %v", len(results), results)
	}

	if results[0] != "death pneumatic" {
		t.Errorf("Expected 'death pneumatic', got %q", results[0])
	}
}

func TestAnagramsMultiWord(t *testing.T) {
	// Verify multi-word anagrams are found
	input := "seat"
	results := collectAll(FindAnagrams(input, nil, mediumDict))

	// Should include multi-word results like "a set", "ate", "eat", "eta", "tea", etc.
	if len(results) == 0 {
		t.Error("Expected some results for 'seat'")
	}

	// Every result must be a valid anagram
	inputRC := NewRuneCluster(input)
	for _, r := range results {
		rc := NewRuneCluster(r)
		if !rc.Equals(inputRC) {
			t.Errorf("Result %q doesn't use the same letters as %q", r, input)
		}
	}
}

func TestFilterAnnotatedDict(t *testing.T) {
	filtered, rc := FilterAnnotatedDict("cat", mediumDict)

	// Every word in filtered must be a subset of "cat"
	for _, dp := range filtered {
		if !dp.cluster.SubSetOf(rc) {
			t.Errorf("Filtered word %q is not a subset of 'cat'", dp.Word)
		}
	}

	// "star" should not be in filtered (has 's' and 'r')
	for _, dp := range filtered {
		if dp.Word == "star" {
			t.Error("'star' should not be in filtered dict for 'cat'")
		}
	}

	// "act" and "cat" should be in filtered
	found := map[string]bool{}
	for _, dp := range filtered {
		found[dp.Word] = true
	}
	if !found["act"] {
		t.Error("'act' should be in filtered dict for 'cat'")
	}
	if !found["cat"] {
		t.Error("'cat' should be in filtered dict for 'cat'")
	}
}

func TestAnnotatedDictCache(t *testing.T) {
	dict := &Dictionary{Name: "test", Words: []string{"foo", "bar", "baz"}}

	ad1 := GetAnnotatedDict(dict)
	ad2 := GetAnnotatedDict(dict)

	if &ad1[0] != &ad2[0] {
		t.Error("GetAnnotatedDict should return the same cached slice")
	}
}

// --- Benchmarks ---

// loadUSDict loads the real US dictionary for benchmarks. Skips if not available.
func loadUSDict(b *testing.B) *Dictionary {
	b.Helper()
	mainDicts, _, err := ReadDictionaries()
	if err != nil || len(mainDicts) == 0 {
		b.Skip("Could not load dictionaries")
	}
	return mainDicts[0]
}

func BenchmarkNewRuneCluster(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewRuneCluster("California Polytechnic University")
	}
}

func BenchmarkSubSetOf(b *testing.B) {
	rc1 := NewRuneCluster("California Polytechnic University")
	rc2 := NewRuneCluster("polynomial")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rc2.SubSetOf(rc1)
	}
}

func BenchmarkFilterSmallDict(b *testing.B) {
	for i := 0; i < b.N; i++ {
		mediumDict.annotated = nil
		FilterAnnotatedDict("star eats", mediumDict)
	}
}

func BenchmarkFilterSmallDictCached(b *testing.B) {
	GetAnnotatedDict(mediumDict)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FilterAnnotatedDict("star eats", mediumDict)
	}
}

func BenchmarkFilterRealDict(b *testing.B) {
	dict := loadUSDict(b)
	GetAnnotatedDict(dict) // pre-cache
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FilterAnnotatedDict("Karma Manager", dict)
	}
}

func BenchmarkFilterRealDictLongInput(b *testing.B) {
	dict := loadUSDict(b)
	GetAnnotatedDict(dict)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FilterAnnotatedDict("California Polytechnic University", dict)
	}
}

func BenchmarkFindAnagramsSmall(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ch := FindAnagrams("Mitch Patenaude", nil, smallDict)
		for range ch {
		}
	}
}

func BenchmarkFindAnagramsMedium(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ch := FindAnagrams("star eats", nil, mediumDict)
		for range ch {
		}
	}
}

func BenchmarkFindAnagramsRealShort(b *testing.B) {
	dict := loadUSDict(b)
	GetAnnotatedDict(dict)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ch := FindAnagrams("Karma Manager", nil, dict)
		for range ch {
		}
	}
}

func BenchmarkFindAnagramsRealMedium(b *testing.B) {
	dict := loadUSDict(b)
	GetAnnotatedDict(dict)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ch := FindAnagrams("Mitch Patenaude", nil, dict)
		// Collect first 1000 results only to keep benchmark feasible
		n := 0
		for range ch {
			n++
			if n >= 1000 {
				break
			}
		}
	}
}

func BenchmarkFindAnagramsRealLong(b *testing.B) {
	dict := loadUSDict(b)
	GetAnnotatedDict(dict)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ch := FindAnagrams("California Polytechnic", nil, dict)
		// Collect first 1000 results only
		n := 0
		for range ch {
			n++
			if n >= 1000 {
				break
			}
		}
	}
}
