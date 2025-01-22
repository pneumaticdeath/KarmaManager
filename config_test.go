package main

import (
	"testing"
)

func TestReadDictionaryConfigs(t *testing.T) {
	mainDicts, addedDicts, err := ReadDictionaries()
	if err != nil {
		t.Error(err)
		return
	}

	if mainDicts == nil || addedDicts == nil {
		t.Error("One dictionary list is nil")
		return
	}

	if len(mainDicts) == 0 || len(addedDicts) == 0 {
		t.Error("One of the lists is 0 length")
	}
}
