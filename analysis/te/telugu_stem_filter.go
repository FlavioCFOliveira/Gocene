// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package te

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// TeluguStemFilter applies TeluguStemmer to each token.
//
// Tokens marked as keywords (via KeywordAttribute) are not modified.
//
// Go port of org.apache.lucene.analysis.te.TeluguStemFilter
// (Apache Lucene 10.4.0).
type TeluguStemFilter struct {
	*analysis.BaseTokenFilter
	stemmer teluguStemmer
}

// NewTeluguStemFilter creates a new TeluguStemFilter.
func NewTeluguStemFilter(input analysis.TokenStream) *TeluguStemFilter {
	return &TeluguStemFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
	}
}

// IncrementToken processes the next token, applying Telugu stemming unless
// the token is marked as a keyword.
func (f *TeluguStemFilter) IncrementToken() (bool, error) {
	hasToken, err := f.GetInput().IncrementToken()
	if err != nil {
		return false, err
	}
	if hasToken {
		as := f.GetAttributeSource()
		isKeyword := false
		if kAttr := as.GetAttribute(analysis.KeywordAttributeType); kAttr != nil {
			if ka, ok := kAttr.(analysis.KeywordAttribute); ok {
				isKeyword = ka.IsKeywordToken()
			}
		}
		if !isKeyword {
			if tAttr := as.GetAttribute(analysis.CharTermAttributeType); tAttr != nil {
				if termAttr, ok := tAttr.(analysis.CharTermAttribute); ok {
					runes := []rune(termAttr.String())
					newLen := f.stemmer.stem(runes, len(runes))
					termAttr.SetValue(string(runes[:newLen]))
				}
			}
		}
	}
	return hasToken, nil
}

// Ensure TeluguStemFilter implements TokenFilter.
var _ analysis.TokenFilter = (*TeluguStemFilter)(nil)

// TeluguStemFilterFactory creates TeluguStemFilter instances.
//
// Go port of org.apache.lucene.analysis.te.TeluguStemFilterFactory.
//
// SPI name: "teluguStem"
type TeluguStemFilterFactory struct{}

// NewTeluguStemFilterFactory creates a new TeluguStemFilterFactory.
func NewTeluguStemFilterFactory() *TeluguStemFilterFactory {
	return &TeluguStemFilterFactory{}
}

// Create wraps the given input with a TeluguStemFilter.
func (f *TeluguStemFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	return NewTeluguStemFilter(input)
}

// Ensure interface compliance.
var _ analysis.TokenFilterFactory = (*TeluguStemFilterFactory)(nil)
