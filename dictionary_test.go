package main

import (
	"fmt"
	"os"
	"testing"
)

func TestDictionary(t *testing.T) {
	data := []byte(`["Foo","Bar","baz","QUUX"]`)

	dict, err := ParseDictionary("test_dict", data)

	if err != nil {
		t.Error(err)
		return
	}

	if dict == nil {
		t.Error("got nil dictionary back")
		return
	}

	if dict.Name != "test_dict" {
		t.Error("dictionary name not correct")
	}

	if len(dict.Words) != 4 {
		t.Error("Didn't parse proper number of words")
	}

	if dict.Words[3] != "QUUX" {
		t.Error("Didn't ready words in order")
	}

	otherDict, _ := ParseDictionary("other", []byte(`["Fie", "Fy", "Foe", "Foo"]`))
	combined := MergeDictionaries(make([]string, 0), dict, otherDict)

	if len(combined.Words) == len(dict.Words)+len(otherDict.Words) {
		t.Error("Merge didn't deduplicate")
	} else if len(combined.Words) != len(dict.Words)+len(otherDict.Words)-1 {
		t.Error("Merge didn't deduplicate")
	}

	if len(combined.Words) == len(dict.Words)+len(otherDict.Words) {
		t.Error("Merge didn't deduplicate")
	}

	if combined.Name != "test_dict + other" {
		t.Error("Merge didn't combine names properly")
	}
}

func TestDictionaryParserFailure(t *testing.T) {
	data := []byte(`{"key": "value"}`)

	dict, err := ParseDictionary("bad_dictionary", data)

	if err == nil {
		t.Error("Didn't return error on malformed dictionary")
	}

	if dict != nil {
		t.Error("Parsing malformed data didn't return nil dict")
	}
}

func TestBigDictionary(t *testing.T) {
	data, rerr := os.ReadFile("json/full-dict.json")
	if rerr != nil {
		t.Error(rerr)
		return
	}

	dict, perr := ParseDictionary("Full", data)
	if perr != nil {
		t.Error(perr)
		return
	}

	if len(dict.Words) < 20000 {
		t.Error("Got truncated dictionary")
	}
}

func TestReadConfigs(t *testing.T) {
	mainConfigs, addedConfigs, err := ReadConfigs()
	if err != nil {
		t.Error(err)
		return
	}

	if mainConfigs == nil || addedConfigs == nil {
		t.Error("One config list is nil")
		return
	}

	if len(mainConfigs) == 0 || len(addedConfigs) == 0 {
		t.Error("One of the lists is 0 length")
	}

	for i, mc := range mainConfigs {
		if mc.File == "" {
			t.Error(fmt.Sprintf("main config %d has blank filename", i))
		}

		if mc.Description == "" {
			t.Error(fmt.Sprintf("main config %d has blank description", i))
		}
	}

	for i, ac := range addedConfigs {
		if ac.File == "" {
			t.Error(fmt.Sprintf("added config %d has blank filename", i))
		}

		if ac.Description == "" {
			t.Error(fmt.Sprintf("added config %d has blank description", i))
		}
	}
}

func TestReadDictionaries(t *testing.T) {
	mainDicts, addedDicts, err := ReadDictionaries()

	if err != nil {
		t.Error(err)
		return
	}

	if len(mainDicts) == 0 {
		t.Error("No main dictionaries")
	}

	if len(addedDicts) == 0 {
		t.Error("No added dictionaries")
	}
}
