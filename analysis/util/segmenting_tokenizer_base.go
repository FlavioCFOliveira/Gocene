// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"io"
)

// BufferMax is the size of the internal character buffer used by
// SegmentingTokenizerBase. Mirrors SegmentingTokenizerBase.BUFFERMAX = 1024.
const BufferMax = 1024

// BreakIterator is the Go analogue of java.text.BreakIterator.
// It is supplied by the concrete subclass and must not be shared across
// multiple tokenizers.
//
// Methods mirror the java.text.BreakIterator contract:
//   - SetText sets the text slice to iterate over.
//   - Current returns the current boundary position.
//   - Next advances and returns the next boundary, or BreakDone when exhausted.
type BreakIterator interface {
	// SetText configures the iterator to operate on buf[0:length].
	SetText(buf []rune, length int)
	// Current returns the current boundary index.
	Current() int
	// Next advances to the next boundary and returns it, or BreakDone.
	Next() int
}

// BreakDone is the sentinel returned by BreakIterator.Next when the text
// is exhausted. Mirrors java.text.BreakIterator.DONE = -1.
const BreakDone = -1

// SegmentingTokenizerBase breaks text into sentences with a BreakIterator
// and lets subclasses decompose those sentences into words.
//
// Go port of org.apache.lucene.analysis.util.SegmentingTokenizerBase
// (Apache Lucene 10.4.0).
//
// Usage:
//  1. Embed *SegmentingTokenizerBase in your concrete tokenizer struct.
//  2. Implement the two hooks by assigning them after construction:
//       base.SetNextSentenceFn = func(start, end int) { ... }
//       base.IncrementWordFn   = func() bool { return ... }
//  3. Subclass must expose Buffer and Offset so its IncrementWordFn can read
//     the current sentence slice.
type SegmentingTokenizerBase struct {
	// Buffer is the shared character buffer (length BufferMax).
	// Subclasses access it via their IncrementWordFn closure.
	Buffer []rune

	// Offset is the accumulated rune offset of previous buffer fills.
	Offset int

	// SetNextSentenceFn is called with the [start, end) rune indices
	// within Buffer for the current sentence. Subclass must set this.
	SetNextSentenceFn func(sentenceStart, sentenceEnd int)

	// IncrementWordFn returns true if there is another word token in the
	// current sentence, emitting it into the shared attribute source.
	// Subclass must set this.
	IncrementWordFn func() bool

	length      int // true length of valid runes in Buffer
	usableLength int // last safe break point within Buffer

	iterator BreakIterator
	input    io.Reader
}

// NewSegmentingTokenizerBase constructs a new base given a BreakIterator.
// The caller must set SetNextSentenceFn and IncrementWordFn before use.
func NewSegmentingTokenizerBase(iter BreakIterator) *SegmentingTokenizerBase {
	s := &SegmentingTokenizerBase{
		Buffer:   make([]rune, BufferMax),
		iterator: iter,
	}
	return s
}

// SetReader sets the input reader for this tokenization session.
func (s *SegmentingTokenizerBase) SetReader(r io.Reader) {
	s.input = r
}

// Reset resets internal state; call before each new tokenization session.
func (s *SegmentingTokenizerBase) Reset() {
	s.length = 0
	s.usableLength = 0
	s.Offset = 0
	s.iterator.SetText(s.Buffer, 0)
}

// IncrementToken drives the outer loop: fills sentences from the buffer
// and delegates word extraction to IncrementWordFn.
// Returns false when the input is exhausted.
func (s *SegmentingTokenizerBase) IncrementToken() (bool, error) {
	if s.length == 0 || !s.IncrementWordFn() {
		for {
			ok, err := s.incrementSentence()
			if err != nil {
				return false, err
			}
			if ok {
				break
			}
			if err2 := s.refill(); err2 != nil {
				return false, err2
			}
			if s.length <= 0 {
				return false, nil
			}
		}
	}
	return true, nil
}

// End returns the final offset (rune count) consumed from the input.
// The caller is responsible for stamping the offset attribute with this value.
func (s *SegmentingTokenizerBase) End() int {
	if s.length < 0 {
		return s.Offset
	}
	return s.Offset + s.length
}

// isSafeEnd returns true for characters that are unambiguous sentence ends,
// used to find a safe refill boundary. Mirrors SegmentingTokenizerBase.isSafeEnd.
func (s *SegmentingTokenizerBase) isSafeEnd(ch rune) bool {
	switch ch {
	case 0x000D, 0x000A, 0x0085, 0x2028, 0x2029:
		return true
	}
	return false
}

// findSafeEnd returns the last position in Buffer[0:length] that is a safe
// break point, or -1 if none found.
func (s *SegmentingTokenizerBase) findSafeEnd() int {
	for i := s.length - 1; i >= 0; i-- {
		if s.isSafeEnd(s.Buffer[i]) {
			return i + 1
		}
	}
	return -1
}

// refill reads more runes from s.input into Buffer, sliding any leftover
// content to the front and updating usableLength and the iterator.
func (s *SegmentingTokenizerBase) refill() error {
	s.Offset += s.usableLength
	leftover := s.length - s.usableLength
	copy(s.Buffer, s.Buffer[s.usableLength:s.length])

	requested := len(s.Buffer) - leftover
	returned, err := readFullRunes(s.input, s.Buffer, leftover, requested)
	if err != nil && err != io.EOF {
		return err
	}

	if returned < 0 {
		returned = 0
	}
	s.length = returned + leftover

	if returned < requested {
		// reader exhausted — process whatever remains
		s.usableLength = s.length
	} else {
		s.usableLength = s.findSafeEnd()
		if s.usableLength < 0 {
			s.usableLength = s.length
		}
	}

	usable := s.usableLength
	if usable < 0 {
		usable = 0
	}
	s.iterator.SetText(s.Buffer, usable)
	return nil
}

// readFullRunes reads up to length runes from r into buf[offset:offset+length].
// Returns the number of runes actually read, or 0 on EOF.
func readFullRunes(r io.Reader, buf []rune, offset, length int) (int, error) {
	if length <= 0 {
		return 0, nil
	}
	// Read bytes and decode runes.
	// We decode one rune at a time to avoid partial-rune issues.
	tmp := make([]byte, length*4) // worst case 4 bytes per rune
	n, err := io.ReadAtLeast(r, tmp, 1)
	if n == 0 {
		return 0, err
	}
	// Trim to bytes actually read.
	tmp = tmp[:n]
	count := 0
	for len(tmp) > 0 && count < length {
		r1, size := decodeRune(tmp)
		if size == 0 {
			break
		}
		buf[offset+count] = r1
		count++
		tmp = tmp[size:]
	}
	return count, err
}

// decodeRune decodes the first UTF-8-encoded rune in b.
// Returns (RuneError, 0) on empty input.
func decodeRune(b []byte) (rune, int) {
	if len(b) == 0 {
		return 0, 0
	}
	// Fast path: ASCII
	if b[0] < 0x80 {
		return rune(b[0]), 1
	}
	// Multi-byte
	var r rune
	var size int
	switch {
	case b[0]&0xE0 == 0xC0 && len(b) >= 2:
		r = rune(b[0]&0x1F)<<6 | rune(b[1]&0x3F)
		size = 2
	case b[0]&0xF0 == 0xE0 && len(b) >= 3:
		r = rune(b[0]&0x0F)<<12 | rune(b[1]&0x3F)<<6 | rune(b[2]&0x3F)
		size = 3
	case b[0]&0xF8 == 0xF0 && len(b) >= 4:
		r = rune(b[0]&0x07)<<18 | rune(b[1]&0x3F)<<12 | rune(b[2]&0x3F)<<6 | rune(b[3]&0x3F)
		size = 4
	default:
		return 0xFFFD, 1 // replacement character
	}
	return r, size
}

// incrementSentence advances to the next sentence in the current buffer window.
// Returns true and calls SetNextSentenceFn when a sentence with tokens is found.
func (s *SegmentingTokenizerBase) incrementSentence() (bool, error) {
	if s.length == 0 {
		return false, nil
	}
	for {
		start := s.iterator.Current()
		if start == BreakDone {
			return false, nil
		}
		end := s.iterator.Next()
		if end == BreakDone {
			return false, nil
		}
		s.SetNextSentenceFn(start, end)
		if s.IncrementWordFn() {
			return true, nil
		}
	}
}
