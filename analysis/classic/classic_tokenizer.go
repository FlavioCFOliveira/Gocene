// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package classic

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/analysis"
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

	scanner       *ClassicTokenizerImpl
	maxTokenLength int

	termAttr   analysis.CharTermAttribute
	offsetAttr analysis.OffsetAttribute
	typeAttr   analysis.TypeAttribute
	posIncAttr analysis.PositionIncrementAttribute

	skippedPositions int
}

// NewClassicTokenizer creates a ClassicTokenizer with default max token length.
func NewClassicTokenizer() *ClassicTokenizer {
	t := &ClassicTokenizer{
		BaseTokenizer:  analysis.NewBaseTokenizer(),
		maxTokenLength: defaultMaxTokenLength,
	}
	src := t.GetAttributeSource()
	if src != nil {
		if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
			t.termAttr = a.(analysis.CharTermAttribute)
		}
		if a := src.GetAttribute(analysis.OffsetAttributeType); a != nil {
			t.offsetAttr = a.(analysis.OffsetAttribute)
		}
		if a := src.GetAttribute(analysis.TypeAttributeType); a != nil {
			t.typeAttr = a.(analysis.TypeAttribute)
		}
		if a := src.GetAttribute(analysis.PositionIncrementAttributeType); a != nil {
			t.posIncAttr = a.(analysis.PositionIncrementAttribute)
		}
	}
	return t
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

// SetReader sets the input reader and initialises the scanner.
func (t *ClassicTokenizer) SetReader(r io.Reader) error {
	if err := t.BaseTokenizer.SetReader(r); err != nil {
		return err
	}
	t.scanner = NewClassicTokenizerImpl(r)
	t.skippedPositions = 0
	return nil
}

// Reset reinitialises the scanner over the stored reader.
func (t *ClassicTokenizer) Reset() error {
	if err := t.BaseTokenizer.Reset(); err != nil {
		return err
	}
	r := t.GetReader()
	if r != nil {
		t.scanner = NewClassicTokenizerImpl(r)
	} else {
		t.scanner = nil
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
func (f *ClassicTokenizerFactory) Create() analysis.Tokenizer {
	t := NewClassicTokenizer()
	t.SetMaxTokenLength(f.maxTokenLength)
	return t
}

// Ensure factory implements TokenizerFactory.
var _ analysis.TokenizerFactory = (*ClassicTokenizerFactory)(nil)
