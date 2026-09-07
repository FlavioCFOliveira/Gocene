// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// NewStandardAnalyzer creates a new StandardAnalyzer with no stop words.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.standard.StandardAnalyzer.
func NewStandardAnalyzer() *Analyzer {
	return NewStandardAnalyzerWithStopWords(nil)
}

// NewStandardAnalyzerWithStopWords creates a new StandardAnalyzer with the given stop words.
func NewStandardAnalyzerWithStopWords(stopWords []string) *Analyzer {
	a := NewAnalyzer(GlobalReuseStrategy)

	a.CreateComponents = func(fieldName string) *TokenStreamComponents {
		src := NewStandardTokenizer()
		// StandardAnalyzer.DEFAULT_MAX_TOKEN_LENGTH is used by default.

		var tok TokenStream = NewLowerCaseFilter(src)
		if stopWords != nil {
			tok = NewStopFilter(tok, stopWords)
		}

		return &TokenStreamComponents{
			source: func(r io.Reader) error {
				src.SetReader(r)
return nil
			},
			sink: tok,
		}
	}

	a.normalizeFilter = func(fieldName string, in TokenStream) TokenStream {
		return NewLowerCaseFilter(in)
	}

	return a
}
