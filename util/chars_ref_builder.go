// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

// CharsRefBuilder is a builder for CharsRef instances.
// This is the Go port of Lucene's org.apache.lucene.util.CharsRefBuilder.
type CharsRefBuilder struct {
	ref *CharsRef
}

// NewCharsRefBuilder creates a new CharsRefBuilder
func NewCharsRefBuilder() *CharsRefBuilder {
	return &CharsRefBuilder{
		ref: NewCharsRef(),
	}
}

// Chars returns a reference to the chars of this builder
func (b *CharsRefBuilder) Chars() []rune {
	return b.ref.Chars
}

// Length returns the number of chars in this buffer
func (b *CharsRefBuilder) Length() int {
	return b.ref.Length
}

// SetLength sets the length
func (b *CharsRefBuilder) SetLength(length int) {
	b.ref.Length = length
}

// CharAt returns the char at the given offset
func (b *CharsRefBuilder) CharAt(offset int) rune {
	return b.ref.Chars[offset]
}

// SetCharAt sets a char at the given offset
func (b *CharsRefBuilder) SetCharAt(offset int, ch rune) {
	b.ref.Chars[offset] = ch
}

// Clear resets this builder to the empty state
func (b *CharsRefBuilder) Clear() {
	b.ref.Length = 0
}

// Append appends a CharSequence to this builder
func (b *CharsRefBuilder) Append(csq string) *CharsRefBuilder {
	if csq == "" {
		return b
	}
	return b.AppendRange(csq, 0, len(csq))
}

// AppendRange appends a substring of a CharSequence to this builder
func (b *CharsRefBuilder) AppendRange(csq string, start, end int) *CharsRefBuilder {
	if csq == "" {
		return b
	}
	b.Grow(b.ref.Length + end - start)
	for i := start; i < end; i++ {
		b.SetCharAt(b.ref.Length, rune(csq[i]))
		b.ref.Length++
	}
	return b
}

// AppendChar appends a single char to this builder
func (b *CharsRefBuilder) AppendChar(c rune) *CharsRefBuilder {
	newLen := b.ref.Length + 1
	b.Grow(newLen)
	// Extend the slice to accommodate the new character
	if len(b.ref.Chars) < newLen {
		b.ref.Chars = b.ref.Chars[:newLen]
	}
	b.ref.Chars[b.ref.Length] = c
	b.ref.Length = newLen
	return b
}

// AppendRunes appends a rune slice to this builder
func (b *CharsRefBuilder) AppendRunes(otherChars []rune, otherOffset, otherLength int) {
	if otherLength <= 0 {
		return
	}
	newLen := b.ref.Length + otherLength
	b.Grow(newLen)
	// Extend the slice to accommodate the new characters
	if len(b.ref.Chars) < newLen {
		b.ref.Chars = b.ref.Chars[:newLen]
	}
	// Copy to the end of current content
	copy(b.ref.Chars[b.ref.Length:newLen], otherChars[otherOffset:otherOffset+otherLength])
	b.ref.Length = newLen
}

// CopyChars copies the given CharsRef referenced content into this instance
func (b *CharsRefBuilder) CopyChars(other *CharsRef) {
	if other == nil {
		b.Clear()
		return
	}
	b.CopyRunes(other.Chars, other.Offset, other.Length)
}

// CopyRunes copies the given array into this instance
func (b *CharsRefBuilder) CopyRunes(otherChars []rune, otherOffset, otherLength int) {
	b.Grow(otherLength)
	// Extend the slice to accommodate the new characters
	if len(b.ref.Chars) < otherLength {
		b.ref.Chars = b.ref.Chars[:otherLength]
	}
	copy(b.ref.Chars, otherChars[otherOffset:otherOffset+otherLength])
	b.ref.Length = otherLength
}

// Grow grows the reference array to accommodate at least newLength elements
func (b *CharsRefBuilder) Grow(newLength int) {
	if cap(b.ref.Chars) >= newLength {
		return
	}
	// Need to grow - allocate new slice with sufficient capacity
	newChars := make([]rune, len(b.ref.Chars), newLength)
	copy(newChars, b.ref.Chars)
	b.ref.Chars = newChars
}

// Get returns a CharsRef that points to the internal content of this builder
// Any update to the content of this builder might invalidate the returned ref
func (b *CharsRefBuilder) Get() *CharsRef {
	if b.ref.Offset != 0 {
		panic("Modifying the offset of the returned ref is illegal")
	}
	return b.ref
}

// ToCharsRef builds a new CharsRef that has the same content as this builder
func (b *CharsRefBuilder) ToCharsRef() *CharsRef {
	return DeepCopyOf(b.ref)
}

// String returns a string representation of this builder
func (b *CharsRefBuilder) String() string {
	return b.Get().String()
}

// Equals is not supported and will panic
func (b *CharsRefBuilder) Equals(obj interface{}) bool {
	panic("UnsupportedOperationException: equals is not supported")
}

// HashCode is not supported and will panic
func (b *CharsRefBuilder) HashCode() int {
	panic("UnsupportedOperationException: hashCode is not supported")
}
