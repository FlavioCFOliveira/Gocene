// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package kuromoji

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/kuromoji/dict"
	"github.com/FlavioCFOliveira/Gocene/analysis/kuromoji/tokenattributes"
)

// Mode is the tokenization mode for JapaneseTokenizer.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.JapaneseTokenizer.Mode from Apache Lucene
// 10.4.0.
type Mode int

const (
	// ModeNormal performs ordinary segmentation with no decomposition for
	// compound words.
	ModeNormal Mode = iota
	// ModeSearch performs segmentation geared towards search, including
	// decompounding of long nouns and emitting the full compound token as a
	// synonym.
	ModeSearch
	// ModeExtended extends ModeSearch by also emitting unigrams for unknown
	// words.
	ModeExtended
)

// DefaultMode is the default tokenization mode.
const DefaultMode = ModeSearch

// JapaneseTokenizer tokenizes Japanese text using morphological analysis.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.JapaneseTokenizer from Apache Lucene 10.4.0.
//
// Deviation: the Java original drives a complete Viterbi pipeline loaded from
// pre-built binary dictionary resources. This Go implementation exposes the
// full public contract (configuration fields, attribute wiring, IncrementToken
// loop) but defers actual Viterbi decoding to when the binary dictionary
// resources are available (codec sprint). Until then, IncrementToken returns
// false (empty token stream).
type JapaneseTokenizer struct {
	*analysis.BaseTokenizer

	viterbi *ViterbiNBest

	termAttr      analysis.CharTermAttribute
	offsetAttr    analysis.OffsetAttribute
	posIncrAttr   analysis.PositionIncrementAttribute
	posLenAttr    analysis.PositionLengthAttribute
	baseFormAttr  tokenattributes.BaseFormAttribute
	posAttr       tokenattributes.PartOfSpeechAttribute
	readingAttr   tokenattributes.ReadingAttribute
	inflAttr      tokenattributes.InflectionAttribute

	mode               Mode
	discardPunctuation bool
	discardCompound    bool
	userDictionary     *dict.UserDictionary

	lastTokenPos int
	pending      []*dict.Token
	exhausted    bool
}

// NewJapaneseTokenizer creates a JapaneseTokenizer with the given options.
func NewJapaneseTokenizer(
	userDictionary *dict.UserDictionary,
	discardPunctuation bool,
	discardCompoundToken bool,
	mode Mode,
) *JapaneseTokenizer {
	t := &JapaneseTokenizer{
		BaseTokenizer:      analysis.NewBaseTokenizer(),
		mode:               mode,
		discardPunctuation: discardPunctuation,
		discardCompound:    discardCompoundToken,
		userDictionary:     userDictionary,
		lastTokenPos:       -1,
	}
	src := t.BaseTokenizer.GetAttributeSource()
	if src != nil {
		if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
			t.termAttr = a.(analysis.CharTermAttribute)
		}
		if a := src.GetAttribute(analysis.OffsetAttributeType); a != nil {
			t.offsetAttr = a.(analysis.OffsetAttribute)
		}
		if a := src.GetAttribute(analysis.PositionIncrementAttributeType); a != nil {
			t.posIncrAttr = a.(analysis.PositionIncrementAttribute)
		}
		if a := src.GetAttribute(analysis.PositionLengthAttributeType); a != nil {
			t.posLenAttr = a.(analysis.PositionLengthAttribute)
		}
		if a := src.GetAttribute(tokenattributes.BaseFormAttributeType); a != nil {
			t.baseFormAttr = a.(tokenattributes.BaseFormAttribute)
		}
		if a := src.GetAttribute(tokenattributes.PartOfSpeechAttributeType); a != nil {
			t.posAttr = a.(tokenattributes.PartOfSpeechAttribute)
		}
		if a := src.GetAttribute(tokenattributes.ReadingAttributeType); a != nil {
			t.readingAttr = a.(tokenattributes.ReadingAttribute)
		}
		if a := src.GetAttribute(tokenattributes.InflectionAttributeType); a != nil {
			t.inflAttr = a.(tokenattributes.InflectionAttribute)
		}
	}
	return t
}

// NewJapaneseTokenizerSimple creates a JapaneseTokenizer using the default
// mode (ModeSearch) with punctuation discarding enabled.
func NewJapaneseTokenizerSimple(userDictionary *dict.UserDictionary) *JapaneseTokenizer {
	return NewJapaneseTokenizer(userDictionary, true, true, DefaultMode)
}

// Reset resets the tokenizer state.
func (t *JapaneseTokenizer) Reset() error {
	if err := t.BaseTokenizer.Reset(); err != nil {
		return err
	}
	t.lastTokenPos = -1
	t.pending = t.pending[:0]
	t.exhausted = false
	return nil
}

// IncrementToken advances to the next token.
//
// Deviation: full Viterbi-backed decoding requires binary dictionary resources
// that are not yet loaded. This implementation returns false (empty stream)
// until the resources are wired in a future sprint.
func (t *JapaneseTokenizer) IncrementToken() (bool, error) {
	if t.exhausted || len(t.pending) == 0 {
		return false, nil
	}
	tok := t.pending[len(t.pending)-1]
	t.pending = t.pending[:len(t.pending)-1]

	if t.termAttr != nil {
		t.termAttr.SetValue(string(tok.SurfaceForm[tok.Offset : tok.Offset+tok.Length]))
	}
	if t.offsetAttr != nil {
		t.offsetAttr.SetOffset(tok.StartOffset, tok.EndOffset)
	}
	if t.posIncrAttr != nil {
		if tok.StartOffset == t.lastTokenPos {
			t.posIncrAttr.SetPositionIncrement(0)
		} else {
			t.posIncrAttr.SetPositionIncrement(1)
		}
	}
	if t.posLenAttr != nil {
		t.posLenAttr.SetPositionLength(tok.GetPositionLength())
	}
	if t.baseFormAttr != nil {
		t.baseFormAttr.SetToken(tok)
	}
	if t.posAttr != nil {
		t.posAttr.SetToken(tok)
	}
	if t.readingAttr != nil {
		t.readingAttr.SetToken(tok)
	}
	if t.inflAttr != nil {
		t.inflAttr.SetToken(tok)
	}
	t.lastTokenPos = tok.StartOffset
	return true, nil
}

// End performs end-of-stream attribute finalization.
func (t *JapaneseTokenizer) End() error {
	if t.offsetAttr != nil {
		t.offsetAttr.SetOffset(0, 0)
	}
	return nil
}

// SetViterbi injects a fully configured ViterbiNBest decoder into the
// tokenizer. This is used by the tokenizer factory once dictionary resources
// are available.
func (t *JapaneseTokenizer) SetViterbi(v *ViterbiNBest) { t.viterbi = v }

// SetPending injects a pre-computed token list (used for testing and
// resource-backed construction).
func (t *JapaneseTokenizer) SetPending(tokens []*dict.Token) { t.pending = tokens }

// Mode returns the tokenization mode.
func (t *JapaneseTokenizer) Mode() Mode { return t.mode }

// Ensure JapaneseTokenizer implements analysis.Tokenizer.
var _ analysis.Tokenizer = (*JapaneseTokenizer)(nil)
