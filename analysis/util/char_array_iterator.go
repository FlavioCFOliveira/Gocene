// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

// NewCharArrayIterator creates an empty CharArrayIterator.
func NewCharArrayIterator() *CharArrayIterator { return &CharArrayIterator{} }

// CharArrayIterator provides a rune-slice view used internally by
// SegmentingTokenizerBase and its concrete BreakIterator implementations.
//
// Go port of org.apache.lucene.analysis.util.CharArrayIterator (Apache Lucene
// 10.4.0). The Java original extends java.text.CharacterIterator to bridge
// char[] buffers to java.text.BreakIterator; in Go, the same role is played by
// implementing the BreakIterator interface via a concrete struct.
//
// CharArrayIterator itself is a low-level helper: it exposes the backing rune
// slice, the start position, the logical length, and the current index so that
// higher-level BreakIterator implementations can build on it.
type CharArrayIterator struct {
	array  []rune
	start  int
	index  int
	length int
	limit  int // start + length
}

// SetText configures the iterator to operate on array[0:length], starting at
// offset start within array.
func (it *CharArrayIterator) SetText(array []rune, start, length int) {
	it.array = array
	it.start = start
	it.index = start
	it.length = length
	it.limit = start + length
}

// GetText returns the backing rune slice.
func (it *CharArrayIterator) GetText() []rune { return it.array }

// GetStart returns the start offset within the backing array.
func (it *CharArrayIterator) GetStart() int { return it.start }

// GetLength returns the logical length of the region.
func (it *CharArrayIterator) GetLength() int { return it.length }

// Current returns the rune at the current index, or 0 if at the limit.
func (it *CharArrayIterator) Current() rune {
	if it.index == it.limit {
		return 0
	}
	return it.array[it.index]
}

// First moves the index to the start and returns the rune there.
func (it *CharArrayIterator) First() rune {
	it.index = it.start
	return it.Current()
}

// Last moves the index to the last valid position.
func (it *CharArrayIterator) Last() rune {
	if it.limit == it.start {
		it.index = it.limit
	} else {
		it.index = it.limit - 1
	}
	return it.Current()
}

// Next advances the index and returns the rune there, or 0 at the limit.
func (it *CharArrayIterator) Next() rune {
	it.index++
	if it.index >= it.limit {
		it.index = it.limit
		return 0
	}
	return it.Current()
}

// Previous retreats the index and returns the rune there, or 0 at the start.
func (it *CharArrayIterator) Previous() rune {
	it.index--
	if it.index < it.start {
		it.index = it.start
		return 0
	}
	return it.Current()
}

// SetIndex sets the current index to position (relative to start) and returns
// the rune there.  Panics if position is out of [0, length].
func (it *CharArrayIterator) SetIndex(position int) rune {
	if position < 0 || position > it.length {
		panic("CharArrayIterator: position out of bounds")
	}
	it.index = it.start + position
	return it.Current()
}

// BeginIndex returns 0 (the logical begin index).
func (it *CharArrayIterator) BeginIndex() int { return 0 }

// EndIndex returns the logical length.
func (it *CharArrayIterator) EndIndex() int { return it.length }

// GetIndex returns the current logical index (relative to start).
func (it *CharArrayIterator) GetIndex() int { return it.index - it.start }

// Clone returns a shallow copy of this iterator.
func (it *CharArrayIterator) Clone() *CharArrayIterator {
	cp := *it
	return &cp
}
