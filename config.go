package main

import (
	"encoding/json"
	"os"
)

type MainDictionaryJson struct {
	Description string
	Filename    string
}

type AddedDictionaryJson struct {
	Description string
	Filename    string
	Enabled     bool
}

const (
	main_dicts_file = "main-dicts.json"
	added_dicts_file = "added-dicts.json"
)

func ReadDictionaries() ([]MainDictionaryJson, []AddedDictionaryJson, error) {
	mainData, err := os.ReadFile("json/"+main_dicts_file)
	if err != nil {
		return nil, nil, err
	}

	var mainDicts []MainDictionaryJson
	err = json.Unmarshal(mainData, &mainDicts)
	if err != nil {
		return nil, nil, err
	}

	addedData, err := os.ReadFile("json/"+added_dicts_file)
	if err != nil {
		return mainDicts, nil, err
	}

	var addedDicts []AddedDictionaryJson
	err = json.Unmarshal(addedData, &addedDicts)
	if err != nil {
		return mainDicts, nil, err
	}
	return mainDicts, addedDicts, nil
}
