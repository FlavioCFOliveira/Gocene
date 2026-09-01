// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package suggest

// Lookup is the common interface for all suggestion lookups.
//
// This is the Go port of Lucene's org.apache.lucene.search.suggest.Lookup.
type Lookup interface {
	// Lookup returns the top suggestions for the given input.
	Lookup(input string, numResults int) ([]Suggestion, error)
}

// Suggestion represents a single suggestion result.
//
// This is the Go port of Lucene's org.apache.lucene.search.suggest.Lookup.Suggestion.
type Suggestion struct {
	Value string
	Score float64
}

// InputIterator is an iterator over the input for building a dictionary.
//
// This is the Go port of Lucene's org.apache.lucene.search.suggest.InputIterator.
type InputIterator interface {
	// Next returns the next input.
	Next() (string, float64, bool)
	// Reset resets the iterator.
	Reset()
}

// Dictionary is the interface for a dictionary used for suggestions.
//
// This is the Go port of Lucene's org.apache.lucene.search.suggest.Dictionary.
type Dictionary interface {
	// Build builds the dictionary from the given iterator.
	Build(iterator InputIterator) error
	// Lookup returns the suggestions for the given input.
	Lookup(input string, numResults int) ([]Suggestion, error)
}
