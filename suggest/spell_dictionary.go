// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package suggest

import (
	"fmt"
)

// SpellDictionary is the interface for a dictionary used by SpellChecker.
//
// This is the Go port of Lucene's org.apache.lucene.search.spell.Dictionary.
type SpellDictionary interface {
	// GetFreq returns the frequency of the given word.
	GetFreq(word string) int
	// GetSuggestedWords returns a list of words that are similar to the given word.
	GetSuggestedWords(word string, numSuggestions int) ([]string, error)
	// Close closes the dictionary.
	Close() error
}

// SuggestWord represents a suggested word with its frequency and score.
//
// This is the Go port of Lucene's org.apache.lucene.search.spell.SuggestWord.
type SuggestWord struct {
	Word      string
	Frequency int
	Score     float64
}

// PlainTextDictionary is a simple dictionary based on a plain text file.
//
// This is the Go port of Lucene's org.apache.lucene.search.spell.PlainTextDictionary.
type PlainTextDictionary struct {
	frequencies map[string]int
}

func NewPlainTextDictionary(words map[string]int) *PlainTextDictionary {
	return &PlainTextDictionary{
		frequencies: words,
	}
}

func (pd *PlainTextDictionary) GetFreq(word string) int {
	return pd.frequencies[word]
}

func (pd *PlainTextDictionary) GetSuggestedWords(word string, numSuggestions int) ([]string, error) {
	// Simplified suggestion logic based on frequencies.
	var suggestions []string
	for w := range pd.frequencies {
		if w != word {
			suggestions = append(suggestions, w)
		}
		if len(suggestions) >= numSuggestions {
			break
		}
	}
	return suggestions, nil
}

func (pd *PlainTextDictionary) Close() error {
	return nil
}
