// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// StandardAnalyzer filters [StandardTokenizer] with
// [LowerCaseFilter] and [StopFilter], using a configurable list of
// stop words.
//
// This is the Go port of
// org.apache.lucene.analysis.standard.StandardAnalyzer from Lucene
// 10.4.0.
//
// The Lucene default is to ship with no stop words (CharArraySet.EMPTY_SET);
// callers that want the English stop list must construct the
// analyzer explicitly via [NewStandardAnalyzerWithStopWords] passing
// [EnglishStopWords]. This mirrors Lucene's StandardAnalyzer(stopWords)
// constructor.
type StandardAnalyzer struct {
	*BaseAnalyzer

	// stopWords is the set of stop words to filter. Empty by
	// default, matching the Lucene reference.
	stopWords []string

	// maxTokenLength is propagated to every StandardTokenizer
	// created by this analyzer. Defaults to
	// [DefaultMaxTokenLength].
	maxTokenLength int
}

// NewStandardAnalyzer builds a StandardAnalyzer with no stop words,
// matching the Lucene no-arg constructor (which calls
// `this(CharArraySet.EMPTY_SET)`).
func NewStandardAnalyzer() *StandardAnalyzer {
	return &StandardAnalyzer{
		BaseAnalyzer:   NewAnalyzer(),
		stopWords:      nil,
		maxTokenLength: DefaultMaxTokenLength,
	}
}

// NewStandardAnalyzerWithStopWords builds a StandardAnalyzer using
// the given list of stop words. Pass [EnglishStopWords] for the
// classic English stop set.
func NewStandardAnalyzerWithStopWords(stopWords []string) *StandardAnalyzer {
	cp := make([]string, len(stopWords))
	copy(cp, stopWords)
	return &StandardAnalyzer{
		BaseAnalyzer:   NewAnalyzer(),
		stopWords:      cp,
		maxTokenLength: DefaultMaxTokenLength,
	}
}

// MaxTokenLength returns the max token length applied to every
// [StandardTokenizer] this analyzer creates.
func (a *StandardAnalyzer) MaxTokenLength() int {
	return a.maxTokenLength
}

// SetMaxTokenLength stores the maxTokenLength to be applied to
// every [StandardTokenizer] created by [StandardAnalyzer.TokenStream].
//
// Returns an error matching Lucene's IllegalArgumentException when
// length is out of range; the analyzer's internal state is left
// untouched on error.
func (a *StandardAnalyzer) SetMaxTokenLength(length int) error {
	// Validate by exercising a throw-away tokenizer's setter so the
	// range check stays in one place.
	probe := NewStandardTokenizer()
	defer probe.Close()
	if err := probe.SetMaxTokenLength(length); err != nil {
		return err
	}
	a.maxTokenLength = length
	return nil
}

// TokenStream wires StandardTokenizer -> LowerCaseFilter ->
// StopFilter and returns the sink.
//
// fieldName is accepted for parity with Lucene's API but is unused.
func (a *StandardAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	_ = fieldName
	tokenizer := NewStandardTokenizer()
	if err := tokenizer.SetMaxTokenLength(a.maxTokenLength); err != nil {
		return nil, err
	}
	if err := tokenizer.SetReader(reader); err != nil {
		return nil, err
	}
	var stream TokenStream = NewLowerCaseFilter(tokenizer)
	if len(a.stopWords) > 0 {
		stream = NewStopFilter(stream, a.stopWords)
	}
	return stream, nil
}

// GetStopWords returns a copy of the stop word list used by this
// analyzer.
func (a *StandardAnalyzer) GetStopWords() []string {
	out := make([]string, len(a.stopWords))
	copy(out, a.stopWords)
	return out
}

// SetStopWords replaces the stop word list. The new list is copied
// so the caller's slice can be reused freely.
func (a *StandardAnalyzer) SetStopWords(stopWords []string) {
	cp := make([]string, len(stopWords))
	copy(cp, stopWords)
	a.stopWords = cp
}

// Ensure StandardAnalyzer implements Analyzer.
var _ Analyzer = (*StandardAnalyzer)(nil)
