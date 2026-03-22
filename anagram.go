package main

import (
	// "fmt"
	"log"
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

// GetAnnotatedDict returns a cached annotated dict, building it on first call.
func GetAnnotatedDict(d *Dictionary) annotatedDict {
	if d.annotated == nil {
		d.annotated = NewAnnotatedDict(d)
	}
	return d.annotated
}

func FilterAnnotatedDict(input string, d *Dictionary) (annotatedDict, *RuneCluster) {
	if d == nil {
		log.Panicln("Got null dictionary for input ", input)
	}

	ad := GetAnnotatedDict(d)

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
	defer func() {
		log.Println("Closing output channel for ", input)
		close(output)
	}()

	if strings.TrimSpace(input) == "" {
		return
	}

	filtered, target := FilterAnnotatedDict(input, dictionary)

	// fmt.Printf("For input \"%s\" filtered is %d elements\n", input, len(filtered))
	// for _, dp := range filtered[:10] {
	// 	fmt.Print(dp.Word, " ")
	// }
	// fmt.Println("")

	if len(include) > 0 {
		includedDone := 0
		for _, phrase := range include {
			trimmedPhrase := strings.TrimSpace(phrase)
			if trimmedPhrase == "" {
				continue
			}
			phraseRC := NewRuneCluster(trimmedPhrase)
			if !phraseRC.SubSetOf(target) {
				log.Println("Phrase \"" + trimmedPhrase + "\" not a subset of input")
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
		if includedDone == 0 {
			log.Println("Can't make anything with these included phrases")
		}
	} else {
		findTuples("", target, filtered, output)
	}
}

var trials int = 0
var pruneCount int = 0

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

	if len(dict) == 0 {
		return
	}

	// Build suffix-sum array: suffixCounts[i] holds the combined rune counts
	// for dict[i:]. This lets us check feasibility at each loop iteration in
	// O(26) instead of rebuilding from scratch in O(n*26).
	suffixCounts := make([]RuneCluster, len(dict))
	suffixCounts[len(dict)-1] = *dict[len(dict)-1].cluster
	for i := len(dict) - 2; i >= 0; i-- {
		suffixCounts[i] = suffixCounts[i+1]
		suffixCounts[i].Add(dict[i].cluster)
	}

	// Check if the full dictionary can cover the target at all
	if !target.SubSetOf(&suffixCounts[0]) {
		if len(dict) >= 10 {
			pruneCount = 0
		} else {
			pruneCount += 1
		}
		return
	}

	for index, dp := range dict {
		// Check if dict[index:] can still cover the target
		if !target.SubSetOf(&suffixCounts[index]) {
			break // remaining words can't help, and they only get smaller
		}

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
