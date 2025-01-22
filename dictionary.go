package main

import (
	"encoding/json"
	"io"
	"log"
)

const read_buf_size = 10240

type Dictionary struct {
	Name string
	Words []string
}

func NewDictionary(name string) *Dictionary {
	d := &Dictionary{name, make([]string, 50)}
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

func ReadDictionary(name string, reader io.Reader) (*Dictionary, error) {
	data, err := io.ReadAll(reader)
	if err != nil && err != io.EOF {
		log.Println("Error reading dictionary ", name, " because ", err)
		return nil, err
	}

	return ParseDictionary(name, data)
}
