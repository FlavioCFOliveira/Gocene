// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"io"
)

// RollingCharBuffer acts like a forever-growing rune slice as you read runes
// into it from the provided reader, but internally it uses a circular buffer to
// hold only the runes that have not been freed yet.
//
// This is like a PushbackReader, except you do not have to specify up-front the
// maximum size of the buffer; you do have to periodically call FreeBefore.
//
// Go port of org.apache.lucene.analysis.util.RollingCharBuffer (Apache Lucene
// 10.4.0). Uses rune (Unicode code point) rather than Java char, so surrogate
// pairs are handled transparently.
type RollingCharBuffer struct {
	reader io.RuneReader

	buffer []rune

	// nextWrite is the next index in buffer to write into.
	nextWrite int

	// nextPos is the next absolute position to read from the reader.
	nextPos int

	// count is the number of valid (not-yet-freed) runes in the buffer.
	count int

	// end is true once EOF has been reached.
	end bool
}

// NewRollingCharBuffer allocates a RollingCharBuffer with the default initial
// capacity. Call Reset before first use.
func NewRollingCharBuffer() *RollingCharBuffer {
	return &RollingCharBuffer{buffer: make([]rune, 512)}
}

// Reset clears state and switches to a new reader.
func (b *RollingCharBuffer) Reset(r io.RuneReader) {
	b.reader = r
	b.nextPos = 0
	b.nextWrite = 0
	b.count = 0
	b.end = false
}

// Get returns the rune at absolute position pos, or -1 on EOF.
//
// pos must not jump ahead by more than one beyond the current nextPos; it may
// read arbitrarily far back (as long as the position has not been freed).
func (b *RollingCharBuffer) Get(pos int) (rune, error) {
	if pos == b.nextPos {
		if b.end {
			return -1, nil
		}
		if b.count == len(b.buffer) {
			// Grow — mirror ArrayUtil.oversize heuristic (150% + 3).
			newCap := b.count + b.count/2 + 3
			newBuf := make([]rune, newCap)
			// Unwrap circular buffer into newBuf.
			firstPart := len(b.buffer) - b.nextWrite
			copy(newBuf, b.buffer[b.nextWrite:])
			copy(newBuf[firstPart:], b.buffer[:b.nextWrite])
			b.nextWrite = len(b.buffer)
			b.buffer = newBuf
		}
		if b.nextWrite == len(b.buffer) {
			b.nextWrite = 0
		}

		ch, _, err := b.reader.ReadRune()
		if err == io.EOF {
			b.end = true
			return -1, nil
		}
		if err != nil {
			return 0, err
		}
		b.buffer[b.nextWrite] = ch
		b.nextWrite++
		b.count++
		b.nextPos++
		return ch, nil
	}

	// Read from already-buffered position.
	return b.buffer[b.getIndex(pos)], nil
}

// GetSlice returns a fresh rune slice of length length starting at absolute
// position posStart.  All positions in [posStart, posStart+length) must
// already have been read (i.e. pos < nextPos) and not yet freed.
func (b *RollingCharBuffer) GetSlice(posStart, length int) []rune {
	result := make([]rune, length)
	startIndex := b.getIndex(posStart)
	endIndex := b.getIndex(posStart + length)

	if endIndex >= startIndex && length < len(b.buffer) {
		copy(result, b.buffer[startIndex:startIndex+(endIndex-startIndex)])
	} else {
		// Wrapped.
		part1 := len(b.buffer) - startIndex
		copy(result, b.buffer[startIndex:])
		copy(result[part1:], b.buffer[:length-part1])
	}
	return result
}

// FreeBefore notifies the buffer that no positions before pos are needed any
// more, allowing the circular buffer to reuse that space.
func (b *RollingCharBuffer) FreeBefore(pos int) {
	b.count = b.nextPos - pos
}

// getIndex translates an absolute position to an index inside buffer.
func (b *RollingCharBuffer) getIndex(pos int) int {
	idx := b.nextWrite - (b.nextPos - pos)
	if idx < 0 {
		idx += len(b.buffer)
	}
	return idx
}
