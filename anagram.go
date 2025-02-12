package main

import (
	// "log"
	"sort"
	"strings"
)

type dictPair struct {
	word    string
	cluster *RuneCluster
}

type annotatedDict []dictPair

func NewAnnotatedDict(d *Dictionary) annotatedDict {
	var ad annotatedDict = make(annotatedDict, len(d.Words))

	for index, word := range d.Words {
		ad[index] = dictPair{word, NewRuneCluster(word)}
	}

	return ad
}

func (ad annotatedDict) Filter(target *RuneCluster) annotatedDict {
	retVal := make(annotatedDict, 0, len(ad)/2) // half is probably overly generous

	for _, dp := range ad {
		if dp.cluster.SubSetOf(target) {
			retVal = append(retVal, dp)
		}
	}

	return retVal
}

func (ad annotatedDict) Swap(i, j int) {
	ad[i], ad[j] = ad[j], ad[i]
}

func (ad annotatedDict) Len() int {
	return len(ad)
}

func (ad annotatedDict) Less(i, j int) bool {
	// sort first by length (decending) then by alphabet (decending)

	if len(ad[i].word) == len(ad[j].word) {
		return ad[i].word > ad[j].word
	}
	return len(ad[i].word) > len(ad[j].word)
}

func FindAnagrams(input string, include []string, dictionary *Dictionary) <-chan string {
	outputChan := make(chan string, 10)

	go makeAnagrams(input, include, dictionary, outputChan)

	return outputChan
}

func makeAnagrams(input string, include []string, dictionary *Dictionary, output chan<- string) {
	defer close(output)

	ad := NewAnnotatedDict(dictionary)

	target := NewRuneCluster(input)

	filtered := ad.Filter(target)

	sort.Sort(filtered) // for efficientcy we need ot sort decending by size

	includedDone := 0
	if len(include) > 0 {
		for _, phrase := range include {
			trimmedPhrase := strings.Trim(phrase, " \t\r\n")
			if trimmedPhrase == "" {
				continue
			}
			phraseRC := NewRuneCluster(phrase)
			if !phraseRC.SubSetOf(target) {
				// log.Println("Phrase \"" + phrase + "\" not a subset of input")
				continue
			}
			includedDone += 1
			newTarget, _ := target.Minus(phraseRC)
			newFiltered := filtered.Filter(newTarget)
			findTuples(trimmedPhrase, newTarget, newFiltered, output)
		}
	}
	if includedDone == 0 {
		findTuples("", target, filtered, output)
	}
}

func findTuples(current string, target *RuneCluster, dict annotatedDict, output chan<- string) {
	if target.IsEmpty() {
		if current != "" {
			output <- current
		}
		return
	}

	for index, dp := range dict {
		var trial string
		if current == "" {
			trial = dp.word
		} else {
			trial = current + " " + dp.word
		}

		newTarget, err := target.Minus(dp.cluster)
		if err != nil {
			panic(err) // this shouldn't be possible
		}
		newDict := dict[index+1:].Filter(newTarget)

		findTuples(trial, newTarget, newDict, output)
	}
}
