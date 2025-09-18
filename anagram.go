package main

import (
	// "fmt"
	// "log"
	"sort"
	"strings"
)

type dictPair struct {
	Word    string
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

func FilterAnnotatedDict(input string, d *Dictionary) (annotatedDict, *RuneCluster) {
	ad := NewAnnotatedDict(d)

	rc := NewRuneCluster(input)

	filtered := ad.Filter(rc)

	sort.Sort(filtered) // for efficientcy we need ot sort decending by size

	return filtered, rc
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

	if len(ad[i].Word) == len(ad[j].Word) {
		return ad[i].Word > ad[j].Word
	}
	return len(ad[i].Word) > len(ad[j].Word)
}

func FindAnagrams(input string, include []string, dictionary *Dictionary) <-chan string {
	outputChan := make(chan string, 10)

	go makeAnagrams(input, include, dictionary, outputChan)

	return outputChan
}

func makeAnagrams(input string, include []string, dictionary *Dictionary, output chan<- string) {
	defer close(output)

	filtered, target := FilterAnnotatedDict(input, dictionary)

	// fmt.Printf("For input \"%s\" filtered is %d elements\n", input, len(filtered))
	// for _, dp := range filtered[:10] {
	// 	fmt.Print(dp.Word, " ")
	// }
	// fmt.Println("")

	includedDone := 0
	if len(include) > 0 {
		for _, phrase := range include {
			trimmedPhrase := strings.TrimSpace(phrase)
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

			// fmt.Printf("For included phrase \"%s\" filtered is %d elements\n", phrase, len(newFiltered))
			// for _, dp := range newFiltered[:10] {
			// 	fmt.Print(dp.Word, " ")
			// }
			// fmt.Println("")

			findTuples(trimmedPhrase, newTarget, newFiltered, output)
		}
	}
	if includedDone == 0 {
		findTuples("", target, filtered, output)
	}
}

var trials int = 0

func findTuples(current string, target *RuneCluster, dict annotatedDict, output chan<- string) {
	if target.IsEmpty() {
		if current != "" {
			// fmt.Printf("Found %s after %d trials\n", current, trials)
			output <- current
		}
		trials = 0
		return
	}

	trials += 1
	for index, dp := range dict {
		var trial string
		if current == "" {
			trial = dp.Word
		} else {
			trial = current + " " + dp.Word
		}

		newTarget, err := target.Minus(dp.cluster)
		if err != nil {
			panic(err) // this shouldn't be possible
		}
		newDict := dict[index:].Filter(newTarget)

		// fmt.Printf("working on '%s', %d possibilities left\n", trial, len(newDict))

		findTuples(trial, newTarget, newDict, output)
	}
}
