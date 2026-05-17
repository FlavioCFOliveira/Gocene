// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:generate go run ../cmd/gen-unicode-wb

package analysis

import (
	"fmt"
	"io"
	"reflect"
)

// DefaultMaxTokenLength is declared in uax29_url_email_tokenizer.go
// and shared by both tokenizers; it matches
// org.apache.lucene.analysis.standard.StandardAnalyzer.DEFAULT_MAX_TOKEN_LENGTH.

// StandardTokenizer is the grammar-based tokenizer that implements
// the Word Break rules from the Unicode Text Segmentation algorithm
// (Unicode Standard Annex #29). It is the Go port of
// org.apache.lucene.analysis.standard.StandardTokenizer from Lucene
// 10.4.0.
//
// Tokens produced are typed by [StandardTokenTypes] and one of the
// following type integers:
//
//   - [TokenTypeAlphanum]:        a run of alphabetic / numeric runes.
//   - [TokenTypeNum]:             a number.
//   - [TokenTypeSoutheastAsian]:  a run of South-East Asian runes (Thai,
//     Lao, Myanmar, Khmer, ...).
//   - [TokenTypeIdeographic]:     a single CJKV ideographic rune.
//   - [TokenTypeHiragana]:        a single Hiragana rune.
//   - [TokenTypeKatakana]:        a run of Katakana runes.
//   - [TokenTypeHangul]:          a run of Hangul runes.
//   - [TokenTypeEmoji]:           an Emoji sequence (modifier, ZWJ,
//     key-cap, regional indicator pair, or tag).
//
// The tokenizer exposes four attributes through its
// [AttributeSource]: [CharTermAttribute], [OffsetAttribute],
// [PositionIncrementAttribute] and [TypeAttribute].
type StandardTokenizer struct {
	*BaseTokenizer

	// scanner is the UAX#29 word-break state machine.
	scanner *standardTokenizerImpl

	// maxTokenLength caps the length, in runes, of any emitted
	// token. Tokens longer than this are chunked at the boundary;
	// the scanner is rewound to the chunk's end so the remainder
	// is re-tokenised on the next call.
	maxTokenLength int

	// Bound attributes. Captured once at construction so the hot
	// path does not pay the AttributeSource lookup cost on every
	// token.
	termAttr    CharTermAttribute
	offsetAttr  OffsetAttribute
	posIncrAttr PositionIncrementAttribute
	typeAttr    *TypeAttribute
}

// NewStandardTokenizer creates a new StandardTokenizer with default
// maxTokenLength ([DefaultMaxTokenLength]).
//
// The caller must invoke [StandardTokenizer.SetReader] before the
// first call to IncrementToken.
func NewStandardTokenizer() *StandardTokenizer {
	t := &StandardTokenizer{
		BaseTokenizer:  NewBaseTokenizer(),
		scanner:        newStandardTokenizerImpl(),
		maxTokenLength: DefaultMaxTokenLength,
	}

	t.termAttr = NewCharTermAttribute()
	t.offsetAttr = NewOffsetAttribute()
	t.posIncrAttr = NewPositionIncrementAttribute()
	t.typeAttr = NewTypeAttribute()

	t.AddAttribute(t.termAttr)
	t.AddAttribute(t.offsetAttr)
	t.AddAttribute(t.posIncrAttr)
	t.AddAttribute(t.typeAttr)

	return t
}

// MaxTokenLength returns the current maximum token length in
// characters.
func (t *StandardTokenizer) MaxTokenLength() int {
	return t.maxTokenLength
}

// SetMaxTokenLength configures the maximum token length. Length must
// be in the inclusive range [1, MaxTokenLengthLimit]. Returns an
// error matching the IllegalArgumentException thrown by Lucene's
// StandardTokenizer.setMaxTokenLength when length is out of range.
func (t *StandardTokenizer) SetMaxTokenLength(length int) error {
	if length < 1 {
		return fmt.Errorf("maxTokenLength must be greater than zero")
	}
	if length > MaxTokenLengthLimit {
		return fmt.Errorf("maxTokenLength may not exceed %d", MaxTokenLengthLimit)
	}
	t.maxTokenLength = length
	return nil
}

// SetReader attaches the input source and resets the underlying
// scanner. It satisfies the [Tokenizer] contract.
func (t *StandardTokenizer) SetReader(input io.Reader) error {
	if err := t.BaseTokenizer.SetReader(input); err != nil {
		return err
	}
	if err := t.scanner.yyreset(input); err != nil {
		return err
	}
	return nil
}

// IncrementToken advances to the next token. Returns (false, nil) at
// end of input. Tokens larger than the configured maxTokenLength are
// chunked into pieces of at most maxTokenLength runes, mirroring the
// effect of Lucene's StandardTokenizer.setBufferSize on the
// JFlex-generated scanner: the buffer cap forces the scanner to
// return partial matches and the wrapper resumes from the cut.
func (t *StandardTokenizer) IncrementToken() (bool, error) {
	t.ClearAttributes()

	tokenType := t.scanner.getNextToken()
	if tokenType == yyeof {
		return false, nil
	}

	length := t.scanner.yylength()
	if length > t.maxTokenLength {
		// Cut the match to the maxTokenLength boundary and rewind
		// the scanner so the remainder is re-tokenised on the next
		// call. The retained chunk inherits the type of the
		// original match.
		t.scanner.markedPos = t.scanner.startRead + t.maxTokenLength
		t.scanner.pos = t.scanner.markedPos
		length = t.maxTokenLength
	}

	t.posIncrAttr.SetPositionIncrement(1)
	t.scanner.getText(t.termAttr)
	start := t.scanner.yychar()
	// Offsets are reported in runes for consistency with Gocene's
	// UTF-8 attribute storage; the underlying scanner counts code
	// points.
	t.offsetAttr.SetOffset(start, start+length)
	t.typeAttr.SetType(StandardTokenTypes[tokenType])
	return true, nil
}

// End performs end-of-stream operations. It sets the final offset
// to the rune index just past the last token, matching Lucene's
// StandardTokenizer.end().
func (t *StandardTokenizer) End() error {
	if err := t.BaseTokenizer.End(); err != nil {
		return err
	}
	finalOffset := t.scanner.yychar() + t.scanner.yylength()
	t.offsetAttr.SetOffset(finalOffset, finalOffset)
	return nil
}

// Reset prepares the tokenizer for re-use against a freshly attached
// reader. It satisfies the [Tokenizer] contract.
func (t *StandardTokenizer) Reset() error {
	if err := t.BaseTokenizer.Reset(); err != nil {
		return err
	}
	if err := t.scanner.yyreset(t.GetReader()); err != nil {
		return err
	}
	return nil
}

// Close releases resources held by the tokenizer.
func (t *StandardTokenizer) Close() error {
	// Detach the reader so a subsequent SetReader call gets a
	// pristine scanner state. Lucene's StandardTokenizer.close()
	// calls yyreset(input) to release input buffers.
	_ = t.scanner.yyreset(nil)
	return t.BaseTokenizer.Close()
}

// Ensure StandardTokenizer implements the Tokenizer contract.
var _ Tokenizer = (*StandardTokenizer)(nil)

// charTermAttributeType is the cached reflect.Type for the default
// CharTermAttribute impl, used by downstream filters to look the
// attribute up by type. Computed lazily to avoid an init-time cost.
var charTermAttributeType = reflect.TypeOf(&charTermAttribute{})
