// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package te

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// TeluguNormalizationFilter applies TeluguNormalizer to each token.
//
// Tokens marked as keywords (via KeywordAttribute) are not modified.
//
// Go port of org.apache.lucene.analysis.te.TeluguNormalizationFilter
// (Apache Lucene 10.4.0).
type TeluguNormalizationFilter struct {
	*analysis.BaseTokenFilter
	normalizer *TeluguNormalizer
}

// NewTeluguNormalizationFilter creates a new TeluguNormalizationFilter.
func NewTeluguNormalizationFilter(input analysis.TokenStream) *TeluguNormalizationFilter {
	return &TeluguNormalizationFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
		normalizer:      NewTeluguNormalizer(),
	}
}

// IncrementToken processes the next token, applying Telugu normalization
// unless the token is marked as a keyword.
func (f *TeluguNormalizationFilter) IncrementToken() (bool, error) {
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
					newLen := f.normalizer.Normalize(runes, len(runes))
					termAttr.SetValue(string(runes[:newLen]))
				}
			}
		}
	}
	return hasToken, nil
}

// Ensure TeluguNormalizationFilter implements TokenFilter.
var _ analysis.TokenFilter = (*TeluguNormalizationFilter)(nil)

// TeluguNormalizationFilterFactory creates TeluguNormalizationFilter instances.
//
// Go port of org.apache.lucene.analysis.te.TeluguNormalizationFilterFactory.
//
// SPI name: "teluguNormalization"
type TeluguNormalizationFilterFactory struct{}

// NewTeluguNormalizationFilterFactory creates a new TeluguNormalizationFilterFactory.
func NewTeluguNormalizationFilterFactory() *TeluguNormalizationFilterFactory {
	return &TeluguNormalizationFilterFactory{}
}

// Create wraps the given input with a TeluguNormalizationFilter.
func (f *TeluguNormalizationFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	return NewTeluguNormalizationFilter(input)
}

// Ensure interface compliance.
var _ analysis.TokenFilterFactory = (*TeluguNormalizationFilterFactory)(nil)
