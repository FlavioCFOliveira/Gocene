// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package commongrams

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// CommonGramsQueryFilter wraps a CommonGramsFilter and optimises phrase
// queries by only returning single words when they are not a member of a
// bigram.
//
// Example:
//
//	input:  "the rain in spain falls mainly"
//	output: "the_rain", "rain_in", "in_spain", "falls", "mainly"
//
// Go port of org.apache.lucene.analysis.commongrams.CommonGramsQueryFilter
// (Apache Lucene 10.4.0).
type CommonGramsQueryFilter struct {
	*analysis.BaseTokenFilter

	typeAttr   analysis.TypeAttribute
	posIncAttr analysis.PositionIncrementAttribute
	posLenAttr analysis.PositionLengthAttribute

	previous     *util.AttributeState
	previousType string
	exhausted    bool
}

// NewCommonGramsQueryFilter creates a CommonGramsQueryFilter wrapping input.
func NewCommonGramsQueryFilter(input *CommonGramsFilter) *CommonGramsQueryFilter {
	f := &CommonGramsQueryFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
	}
	as := f.GetAttributeSource()
	if a := as.GetAttribute(analysis.TypeAttributeType); a != nil {
		f.typeAttr, _ = a.(analysis.TypeAttribute)
	}
	if a := as.GetAttribute(analysis.PositionIncrementAttributeType); a != nil {
		f.posIncAttr, _ = a.(analysis.PositionIncrementAttribute)
	}
	if a := as.GetAttribute(analysis.PositionLengthAttributeType); a != nil {
		f.posLenAttr, _ = a.(analysis.PositionLengthAttribute)
	}
	return f
}

// Reset resets the filter state and forwards to the underlying input.
func (f *CommonGramsQueryFilter) Reset() error {
	if resetter, ok := f.GetInput().(interface{ Reset() error }); ok {
		if err := resetter.Reset(); err != nil {
			return err
		}
	}
	f.previous = nil
	f.previousType = ""
	f.exhausted = false
	return nil
}

// IncrementToken outputs bigrams whenever possible and suppresses unigrams
// that are part of a bigram.
func (f *CommonGramsQueryFilter) IncrementToken() (bool, error) {
	as := f.GetAttributeSource()

	for !f.exhausted {
		ok, err := f.GetInput().IncrementToken()
		if err != nil {
			return false, err
		}
		if !ok {
			break
		}
		current := as.CaptureState()

		if f.previous != nil && !f.isGramType() {
			as.RestoreState(f.previous)
			f.previous = current
			f.previousType = f.currentType()

			if f.isGramType() {
				if f.posIncAttr != nil {
					f.posIncAttr.SetPositionIncrement(1)
				}
				// Reset position length to 1: the restored token (a gram) is
				// now being returned as a standalone token in the query stream.
				if f.posLenAttr != nil {
					f.posLenAttr.SetPositionLength(1)
				}
			}
			return true, nil
		}

		f.previous = current
	}

	f.exhausted = true

	if f.previous == nil || f.previousType == GramType {
		return false, nil
	}

	as.RestoreState(f.previous)
	f.previous = nil

	if f.isGramType() {
		if f.posIncAttr != nil {
			f.posIncAttr.SetPositionIncrement(1)
		}
		if f.posLenAttr != nil {
			f.posLenAttr.SetPositionLength(1)
		}
	}
	return true, nil
}

// IsGramType reports whether the current token type is GramType.
func (f *CommonGramsQueryFilter) IsGramType() bool {
	return f.isGramType()
}

// isGramType reports whether the current token type is GramType (unexported
// helper).
func (f *CommonGramsQueryFilter) isGramType() bool {
	if f.typeAttr == nil {
		return false
	}
	return f.typeAttr.GetType() == GramType
}

// currentType returns the current token type string.
func (f *CommonGramsQueryFilter) currentType() string {
	if f.typeAttr == nil {
		return ""
	}
	return f.typeAttr.GetType()
}

// Ensure CommonGramsQueryFilter implements TokenFilter.
var _ analysis.TokenFilter = (*CommonGramsQueryFilter)(nil)
