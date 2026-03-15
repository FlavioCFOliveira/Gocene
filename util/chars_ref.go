// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"fmt"
	"math/rand"
	"strings"
)

// EmptyChars is an empty character array for convenience
var EmptyChars = []rune{}

// CharsRef represents a slice (offset + length) into an existing rune slice.
// This is the Go port of Lucene's org.apache.lucene.util.CharsRef.
type CharsRef struct {
	// Chars is the underlying rune slice. Should never be nil.
	Chars []rune

	// Offset is the offset of the first valid character
	Offset int

	// Length is the length of used characters
	Length int
}

// NewCharsRef creates a new CharsRef initialized with an empty array
func NewCharsRef() *CharsRef {
	return &CharsRef{
		Chars:  EmptyChars,
		Offset: 0,
		Length: 0,
	}
}

// NewCharsRefWithCapacity creates a new CharsRef with an array of the given capacity
func NewCharsRefWithCapacity(capacity int) *CharsRef {
	return &CharsRef{
		Chars:  make([]rune, 0, capacity),
		Offset: 0,
		Length: 0,
	}
}

// NewCharsRefFromRunes creates a new CharsRef from a rune slice
func NewCharsRefFromRunes(chars []rune, offset, length int) *CharsRef {
	ref := &CharsRef{
		Chars:  chars,
		Offset: offset,
		Length: length,
	}
	return ref
}

// NewCharsRefFromString creates a new CharsRef from a string
func NewCharsRefFromString(s string) *CharsRef {
	chars := []rune(s)
	return &CharsRef{
		Chars:  chars,
		Offset: 0,
		Length: len(chars),
	}
}

// Clone returns a shallow clone of this instance (the underlying runes are NOT copied)
func (c *CharsRef) Clone() *CharsRef {
	if c == nil {
		return nil
	}
	return &CharsRef{
		Chars:  c.Chars,
		Offset: c.Offset,
		Length: c.Length,
	}
}

// HashCode returns the hash code for this CharsRef
func (c *CharsRef) HashCode() int {
	if c == nil || c.Length == 0 {
		return 0
	}
	return StringHashCode(c.Chars, c.Offset, c.Length)
}

// StringHashCode calculates the hash code of a rune sub-array
func StringHashCode(chars []rune, offset, length int) int {
	end := offset + length
	result := 0
	for i := offset; i < end; i++ {
		result = 31*result + int(chars[i])
	}
	return result
}

// Equals checks if this CharsRef equals another object
func (c *CharsRef) Equals(other interface{}) bool {
	if other == nil {
		return false
	}
	if o, ok := other.(*CharsRef); ok {
		return c.CharsEquals(o)
	}
	return false
}

// CharsEquals checks if this CharsRef equals another CharsRef
func (c *CharsRef) CharsEquals(other *CharsRef) bool {
	if c == other {
		return true
	}
	if c == nil || other == nil {
		return false
	}
	if c.Length != other.Length {
		return false
	}
	for i := 0; i < c.Length; i++ {
		if c.Chars[c.Offset+i] != other.Chars[other.Offset+i] {
			return false
		}
	}
	return true
}

// CompareTo compares this CharsRef to another (signed int order comparison)
func (c *CharsRef) CompareTo(other *CharsRef) int {
	if c == other {
		return 0
	}
	if c == nil {
		return -1
	}
	if other == nil {
		return 1
	}

	minLen := c.Length
	if other.Length < minLen {
		minLen = other.Length
	}

	for i := 0; i < minLen; i++ {
		aChar := c.Chars[c.Offset+i]
		bChar := other.Chars[other.Offset+i]
		if aChar < bChar {
			return -1
		}
		if aChar > bChar {
			return 1
		}
	}

	return c.Length - other.Length
}

// String returns a string representation of this CharsRef
func (c *CharsRef) String() string {
	if c == nil || c.Length == 0 {
		return ""
	}
	return string(c.Chars[c.Offset : c.Offset+c.Length])
}

// Len returns the length of this CharsRef (for sort.Interface compatibility)
func (c *CharsRef) Len() int {
	if c == nil {
		return 0
	}
	return c.Length
}

// CharAt returns the character at the given index
// NOTE: must do a real check here to meet the specs of CharSequence
func (c *CharsRef) CharAt(index int) rune {
	if index < 0 || index >= c.Length {
		panic(fmt.Sprintf("Index out of bounds: index=%d, length=%d", index, c.Length))
	}
	return c.Chars[c.Offset+index]
}

// SubSequence returns a sub-sequence of this CharsRef
// NOTE: must do a real check here to meet the specs of CharSequence
func (c *CharsRef) SubSequence(start, end int) *CharsRef {
	if start < 0 || end < 0 || end > c.Length || start > end {
		panic(fmt.Sprintf("Index out of bounds: start=%d, end=%d, length=%d", start, end, c.Length))
	}
	return &CharsRef{
		Chars:  c.Chars,
		Offset: c.Offset + start,
		Length: end - start,
	}
}

// UTF16SortedAsUTF8Comparator returns a comparator that sorts UTF-16 as UTF-8
// This is a transition mechanism and is deprecated in Lucene
func UTF16SortedAsUTF8Comparator() func(a, b *CharsRef) int {
	return func(a, b *CharsRef) int {
		_ = a.Offset + a.Length // aEnd - not used but kept for documentation
		_ = b.Offset + b.Length // bEnd - not used but kept for documentation

		// Find first mismatch
		minLen := a.Length
		if b.Length < minLen {
			minLen = b.Length
		}

		for i := 0; i < minLen; i++ {
			aChar := a.Chars[a.Offset+i]
			bChar := b.Chars[b.Offset+i]

			if aChar != bChar {
				// Fix up each one if they're both in or above the surrogate range
				// http://icu-project.org/docs/papers/utf16_code_point_order.html
				if aChar >= 0xd800 && bChar >= 0xd800 {
					if aChar >= 0xe000 {
						aChar -= 0x800
					} else {
						aChar += 0x2000
					}

					if bChar >= 0xe000 {
						bChar -= 0x800
					} else {
						bChar += 0x2000
					}
				}
				return int(aChar) - int(bChar)
			}
		}

		// One is a prefix of the other, or they are equal
		return a.Length - b.Length
	}
}

// DeepCopyOf creates a new CharsRef that points to a copy of the chars from other
func DeepCopyOf(other *CharsRef) *CharsRef {
	if other == nil {
		return nil
	}
	if other.Length == 0 {
		return NewCharsRef()
	}
	copied := make([]rune, other.Length)
	copy(copied, other.Chars[other.Offset:other.Offset+other.Length])
	return &CharsRef{
		Chars:  copied,
		Offset: 0,
		Length: other.Length,
	}
}

// IsValid performs internal consistency checks
func (c *CharsRef) IsValid() bool {
	if c.Chars == nil {
		return c.Length == 0
	}
	if c.Length < 0 {
		return false
	}
	if c.Length > len(c.Chars) {
		return false
	}
	if c.Offset < 0 {
		return false
	}
	if c.Offset > len(c.Chars) {
		return false
	}
	if c.Offset+c.Length > len(c.Chars) {
		return false
	}
	return true
}

// ValidChars returns the slice of valid runes
func (c *CharsRef) ValidChars() []rune {
	if c == nil || c.Length == 0 {
		return nil
	}
	return c.Chars[c.Offset : c.Offset+c.Length]
}

// CharsRefSlice is a slice of CharsRef pointers for sorting
type CharsRefSlice []*CharsRef

func (s CharsRefSlice) Len() int           { return len(s) }
func (s CharsRefSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s CharsRefSlice) Less(i, j int) bool { return s[i].CompareTo(s[j]) < 0 }

// CharsRefSliceUTF8 is a slice of CharsRef pointers for UTF-8 sorting
type CharsRefSliceUTF8 []*CharsRef

func (s CharsRefSliceUTF8) Len() int      { return len(s) }
func (s CharsRefSliceUTF8) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s CharsRefSliceUTF8) Less(i, j int) bool {
	cmp := UTF16SortedAsUTF8Comparator()
	return cmp(s[i], s[j]) < 0
}

// CopyOfSubArray copies a sub-array of runes
func CopyOfSubArray(chars []rune, from, to int) []rune {
	if from < 0 {
		from = 0
	}
	if to > len(chars) {
		to = len(chars)
	}
	if from >= to {
		return EmptyChars
	}
	result := make([]rune, to-from)
	copy(result, chars[from:to])
	return result
}

// Grow grows the given rune slice to accommodate at least minSize elements
func Grow(chars []rune, minSize int) []rune {
	if cap(chars) >= minSize {
		if len(chars) < minSize {
			return chars[:minSize]
		}
		return chars
	}
	newSize := cap(chars)
	if newSize == 0 {
		newSize = 1
	}
	for newSize < minSize {
		newSize = newSize << 1
		if newSize < 0 {
			newSize = minSize
			break
		}
	}
	newChars := make([]rune, len(chars), newSize)
	copy(newChars, chars)
	return newChars
}

// RandomUnicodeString generates a random Unicode string for testing
func RandomUnicodeString(r *rand.Rand, maxLength int) string {
	if maxLength <= 0 {
		return ""
	}
	length := r.Intn(maxLength) + 1
	var sb strings.Builder
	for i := 0; i < length; i++ {
		// Generate random Unicode code point (including supplementary characters)
		codePoint := r.Intn(0x10FFFF + 1)
		if codePoint < 0x10000 {
			sb.WriteRune(rune(codePoint))
		} else {
			// Supplementary character - write as surrogate pair would in UTF-16
			sb.WriteRune(rune(codePoint))
		}
	}
	return sb.String()
}

// RandomRealisticUnicodeString generates a more realistic random Unicode string
func RandomRealisticUnicodeString(r *rand.Rand, minLength, maxLength int) string {
	if maxLength <= 0 || minLength > maxLength {
		return ""
	}
	length := minLength
	if maxLength > minLength {
		length += r.Intn(maxLength - minLength)
	}
	var sb strings.Builder
	for i := 0; i < length; i++ {
		// Mostly ASCII with some higher Unicode
		var codePoint int
		if r.Float64() < 0.9 {
			codePoint = r.Intn(128) // ASCII
		} else {
			codePoint = r.Intn(0xFFFF + 1)
		}
		sb.WriteRune(rune(codePoint))
	}
	return sb.String()
}

// AtLeast returns at least the given value (for test iterations)
func AtLeast(r *rand.Rand, n int) int {
	return n + r.Intn(n)
}
