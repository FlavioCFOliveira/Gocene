// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package th provides Thai language analysis components.
package th

import (
	"io"
	"unicode"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// ThaiTokenizer tokenises Thai text by treating each consecutive Thai Unicode
// character as a separate token candidate while treating non-Thai runs as
// whitespace-delimited tokens.
//
// This is the Go port of org.apache.lucene.analysis.th.ThaiTokenizer from
// Apache Lucene 10.4.0.
//
// Deviation: the Java reference uses Java's dictionary-based BreakIterator
// for Thai word segmentation. This Go port uses a Unicode Thai-block
// character-per-token heuristic. Full dictionary-based Thai word segmentation
// is deferred until a suitable Go ICU binding is available.
type ThaiTokenizer struct {
	*analysis.BaseTokenizer

	buf []rune
	pos int

	termAttr   analysis.CharTermAttribute
	offsetAttr analysis.OffsetAttribute
}

// NewThaiTokenizer creates a new ThaiTokenizer.
func NewThaiTokenizer() *ThaiTokenizer {
	t := &ThaiTokenizer{
		BaseTokenizer: analysis.NewBaseTokenizer(),
	}
	src := t.GetAttributeSource()
	if src != nil {
		if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
			t.termAttr = a.(analysis.CharTermAttribute)
		}
		if a := src.GetAttribute(analysis.OffsetAttributeType); a != nil {
			t.offsetAttr = a.(analysis.OffsetAttribute)
		}
	}
	return t
}

// SetReader sets the input reader and eagerly reads all runes.
func (t *ThaiTokenizer) SetReader(r io.Reader) error {
	if err := t.BaseTokenizer.SetReader(r); err != nil {
		return err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	t.buf = []rune(string(data))
	t.pos = 0
	return nil
}

// Reset resets internal state for a new tokenisation session.
func (t *ThaiTokenizer) Reset() error {
	if err := t.BaseTokenizer.Reset(); err != nil {
		return err
	}
	// Re-read from the stored reader if available.
	r := t.GetReader()
	if r != nil {
		data, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		t.buf = []rune(string(data))
	}
	t.pos = 0
	return nil
}

// IncrementToken advances to the next token.
func (t *ThaiTokenizer) IncrementToken() (bool, error) {
	if t.buf == nil {
		return false, nil
	}
	// Skip whitespace.
	for t.pos < len(t.buf) && unicode.IsSpace(t.buf[t.pos]) {
		t.pos++
	}
	if t.pos >= len(t.buf) {
		return false, nil
	}
	start := t.pos
	r := t.buf[t.pos]
	if isThai(r) {
		// One Thai character per token (heuristic).
		t.pos++
	} else {
		// Non-Thai run: advance until whitespace or Thai character.
		for t.pos < len(t.buf) && !unicode.IsSpace(t.buf[t.pos]) && !isThai(t.buf[t.pos]) {
			t.pos++
		}
	}
	token := string(t.buf[start:t.pos])
	t.ClearAttributes()
	if t.termAttr != nil {
		t.termAttr.SetEmpty()
		t.termAttr.AppendString(token)
	}
	if t.offsetAttr != nil {
		t.offsetAttr.SetOffset(start, t.pos)
	}
	return true, nil
}

// isThai reports whether r is in the Thai Unicode block (U+0E00–U+0E7F).
func isThai(r rune) bool { return r >= 0x0E00 && r <= 0x0E7F }

// End performs end-of-stream housekeeping.
func (t *ThaiTokenizer) End() error { return nil }

// Close releases resources.
func (t *ThaiTokenizer) Close() error {
	t.buf = nil
	return nil
}

// Ensure ThaiTokenizer implements Tokenizer.
var _ analysis.Tokenizer = (*ThaiTokenizer)(nil)

// ThaiTokenizerFactory creates ThaiTokenizer instances.
//
// This is the Go port of org.apache.lucene.analysis.th.ThaiTokenizerFactory
// from Apache Lucene 10.4.0.
type ThaiTokenizerFactory struct{}

// NewThaiTokenizerFactory returns a new ThaiTokenizerFactory.
func NewThaiTokenizerFactory() *ThaiTokenizerFactory { return &ThaiTokenizerFactory{} }

// Create returns a new ThaiTokenizer.
func (f *ThaiTokenizerFactory) Create() analysis.Tokenizer { return NewThaiTokenizer() }

// Ensure ThaiTokenizerFactory implements TokenizerFactory.
var _ analysis.TokenizerFactory = (*ThaiTokenizerFactory)(nil)
