package main

import (
	"embed"
	"encoding/json"
	"strings"
)

//go:embed json
var jsonFS embed.FS

type Dictionary struct {
	Name    string
	Words   []string
	Enabled bool
}

type MainDictionaryConfig struct {
	Description string
	File        string
}

type AddedDictionaryConfig struct {
	Description string
	File        string
	Enabled     bool
}

const (
	main_dicts_file  = "main-dicts.json"
	added_dicts_file = "added-dicts.json"
)

func NewDictionary(name string) *Dictionary {
	d := &Dictionary{name, make([]string, 50), true}
	return d
}

func ParseDictionary(name string, jsondata []byte) (*Dictionary, error) {
	d := &Dictionary{Name: name}

	err := json.Unmarshal(jsondata, &(d.Words))
	if err == nil {
		return d, err
	}
	return nil, err
}

func ReadConfigs() ([]MainDictionaryConfig, []AddedDictionaryConfig, error) {
	mainData, err := jsonFS.ReadFile("json/" + main_dicts_file)
	if err != nil {
		return nil, nil, err
	}

	var mainDicts []MainDictionaryConfig
	err = json.Unmarshal(mainData, &mainDicts)
	if err != nil {
		return nil, nil, err
	}

	addedData, err := jsonFS.ReadFile("json/" + added_dicts_file)
	if err != nil {
		return mainDicts, nil, err
	}

	var addedDicts []AddedDictionaryConfig
	err = json.Unmarshal(addedData, &addedDicts)
	if err != nil {
		return mainDicts, nil, err
	}
	return mainDicts, addedDicts, nil
}

func ReadDictionaries() ([]*Dictionary, []*Dictionary, error) {
	mainDictConfigs, addedDictConfigs, err := ReadConfigs()
	if err != nil {
		return nil, nil, err
	}

	var mainDicts []*Dictionary = make([]*Dictionary, len(mainDictConfigs))
	for i, mdc := range mainDictConfigs {
		data, err := jsonFS.ReadFile("json/" + mdc.File)
		if err != nil {
			return nil, nil, err
		}
		mainDicts[i], err = ParseDictionary(mdc.Description, data)
		if err != nil {
			return nil, nil, err
		}
	}

	var addedDicts []*Dictionary = make([]*Dictionary, len(addedDictConfigs))
	for i, adc := range addedDictConfigs {
		data, err := jsonFS.ReadFile("json/" + adc.File)
		if err != nil {
			return mainDicts, nil, err
		}

		addedDicts[i], err = ParseDictionary(adc.Description, data)
		if err != nil {
			return mainDicts, nil, err
		}
		addedDicts[i].Enabled = adc.Enabled
	}

	return mainDicts, addedDicts, nil
}

func MergeDictionaries(excluded []string, dicts ...*Dictionary) *Dictionary {
	var length int = 0
	names := make([]string, 0, len(dicts))
	words := make([]string, 0, len(dicts[0].Words))

	knownWords := make(map[string]bool)
	for _, word := range excluded {
		knownWords[word] = true
	} // if they're already "known" they won't be added again

	for _, d := range dicts {
		names = append(names, d.Name)
		for _, word := range d.Words {
			if !knownWords[word] {
				knownWords[word] = true
				length += 1
				words = append(words, word)
			}
		}
	}

	result := NewDictionary(strings.Join(names, " + "))
	result.Words = words

	return result
}
