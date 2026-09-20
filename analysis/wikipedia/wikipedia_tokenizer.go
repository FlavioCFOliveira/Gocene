// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package wikipedia

import (
	"io"
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/tokenattributes"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Output mode constants — mirror WikipediaTokenizer.TOKENS_ONLY etc.
const (
	// TokensOnly outputs individual tokens.
	TokensOnly = 0
	// UntokenizedOnly outputs collapsed untokenized spans.
	UntokenizedOnly = 1
	// Both outputs both forms.
	Both = 2
)

// WikipediaTokenizer is a Tokenizer that understands Wikipedia markup syntax.
// It is backed by WikipediaTokenizerImpl (a hand-written state machine that
// follows the JFlex grammar from Apache Lucene 10.4.0).
//
// Go port of org.apache.lucene.analysis.wikipedia.WikipediaTokenizer
// (Apache Lucene 10.4.0).
type WikipediaTokenizer struct {
	*analysis.BaseTokenizer

	scanner     *WikipediaTokenizerImpl
	tokenOutput int

	termAttr    analysis.CharTermAttribute
	offsetAttr  analysis.OffsetAttribute
	posIncrAttr tokenattributes.PositionIncrementAttribute
	typeAttr    analysis.TypeAttribute

	first bool
}

// NewWikipediaTokenizerWithMode creates a WikipediaTokenizer with the given
// output mode (TokensOnly, UntokenizedOnly, or Both) and the supplied attribute factory.
func NewWikipediaTokenizerWithMode(factory util.AttributeFactory, tokenOutput int) *WikipediaTokenizer {
	t := &WikipediaTokenizer{
		BaseTokenizer: analysis.NewBaseTokenizerWithFactory(factory),
		scanner:       NewWikipediaTokenizerImpl(nil),
		tokenOutput:   tokenOutput,
		first:         true,
	}

	t.termAttr = factory.CreateAttributeInstance(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	t.offsetAttr = factory.CreateAttributeInstance(analysis.OffsetAttributeType).(analysis.OffsetAttribute)
	t.posIncrAttr = factory.CreateAttributeInstance(tokenattributes.PositionIncrementAttributeType).(tokenattributes.PositionIncrementAttribute)
	t.typeAttr = factory.CreateAttributeInstance(analysis.TypeAttributeType).(analysis.TypeAttribute)

	t.AddAttribute(reflect.TypeOf((*analysis.CharTermAttribute)(nil)).Elem())
	t.AddAttribute(reflect.TypeOf((*analysis.OffsetAttribute)(nil)).Elem())
	t.AddAttribute(reflect.TypeOf((*tokenattributes.PositionIncrementAttribute)(nil)).Elem())
	t.AddAttribute(reflect.TypeOf((*analysis.TypeAttribute)(nil)).Elem())

	return t
}

// NewWikipediaTokenizer creates a WikipediaTokenizer with TokensOnly output
// and default attribute factory.
func NewWikipediaTokenizer() *WikipediaTokenizer {
	return NewWikipediaTokenizerWithMode(util.DefaultAttributeFactoryInstance, TokensOnly)
}

// SetReader attaches a new input reader to this tokenizer.
func (t *WikipediaTokenizer) SetReader(r io.Reader) {
	t.BaseTokenizer.SetReader(r)
	t.scanner.Reset(r)
}

// Reset resets the tokenizer for a new tokenization session.
func (t *WikipediaTokenizer) Reset() error {
	if err := t.BaseTokenizer.Reset(); err != nil {
		return err
	}
	if t.GetReader() != nil {
		t.scanner.Reset(t.GetReader())
	}
	t.first = true
	return nil
}

// IncrementToken advances to the next token.
func (t *WikipediaTokenizer) IncrementToken() (bool, error) {
	t.ClearAttributes()

	tokType := t.scanner.GetNextToken()
	if tokType == YYEOF {
		return false, nil
	}

	text := t.scanner.GetText()
	t.termAttr.SetValue(text)

	start := t.scanner.YYChar()
	end := start + len([]rune(text))
	t.offsetAttr.SetStartOffset(start)
	t.offsetAttr.SetEndOffset(end)

	posinc := t.scanner.GetPositionIncrement()
	if t.first && posinc == 0 {
		posinc = 1
	}
	t.posIncrAttr.SetPositionIncrement(posinc)
	t.first = false

	if tokType >= 0 && tokType < len(TokenTypes) {
		t.typeAttr.SetType(TokenTypes[tokType])
	}
	return true, nil
}

// End finalises the offset after tokenization is complete.
func (t *WikipediaTokenizer) End() error {
	finalOffset := t.scanner.YYChar() + t.scanner.YYLength()
	t.offsetAttr.SetStartOffset(finalOffset)
	t.offsetAttr.SetEndOffset(finalOffset)
	return nil
}

// Ensure WikipediaTokenizer implements analysis.Tokenizer.
var _ analysis.Tokenizer = (*WikipediaTokenizer)(nil)

// ──────────────────────────────────────────────────────────────────────────────
// WikipediaTokenizerFactory
// ──────────────────────────────────────────────────────────────────────────────

// WikipediaTokenizerFactory creates WikipediaTokenizer instances.
//
// Go port of org.apache.lucene.analysis.wikipedia.WikipediaTokenizerFactory
// (Apache Lucene 10.4.0).
type WikipediaTokenizerFactory struct{}

// NewWikipediaTokenizerFactory creates a new factory.
func NewWikipediaTokenizerFactory() *WikipediaTokenizerFactory {
	return &WikipediaTokenizerFactory{}
}

// Create creates a new WikipediaTokenizer.
func (f *WikipediaTokenizerFactory) Create(factory util.AttributeFactory) analysis.Tokenizer {
	return NewWikipediaTokenizerWithMode(factory, TokensOnly)
}

// Ensure WikipediaTokenizerFactory implements analysis.TokenizerFactory.
var _ analysis.TokenizerFactory = (*WikipediaTokenizerFactory)(nil)
