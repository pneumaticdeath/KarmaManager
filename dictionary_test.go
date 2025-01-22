package main

import (
	"os"
	"strings"
	"testing"
)

func TestDictionaryParsing(t *testing.T) {
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

func TestDictReader(t *testing.T) {
	r := strings.NewReader("[\"Twinkle\",\"Little\",\"Star\"]")
	dict, err := ReadDictionary("test", r)

	if err != nil {
		t.Error(err)
		return
	}

	if dict == nil {
		t.Error("Nil dict returned from ReadDictionary()")
	}

	if dict.Name != "test" {
		t.Error("Dictionary name not test, but ", dict.Name)
	}

	if len(dict.Words) != 3 {
		t.Error("Reader didn't parse all words")
	}
}

func TestBigDictionary(t *testing.T) {
	r, rerr := os.Open("json/full-dict.json")
	if rerr != nil {
		t.Error(rerr)
		return
	}
	defer r.Close()
	
	dict, perr := ReadDictionary("Full", r)
	if perr != nil {
		t.Error(perr)
		return
	}

	if len(dict.Words) < 20000 {
		t.Error("Got truncated dictionary")
	}
}
