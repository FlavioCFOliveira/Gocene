// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/analysis/api"
)

// StandardAnalyzerDefaultMaxTokenLength is the default maximum allowed token
// length (StandardAnalyzer.DEFAULT_MAX_TOKEN_LENGTH).
const StandardAnalyzerDefaultMaxTokenLength = 255

// StandardAnalyzer filters StandardTokenizer with LowerCaseFilter and
// StopFilter, using a configurable list of stop words.
//
// This is the Go port of Lucene's
// org.apache.lucene.analysis.standard.StandardAnalyzer
// (lucene/core/src/java/org/apache/lucene/analysis/standard/StandardAnalyzer.java).
type StandardAnalyzer struct {
	*StopwordAnalyzerBase

	maxTokenLength int
}

// NewStandardAnalyzerWithStopWords builds an analyzer with the given stop
// words (StandardAnalyzer(CharArraySet)).
func NewStandardAnalyzerWithStopWords(stopWords *CharArraySet) *StandardAnalyzer {
	a := &StandardAnalyzer{
		StopwordAnalyzerBase: NewStopwordAnalyzerBase(stopWords),
		maxTokenLength:       StandardAnalyzerDefaultMaxTokenLength,
	}
	a.CreateComponents = a.createComponents
	a.normalizeFilter = a.normalize
	return a
}

// NewStandardAnalyzer builds an analyzer with no stop words
// (StandardAnalyzer()).
func NewStandardAnalyzer() *StandardAnalyzer {
	return NewStandardAnalyzerWithStopWords(NewEmptyCharArraySet().CharArraySet)
}

// NewStandardAnalyzerFromReader builds an analyzer with the stop words from
// the given reader (StandardAnalyzer(Reader)); see
// WordlistLoader.getWordSet(Reader).
func NewStandardAnalyzerFromReader(stopwords io.Reader) (*StandardAnalyzer, error) {
	set, err := LoadStopwordSet(stopwords)
	if err != nil {
		return nil, err
	}
	return NewStandardAnalyzerWithStopWords(set), nil
}

// SetMaxTokenLength sets the max allowed token length. Tokens larger than
// this will be chopped up at this token length and emitted as multiple
// tokens. If you need to skip such large tokens, you could increase this max
// length, and then use LengthFilter to remove long tokens. The default is
// StandardAnalyzerDefaultMaxTokenLength.
func (a *StandardAnalyzer) SetMaxTokenLength(length int) {
	a.maxTokenLength = length
}

// GetMaxTokenLength returns the current maximum token length.
func (a *StandardAnalyzer) GetMaxTokenLength() int {
	return a.maxTokenLength
}

// createComponents mirrors StandardAnalyzer.createComponents(String). The
// IllegalArgumentException that StandardTokenizer.setMaxTokenLength throws
// for an out-of-range length is unchecked in Java; it is raised as a panic
// here and returned as an error from the reader-setting consumer.
func (a *StandardAnalyzer) createComponents(fieldName string) *TokenStreamComponents {
	src := NewStandardTokenizer()
	if err := src.SetMaxTokenLength(a.maxTokenLength); err != nil {
		panic(err)
	}
	var tok TokenStream = NewLowerCaseFilter(src)
	tok = NewStopFilterWithWords(tok, a.Stopwords.CharArraySet)
	return &TokenStreamComponents{
		Source: func(r io.Reader) error {
			if err := src.SetMaxTokenLength(a.maxTokenLength); err != nil {
				return err
			}
			src.SetReader(r)
			return nil
		},
		Sink: tok,
	}
}

// normalize mirrors StandardAnalyzer.normalize(String, TokenStream).
func (a *StandardAnalyzer) normalize(fieldName string, in api.TokenStream) api.TokenStream {
	return NewLowerCaseFilter(in)
}

var _ api.Analyzer = (*StandardAnalyzer)(nil)
