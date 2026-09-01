// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package suggest

import (
	"fmt"
)

// FSTCompletion is a completion lookup based on a Finite State Transducer.
//
// This is the Go port of Lucene's org.apache.lucene.search.suggest.fst.FSTCompletion.
type FSTCompletion struct {
	// In a real implementation, this would hold the FST.
	fstData map[string]float64
}

func (fc *FSTCompletion) Lookup(input string, numResults int) ([]Suggestion, error) {
	var results []Suggestion
	for word, weight := range fc.fstData {
		if len(word) >= len(input) && word[:len(input)] == input {
			results = append(results, Suggestion{Value: word, Score: weight})
		}
		if len(results) >= numResults {
			break
		}
	}
	return results, nil
}

// FSTCompletionBuilder is used to build an FSTCompletion.
//
// This is the Go port of Lucene's org.apache.lucene.search.suggest.fst.FSTCompletionBuilder.
type FSTCompletionBuilder struct {
	data map[string]float64
}

func NewFSTCompletionBuilder() *FSTCompletionBuilder {
	return &FSTCompletionBuilder{
		data: make(map[string]float64),
	}
}

func (fb *FSTCompletionBuilder) Add(word string, weight float64) {
	fb.data[word] = weight
}

func (fb *FSTCompletionBuilder) Build() *FSTCompletion {
	return &FSTCompletion{
		fstData: fb.data,
	}
}
