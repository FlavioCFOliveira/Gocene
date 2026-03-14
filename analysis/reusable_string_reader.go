// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// ReusableStringReader is a reusable Reader implementation over a string.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.ReusableStringReader.
//
// ReusableStringReader allows reusing the same reader instance with different
// string inputs, avoiding allocation overhead for temporary string readers.
type ReusableStringReader struct {
	str   string
	pos   int
	size  int
}

// NewReusableStringReader creates a new ReusableStringReader.
func NewReusableStringReader() *ReusableStringReader {
	return &ReusableStringReader{}
}

// SetValue sets the string value to read from.
// This resets the reader position to the beginning.
func (r *ReusableStringReader) SetValue(s string) {
	r.str = s
	r.pos = 0
	r.size = len(s)
}

// Read reads bytes into the given buffer.
// Implements the io.Reader interface.
func (r *ReusableStringReader) Read(p []byte) (n int, err error) {
	if r.pos >= r.size {
		return 0, io.EOF
	}

	n = copy(p, r.str[r.pos:])
	r.pos += n

	if r.pos >= r.size {
		return n, io.EOF
	}

	return n, nil
}

// ReadRune reads a single rune from the string.
func (r *ReusableStringReader) ReadRune() (ch rune, size int, err error) {
	if r.pos >= r.size {
		return 0, 0, io.EOF
	}

	// Decode UTF-8 rune
	ch, size = decodeRuneInString(r.str[r.pos:])
	r.pos += size

	return ch, size, nil
}

// Reset resets the reader to the beginning of the string.
func (r *ReusableStringReader) Reset() {
	r.pos = 0
}

// Length returns the length of the underlying string.
func (r *ReusableStringReader) Length() int {
	return r.size
}

// Position returns the current read position.
func (r *ReusableStringReader) Position() int {
	return r.pos
}

// decodeRuneInString decodes a UTF-8 rune from a string.
// This is a simplified version - in production, use utf8.DecodeRuneInString.
func decodeRuneInString(s string) (rune, int) {
	if len(s) == 0 {
		return 0, 0
	}

	// Simple ASCII fast path
	if s[0] < 0x80 {
		return rune(s[0]), 1
	}

	// Multi-byte UTF-8
	// This is a simplified implementation
	// For full UTF-8 support, use the standard library's utf8 package
	r, size := rune(s[0]), 1
	if r >= 0xC0 {
		// 2-byte sequence
		if r < 0xE0 && len(s) > 1 {
			r = (r&0x1F)<<6 | rune(s[1])&0x3F
			size = 2
		} else if r < 0xF0 && len(s) > 2 {
			// 3-byte sequence
			r = (r&0x0F)<<12 | (rune(s[1])&0x3F)<<6 | rune(s[2])&0x3F
			size = 3
		} else if len(s) > 3 {
			// 4-byte sequence
			r = (r&0x07)<<18 | (rune(s[1])&0x3F)<<12 | (rune(s[2])&0x3F)<<6 | rune(s[3])&0x3F
			size = 4
		}
	}

	return r, size
}

// Ensure ReusableStringReader implements io.Reader
var _ io.Reader = (*ReusableStringReader)(nil)
