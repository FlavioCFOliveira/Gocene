// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package segmentation provides ICU-based text segmentation components.
//
// Go port of org.apache.lucene.analysis.icu.segmentation (Apache Lucene
// 10.4.0).
//
// Deviation: This package depends on ICU4J (com.ibm.icu) for
// RuleBasedBreakIterator, UScript, and UCharacter. Go has no CGO-free
// equivalent with full ICU4J API parity. BreakIteratorWrapper and related
// types are modelled as Go interfaces; callers must supply concrete
// implementations.
package segmentation

import "unicode/utf16"

// Done is the sentinel value returned by break iterators when there is no
// further break position, matching BreakIterator.DONE = -1.
const Done = -1

// CharArrayIterator wraps a UTF-16 char array as a text source for
// RuleBasedBreakIterator implementations.
//
// Go port of
// org.apache.lucene.analysis.icu.segmentation.CharArrayIterator
// (Apache Lucene 10.4.0).
//
// Deviation: The Java original implements java.text.CharacterIterator (a
// UTF-16 char iterator). In Go the text is represented as a []rune slice
// (Unicode code points). The CharArrayIterator wraps a rune slice and
// exposes the interface needed by BreakIteratorWrapper.
//
// Task 2976: replace the existing analysis/util/CharArrayIterator stub with
// this ICU-specific implementation that lives in the segmentation package.
type CharArrayIterator struct {
	text   []rune
	start  int
	index  int
	length int
	limit  int // start + length
}

// NewCharArrayIterator creates an empty CharArrayIterator.
func NewCharArrayIterator() *CharArrayIterator { return &CharArrayIterator{} }

// SetText configures the iterator to operate on text[start:start+length].
func (it *CharArrayIterator) SetText(text []rune, start, length int) {
	it.text = text
	it.start = start
	it.index = start
	it.length = length
	it.limit = start + length
}

// GetText returns the backing rune slice.
func (it *CharArrayIterator) GetText() []rune { return it.text }

// GetStart returns the start offset within the backing array.
func (it *CharArrayIterator) GetStart() int { return it.start }

// GetLength returns the logical length of the region.
func (it *CharArrayIterator) GetLength() int { return it.length }

// Current returns the rune at the current index, or 0 at the limit.
func (it *CharArrayIterator) Current() rune {
	if it.index >= it.limit {
		return 0
	}
	return it.text[it.index]
}

// First moves to the start and returns the first rune.
func (it *CharArrayIterator) First() rune {
	it.index = it.start
	return it.Current()
}

// Last moves to the last valid position and returns the rune there.
func (it *CharArrayIterator) Last() rune {
	if it.limit == it.start {
		it.index = it.limit
	} else {
		it.index = it.limit - 1
	}
	return it.Current()
}

// Next advances and returns the rune, or 0 at the limit.
func (it *CharArrayIterator) Next() rune {
	it.index++
	if it.index >= it.limit {
		it.index = it.limit
		return 0
	}
	return it.Current()
}

// Previous retreats and returns the rune, or 0 at the start.
func (it *CharArrayIterator) Previous() rune {
	it.index--
	if it.index < it.start {
		it.index = it.start
		return 0
	}
	return it.Current()
}

// SetIndex sets the current logical index (0-based relative to start).
// Panics if out of [0, length].
func (it *CharArrayIterator) SetIndex(position int) rune {
	if position < 0 || position > it.length {
		panic("CharArrayIterator.SetIndex: position out of bounds")
	}
	it.index = it.start + position
	return it.Current()
}

// BeginIndex returns 0.
func (it *CharArrayIterator) BeginIndex() int { return 0 }

// EndIndex returns the logical length.
func (it *CharArrayIterator) EndIndex() int { return it.length }

// GetIndex returns the current logical index (relative to start).
func (it *CharArrayIterator) GetIndex() int { return it.index - it.start }

// Clone returns a shallow copy.
func (it *CharArrayIterator) Clone() *CharArrayIterator {
	cp := *it
	return &cp
}

// AsUTF16 converts the active region to a UTF-16 encoded string suitable
// for passing to Java-style break iterators.
func (it *CharArrayIterator) AsUTF16() []uint16 {
	return utf16.Encode(it.text[it.start:it.limit])
}
