// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"io"
	"unicode/utf8"
)

// DefaultMaxWordLen is the default maximum token length emitted by CharTokenizer.
// Mirrors CharTokenizer.DEFAULT_MAX_WORD_LEN = 255.
const DefaultMaxWordLen = 255

// CharTokenizerMaxTokenLengthLimit is the hard upper bound for maxTokenLen.
// Mirrors StandardTokenizer.MAX_TOKEN_LENGTH_LIMIT = 1024 * 1024.
const CharTokenizerMaxTokenLengthLimit = 1024 * 1024

const charTokenizerIOBufferSize = 4096

// CharToken holds a single token produced by CharTokenizer.
type CharToken struct {
	// Text is the token text.
	Text string
	// StartOffset is the rune-level start offset in the input.
	StartOffset int
	// EndOffset is the rune-level end offset (exclusive) in the input.
	EndOffset int
}

// CharTokenizer is an abstract base for simple character-oriented tokenizers.
//
// Go port of org.apache.lucene.analysis.util.CharTokenizer (Apache Lucene
// 10.4.0).
//
// Set IsTokenChar to a function that returns true for runes that belong to a
// token.  Alternatively, construct via FromTokenCharPredicate or
// FromSeparatorCharPredicate.
//
// CharTokenizer lives in the analysis/util package to avoid a circular import
// with the main analysis package; concrete subclasses in analysis/ embed it.
type CharTokenizer struct {
	// IsTokenChar returns true if the rune belongs to a token.
	IsTokenChar func(r rune) bool

	maxTokenLen int

	reader io.Reader

	// io buffer (bytes)
	ioBuf     []byte
	ioBufLen  int
	ioBufPos  int

	// running rune offset in the current reader session
	runeOffset int

	// finalOffset is the last offset emitted (for End())
	finalOffset int
}

// NewCharTokenizer creates a CharTokenizer with DefaultMaxWordLen.
// Set ct.IsTokenChar before calling Next.
func NewCharTokenizer() *CharTokenizer {
	return &CharTokenizer{
		maxTokenLen: DefaultMaxWordLen,
		ioBuf:       make([]byte, charTokenizerIOBufferSize),
	}
}

// NewCharTokenizerWithMaxLen creates a CharTokenizer with a custom max token length.
func NewCharTokenizerWithMaxLen(maxTokenLen int) *CharTokenizer {
	if maxTokenLen <= 0 || maxTokenLen > CharTokenizerMaxTokenLengthLimit {
		panic("charTokenizer: maxTokenLen out of range")
	}
	return &CharTokenizer{
		maxTokenLen: maxTokenLen,
		ioBuf:       make([]byte, charTokenizerIOBufferSize),
	}
}

// FromTokenCharPredicate creates a CharTokenizer whose IsTokenChar is pred.
func FromTokenCharPredicate(pred func(r rune) bool) *CharTokenizer {
	ct := NewCharTokenizer()
	ct.IsTokenChar = pred
	return ct
}

// FromSeparatorCharPredicate creates a CharTokenizer whose token chars are
// the complement of sep (i.e. a rune is a token char iff !sep(r)).
func FromSeparatorCharPredicate(sep func(r rune) bool) *CharTokenizer {
	return FromTokenCharPredicate(func(r rune) bool { return !sep(r) })
}

// Reset prepares the tokenizer for a new reader.
func (ct *CharTokenizer) Reset(r io.Reader) {
	ct.reader = r
	ct.ioBufLen = 0
	ct.ioBufPos = 0
	ct.runeOffset = 0
	ct.finalOffset = 0
}

// FinalOffset returns the offset at end-of-stream (for use by End()).
func (ct *CharTokenizer) FinalOffset() int { return ct.finalOffset }

// Next advances to the next token and returns it, or (zero, false) on EOF.
func (ct *CharTokenizer) Next() (CharToken, bool, error) {
	tokenBuf := make([]rune, 0, 32)
	startOffset := -1
	endOffset := -1

	for {
		r, err := ct.readRune()
		if err == io.EOF {
			if len(tokenBuf) > 0 {
				ct.finalOffset = endOffset
				return CharToken{string(tokenBuf), startOffset, endOffset}, true, nil
			}
			ct.finalOffset = ct.runeOffset
			return CharToken{}, false, nil
		}
		if err != nil {
			return CharToken{}, false, err
		}

		if ct.IsTokenChar(r) {
			if len(tokenBuf) == 0 {
				startOffset = ct.runeOffset - 1
				endOffset = startOffset
			}
			tokenBuf = append(tokenBuf, r)
			endOffset++
			if len(tokenBuf) >= ct.maxTokenLen {
				ct.finalOffset = endOffset
				return CharToken{string(tokenBuf), startOffset, endOffset}, true, nil
			}
		} else if len(tokenBuf) > 0 {
			ct.finalOffset = endOffset
			return CharToken{string(tokenBuf), startOffset, endOffset}, true, nil
		}
	}
}

// readRune reads the next rune from the buffered io stream, advancing runeOffset.
func (ct *CharTokenizer) readRune() (rune, error) {
	for {
		// Try to decode from what's in ioBuf.
		if ct.ioBufPos < ct.ioBufLen {
			r, size := utf8.DecodeRune(ct.ioBuf[ct.ioBufPos:ct.ioBufLen])
			if size > 0 && r != utf8.RuneError {
				ct.ioBufPos += size
				ct.runeOffset++
				return r, nil
			}
			if r == utf8.RuneError && size == 1 {
				// Incomplete multi-byte sequence at end of buffer — shift and refill.
				remaining := ct.ioBufLen - ct.ioBufPos
				copy(ct.ioBuf, ct.ioBuf[ct.ioBufPos:ct.ioBufLen])
				n, err := ct.reader.Read(ct.ioBuf[remaining:])
				if n == 0 {
					if err != nil {
						if err == io.EOF {
							return 0, io.EOF
						}
						return 0, err
					}
					continue
				}
				ct.ioBufLen = remaining + n
				ct.ioBufPos = 0
				continue
			}
		}
		// Buffer exhausted — refill.
		n, err := ct.reader.Read(ct.ioBuf)
		ct.ioBufPos = 0
		ct.ioBufLen = n
		if n == 0 {
			if err == io.EOF {
				return 0, io.EOF
			}
			return 0, err
		}
	}
}
