package main

import (
	"errors"
	"unicode"
)

type RuneCluster map[rune]int

func NewRuneCluster(input string) *RuneCluster {
	rc := make(RuneCluster)
	for i := 0; i < len(input); i++ {
		r := rune(input[i])
		if !unicode.IsSpace(r) {
			rc[unicode.ToLower(r)] += 1
		}
	}
	return &rc
}

func (rc *RuneCluster) Count(r rune) int {
	return (*rc)[unicode.ToLower(r)]
}

func (rc *RuneCluster) Has(r rune) bool {
	return rc.Count(r) > 0
}

func (rc *RuneCluster) SubSetOf(other *RuneCluster) bool {
	for r, c := range *rc {
		if c > other.Count(r) {
			return false
		}
	}
	return true
}

func (rc *RuneCluster) Equals(other *RuneCluster) bool {
	if len(*rc) != len(*other) {
		return false
	}

	for r, c := range *rc {
		if c != other.Count(r) {
			return false
		}
	}

	return true
}

func (rc *RuneCluster) Minus(other *RuneCluster) (*RuneCluster, error) {
	result := NewRuneCluster("")
	for r, c := range *rc {
		oc := other.Count(r)
		if c > oc {
			(*result)[r] = c - oc
		} else if c < oc {
			return nil, errors.New("Not a superset of other cluster")
		}
	}

	return result, nil
}

func (rc *RuneCluster) IsEmpty() bool {
	return len(*rc) == 0
}
