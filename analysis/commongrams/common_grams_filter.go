// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package commongrams provides CommonGramsFilter and CommonGramsQueryFilter,
// which construct bigrams for frequently occurring terms while indexing.
package commongrams

import (
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// GramType is the token type assigned to bigram tokens produced by
// CommonGramsFilter.
const GramType = "gram"

// gramSeparator is the character used to join two terms into a bigram.
const gramSeparator = '_'

// CommonGramsFilter constructs bigrams for frequently occurring terms while
// indexing. Single terms are still indexed too, with bigrams overlaid. This
// is achieved through the use of PositionIncrementAttribute. Bigrams have a
// type of GramType.
//
// Example:
//
//	input:  "the quick brown fox"
//	output: |"the","the_quick"|"brown"|"fox"|
//
// "the_quick" has a position increment of 0 so it is in the same position as
// "the". "the_quick" has a term.type() of "gram".
//
// Go port of org.apache.lucene.analysis.commongrams.CommonGramsFilter
// (Apache Lucene 10.4.0).
type CommonGramsFilter struct {
	*analysis.BaseTokenFilter

	commonWords *analysis.CharArraySet

	// buffer holds the left term plus the separator, ready to append the
	// right term to form a bigram.
	buffer strings.Builder

	// Attributes resolved once from the shared AttributeSource.
	termAttr   analysis.CharTermAttribute
	offsetAttr analysis.OffsetAttribute
	typeAttr   analysis.TypeAttribute
	posIncAttr analysis.PositionIncrementAttribute
	posLenAttr analysis.PositionLengthAttribute

	lastStartOffset int
	lastWasCommon   bool
	savedState      *util.AttributeState
}

// NewCommonGramsFilter creates a CommonGramsFilter wrapping input, producing
// bigrams for tokens that appear in commonWords.
func NewCommonGramsFilter(input analysis.TokenStream, commonWords *analysis.CharArraySet) *CommonGramsFilter {
	f := &CommonGramsFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
		commonWords:     commonWords,
	}
	// Resolve attributes eagerly so IncrementToken is allocation-free.
	as := f.GetAttributeSource()
	if a := as.GetAttribute(analysis.CharTermAttributeType); a != nil {
		f.termAttr, _ = a.(analysis.CharTermAttribute)
	}
	if a := as.GetAttribute(analysis.OffsetAttributeType); a != nil {
		f.offsetAttr, _ = a.(analysis.OffsetAttribute)
	}
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

// IncrementToken emits the next token. When the current token and/or the
// following token are in the common-words set, a bigram is emitted with
// position increment 0 and type=GramType.
func (f *CommonGramsFilter) IncrementToken() (bool, error) {
	if f.savedState != nil {
		f.GetAttributeSource().RestoreState(f.savedState)
		f.savedState = nil
		f.saveTermBuffer()
		return true, nil
	}

	ok, err := f.GetInput().IncrementToken()
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}

	if f.lastWasCommon || (f.isCommon() && f.buffer.Len() > 0) {
		f.savedState = f.GetAttributeSource().CaptureState()
		f.gramToken()
		return true, nil
	}

	f.saveTermBuffer()
	return true, nil
}

// Reset resets filter state and forwards to the underlying input.
func (f *CommonGramsFilter) Reset() error {
	if resetter, ok := f.GetInput().(interface{ Reset() error }); ok {
		if err := resetter.Reset(); err != nil {
			return err
		}
	}
	f.lastWasCommon = false
	f.savedState = nil
	f.buffer.Reset()
	return nil
}

// isCommon reports whether the current token is a member of the common-word
// set.
func (f *CommonGramsFilter) isCommon() bool {
	if f.commonWords == nil || f.termAttr == nil {
		return false
	}
	term := []rune(f.termAttr.String())
	return f.commonWords.Contains(term, 0, len(term))
}

// saveTermBuffer saves the current term as the left part of a potential
// bigram for use in a subsequent gramToken call.
func (f *CommonGramsFilter) saveTermBuffer() {
	f.buffer.Reset()
	if f.termAttr != nil {
		f.buffer.WriteString(f.termAttr.String())
	}
	f.buffer.WriteByte(gramSeparator)
	if f.offsetAttr != nil {
		f.lastStartOffset = f.offsetAttr.StartOffset()
	}
	f.lastWasCommon = f.isCommon()
}

// gramToken assembles a compound bigram token from the saved buffer (left
// term + separator) and the current term (right term).
func (f *CommonGramsFilter) gramToken() {
	if f.termAttr != nil {
		f.buffer.WriteString(f.termAttr.String())
	}
	endOffset := 0
	if f.offsetAttr != nil {
		endOffset = f.offsetAttr.EndOffset()
	}

	f.GetAttributeSource().ClearAttributes()

	gram := f.buffer.String()
	if f.termAttr != nil {
		f.termAttr.SetValue(gram)
	}
	if f.posIncAttr != nil {
		f.posIncAttr.SetPositionIncrement(0)
	}
	if f.posLenAttr != nil {
		f.posLenAttr.SetPositionLength(2) // bigram spans two positions
	}
	if f.offsetAttr != nil {
		f.offsetAttr.SetOffset(f.lastStartOffset, endOffset)
	}
	if f.typeAttr != nil {
		f.typeAttr.SetType(GramType)
	}
	f.buffer.Reset()
}

// Ensure CommonGramsFilter implements TokenFilter.
var _ analysis.TokenFilter = (*CommonGramsFilter)(nil)
