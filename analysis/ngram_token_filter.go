// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"fmt"
	"reflect"
)

// EdgeNGramTokenFilterDefaultPreserveOriginal is the upstream default
// for preserveOriginal.
const EdgeNGramTokenFilterDefaultPreserveOriginal = false

// EdgeNGramTokenFilter emits incremental n-gram prefixes of each
// input token. Given an input token "abcd" with minGram=2 and
// maxGram=3, the filter emits "ab" and "abc" at the same position
// (position increment 0 for the second).
//
// This is the Go port of
// org.apache.lucene.analysis.ngram.EdgeNGramTokenFilter from Apache
// Lucene 10.4.0.
//
// Deviation from Lucene: the reference uses Java's captureState /
// restoreState to preserve every attribute across the gram emission.
// Gocene's pipeline only routes CharTermAttribute and
// PositionIncrementAttribute, so we preserve those two explicitly;
// callers that need other attributes (offsets, type, payload) should
// keep them in the input filter where appropriate.
type EdgeNGramTokenFilter struct {
	*BaseTokenFilter

	minGram          int
	maxGram          int
	preserveOriginal bool

	curRunes    []rune
	curGramSize int
	curPosIncr  int

	termAttr    CharTermAttribute
	posIncrAttr PositionIncrementAttribute
}

// NewEdgeNGramTokenFilter wraps input with [minGram, maxGram] gram
// emission. Both bounds must be positive and minGram must be <=
// maxGram; otherwise an error is returned.
func NewEdgeNGramTokenFilter(input TokenStream, minGram, maxGram int, preserveOriginal bool) (*EdgeNGramTokenFilter, error) {
	if minGram < 1 {
		return nil, fmt.Errorf("EdgeNGramTokenFilter: minGram must be > 0, got %d", minGram)
	}
	if minGram > maxGram {
		return nil, fmt.Errorf("EdgeNGramTokenFilter: minGram (%d) must be <= maxGram (%d)", minGram, maxGram)
	}
	f := &EdgeNGramTokenFilter{
		BaseTokenFilter:  NewBaseTokenFilter(input),
		minGram:          minGram,
		maxGram:          maxGram,
		preserveOriginal: preserveOriginal,
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); a != nil {
			f.termAttr = a.(CharTermAttribute)
		}
		if a := src.GetAttributeByType(reflect.TypeOf(&positionIncrementAttribute{})); a != nil {
			f.posIncrAttr = a.(PositionIncrementAttribute)
		}
	}
	return f, nil
}

// IncrementToken emits the next n-gram or pulls the next input token
// when the current token is exhausted.
func (f *EdgeNGramTokenFilter) IncrementToken() (bool, error) {
	for {
		if f.curRunes == nil {
			ok, err := f.input.IncrementToken()
			if err != nil || !ok {
				return ok, err
			}
			if f.termAttr == nil {
				return true, nil
			}
			f.curRunes = []rune(f.termAttr.String())
			if f.posIncrAttr != nil {
				f.curPosIncr += f.posIncrAttr.GetPositionIncrement()
			}
			if f.preserveOriginal && len(f.curRunes) < f.minGram {
				if f.posIncrAttr != nil {
					f.posIncrAttr.SetPositionIncrement(f.curPosIncr)
					f.curPosIncr = 0
				}
				// Keep the original token as the emission.
				f.curRunes = nil
				return true, nil
			}
			f.curGramSize = f.minGram
		}
		if f.curGramSize <= len(f.curRunes) {
			if f.curGramSize <= f.maxGram {
				prefix := string(f.curRunes[:f.curGramSize])
				if f.termAttr != nil {
					f.termAttr.SetEmpty()
					f.termAttr.AppendString(prefix)
				}
				if f.posIncrAttr != nil {
					f.posIncrAttr.SetPositionIncrement(f.curPosIncr)
					f.curPosIncr = 0
				}
				f.curGramSize++
				return true, nil
			} else if f.preserveOriginal {
				// Emit the original full token after exceeding maxGram.
				if f.termAttr != nil {
					f.termAttr.SetEmpty()
					f.termAttr.AppendString(string(f.curRunes))
				}
				if f.posIncrAttr != nil {
					f.posIncrAttr.SetPositionIncrement(0)
				}
				f.curRunes = nil
				return true, nil
			}
		}
		f.curRunes = nil
	}
}

// Reset clears generator state.
func (f *EdgeNGramTokenFilter) Reset() error {
	if r, ok := f.input.(interface{ Reset() error }); ok {
		if err := r.Reset(); err != nil {
			return err
		}
	}
	f.curRunes = nil
	f.curPosIncr = 0
	return nil
}

// End forwards the End() call and sets the trailing position
// increment to consume any pending value.
func (f *EdgeNGramTokenFilter) End() error {
	if err := f.input.End(); err != nil {
		return err
	}
	if f.posIncrAttr != nil {
		f.posIncrAttr.SetPositionIncrement(f.curPosIncr)
	}
	return nil
}

// Ensure EdgeNGramTokenFilter implements TokenFilter.
var _ TokenFilter = (*EdgeNGramTokenFilter)(nil)

// NGramTokenFilter emits every n-gram of each input token with
// length between minGram and maxGram (inclusive), at the same
// position.
//
// This is the Go port of
// org.apache.lucene.analysis.ngram.NGramTokenFilter from Apache
// Lucene 10.4.0.
//
// Deviation from Lucene: like EdgeNGramTokenFilter the reference
// uses captureState/restoreState; Gocene preserves the term and
// position-increment attributes explicitly.
type NGramTokenFilter struct {
	*BaseTokenFilter

	minGram          int
	maxGram          int
	preserveOriginal bool

	curRunes    []rune
	curPos      int
	curGramSize int
	curPosIncr  int

	originalEmitted bool

	termAttr    CharTermAttribute
	posIncrAttr PositionIncrementAttribute
}

// NewNGramTokenFilter wraps input. minGram must be > 0 and <=
// maxGram.
func NewNGramTokenFilter(input TokenStream, minGram, maxGram int, preserveOriginal bool) (*NGramTokenFilter, error) {
	if minGram < 1 {
		return nil, fmt.Errorf("NGramTokenFilter: minGram must be > 0, got %d", minGram)
	}
	if minGram > maxGram {
		return nil, fmt.Errorf("NGramTokenFilter: minGram (%d) must be <= maxGram (%d)", minGram, maxGram)
	}
	f := &NGramTokenFilter{
		BaseTokenFilter:  NewBaseTokenFilter(input),
		minGram:          minGram,
		maxGram:          maxGram,
		preserveOriginal: preserveOriginal,
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); a != nil {
			f.termAttr = a.(CharTermAttribute)
		}
		if a := src.GetAttributeByType(reflect.TypeOf(&positionIncrementAttribute{})); a != nil {
			f.posIncrAttr = a.(PositionIncrementAttribute)
		}
	}
	return f, nil
}

// IncrementToken emits the next n-gram or advances to the next input
// token when grams for the current token are exhausted.
func (f *NGramTokenFilter) IncrementToken() (bool, error) {
	for {
		if f.curRunes == nil {
			ok, err := f.input.IncrementToken()
			if err != nil || !ok {
				return ok, err
			}
			if f.termAttr == nil {
				return true, nil
			}
			f.curRunes = []rune(f.termAttr.String())
			if f.posIncrAttr != nil {
				f.curPosIncr += f.posIncrAttr.GetPositionIncrement()
			}
			f.curPos = 0
			f.curGramSize = f.minGram
			f.originalEmitted = false
			if f.preserveOriginal && len(f.curRunes) < f.minGram {
				if f.termAttr != nil {
					f.termAttr.SetEmpty()
					f.termAttr.AppendString(string(f.curRunes))
				}
				if f.posIncrAttr != nil {
					f.posIncrAttr.SetPositionIncrement(f.curPosIncr)
					f.curPosIncr = 0
				}
				f.curRunes = nil
				return true, nil
			}
		}
		// Emit grams: outer loop over start position, inner over size.
		for f.curPos+f.curGramSize <= len(f.curRunes) && f.curGramSize <= f.maxGram {
			gram := string(f.curRunes[f.curPos : f.curPos+f.curGramSize])
			if f.termAttr != nil {
				f.termAttr.SetEmpty()
				f.termAttr.AppendString(gram)
			}
			if f.posIncrAttr != nil {
				if f.curPos == 0 && f.curGramSize == f.minGram {
					f.posIncrAttr.SetPositionIncrement(f.curPosIncr)
					f.curPosIncr = 0
				} else {
					f.posIncrAttr.SetPositionIncrement(0)
				}
			}
			f.curGramSize++
			return true, nil
		}
		// Move to next start position.
		f.curPos++
		f.curGramSize = f.minGram
		if f.curPos >= len(f.curRunes) {
			if f.preserveOriginal && !f.originalEmitted && len(f.curRunes) > f.maxGram {
				f.originalEmitted = true
				if f.termAttr != nil {
					f.termAttr.SetEmpty()
					f.termAttr.AppendString(string(f.curRunes))
				}
				if f.posIncrAttr != nil {
					f.posIncrAttr.SetPositionIncrement(0)
				}
				f.curRunes = nil
				return true, nil
			}
			f.curRunes = nil
		}
	}
}

// Reset clears generator state.
func (f *NGramTokenFilter) Reset() error {
	if r, ok := f.input.(interface{ Reset() error }); ok {
		if err := r.Reset(); err != nil {
			return err
		}
	}
	f.curRunes = nil
	f.curPosIncr = 0
	f.curPos = 0
	return nil
}

// End forwards End() and sets the trailing position increment.
func (f *NGramTokenFilter) End() error {
	if err := f.input.End(); err != nil {
		return err
	}
	if f.posIncrAttr != nil {
		f.posIncrAttr.SetPositionIncrement(f.curPosIncr)
	}
	return nil
}

// Ensure NGramTokenFilter implements TokenFilter.
var _ TokenFilter = (*NGramTokenFilter)(nil)

// NGramTokenizerFactory creates instances of the existing
// NGramTokenizer (defined elsewhere in the package). This factory is
// the Go port of
// org.apache.lucene.analysis.ngram.NGramTokenizerFactory.
type NGramTokenizerFactory struct {
	minGram int
	maxGram int
}

// NewNGramTokenizerFactory returns a factory with the upstream
// defaults (minGram=1, maxGram=2).
func NewNGramTokenizerFactory() *NGramTokenizerFactory {
	return &NGramTokenizerFactory{minGram: 1, maxGram: 2}
}

// NewNGramTokenizerFactoryWithConfig returns a configured factory.
func NewNGramTokenizerFactoryWithConfig(minGram, maxGram int) *NGramTokenizerFactory {
	return &NGramTokenizerFactory{minGram: minGram, maxGram: maxGram}
}

// MinGram returns the configured minimum gram size.
func (f *NGramTokenizerFactory) MinGram() int { return f.minGram }

// MaxGram returns the configured maximum gram size.
func (f *NGramTokenizerFactory) MaxGram() int { return f.maxGram }
