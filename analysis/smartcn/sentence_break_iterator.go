// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package smartcn

import (
	analysisutil "github.com/FlavioCFOliveira/Gocene/analysis/util"
)

// SentenceBreakIterator is a simple sentence-boundary iterator that mimics
// java.text.BreakIterator.getSentenceInstance(Locale.ROOT).
//
// It splits on sentence-terminating characters: '.', '!', '?', and their
// full-width equivalents, as well as newlines and other line separators.
// The boundary is placed after the terminating character (and any following
// whitespace that Lucene's tokeniser would skip anyway).
type SentenceBreakIterator struct {
	buf     []rune
	length  int
	current int
}

// NewSentenceBreakIterator creates a new SentenceBreakIterator.
func NewSentenceBreakIterator() *SentenceBreakIterator {
	return &SentenceBreakIterator{current: 0}
}

// SetText sets the text to iterate over.
func (it *SentenceBreakIterator) SetText(buf []rune, length int) {
	it.buf = buf
	it.length = length
	it.current = 0
}

// Current returns the current boundary position.
func (it *SentenceBreakIterator) Current() int {
	return it.current
}

// Next advances to the next sentence boundary and returns it.
// Returns BreakDone when exhausted.
func (it *SentenceBreakIterator) Next() int {
	if it.current >= it.length {
		return analysisutil.BreakDone
	}

	start := it.current
	for i := start; i < it.length; i++ {
		ch := it.buf[i]
		if isSentenceEnd(ch) {
			// Advance past the terminator and any trailing whitespace.
			j := i + 1
			for j < it.length && isSpaceLike(it.buf[j]) {
				j++
			}
			it.current = j
			return it.current
		}
	}

	// No terminator found — rest of buffer is one sentence.
	it.current = it.length
	return it.current
}

// isSentenceEnd returns true for characters that end a sentence.
func isSentenceEnd(ch rune) bool {
	switch ch {
	case '.', '!', '?', '\n', '\r',
		'。', '！', '？', // Full-width
		' ', ' ': // Line/paragraph separator
		return true
	}
	return false
}

// isSpaceLike returns true for whitespace characters.
func isSpaceLike(ch rune) bool {
	return ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' || ch == '　'
}

// Ensure SentenceBreakIterator implements analysisutil.BreakIterator.
var _ analysisutil.BreakIterator = (*SentenceBreakIterator)(nil)
