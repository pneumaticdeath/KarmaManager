package main

import (
	"testing"
)

func TestAnagrams(t *testing.T) {
	testDict := NewDictionary("testing")
	testDict.Words = []string{"pneumatic", "death", "hated", "foobar"}

	ad := NewAnnotatedDict(testDict)

	if len(ad) != 4 {
		t.Error("Didn't annotate the whole test dictionary")
	}

	results := FindAnagrams("Mitch Patenaude", testDict)

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

	noresults := FindAnagrams("Quixotic", testDict)
	unresult, ok4 := <-noresults
	if ok4 {
		t.Error("Got anagram that shouldn't exist" + unresult)
	}
}
