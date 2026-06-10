// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package kuromoji

import (
	"io"
	"unicode/utf8"

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

	// viterbiCore is the concrete Viterbi decoder used for morphological
	// analysis. It is injected by NewJapaneseTokenizerWithDefaults or set
	// directly for tests.
	viterbiCore *Viterbi
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

	// Create and register standard attributes.
	t.termAttr = analysis.NewCharTermAttribute()
	t.offsetAttr = analysis.NewOffsetAttribute()
	t.posIncrAttr = analysis.NewPositionIncrementAttribute()
	t.posLenAttr = analysis.NewPositionLengthAttribute()

	t.AddAttribute(t.termAttr)
	t.AddAttribute(t.offsetAttr)
	t.AddAttribute(t.posIncrAttr)
	t.AddAttribute(t.posLenAttr)

	// Create and register Japanese-specific attributes.
	baseFormImpl := tokenattributes.NewBaseFormAttributeImpl()
	posImpl := tokenattributes.NewPartOfSpeechAttributeImpl()
	readingImpl := tokenattributes.NewReadingAttributeImpl()
	inflImpl := tokenattributes.NewInflectionAttributeImpl()

	t.baseFormAttr = baseFormImpl
	t.posAttr = posImpl
	t.readingAttr = readingImpl
	t.inflAttr = inflImpl

	t.AddAttribute(baseFormImpl)
	t.AddAttribute(posImpl)
	t.AddAttribute(readingImpl)
	t.AddAttribute(inflImpl)

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
func (t *JapaneseTokenizer) IncrementToken() (bool, error) {
	if t.exhausted {
		return false, nil
	}

	// If there are no pending tokens, run Viterbi over the full input.
	if len(t.pending) == 0 {
		if t.viterbiCore == nil {
			return false, nil
		}
		if err := t.fillPending(); err != nil {
			return false, err
		}
		if len(t.pending) == 0 {
			t.exhausted = true
			return false, nil
		}
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

// fillPending reads the full input, runs the Viterbi decoder, and populates
// t.pending with the resulting tokens (in reverse order so they can be popped
// from the tail).
func (t *JapaneseTokenizer) fillPending() error {
	reader := t.GetReader()
	if reader == nil {
		return nil
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}

	// Decode UTF-8 to runes, preserving offsets.
	input := make([]rune, 0, len(data))
	for len(data) > 0 {
		r, size := utf8.DecodeRune(data)
		input = append(input, r)
		data = data[size:]
	}

	t.viterbiCore.ResetBuffer(input)
	t.viterbiCore.ResetState()
	for !t.viterbiCore.IsEnd() {
		t.viterbiCore.Forward()
	}

	pending := t.viterbiCore.GetPending()
	if len(pending) == 0 {
		return nil
	}

	// Reverse pending so we can pop from the tail in IncrementToken.
	for i, j := 0, len(pending)-1; i < j; i, j = i+1, j-1 {
		pending[i], pending[j] = pending[j], pending[i]
	}
	t.pending = pending
	return nil
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

// SetViterbiCore injects a concrete Viterbi decoder into the tokenizer.
func (t *JapaneseTokenizer) SetViterbiCore(v *Viterbi) { t.viterbiCore = v }

// SetPending injects a pre-computed token list (used for testing and
// resource-backed construction).
func (t *JapaneseTokenizer) SetPending(tokens []*dict.Token) { t.pending = tokens }

// NewJapaneseTokenizerWithDefaults creates a JapaneseTokenizer backed by the
// embedded binary dictionary resources.
func NewJapaneseTokenizerWithDefaults(
	userDictionary *dict.UserDictionary,
	discardPunctuation bool,
	discardCompoundToken bool,
	mode Mode,
) *JapaneseTokenizer {
	t := NewJapaneseTokenizer(userDictionary, discardPunctuation, discardCompoundToken, mode)

	sysDict := dict.GetTokenInfoDictionaryInstance()
	unkDict := dict.GetUnknownDictionaryInstance()
	connCosts := dict.GetConnectionCostsInstance()
	charDef := dict.GetCharacterDefinitionInstance()

	searchMode := mode == ModeSearch || mode == ModeExtended
	extendedMode := mode == ModeExtended

	v := NewViterbi(
		sysDict,
		unkDict,
		connCosts,
		userDictionary,
		charDef,
		discardPunctuation,
		searchMode,
		extendedMode,
		!discardCompoundToken,
	)
	t.viterbiCore = v
	return t
}

// Mode returns the tokenization mode.
func (t *JapaneseTokenizer) Mode() Mode { return t.mode }

// Ensure JapaneseTokenizer implements analysis.Tokenizer.
var _ analysis.Tokenizer = (*JapaneseTokenizer)(nil)
