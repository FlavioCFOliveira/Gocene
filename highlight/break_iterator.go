// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package highlight

import (
	"unicode"
	"unicode/utf8"
)

// BreakIterator breaks text into fragments at appropriate boundaries.
// This is used by highlighters to find good places to break text
// when creating passages.
//
// This is the Go port of Lucene's org.apache.lucene.search.highlight.BreakIterator.
type BreakIterator interface {
	// SetText sets the text to be analyzed.
	//
	// Parameters:
	//   - text: the text to analyze
	SetText(text string)

	// Current returns the current position.
	//
	// Returns:
	//   - the current position
	Current() int

	// First returns the first boundary position.
	//
	// Returns:
	//   - the first boundary position
	First() int

	// Last returns the last boundary position.
	//
	// Returns:
	//   - the last boundary position
	Last() int

	// Next returns the next boundary position.
	//
	// Returns:
	//   - the next boundary position, or -1 if there are no more boundaries
	Next() int

	// NextWithIndex returns the nth boundary from the current position.
	//
	// Parameters:
	//   - n: the number of boundaries to advance (negative for previous)
	//
	// Returns:
	//   - the boundary position, or -1 if there are no more boundaries
	NextWithIndex(n int) int

	// Previous returns the previous boundary position.
	//
	// Returns:
	//   - the previous boundary position, or -1 if there are no more boundaries
	Previous() int

	// IsBoundary returns true if the given position is a boundary.
	//
	// Parameters:
	//   - position: the position to check
	//
	// Returns:
	//   - true if the position is a boundary
	IsBoundary(position int) bool
}

// SentenceBreakIterator breaks text at sentence boundaries.
//
// This is the Go port of Lucene's org.apache.lucene.search.highlight.SentenceBreakIterator.
type SentenceBreakIterator struct {
	text     string
	position int
	length   int
}

// NewSentenceBreakIterator creates a new SentenceBreakIterator.
//
// Returns:
//   - a new SentenceBreakIterator instance
func NewSentenceBreakIterator() *SentenceBreakIterator {
	return &SentenceBreakIterator{
		position: 0,
	}
}

// SetText sets the text to be analyzed.
//
// Parameters:
//   - text: the text to analyze
func (bi *SentenceBreakIterator) SetText(text string) {
	bi.text = text
	bi.position = 0
	bi.length = len(text)
}

// Current returns the current position.
//
// Returns:
//   - the current position
func (bi *SentenceBreakIterator) Current() int {
	return bi.position
}

// First returns the first boundary position.
//
// Returns:
//   - the first boundary position (always 0)
func (bi *SentenceBreakIterator) First() int {
	bi.position = 0
	return bi.position
}

// Last returns the last boundary position.
//
// Returns:
//   - the last boundary position (always the length of the text)
func (bi *SentenceBreakIterator) Last() int {
	bi.position = bi.length
	return bi.position
}

// Next returns the next boundary position.
//
// Returns:
//   - the next boundary position, or -1 if there are no more boundaries
func (bi *SentenceBreakIterator) Next() int {
	if bi.position >= bi.length {
		return -1
	}

	// Find the next sentence end
	for i := bi.position; i < bi.length; i++ {
		if i < bi.length-1 {
			// Check for sentence-ending punctuation followed by space or end
			if (bi.text[i] == '.' || bi.text[i] == '!' || bi.text[i] == '?') &&
				(i+1 >= bi.length || bi.text[i+1] == ' ' || bi.text[i+1] == '\n' || bi.text[i+1] == '\t') {
				bi.position = i + 1
				return bi.position
			}
		}
	}

	// No more sentence boundaries found
	bi.position = bi.length
	return bi.position
}

// NextWithIndex returns the nth boundary from the current position.
//
// Parameters:
//   - n: the number of boundaries to advance (negative for previous)
//
// Returns:
//   - the boundary position, or -1 if there are no more boundaries
func (bi *SentenceBreakIterator) NextWithIndex(n int) int {
	if n == 0 {
		return bi.position
	}

	if n > 0 {
		for i := 0; i < n; i++ {
			if bi.Next() == -1 {
				return -1
			}
		}
	} else {
		for i := 0; i > n; i-- {
			if bi.Previous() == -1 {
				return -1
			}
		}
	}

	return bi.position
}

// Previous returns the previous boundary position.
//
// Returns:
//   - the previous boundary position, or -1 if there are no more boundaries
func (bi *SentenceBreakIterator) Previous() int {
	if bi.position <= 0 {
		return -1
	}

	// Find the previous sentence end
	for i := bi.position - 1; i >= 0; i-- {
		if i > 0 {
			// Check for sentence-ending punctuation
			if (bi.text[i] == '.' || bi.text[i] == '!' || bi.text[i] == '?') &&
				(i+1 >= bi.length || bi.text[i+1] == ' ' || bi.text[i+1] == '\n' || bi.text[i+1] == '\t') {
				bi.position = i + 1
				return bi.position
			}
		}
	}

	// No more sentence boundaries found
	bi.position = 0
	return bi.position
}

// IsBoundary returns true if the given position is a boundary.
//
// Parameters:
//   - position: the position to check
//
// Returns:
//   - true if the position is a boundary
func (bi *SentenceBreakIterator) IsBoundary(position int) bool {
	if position < 0 || position > bi.length {
		return false
	}

	if position == 0 || position == bi.length {
		return true
	}

	// Check if this is a sentence boundary
	if position > 0 {
		prevChar := bi.text[position-1]
		if prevChar == '.' || prevChar == '!' || prevChar == '?' {
			return true
		}
	}

	return false
}

// Ensure SentenceBreakIterator implements BreakIterator
var _ BreakIterator = (*SentenceBreakIterator)(nil)

// WordBreakIterator breaks text at word boundaries.
//
// This is the Go port of Lucene's org.apache.lucene.search.highlight.WordBreakIterator.
type WordBreakIterator struct {
	text     string
	position int
	length   int
}

// NewWordBreakIterator creates a new WordBreakIterator.
//
// Returns:
//   - a new WordBreakIterator instance
func NewWordBreakIterator() *WordBreakIterator {
	return &WordBreakIterator{
		position: 0,
	}
}

// SetText sets the text to be analyzed.
//
// Parameters:
//   - text: the text to analyze
func (bi *WordBreakIterator) SetText(text string) {
	bi.text = text
	bi.position = 0
	bi.length = len(text)
}

// Current returns the current position.
//
// Returns:
//   - the current position
func (bi *WordBreakIterator) Current() int {
	return bi.position
}

// First returns the first boundary position.
//
// Returns:
//   - the first boundary position (always 0)
func (bi *WordBreakIterator) First() int {
	bi.position = 0
	return bi.position
}

// Last returns the last boundary position.
//
// Returns:
//   - the last boundary position (always the length of the text)
func (bi *WordBreakIterator) Last() int {
	bi.position = bi.length
	return bi.position
}

// Next returns the next boundary position.
//
// Returns:
//   - the next boundary position, or -1 if there are no more boundaries
func (bi *WordBreakIterator) Next() int {
	if bi.position >= bi.length {
		return -1
	}

	// Skip non-word characters
	for bi.position < bi.length && !isWordChar(rune(bi.text[bi.position])) {
		bi.position++
	}

	// Skip word characters
	for bi.position < bi.length && isWordChar(rune(bi.text[bi.position])) {
		bi.position++
	}

	return bi.position
}

// NextWithIndex returns the nth boundary from the current position.
//
// Parameters:
//   - n: the number of boundaries to advance (negative for previous)
//
// Returns:
//   - the boundary position, or -1 if there are no more boundaries
func (bi *WordBreakIterator) NextWithIndex(n int) int {
	if n == 0 {
		return bi.position
	}

	if n > 0 {
		for i := 0; i < n; i++ {
			if bi.Next() == -1 {
				return -1
			}
		}
	} else {
		for i := 0; i > n; i-- {
			if bi.Previous() == -1 {
				return -1
			}
		}
	}

	return bi.position
}

// Previous returns the previous boundary position.
//
// Returns:
//   - the previous boundary position, or -1 if there are no more boundaries
func (bi *WordBreakIterator) Previous() int {
	if bi.position <= 0 {
		return -1
	}

	// Skip non-word characters going backwards
	for bi.position > 0 && !isWordChar(rune(bi.text[bi.position-1])) {
		bi.position--
	}

	// Skip word characters going backwards
	for bi.position > 0 && isWordChar(rune(bi.text[bi.position-1])) {
		bi.position--
	}

	return bi.position
}

// IsBoundary returns true if the given position is a boundary.
//
// Parameters:
//   - position: the position to check
//
// Returns:
//   - true if the position is a boundary
func (bi *WordBreakIterator) IsBoundary(position int) bool {
	if position < 0 || position > bi.length {
		return false
	}

	if position == 0 || position == bi.length {
		return true
	}

	// Check if this is a word boundary
	prevChar := rune(bi.text[position-1])
	nextChar := rune(bi.text[position])

	return isWordChar(prevChar) != isWordChar(nextChar)
}

// isWordChar returns true if the character is a word character.
func isWordChar(ch rune) bool {
	return unicode.IsLetter(ch) || unicode.IsDigit(ch)
}

// Ensure WordBreakIterator implements BreakIterator
var _ BreakIterator = (*WordBreakIterator)(nil)

// WholeBreakIterator treats the entire text as a single fragment.
//
// This is useful when you want to highlight the entire text without breaking it.
type WholeBreakIterator struct {
	text     string
	position int
	length   int
}

// NewWholeBreakIterator creates a new WholeBreakIterator.
//
// Returns:
//   - a new WholeBreakIterator instance
func NewWholeBreakIterator() *WholeBreakIterator {
	return &WholeBreakIterator{
		position: 0,
	}
}

// SetText sets the text to be analyzed.
//
// Parameters:
//   - text: the text to analyze
func (bi *WholeBreakIterator) SetText(text string) {
	bi.text = text
	bi.position = 0
	bi.length = utf8.RuneCountInString(text)
}

// Current returns the current position.
//
// Returns:
//   - the current position
func (bi *WholeBreakIterator) Current() int {
	return bi.position
}

// First returns the first boundary position.
//
// Returns:
//   - the first boundary position (always 0)
func (bi *WholeBreakIterator) First() int {
	bi.position = 0
	return bi.position
}

// Last returns the last boundary position.
//
// Returns:
//   - the last boundary position (always the length of the text)
func (bi *WholeBreakIterator) Last() int {
	bi.position = bi.length
	return bi.position
}

// Next returns the next boundary position.
//
// Returns:
//   - the next boundary position, or -1 if there are no more boundaries
func (bi *WholeBreakIterator) Next() int {
	if bi.position >= bi.length {
		return -1
	}
	bi.position = bi.length
	return bi.position
}

// NextWithIndex returns the nth boundary from the current position.
//
// Parameters:
//   - n: the number of boundaries to advance (negative for previous)
//
// Returns:
//   - the boundary position, or -1 if there are no more boundaries
func (bi *WholeBreakIterator) NextWithIndex(n int) int {
	if n == 0 {
		return bi.position
	}
	if n > 0 && bi.position < bi.length {
		bi.position = bi.length
		return bi.position
	}
	if n < 0 && bi.position > 0 {
		bi.position = 0
		return bi.position
	}
	return -1
}

// Previous returns the previous boundary position.
//
// Returns:
//   - the previous boundary position, or -1 if there are no more boundaries
func (bi *WholeBreakIterator) Previous() int {
	if bi.position <= 0 {
		return -1
	}
	bi.position = 0
	return bi.position
}

// IsBoundary returns true if the given position is a boundary.
//
// Parameters:
//   - position: the position to check
//
// Returns:
//   - true if the position is a boundary
func (bi *WholeBreakIterator) IsBoundary(position int) bool {
	return position == 0 || position == bi.length
}

// Ensure WholeBreakIterator implements BreakIterator
var _ BreakIterator = (*WholeBreakIterator)(nil)

// BreakIteratorFactory creates BreakIterator instances.
type BreakIteratorFactory struct {
	breakType string
}

// NewBreakIteratorFactory creates a new BreakIteratorFactory.
//
// Parameters:
//   - breakType: the type of break iterator to create ("sentence", "word", "whole")
//
// Returns:
//   - a new BreakIteratorFactory instance
func NewBreakIteratorFactory(breakType string) *BreakIteratorFactory {
	return &BreakIteratorFactory{
		breakType: breakType,
	}
}

// CreateBreakIterator creates a BreakIterator.
//
// Returns:
//   - a new BreakIterator instance
func (f *BreakIteratorFactory) CreateBreakIterator() BreakIterator {
	switch f.breakType {
	case "sentence":
		return NewSentenceBreakIterator()
	case "word":
		return NewWordBreakIterator()
	case "whole":
		return NewWholeBreakIterator()
	default:
		return NewSentenceBreakIterator()
	}
}

// GetBreakType returns the break type.
//
// Returns:
//   - the break type
func (f *BreakIteratorFactory) GetBreakType() string {
	return f.breakType
}
