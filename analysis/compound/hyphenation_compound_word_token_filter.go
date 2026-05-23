// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package compound

import (
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/compound/hyphenation"
)

// HyphenationCompoundWordTokenFilter decomposes compound words using a
// hyphenation grammar and an optional dictionary.
//
// This is the Go port of
// org.apache.lucene.analysis.compound.HyphenationCompoundWordTokenFilter from
// Apache Lucene 10.4.0.
type HyphenationCompoundWordTokenFilter struct {
	*CompoundWordTokenFilterBase

	hyphenator          *hyphenation.HyphenationTree
	noSubMatches        bool
	noOverlappingMatches bool
	calcSubMatches      bool
}

// NewHyphenationCompoundWordTokenFilter creates a filter with default sizes.
func NewHyphenationCompoundWordTokenFilter(
	input analysis.TokenStream,
	hyp *hyphenation.HyphenationTree,
	dictionary *analysis.CharArraySet,
) (*HyphenationCompoundWordTokenFilter, error) {
	return NewHyphenationCompoundWordTokenFilterFull(
		input, hyp, dictionary,
		DefaultMinWordSize, DefaultMinSubwordSize, DefaultMaxSubwordSize,
		false, false, false)
}

// NewHyphenationCompoundWordTokenFilterFull creates a filter with all
// parameters.
func NewHyphenationCompoundWordTokenFilterFull(
	input analysis.TokenStream,
	hyp *hyphenation.HyphenationTree,
	dictionary *analysis.CharArraySet,
	minWordSize, minSubwordSize, maxSubwordSize int,
	onlyLongestMatch, noSubMatches, noOverlappingMatches bool,
) (*HyphenationCompoundWordTokenFilter, error) {
	if hyp == nil {
		return nil, fmt.Errorf("hyphenator must not be nil")
	}
	base, err := newCompoundWordTokenFilterBase(
		input, dictionary,
		minWordSize, minSubwordSize, maxSubwordSize, onlyLongestMatch)
	if err != nil {
		return nil, err
	}
	f := &HyphenationCompoundWordTokenFilter{
		CompoundWordTokenFilterBase: base,
		hyphenator:                  hyp,
		noSubMatches:                noSubMatches,
		noOverlappingMatches:        noOverlappingMatches,
	}
	f.calcSubMatches = !onlyLongestMatch && !noSubMatches && !noOverlappingMatches
	return f, nil
}

// GetHyphenationTree loads a HyphenationTree from the XML reader r.
func GetHyphenationTree(r io.Reader) (*hyphenation.HyphenationTree, error) {
	tree := hyphenation.NewHyphenationTree()
	if err := tree.LoadPatterns(r); err != nil {
		return nil, err
	}
	return tree, nil
}

// IncrementToken advances to the next token.
func (f *HyphenationCompoundWordTokenFilter) IncrementToken() (bool, error) {
	return f.CompoundWordTokenFilterBase.IncrementToken(f.decompose)
}

func (f *HyphenationCompoundWordTokenFilter) decompose() {
	runes := f.TermRunes()
	tlen := len(runes)
	dict := f.CompoundWordTokenFilterBase.dictionary

	// If token is in dictionary and we are not interested in sub-matches, skip.
	if dict != nil && !f.calcSubMatches {
		if dict.Contains(runes, 0, tlen) ||
			(tlen > 1 && dict.Contains(runes, 0, tlen-1)) {
			return
		}
	}

	hyphens := f.hyphenator.Hyphenate(runes, 0, tlen, 1, 1)
	if hyphens == nil {
		return
	}

	maxSub := f.MaxSubwordSize()
	if maxSub > tlen-1 {
		maxSub = tlen - 1
	}

	hyp := hyphens.GetHyphenationPoints()
	consumed := -1

	for i := 0; i < len(hyp); i++ {
		if f.noOverlappingMatches {
			if i < consumed {
				i = consumed
			}
		}
		start := hyp[i]
		until := i
		if f.noSubMatches && consumed > i {
			until = consumed
		}
		for j := len(hyp) - 1; j > until; j-- {
			partLen := hyp[j] - start
			if partLen > maxSub {
				continue
			}
			if partLen < f.MinSubwordSize() {
				break
			}
			if dict == nil || dict.Contains(runes, start, partLen) {
				f.AddToken(start, partLen)
				consumed = j
				if !f.calcSubMatches {
					break
				}
			} else if dict.Contains(runes, start, partLen-1) {
				f.AddToken(start, partLen-1)
				consumed = j
				if !f.calcSubMatches {
					break
				}
			}
		}
	}
}

// Ensure HyphenationCompoundWordTokenFilter implements TokenFilter.
var _ analysis.TokenFilter = (*HyphenationCompoundWordTokenFilter)(nil)

// HyphenationCompoundWordTokenFilterFactory creates
// HyphenationCompoundWordTokenFilter instances.
//
// This is the Go port of
// org.apache.lucene.analysis.compound.HyphenationCompoundWordTokenFilterFactory
// from Apache Lucene 10.4.0.
//
// Deviation: the Java reference loads hyphenator and dictionary from files via
// ResourceLoader. This Go port accepts pre-built instances to avoid filesystem
// dependencies in the filter layer.
type HyphenationCompoundWordTokenFilterFactory struct {
	dictionary          *analysis.CharArraySet
	hyphenator          *hyphenation.HyphenationTree
	minWordSize         int
	minSubwordSize      int
	maxSubwordSize      int
	onlyLongestMatch    bool
	noSubMatches        bool
	noOverlappingMatches bool
}

// NewHyphenationCompoundWordTokenFilterFactory creates a factory.
func NewHyphenationCompoundWordTokenFilterFactory(
	hyphenator *hyphenation.HyphenationTree,
	dictionary *analysis.CharArraySet,
	minWordSize, minSubwordSize, maxSubwordSize int,
	onlyLongestMatch, noSubMatches, noOverlappingMatches bool,
) *HyphenationCompoundWordTokenFilterFactory {
	return &HyphenationCompoundWordTokenFilterFactory{
		hyphenator:          hyphenator,
		dictionary:          dictionary,
		minWordSize:         minWordSize,
		minSubwordSize:      minSubwordSize,
		maxSubwordSize:      maxSubwordSize,
		onlyLongestMatch:    onlyLongestMatch,
		noSubMatches:        noSubMatches,
		noOverlappingMatches: noOverlappingMatches,
	}
}

// Create creates a HyphenationCompoundWordTokenFilter wrapping input.
func (f *HyphenationCompoundWordTokenFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	filter, err := NewHyphenationCompoundWordTokenFilterFull(
		input, f.hyphenator, f.dictionary,
		f.minWordSize, f.minSubwordSize, f.maxSubwordSize,
		f.onlyLongestMatch, f.noSubMatches, f.noOverlappingMatches)
	if err != nil {
		panic(fmt.Sprintf("compound: HyphenationCompoundWordTokenFilterFactory.Create: %v", err))
	}
	return filter
}

// Ensure factory implements TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*HyphenationCompoundWordTokenFilterFactory)(nil)
