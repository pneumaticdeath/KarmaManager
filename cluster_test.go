package main

import (
	"testing"
)

func TestCharCluster(t *testing.T) {
	abcde := NewRuneCluster("abcde")
	abcd := NewRuneCluster("cba D")
	other_abcd := NewRuneCluster("d C B a")

	if !abcd.Has('d') {
		t.Error("Missing value 'd'")
	}

	if abcde.SubSetOf(abcd) {
		t.Error("abcde should not be a subset of abcd")
	}

	if !abcd.SubSetOf(abcde) {
		t.Error("abcd should be a subset of abcde")
	}

	if abcd.Equals(abcde) {
		t.Error("abcd != abcde")
	}

	if !abcd.Equals(other_abcd) {
		t.Error("These should be equal")
	}

	result, err := abcde.Minus(abcd)
	if err != nil {
		t.Error(err)
	}

	if !result.Has('e') {
		t.Error("'e' should be in abcde-abcd")
	}

	if result.Has('d') {
		t.Error("'d' should not be in abcde-abcd")
	}

	result, err = abcd.Minus(other_abcd)
	if err != nil {
		t.Error(err)
	}

	if !result.Empty() {
		t.Error("Result of abcd-abcd should be empty")
	}
}
