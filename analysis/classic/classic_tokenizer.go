// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package classic

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/tokenattributes"
	"github.com/FlavioCFOliveira/Gocene/util"
	"io"
	"reflect"
)

// defaultMaxTokenLength mirrors StandardAnalyzer.DEFAULT_MAX_TOKEN_LENGTH.
const defaultMaxTokenLength = 255

// ClassicTokenizer is the classic Lucene tokenizer (pre-3.1 StandardTokenizer).
//
// This is the Go port of
// org.apache.lucene.analysis.classic.ClassicTokenizer from
// Apache Lucene 10.4.0.
type ClassicTokenizer struct {
	*analysis.BaseTokenizer

	scanner        *ClassicTokenizerImpl
	maxTokenLength int

	termAttr   analysis.CharTermAttribute
	offsetAttr analysis.OffsetAttribute
	typeAttr   analysis.TypeAttribute
	posIncAttr tokenattributes.PositionIncrementAttribute

	skippedPositions int
}

// NewClassicTokenizerWithFactory creates a ClassicTokenizer using the supplied
// attribute factory.
func NewClassicTokenizerWithFactory(factory util.AttributeFactory) *ClassicTokenizer {
	t := &ClassicTokenizer{
		BaseTokenizer:  analysis.NewBaseTokenizerWithFactory(factory),
		maxTokenLength: defaultMaxTokenLength,
	}
	// Register attribute implementations
	termImpl := factory.CreateAttributeInstance(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	offsetImpl := factory.CreateAttributeInstance(analysis.OffsetAttributeType).(analysis.OffsetAttribute)
	posIncImpl := factory.CreateAttributeInstance(tokenattributes.PositionIncrementAttributeType).(tokenattributes.PositionIncrementAttribute)
	typeImpl := factory.CreateAttributeInstance(analysis.TypeAttributeType).(analysis.TypeAttribute)
	t.AddAttribute(reflect.TypeOf((*analysis.CharTermAttribute)(nil)).Elem())
	t.AddAttribute(reflect.TypeOf((*analysis.OffsetAttribute)(nil)).Elem())
	t.AddAttribute(reflect.TypeOf((*tokenattributes.PositionIncrementAttribute)(nil)).Elem())
	t.AddAttribute(reflect.TypeOf((*analysis.TypeAttribute)(nil)).Elem())
	t.termAttr = termImpl
	t.offsetAttr = offsetImpl
	t.posIncAttr = posIncImpl
	t.typeAttr = typeImpl
	return t
}

// NewClassicTokenizer creates a ClassicTokenizer with default max token length
// and default attribute factory.
func NewClassicTokenizer() *ClassicTokenizer {
	return NewClassicTokenizerWithFactory(util.DefaultAttributeFactoryInstance)
}

// SetMaxTokenLength sets the maximum token length. Tokens longer than this are
// discarded.
func (t *ClassicTokenizer) SetMaxTokenLength(length int) {
	if length < 1 {
		panic("maxTokenLength must be greater than zero")
	}
	t.maxTokenLength = length
}

// GetMaxTokenLength returns the current maximum token length.
func (t *ClassicTokenizer) GetMaxTokenLength() int { return t.maxTokenLength }

// SetReader sets the input reader and initialises the scanner. The scanner
// reads the whole input eagerly (bounded by analysis.MaxTokenizerInputSize);
// an oversized or unreadable input surfaces as an error here rather than as a
// silently truncated token stream.
func (t *ClassicTokenizer) SetReader(r io.Reader) {
	t.BaseTokenizer.SetReader(r)
	t.scanner = NewClassicTokenizerImpl(r)
	t.skippedPositions = 0
}

// Reset reinitialises the scanner over the stored reader.
//
// ClassicTokenizerImpl eagerly reads all input on construction (io.ReadAll);
// Reset therefore rewinds the scanner's token index rather than creating a
// new one, matching the semantics of the Java JFlex-generated impl.
func (t *ClassicTokenizer) Reset() error {
	if err := t.BaseTokenizer.Reset(); err != nil {
		return err
	}
	r := t.GetReader()
	if t.scanner != nil {
		// Re-scan from the stored reader only if we have a fresh one
		// (scanner already consumed the reader; reset to beginning of tokens).
		t.scanner.ResetIndex()
	} else if r != nil {
		t.scanner = NewClassicTokenizerImpl(r)
		if err := t.scanner.Err(); err != nil {
			return err
		}
	}
	t.skippedPositions = 0
	return nil
}

// IncrementToken advances to the next token.
func (t *ClassicTokenizer) IncrementToken() (bool, error) {
	if t.scanner == nil {
		return false, nil
	}
	t.ClearAttributes()
	t.skippedPositions = 0
	for {
		tok := t.scanner.GetNextToken()
		if tok == nil {
			if t.posIncAttr != nil {
				t.posIncAttr.SetPositionIncrement(t.posIncAttr.GetPositionIncrement() + t.skippedPositions)
			}
			return false, nil
		}
		if len([]rune(tok.text)) > t.maxTokenLength {
			t.skippedPositions++
			continue
		}
		if t.termAttr != nil {
			t.termAttr.SetEmpty()
			t.termAttr.AppendString(tok.text)
		}
		if t.offsetAttr != nil {
			t.offsetAttr.SetOffset(tok.startOff, tok.endOff)
		}
		if t.typeAttr != nil && tok.tokenType < len(TokenTypes) {
			t.typeAttr.SetType(TokenTypes[tok.tokenType])
		}
		if t.posIncAttr != nil {
			t.posIncAttr.SetPositionIncrement(1 + t.skippedPositions)
		}
		t.skippedPositions = 0
		return true, nil
	}
}

// End performs end-of-stream processing.
func (t *ClassicTokenizer) End() error {
	if t.posIncAttr != nil {
		t.posIncAttr.SetPositionIncrement(t.posIncAttr.GetPositionIncrement() + t.skippedPositions)
	}
	return nil
}

// Ensure ClassicTokenizer implements Tokenizer.
var _ analysis.Tokenizer = (*ClassicTokenizer)(nil)

// ClassicTokenizerFactory creates ClassicTokenizer instances.
//
// This is the Go port of
// org.apache.lucene.analysis.classic.ClassicTokenizerFactory from
// Apache Lucene 10.4.0.
type ClassicTokenizerFactory struct {
	maxTokenLength int
}

// NewClassicTokenizerFactory creates a factory with default max token length.
func NewClassicTokenizerFactory() *ClassicTokenizerFactory {
	return &ClassicTokenizerFactory{maxTokenLength: defaultMaxTokenLength}
}

// NewClassicTokenizerFactoryWithLength creates a factory with a custom max
// token length.
func NewClassicTokenizerFactoryWithLength(maxTokenLength int) *ClassicTokenizerFactory {
	return &ClassicTokenizerFactory{maxTokenLength: maxTokenLength}
}

// Create creates a new ClassicTokenizer.
func (f *ClassicTokenizerFactory) Create(factory util.AttributeFactory) analysis.Tokenizer {
	t := NewClassicTokenizerWithFactory(factory)
	t.SetMaxTokenLength(f.maxTokenLength)
	return t
}

// Ensure factory implements TokenizerFactory.
var _ analysis.TokenizerFactory = (*ClassicTokenizerFactory)(nil)
