// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// KeywordTokenizer is a tokenizer that emits the entire input as a single token.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.core.KeywordTokenizer.
//
// KeywordTokenizer treats the entire input as a single token, preserving all
// characters exactly as they appear. This is useful for:
// - Preserving exact input values (e.g., product codes, identifiers)
// - Fields that should not be tokenized (e.g., IDs, exact match fields)
// - Building custom tokenizers that process input first
//
// Example:
//
//	Input: "Hello, World!"
//	Output tokens: "Hello, World!" (single token)
type KeywordTokenizer struct {
	*BaseTokenizer

	// done tracks whether we've emitted the token
	done bool

	// termAttr holds the CharTermAttribute
	termAttr CharTermAttribute

	// offsetAttr holds the OffsetAttribute
	offsetAttr OffsetAttribute

	// posIncrAttr holds the PositionIncrementAttribute
	posIncrAttr PositionIncrementAttribute
}

// NewKeywordTokenizer creates a new KeywordTokenizer.
func NewKeywordTokenizer() *KeywordTokenizer {
	t := &KeywordTokenizer{
		BaseTokenizer: NewBaseTokenizer(),
		done:          false,
	}

	// Add attributes
	t.termAttr = NewCharTermAttribute()
	t.offsetAttr = NewOffsetAttribute()
	t.posIncrAttr = NewPositionIncrementAttribute()

	t.AddAttribute(t.termAttr)
	t.AddAttribute(t.offsetAttr)
	t.AddAttribute(t.posIncrAttr)

	return t
}

// SetReader sets the input source for this Tokenizer.
func (t *KeywordTokenizer) SetReader(input io.Reader) error {
	t.BaseTokenizer.SetReader(input)
	t.done = false
	return nil
}

// IncrementToken advances to the next token.
func (t *KeywordTokenizer) IncrementToken() (bool, error) {
	if t.input == nil || t.done {
		return false, nil
	}

	// Clear attributes for new token
	t.ClearAttributes()

	// Read entire input
	buf := make([]byte, 0, 1024)
	temp := make([]byte, 1024)

	for {
		n, err := t.input.Read(temp)
		if n > 0 {
			buf = append(buf, temp[:n]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, err
		}
	}

	// Set the token
	t.termAttr.SetValue(string(buf))
	t.offsetAttr.SetStartOffset(0)
	t.offsetAttr.SetEndOffset(len(buf))
	t.posIncrAttr.SetPositionIncrement(1)

	t.done = true
	return true, nil
}

// Reset resets the tokenizer.
func (t *KeywordTokenizer) Reset() error {
	t.BaseTokenizer.Reset()
	t.done = false
	return nil
}

// End performs end-of-stream operations.
func (t *KeywordTokenizer) End() error {
	if t.offsetAttr != nil {
		t.offsetAttr.SetEndOffset(t.termAttr.Length())
	}
	return nil
}

// Ensure KeywordTokenizer implements Tokenizer
var _ Tokenizer = (*KeywordTokenizer)(nil)

// KeywordTokenizerFactory creates KeywordTokenizer instances.
type KeywordTokenizerFactory struct{}

// NewKeywordTokenizerFactory creates a new KeywordTokenizerFactory.
func NewKeywordTokenizerFactory() *KeywordTokenizerFactory {
	return &KeywordTokenizerFactory{}
}

// Create creates a new KeywordTokenizer.
func (f *KeywordTokenizerFactory) Create() Tokenizer {
	return NewKeywordTokenizer()
}

// Ensure KeywordTokenizerFactory implements TokenizerFactory
var _ TokenizerFactory = (*KeywordTokenizerFactory)(nil)