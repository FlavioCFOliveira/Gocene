// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package compound

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Default size constants mirroring the Java originals.
const (
	// DefaultMinWordSize is the default minimum word length for decomposition.
	DefaultMinWordSize = 5
	// DefaultMinSubwordSize is the default minimum subword length.
	DefaultMinSubwordSize = 2
	// DefaultMaxSubwordSize is the default maximum subword length.
	DefaultMaxSubwordSize = 15
)

// CompoundToken holds a pending subword token produced during decomposition.
type CompoundToken struct {
	// Txt is the subword surface form.
	Txt string
	// StartOffset is the start offset of the original token.
	StartOffset int
	// EndOffset is the end offset of the original token.
	EndOffset int
}

// CompoundWordTokenFilterBase is the abstract base for compound-word
// decomposition token filters.
//
// This is the Go port of
// org.apache.lucene.analysis.compound.CompoundWordTokenFilterBase from
// Apache Lucene 10.4.0.
type CompoundWordTokenFilterBase struct {
	*analysis.BaseTokenFilter

	dictionary       *analysis.CharArraySet
	tokens           []*CompoundToken
	minWordSize      int
	minSubwordSize   int
	maxSubwordSize   int
	onlyLongestMatch bool

	termAttr    analysis.CharTermAttribute
	offsetAttr  analysis.OffsetAttribute
	posIncAttr  analysis.PositionIncrementAttribute

	current *util.AttributeState
}

// newCompoundWordTokenFilterBase creates a base filter with all parameters.
func newCompoundWordTokenFilterBase(
	input analysis.TokenStream,
	dictionary *analysis.CharArraySet,
	minWordSize, minSubwordSize, maxSubwordSize int,
	onlyLongestMatch bool,
) (*CompoundWordTokenFilterBase, error) {
	if minWordSize < 0 {
		return nil, fmt.Errorf("minWordSize cannot be negative")
	}
	if minSubwordSize < 0 {
		return nil, fmt.Errorf("minSubwordSize cannot be negative")
	}
	if maxSubwordSize < 0 {
		return nil, fmt.Errorf("maxSubwordSize cannot be negative")
	}
	f := &CompoundWordTokenFilterBase{
		BaseTokenFilter:  analysis.NewBaseTokenFilter(input),
		dictionary:       dictionary,
		minWordSize:      minWordSize,
		minSubwordSize:   minSubwordSize,
		maxSubwordSize:   maxSubwordSize,
		onlyLongestMatch: onlyLongestMatch,
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
			f.posIncAttr = a.(analysis.PositionIncrementAttribute)
		}
	}
	return f, nil
}

// IncrementToken advances to the next token. Sub-types must call this via
// their own IncrementToken after returning any buffered subtokens.
func (f *CompoundWordTokenFilterBase) IncrementToken(decompose func()) (bool, error) {
	if len(f.tokens) > 0 {
		tok := f.tokens[0]
		f.tokens = f.tokens[1:]
		if f.current != nil {
			f.GetAttributeSource().RestoreState(f.current)
		}
		if f.termAttr != nil {
			f.termAttr.SetEmpty()
			f.termAttr.AppendString(tok.Txt)
		}
		if f.offsetAttr != nil {
			f.offsetAttr.SetOffset(tok.StartOffset, tok.EndOffset)
		}
		if f.posIncAttr != nil {
			f.posIncAttr.SetPositionIncrement(0)
		}
		return true, nil
	}

	f.current = nil
	ok, err := f.GetInput().IncrementToken()
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}

	term := ""
	if f.termAttr != nil {
		term = f.termAttr.String()
	}
	if len([]rune(term)) >= f.minWordSize {
		decompose()
		if len(f.tokens) > 0 {
			f.current = f.GetAttributeSource().CaptureState()
		}
	}
	return true, nil
}

// AddToken adds a pending CompoundToken from the current term at rune slice [offset, offset+length).
func (f *CompoundWordTokenFilterBase) AddToken(runeOffset, runeLength int) {
	term := ""
	startOff := 0
	endOff := 0
	if f.termAttr != nil {
		runes := []rune(f.termAttr.String())
		if runeOffset+runeLength <= len(runes) {
			term = string(runes[runeOffset : runeOffset+runeLength])
		}
	}
	if f.offsetAttr != nil {
		startOff = f.offsetAttr.StartOffset()
		endOff = f.offsetAttr.EndOffset()
	}
	f.tokens = append(f.tokens, &CompoundToken{
		Txt:         term,
		StartOffset: startOff,
		EndOffset:   endOff,
	})
}

// Reset clears pending tokens.
func (f *CompoundWordTokenFilterBase) Reset() error {
	if r, ok := f.GetInput().(interface{ Reset() error }); ok {
		if err := r.Reset(); err != nil {
			return err
		}
	}
	f.tokens = f.tokens[:0]
	f.current = nil
	return nil
}

// TermRunes returns the current term as a rune slice.
func (f *CompoundWordTokenFilterBase) TermRunes() []rune {
	if f.termAttr == nil {
		return nil
	}
	return []rune(f.termAttr.String())
}

// DictionaryContains reports whether the dictionary contains the substring
// runes[start:start+length].
func (f *CompoundWordTokenFilterBase) DictionaryContains(runes []rune, start, length int) bool {
	if f.dictionary == nil {
		return false
	}
	return f.dictionary.Contains(runes, start, length)
}

// MinSubwordSize returns the configured minimum subword size.
func (f *CompoundWordTokenFilterBase) MinSubwordSize() int { return f.minSubwordSize }

// MaxSubwordSize returns the configured maximum subword size.
func (f *CompoundWordTokenFilterBase) MaxSubwordSize() int { return f.maxSubwordSize }

// OnlyLongestMatch reports whether only the longest match is used.
func (f *CompoundWordTokenFilterBase) OnlyLongestMatch() bool { return f.onlyLongestMatch }
