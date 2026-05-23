// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package compound

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// DictionaryCompoundWordTokenFilter decomposes compound words found in many
// Germanic languages using a brute-force dictionary lookup.
//
// This is the Go port of
// org.apache.lucene.analysis.compound.DictionaryCompoundWordTokenFilter from
// Apache Lucene 10.4.0.
type DictionaryCompoundWordTokenFilter struct {
	*CompoundWordTokenFilterBase
	onlyLongestMatchIgnoreSubwords bool
}

// NewDictionaryCompoundWordTokenFilter creates a filter with a dictionary
// and default size settings.
func NewDictionaryCompoundWordTokenFilter(
	input analysis.TokenStream,
	dictionary *analysis.CharArraySet,
) (*DictionaryCompoundWordTokenFilter, error) {
	if dictionary == nil {
		return nil, fmt.Errorf("dictionary must not be nil")
	}
	base, err := newCompoundWordTokenFilterBase(
		input, dictionary,
		DefaultMinWordSize, DefaultMinSubwordSize, DefaultMaxSubwordSize, false)
	if err != nil {
		return nil, err
	}
	return &DictionaryCompoundWordTokenFilter{CompoundWordTokenFilterBase: base}, nil
}

// NewDictionaryCompoundWordTokenFilterFull creates a filter with all parameters.
func NewDictionaryCompoundWordTokenFilterFull(
	input analysis.TokenStream,
	dictionary *analysis.CharArraySet,
	minWordSize, minSubwordSize, maxSubwordSize int,
	onlyLongestMatchIgnoreSubwords bool,
) (*DictionaryCompoundWordTokenFilter, error) {
	if dictionary == nil {
		return nil, fmt.Errorf("dictionary must not be nil")
	}
	base, err := newCompoundWordTokenFilterBase(
		input, dictionary,
		minWordSize, minSubwordSize, maxSubwordSize, false)
	if err != nil {
		return nil, err
	}
	return &DictionaryCompoundWordTokenFilter{
		CompoundWordTokenFilterBase:    base,
		onlyLongestMatchIgnoreSubwords: onlyLongestMatchIgnoreSubwords,
	}, nil
}

// IncrementToken advances to the next token.
func (f *DictionaryCompoundWordTokenFilter) IncrementToken() (bool, error) {
	return f.CompoundWordTokenFilterBase.IncrementToken(f.decompose)
}

func (f *DictionaryCompoundWordTokenFilter) decompose() {
	onlyLongest := f.CompoundWordTokenFilterBase.OnlyLongestMatch() || f.onlyLongestMatchIgnoreSubwords
	runes := f.TermRunes()
	l := len(runes)
	for i := 0; i <= l-f.MinSubwordSize(); i++ {
		var longestMatch *struct{ offset, length int }
		for j := f.MinSubwordSize(); j <= f.MaxSubwordSize(); j++ {
			if i+j > l {
				break
			}
			if f.DictionaryContains(runes, i, j) {
				if onlyLongest {
					if longestMatch == nil || longestMatch.length < j {
						longestMatch = &struct{ offset, length int }{i, j}
					}
				} else {
					f.AddToken(i, j)
				}
			}
		}
		if longestMatch != nil {
			f.AddToken(longestMatch.offset, longestMatch.length)
			if f.onlyLongestMatchIgnoreSubwords {
				i += longestMatch.length - 1
			}
		}
	}
}

// Ensure DictionaryCompoundWordTokenFilter implements TokenFilter.
var _ analysis.TokenFilter = (*DictionaryCompoundWordTokenFilter)(nil)

// DictionaryCompoundWordTokenFilterFactory creates
// DictionaryCompoundWordTokenFilter instances.
//
// This is the Go port of
// org.apache.lucene.analysis.compound.DictionaryCompoundWordTokenFilterFactory
// from Apache Lucene 10.4.0.
//
// Deviation: the Java reference loads the dictionary from a file via
// ResourceLoader. This Go port accepts a pre-built CharArraySet to avoid
// filesystem dependencies in the core filter layer.
type DictionaryCompoundWordTokenFilterFactory struct {
	dictionary                     *analysis.CharArraySet
	minWordSize                    int
	minSubwordSize                 int
	maxSubwordSize                 int
	onlyLongestMatchIgnoreSubwords bool
}

// NewDictionaryCompoundWordTokenFilterFactory creates a factory with default
// size parameters.
func NewDictionaryCompoundWordTokenFilterFactory(
	dictionary *analysis.CharArraySet,
) *DictionaryCompoundWordTokenFilterFactory {
	return &DictionaryCompoundWordTokenFilterFactory{
		dictionary:     dictionary,
		minWordSize:    DefaultMinWordSize,
		minSubwordSize: DefaultMinSubwordSize,
		maxSubwordSize: DefaultMaxSubwordSize,
	}
}

// NewDictionaryCompoundWordTokenFilterFactoryFull creates a factory with all
// parameters.
func NewDictionaryCompoundWordTokenFilterFactoryFull(
	dictionary *analysis.CharArraySet,
	minWordSize, minSubwordSize, maxSubwordSize int,
	onlyLongestMatchIgnoreSubwords bool,
) *DictionaryCompoundWordTokenFilterFactory {
	return &DictionaryCompoundWordTokenFilterFactory{
		dictionary:                     dictionary,
		minWordSize:                    minWordSize,
		minSubwordSize:                 minSubwordSize,
		maxSubwordSize:                 maxSubwordSize,
		onlyLongestMatchIgnoreSubwords: onlyLongestMatchIgnoreSubwords,
	}
}

// Create creates a DictionaryCompoundWordTokenFilter wrapping input.
// Returns input unchanged if the dictionary is nil.
func (f *DictionaryCompoundWordTokenFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	if f.dictionary == nil {
		// empty dictionary — pass through unchanged
		return &passThroughFilter{BaseTokenFilter: analysis.NewBaseTokenFilter(input)}
	}
	filter, err := NewDictionaryCompoundWordTokenFilterFull(
		input, f.dictionary,
		f.minWordSize, f.minSubwordSize, f.maxSubwordSize,
		f.onlyLongestMatchIgnoreSubwords)
	if err != nil {
		panic(fmt.Sprintf("compound: DictionaryCompoundWordTokenFilterFactory.Create: %v", err))
	}
	return filter
}

// Ensure factory implements TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*DictionaryCompoundWordTokenFilterFactory)(nil)
