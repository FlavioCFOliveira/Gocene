// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hunspell

import (
	"sort"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// HunspellStemFilter uses hunspell affix rules and word lists to stem tokens.
// Because Hunspell supports a word having multiple stems, this filter can emit
// multiple tokens for each consumed token.
//
// The filter is aware of KeywordAttribute: to prevent a term from being stemmed,
// set KeywordAttribute.IsKeyword() = true in a prior TokenStream.
//
// This is the Go port of
// org.apache.lucene.analysis.hunspell.HunspellStemFilter from Apache Lucene 10.4.0.
//
// Deviation: Java uses CharsRef to represent stems; Go uses plain strings to avoid
// unnecessary allocations.
type HunspellStemFilter struct {
	*analysis.BaseTokenFilter

	stemmer     *Stemmer
	termAttr    analysis.CharTermAttribute
	posIncAttr  analysis.PositionIncrementAttribute
	keywordAttr analysis.KeywordAttribute

	buffer      []string
	savedState  *util.AttributeState
	dedup       bool
	longestOnly bool
}

// NewHunspellStemFilter creates a new HunspellStemFilter emitting all possible stems.
func NewHunspellStemFilter(input analysis.TokenStream, dictionary *Dictionary) *HunspellStemFilter {
	return NewHunspellStemFilterFull(input, dictionary, true, false)
}

// NewHunspellStemFilterDedup creates a filter with configurable dedup behaviour.
func NewHunspellStemFilterDedup(input analysis.TokenStream, dictionary *Dictionary, dedup bool) *HunspellStemFilter {
	return NewHunspellStemFilterFull(input, dictionary, dedup, false)
}

// NewHunspellStemFilterFull creates a filter with all options.
//
//   - dedup: deduplicate equal stems (has no effect when longestOnly is true)
//   - longestOnly: emit only the longest stem
func NewHunspellStemFilterFull(input analysis.TokenStream, dictionary *Dictionary, dedup, longestOnly bool) *HunspellStemFilter {
	f := &HunspellStemFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
		stemmer:         NewStemmer(dictionary),
		dedup:           dedup && !longestOnly,
		longestOnly:     longestOnly,
	}
	src := f.GetAttributeSource()
	if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
		f.termAttr = a.(analysis.CharTermAttribute)
	} else {
		f.termAttr = analysis.NewCharTermAttribute()
		src.AddAttributeImpl(f.termAttr)
	}
	if a := src.GetAttribute(analysis.PositionIncrementAttributeType); a != nil {
		f.posIncAttr = a.(analysis.PositionIncrementAttribute)
	} else {
		f.posIncAttr = analysis.NewPositionIncrementAttribute()
		src.AddAttributeImpl(f.posIncAttr)
	}
	if a := src.GetAttribute(analysis.KeywordAttributeType); a != nil {
		f.keywordAttr = a.(analysis.KeywordAttribute)
	}
	return f
}

// IncrementToken advances to the next token. Satisfies analysis.TokenStream.
func (f *HunspellStemFilter) IncrementToken() (bool, error) {
	if len(f.buffer) > 0 {
		nextStem := f.buffer[0]
		f.buffer = f.buffer[1:]
		f.GetAttributeSource().RestoreState(f.savedState)
		f.posIncAttr.SetPositionIncrement(0)
		f.termAttr.SetEmpty()
		f.termAttr.AppendString(nextStem)
		return true, nil
	}

	ok, err := f.GetInput().IncrementToken()
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}

	if f.keywordAttr != nil && f.keywordAttr.IsKeywordToken() {
		return true, nil
	}

	term := f.termAttr.String()
	runes := []rune(term)
	var stems []string
	if f.dedup {
		stems = f.stemmer.UniqueStems(runes, len(runes))
	} else {
		stems = f.stemmer.StemRunes(runes, len(runes))
	}

	if len(stems) == 0 {
		// unknown word — pass through unchanged
		return true, nil
	}

	if f.longestOnly && len(stems) > 1 {
		sort.Slice(stems, func(i, j int) bool {
			li := len([]rune(stems[i]))
			lj := len([]rune(stems[j]))
			if li != lj {
				return li > lj
			}
			return stems[i] > stems[j]
		})
		stems = stems[:1]
	}

	f.buffer = stems[1:]
	f.termAttr.SetEmpty()
	f.termAttr.AppendString(stems[0])

	if !f.longestOnly && len(f.buffer) > 0 {
		f.savedState = f.GetAttributeSource().CaptureState()
	}

	return true, nil
}

// Reset resets the filter.
func (f *HunspellStemFilter) Reset() error {
	f.buffer = nil
	f.savedState = nil
	if r, ok := f.GetInput().(interface{ Reset() error }); ok {
		return r.Reset()
	}
	return nil
}

// ── HunspellStemFilterFactory ─────────────────────────────────────────────────

// HunspellStemFilterFactory creates HunspellStemFilter instances.
//
// This is the Go port of
// org.apache.lucene.analysis.hunspell.HunspellStemFilterFactory from Apache Lucene 10.4.0.
//
// Deviation: Java's factory uses a ResourceLoader to load dictionary/affix files
// at analysis-pipeline construction time. Go callers must supply a pre-built
// *Dictionary directly.
type HunspellStemFilterFactory struct {
	dictionary  *Dictionary
	longestOnly bool
}

// NewHunspellStemFilterFactory constructs a factory with the given pre-built Dictionary.
func NewHunspellStemFilterFactory(dictionary *Dictionary, longestOnly bool) *HunspellStemFilterFactory {
	return &HunspellStemFilterFactory{
		dictionary:  dictionary,
		longestOnly: longestOnly,
	}
}

// Create returns a new HunspellStemFilter wrapping the given TokenStream.
func (f *HunspellStemFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	return NewHunspellStemFilterFull(input, f.dictionary, true, f.longestOnly)
}
