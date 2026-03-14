// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
)

// StopFilter removes stop words from the token stream.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.StopFilter.
//
// Stop words are common words that are filtered out because they don't
// carry much semantic meaning (e.g., "the", "a", "is", "in").
// This filter removes tokens that match any word in the stop set.
type StopFilter struct {
	*BaseTokenFilter

	// stopWords is the set of words to filter out
	stopWords map[string]struct{}

	// termAttr holds the CharTermAttribute from the shared attribute source
	termAttr CharTermAttribute

	// posIncrAttr holds the PositionIncrementAttribute from the shared attribute source
	posIncrAttr PositionIncrementAttribute
}

// NewStopFilter creates a new StopFilter with the given stop words.
func NewStopFilter(input TokenStream, stopWords []string) *StopFilter {
	filter := &StopFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		stopWords:       make(map[string]struct{}, len(stopWords)),
	}

	// Build stop word set
	for _, word := range stopWords {
		filter.stopWords[word] = struct{}{}
	}

	// Get attributes from the shared AttributeSource
	attrSource := filter.GetAttributeSource()
	if attrSource != nil {
		attr := attrSource.GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
		if attr != nil {
			filter.termAttr = attr.(CharTermAttribute)
		}
		attr = attrSource.GetAttributeByType(reflect.TypeOf(&positionIncrementAttribute{}))
		if attr != nil {
			filter.posIncrAttr = attr.(PositionIncrementAttribute)
		}
	}

	return filter
}

// NewStopFilterWithEnglishStopWords creates a StopFilter with English stop words.
func NewStopFilterWithEnglishStopWords(input TokenStream) *StopFilter {
	return NewStopFilter(input, EnglishStopWords)
}

// IncrementToken advances to the next token, skipping stop words.
func (f *StopFilter) IncrementToken() (bool, error) {
	increments := 0

	for {
		hasToken, err := f.input.IncrementToken()
		if err != nil {
			return false, err
		}
		if !hasToken {
			return false, nil
		}

		increments++

		if f.termAttr != nil {
			token := f.termAttr.String()
			if _, isStopWord := f.stopWords[token]; !isStopWord {
				// Not a stop word - adjust position increment and return
				if f.posIncrAttr != nil && increments > 1 {
					f.posIncrAttr.SetPositionIncrement(f.posIncrAttr.GetPositionIncrement() + increments - 1)
				}
				return true, nil
			}
			// This is a stop word - continue to next token
		} else {
			// No term attribute - just return the token
			return true, nil
		}
	}
}

// IsStopWord checks if a word is in the stop word set.
func (f *StopFilter) IsStopWord(word string) bool {
	_, exists := f.stopWords[word]
	return exists
}

// AddStopWord adds a word to the stop word set.
func (f *StopFilter) AddStopWord(word string) {
	f.stopWords[word] = struct{}{}
}

// RemoveStopWord removes a word from the stop word set.
func (f *StopFilter) RemoveStopWord(word string) {
	delete(f.stopWords, word)
}

// Ensure StopFilter implements TokenFilter
var _ TokenFilter = (*StopFilter)(nil)

// StopFilterFactory creates StopFilter instances.
type StopFilterFactory struct {
	stopWords *CharArraySet
}

// NewStopFilterFactory creates a new StopFilterFactory with default English stop words.
func NewStopFilterFactory() *StopFilterFactory {
	return NewStopFilterFactoryWithWords(GetWordSetFromStrings(EnglishStopWords, true))
}

// NewStopFilterFactoryWithWords creates a new StopFilterFactory with custom stop words.
func NewStopFilterFactoryWithWords(stopWords *CharArraySet) *StopFilterFactory {
	return &StopFilterFactory{
		stopWords: stopWords,
	}
}

// Create creates a StopFilter wrapping the given input.
func (f *StopFilterFactory) Create(input TokenStream) TokenFilter {
	// Convert CharArraySet to []string for the StopFilter
	words := make([]string, 0, f.stopWords.Size())
	f.stopWords.ForEach(func(item string) bool {
		words = append(words, item)
		return true
	})
	return NewStopFilter(input, words)
}

// GetStopWords returns the stop words set.
func (f *StopFilterFactory) GetStopWords() *CharArraySet {
	return f.stopWords
}

// Ensure StopFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*StopFilterFactory)(nil)
