// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package shingle

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// maxShingleSize is the maximum shingle size supported by FixedShingleFilter.
const maxShingleSize = 4

// FixedShingleFilter constructs shingles (token n-grams) of a fixed size from
// a token stream. Unlike ShingleFilter, it only emits shingles (never unigrams)
// and handles stacked tokens (e.g. synonyms) by outputting stacked shingles.
//
// This is the Go port of
// org.apache.lucene.analysis.shingle.FixedShingleFilter from
// Apache Lucene 10.4.0.
type FixedShingleFilter struct {
	*analysis.GraphTokenFilter

	shingleSize    int
	tokenSeparator string
	fillerToken    string

	termAttr   analysis.CharTermAttribute
	offsetAttr analysis.OffsetAttribute
	incAttr    analysis.PositionIncrementAttribute
	typeAttr   analysis.TypeAttribute
	buffer     strings.Builder
}

// NewFixedShingleFilter creates a FixedShingleFilter with the given shingle
// size and default separators (" " and "_").
func NewFixedShingleFilter(input analysis.TokenStream, shingleSize int) (*FixedShingleFilter, error) {
	return NewFixedShingleFilterFull(input, shingleSize, " ", "_")
}

// NewFixedShingleFilterFull creates a FixedShingleFilter with all parameters.
func NewFixedShingleFilterFull(
	input analysis.TokenStream,
	shingleSize int,
	tokenSeparator, fillerToken string,
) (*FixedShingleFilter, error) {
	if shingleSize <= 1 || shingleSize > maxShingleSize {
		return nil, fmt.Errorf("shingle size must be between 2 and %d, got %d",
			maxShingleSize, shingleSize)
	}
	f := &FixedShingleFilter{
		GraphTokenFilter: analysis.NewGraphTokenFilter(input),
		shingleSize:      shingleSize,
		tokenSeparator:   tokenSeparator,
		fillerToken:      fillerToken,
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
			f.termAttr = a.(analysis.CharTermAttribute)
		}
		if a := src.GetAttribute(analysis.OffsetAttributeType); a != nil {
			f.offsetAttr = a.(analysis.OffsetAttribute)
		}
		if a := src.GetAttribute(analysis.PositionIncrementAttributeType); a != nil {
			f.incAttr = a.(analysis.PositionIncrementAttribute)
		}
		if a := src.GetAttribute(analysis.TypeAttributeType); a != nil {
			f.typeAttr = a.(analysis.TypeAttribute)
		}
	}
	return f, nil
}

// IncrementToken advances to the next shingle token.
func (f *FixedShingleFilter) IncrementToken() (bool, error) {
	var shinglePosInc, startOffset, endOffset int

outer:
	for {
		ok, err := f.IncrementGraph()
		if err != nil {
			return false, err
		}
		if !ok {
			ok2, err2 := f.IncrementBaseToken()
			if err2 != nil {
				return false, err2
			}
			if !ok2 {
				return false, nil
			}
			if f.incAttr != nil {
				shinglePosInc = f.incAttr.GetPositionIncrement()
			} else {
				shinglePosInc = 1
			}
		} else {
			shinglePosInc = 0
		}

		if f.offsetAttr != nil {
			startOffset = f.offsetAttr.StartOffset()
			endOffset = f.offsetAttr.EndOffset()
		}
		f.buffer.Reset()
		if f.termAttr != nil {
			f.buffer.WriteString(f.termAttr.String())
		}

		for i := 1; i < f.shingleSize; i++ {
			tok, err := f.IncrementGraphToken()
			if err != nil {
				return false, err
			}
			if !tok {
				trailing := f.GetTrailingPositions()
				if i+trailing < f.shingleSize {
					continue outer
				}
				for i < f.shingleSize {
					f.buffer.WriteString(f.tokenSeparator)
					f.buffer.WriteString(f.fillerToken)
					i++
				}
				break
			}
			posInc := 1
			if f.incAttr != nil {
				posInc = f.incAttr.GetPositionIncrement()
			}
			if posInc > 1 {
				if i+posInc > f.shingleSize {
					for i < f.shingleSize {
						f.buffer.WriteString(f.tokenSeparator)
						f.buffer.WriteString(f.fillerToken)
						i++
					}
					break
				}
				for posInc > 1 {
					f.buffer.WriteString(f.tokenSeparator)
					f.buffer.WriteString(f.fillerToken)
					posInc--
					i++
				}
			}
			if f.termAttr != nil {
				f.buffer.WriteString(f.tokenSeparator)
				f.buffer.WriteString(f.termAttr.String())
			}
			if f.offsetAttr != nil {
				endOffset = f.offsetAttr.EndOffset()
			}
		}
		break
	}

	f.ClearAttributes()
	if f.offsetAttr != nil {
		f.offsetAttr.SetOffset(startOffset, endOffset)
	}
	if f.incAttr != nil {
		f.incAttr.SetPositionIncrement(shinglePosInc)
	}
	if f.termAttr != nil {
		f.termAttr.SetEmpty()
		f.termAttr.AppendString(f.buffer.String())
	}
	if f.typeAttr != nil {
		f.typeAttr.SetType("shingle")
	}
	return true, nil
}

// Ensure FixedShingleFilter implements TokenFilter.
var _ analysis.TokenFilter = (*FixedShingleFilter)(nil)

// FixedShingleFilterFactory creates FixedShingleFilter instances.
//
// This is the Go port of
// org.apache.lucene.analysis.shingle.FixedShingleFilterFactory from
// Apache Lucene 10.4.0.
type FixedShingleFilterFactory struct {
	shingleSize    int
	tokenSeparator string
	fillerToken    string
}

// NewFixedShingleFilterFactory creates a factory with default parameters
// (shingleSize=2, separator=" ", filler="_").
func NewFixedShingleFilterFactory() *FixedShingleFilterFactory {
	return &FixedShingleFilterFactory{
		shingleSize:    2,
		tokenSeparator: " ",
		fillerToken:    "_",
	}
}

// NewFixedShingleFilterFactoryFull creates a factory with all parameters.
func NewFixedShingleFilterFactoryFull(
	shingleSize int,
	tokenSeparator, fillerToken string,
) *FixedShingleFilterFactory {
	return &FixedShingleFilterFactory{
		shingleSize:    shingleSize,
		tokenSeparator: tokenSeparator,
		fillerToken:    fillerToken,
	}
}

// Create creates a FixedShingleFilter wrapping input.
func (f *FixedShingleFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	filter, err := NewFixedShingleFilterFull(input, f.shingleSize, f.tokenSeparator, f.fillerToken)
	if err != nil {
		panic(fmt.Sprintf("shingle: FixedShingleFilterFactory.Create: %v", err))
	}
	return filter
}

// Ensure factory implements TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*FixedShingleFilterFactory)(nil)
