// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package email

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// DefaultMaxTokenLength is the default maximum token length for
// UAX29URLEmailAnalyzer (mirrors StandardAnalyzer.DEFAULT_MAX_TOKEN_LENGTH).
const DefaultMaxTokenLength = 255

// UAX29URLEmailAnalyzer filters UAX29URLEmailTokenizer with LowerCaseFilter and
// StopFilter, using a list of English stop words.
//
// This is the Go port of
// org.apache.lucene.analysis.email.UAX29URLEmailAnalyzer from
// Apache Lucene 10.4.0.
type UAX29URLEmailAnalyzer struct {
	*analysis.BaseAnalyzer

	stopWords      *analysis.CharArraySet
	maxTokenLength int
}

// NewUAX29URLEmailAnalyzer creates a UAX29URLEmailAnalyzer with default English
// stop words.
func NewUAX29URLEmailAnalyzer() *UAX29URLEmailAnalyzer {
	return NewUAX29URLEmailAnalyzerWithStopWords(
		analysis.GetWordSetFromStrings(analysis.EnglishStopWords, false),
	)
}

// NewUAX29URLEmailAnalyzerWithStopWords creates a UAX29URLEmailAnalyzer with the
// given stop-word set.
func NewUAX29URLEmailAnalyzerWithStopWords(stopWords *analysis.CharArraySet) *UAX29URLEmailAnalyzer {
	a := &UAX29URLEmailAnalyzer{
		BaseAnalyzer:   analysis.NewAnalyzer(),
		stopWords:      stopWords,
		maxTokenLength: DefaultMaxTokenLength,
	}
	a.TokenizerFactory = &uax29URLEmailTokenizerFactory{a: a}
	a.AddTokenFilter(analysis.NewLowerCaseFilterFactory())
	a.AddTokenFilter(analysis.NewStopFilterFactoryWithWords(stopWords))
	return a
}

// SetMaxTokenLength sets the maximum token length. Tokens longer than this are
// discarded by the tokenizer.
func (a *UAX29URLEmailAnalyzer) SetMaxTokenLength(length int) { a.maxTokenLength = length }

// GetMaxTokenLength returns the current maximum token length.
func (a *UAX29URLEmailAnalyzer) GetMaxTokenLength() int { return a.maxTokenLength }

// GetStopWords returns the stop-word set used by this analyzer.
func (a *UAX29URLEmailAnalyzer) GetStopWords() *analysis.CharArraySet { return a.stopWords }

// TokenStream creates a TokenStream for the given reader.
func (a *UAX29URLEmailAnalyzer) TokenStream(fieldName string, reader io.Reader) (analysis.TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// Ensure UAX29URLEmailAnalyzer implements Analyzer.
var _ analysis.Analyzer = (*UAX29URLEmailAnalyzer)(nil)
var _ analysis.AnalyzerInterface = (*UAX29URLEmailAnalyzer)(nil)

// uax29URLEmailTokenizerFactory creates UAX29URLEmailTokenizer instances with
// the analyzer's current maxTokenLength.
type uax29URLEmailTokenizerFactory struct {
	a *UAX29URLEmailAnalyzer
}

func (f *uax29URLEmailTokenizerFactory) Create() analysis.Tokenizer {
	t := analysis.NewUAX29URLEmailTokenizer()
	t.SetMaxTokenLength(f.a.maxTokenLength)
	return t
}
