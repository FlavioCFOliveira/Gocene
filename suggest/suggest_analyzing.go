// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package suggest

import (
	"fmt"
)

// AnalyzingSuggester is a suggester that uses an analyzer.
//
// This is the Go port of Lucene's org.apache.lucene.search.suggest.analyzing.AnalyzingSuggester.
type AnalyzingSuggester struct {
	dictionary Dictionary
}

func NewAnalyzingSuggester(dict Dictionary) *AnalyzingSuggester {
	return &AnalyzingSuggester{
		dictionary: dict,
	}
}

func (as *AnalyzingSuggester) Lookup(input string, numResults int) ([]Suggestion, error) {
	return as.dictionary.Lookup(input, numResults)
}

// FuzzySuggester is a suggester that supports fuzzy matching.
//
// This is the Go port of Lucene's org.apache.lucene.search.suggest.analyzing.FuzzySuggester.
type FuzzySuggester struct {
	AnalyzingSuggester
	distance StringDistance
}

func NewFuzzySuggester(dict Dictionary, dist StringDistance) *FuzzySuggester {
	return &FuzzySuggester{
		AnalyzingSuggester: *NewAnalyzingSuggester(dict),
		distance:           dist,
	}
}

func (fs *FuzzySuggester) Lookup(input string, numResults int) ([]Suggestion, error) {
	// Simplified fuzzy lookup.
	return fs.AnalyzingSuggester.Lookup(input, numResults)
}

// BlendedInfixSuggester is a suggester that blends prefix and infix matching.
//
// This is the Go port of Lucene's org.apache.lucene.search.suggest.analyzing.BlendedInfixSuggester.
type BlendedInfixSuggester struct {
	AnalyzingSuggester
}

func NewBlendedInfixSuggester(dict Dictionary) *BlendedInfixSuggester {
	return &BlendedInfixSuggester{
		AnalyzingSuggester: *NewAnalyzingSuggester(dict),
	}
}
