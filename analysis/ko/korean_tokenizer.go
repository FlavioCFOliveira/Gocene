// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package ko

import (
	"io"
	"unicode/utf8"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/ko/dict"
	"github.com/FlavioCFOliveira/Gocene/analysis/ko/tokenattributes"
)

// DecompoundMode controls how compound, inflected and pre-analysis tokens are
// handled.
//
// This is the Go port of
// org.apache.lucene.analysis.ko.KoreanTokenizer.DecompoundMode from Apache
// Lucene 10.4.0.
type DecompoundMode int

const (
	// DecompoundModeNone produces no decomposition for compound tokens.
	DecompoundModeNone DecompoundMode = iota
	// DecompoundModeDiscard decomposes compounds and discards the original
	// form (default).
	DecompoundModeDiscard
	// DecompoundModeMixed decomposes compounds and keeps the original form.
	DecompoundModeMixed
)

// DefaultDecompoundMode is the default decompound mode (DISCARD).
const DefaultDecompoundMode = DecompoundModeDiscard

// KoreanTokenizer tokenizes Korean text using rolling Viterbi morphological
// analysis.
//
// This is the Go port of org.apache.lucene.analysis.ko.KoreanTokenizer from
// Apache Lucene 10.4.0.
//
// Deviation: the Java implementation drives the Viterbi search incrementally
// from a streaming Reader. The Go port loads the full input in SetReader (a
// field value is bounded) and runs the Viterbi search lazily in
// IncrementToken.
type KoreanTokenizer struct {
	*analysis.BaseTokenizer

	viterbi *Viterbi

	termAttr    analysis.CharTermAttribute
	offsetAttr  analysis.OffsetAttribute
	posIncrAttr analysis.PositionIncrementAttribute
	posLenAttr  analysis.PositionLengthAttribute
	posAtt      tokenattributes.PartOfSpeechAttribute
	readingAtt  tokenattributes.ReadingAttribute

	exhausted bool
}

// NewKoreanTokenizer creates a KoreanTokenizer with default parameters.
//
// Uses the default system and unknown dictionaries shipped with Lucene and
// DecompoundModeDiscard.
func NewKoreanTokenizer() *KoreanTokenizer {
	return NewKoreanTokenizerWithOptions(nil, DefaultDecompoundMode, false, true)
}

// NewKoreanTokenizerWithOptions creates a KoreanTokenizer with custom options.
//
//   - userDict: optional user dictionary; may be nil.
//   - mode: decompound mode.
//   - outputUnknownUnigrams: if true outputs unigrams for unknown words.
//   - discardPunctuation: if true punctuation tokens are dropped.
func NewKoreanTokenizerWithOptions(
	userDict *dict.UserDictionary,
	mode DecompoundMode,
	outputUnknownUnigrams bool,
	discardPunctuation bool,
) *KoreanTokenizer {
	sysDict := dict.GetTokenInfoDictionaryInstance()
	unkDict := dict.GetUnknownDictionaryInstance()
	connCosts := dict.GetConnectionCostsInstance()
	charDef := dict.GetCharacterDefinitionInstance()
	v := NewViterbi(sysDict, unkDict, connCosts, userDict, charDef, discardPunctuation, mode, outputUnknownUnigrams)
	t := &KoreanTokenizer{
		BaseTokenizer: analysis.NewBaseTokenizer(),
		viterbi:       v,
	}
	t.wireAttributes()
	return t
}

// NewKoreanTokenizerFull creates a KoreanTokenizer with explicit dictionary
// components. This constructor is intended for custom dictionary builds.
func NewKoreanTokenizerFull(
	sysDict *dict.TokenInfoDictionary,
	unkDict *dict.UnknownDictionary,
	connCosts *dict.ConnectionCosts,
	userDict *dict.UserDictionary,
	mode DecompoundMode,
	outputUnknownUnigrams bool,
	discardPunctuation bool,
) *KoreanTokenizer {
	charDef := dict.GetCharacterDefinitionInstance()
	if unkDict != nil && unkDict.GetCharacterDefinition() != nil {
		charDef = unkDict.GetCharacterDefinition()
	}
	v := NewViterbi(sysDict, unkDict, connCosts, userDict, charDef, discardPunctuation, mode, outputUnknownUnigrams)
	t := &KoreanTokenizer{
		BaseTokenizer: analysis.NewBaseTokenizer(),
		viterbi:       v,
	}
	t.wireAttributes()
	return t
}

func (t *KoreanTokenizer) wireAttributes() {
	src := t.BaseTokenizer.GetAttributeSource()
	if src == nil {
		return
	}
	if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
		t.termAttr, _ = a.(analysis.CharTermAttribute)
	}
	if a := src.GetAttribute(analysis.OffsetAttributeType); a != nil {
		t.offsetAttr, _ = a.(analysis.OffsetAttribute)
	}
	if a := src.GetAttribute(analysis.PositionIncrementAttributeType); a != nil {
		t.posIncrAttr, _ = a.(analysis.PositionIncrementAttribute)
	}
	if a := src.GetAttribute(analysis.PositionLengthAttributeType); a != nil {
		t.posLenAttr, _ = a.(analysis.PositionLengthAttribute)
	}
	if a := src.GetAttribute(tokenattributes.PartOfSpeechAttributeType); a != nil {
		t.posAtt, _ = a.(tokenattributes.PartOfSpeechAttribute)
	}
	if a := src.GetAttribute(tokenattributes.ReadingAttributeType); a != nil {
		t.readingAtt, _ = a.(tokenattributes.ReadingAttribute)
	}
}

// SetReader sets the input reader and loads all characters for analysis.
func (t *KoreanTokenizer) SetReader(r io.Reader) error {
	if err := t.BaseTokenizer.SetReader(r); err != nil {
		return err
	}
	// Bound the read by analysis.MaxTokenizerInputSize so an oversized input
	// is rejected with analysis.ErrInputTooLarge rather than exhausting memory.
	// Read one byte past the cap to distinguish "at limit" from "over limit".
	data, err := io.ReadAll(io.LimitReader(r, analysis.MaxTokenizerInputSize+1))
	if err != nil {
		return err
	}
	if len(data) > analysis.MaxTokenizerInputSize {
		return analysis.ErrInputTooLarge
	}
	runes := make([]rune, 0, utf8.RuneCount(data))
	for len(data) > 0 {
		ch, size := utf8.DecodeRune(data)
		runes = append(runes, ch)
		data = data[size:]
	}
	t.viterbi.ResetBuffer(runes)
	t.viterbi.ResetState()
	t.exhausted = false
	return nil
}

// Reset resets the tokenizer state for re-use with a new SetReader call.
func (t *KoreanTokenizer) Reset() error {
	if err := t.BaseTokenizer.Reset(); err != nil {
		return err
	}
	t.exhausted = false
	return nil
}

// IncrementToken advances to the next token. Returns false when the stream is
// exhausted.
func (t *KoreanTokenizer) IncrementToken() (bool, error) {
	if t.exhausted {
		return false, nil
	}
	for len(t.viterbi.GetPending()) == 0 {
		if t.viterbi.IsEnd() {
			t.exhausted = true
			return false, nil
		}
		t.viterbi.Forward()
	}

	pending := t.viterbi.GetPending()
	token := pending[len(pending)-1]
	t.viterbi.SetPending(pending[:len(pending)-1])

	t.BaseTokenizer.ClearAttributes()
	if t.termAttr != nil {
		t.termAttr.SetValue(token.GetSurfaceFormString())
	}
	if t.offsetAttr != nil {
		t.offsetAttr.SetOffset(token.GetStartOffset(), token.GetEndOffset())
	}
	if t.posIncrAttr != nil {
		t.posIncrAttr.SetPositionIncrement(token.GetPositionIncrement())
	}
	if t.posLenAttr != nil {
		t.posLenAttr.SetPositionLength(token.GetPositionLength())
	}
	if t.posAtt != nil {
		// Token implements tokenWithPOS via GetPOSType/GetLeftPOS/GetRightPOS/GetMorphemes.
		t.posAtt.SetToken(token)
	}
	if t.readingAtt != nil {
		t.readingAtt.SetToken(token)
	}
	return true, nil
}

// End finalises the token stream, setting the final offset.
func (t *KoreanTokenizer) End() error {
	finalOffset := t.viterbi.GetPos()
	if t.offsetAttr != nil {
		t.offsetAttr.SetOffset(finalOffset, finalOffset)
	}
	return nil
}

// Ensure KoreanTokenizer satisfies TokenStream.
var _ analysis.TokenStream = (*KoreanTokenizer)(nil)
