package main

import (
	"errors"
	"unicode"
)

// RuneCluster is a fixed-size array of letter frequencies (a-z).
// Using [26]int instead of map[rune]int avoids map allocation overhead
// and is cache-friendly for the tight loops in anagram search.
type RuneCluster [26]int

func runeIndex(r rune) int {
	return int(unicode.ToLower(r)) - 'a'
}

func NewRuneCluster(input string) *RuneCluster {
	var rc RuneCluster
	for _, r := range input {
		if unicode.IsLetter(r) {
			rc[runeIndex(r)]++
		}
	}
	return &rc
}

func (rc *RuneCluster) Count(r rune) int {
	idx := runeIndex(r)
	if idx < 0 || idx >= 26 {
		return 0
	}
	return rc[idx]
}

func (rc *RuneCluster) Has(r rune) bool {
	return rc.Count(r) > 0
}

func (rc *RuneCluster) SubSetOf(other *RuneCluster) bool {
	for i := 0; i < 26; i++ {
		if rc[i] > other[i] {
			return false
		}
	}
	return true
}

func (rc *RuneCluster) Equals(other *RuneCluster) bool {
	return *rc == *other
}

func (rc *RuneCluster) Minus(other *RuneCluster) (*RuneCluster, error) {
	var result RuneCluster
	for i := 0; i < 26; i++ {
		diff := rc[i] - other[i]
		if diff < 0 {
			return nil, errors.New("Not a superset of other cluster")
		}
		result[i] = diff
	}
	return &result, nil
}

func (rc *RuneCluster) Add(other *RuneCluster) {
	for i := 0; i < 26; i++ {
		rc[i] += other[i]
	}
}

func (rc *RuneCluster) IsEmpty() bool {
	for i := 0; i < 26; i++ {
		if rc[i] != 0 {
			return false
		}
	}
	return true
}
