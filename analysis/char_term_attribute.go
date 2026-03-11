// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// CharTermAttribute stores the text of a token.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.tokenattributes.CharTermAttribute.
//
// This attribute holds the actual text content of a token after tokenization.
// Tokenizers set this attribute, and filters may modify it.
type CharTermAttribute interface {
	AttributeImpl

	// SetEmpty clears the term buffer and sets it to empty.
	SetEmpty()

	// Append adds bytes to the term buffer.
	// Returns the attribute for method chaining.
	Append(bytes []byte) CharTermAttribute

	// AppendString adds a string to the term buffer.
	// Returns the attribute for method chaining.
	AppendString(str string) CharTermAttribute

	// AppendChar adds a single byte (char) to the term buffer.
	// Returns the attribute for method chaining.
	AppendChar(b byte) CharTermAttribute

	// AppendChars adds a byte slice to the term buffer.
	// Returns the attribute for method chaining.
	AppendChars(chars []byte) CharTermAttribute

	// SetValue sets the term text from a string.
	// Returns the attribute for method chaining.
	SetValue(str string) CharTermAttribute

	// SetEmptyAndGet returns the internal buffer after clearing it.
	// This is useful for filling the buffer directly.
	SetEmptyAndGet() []byte

	// Buffer returns the internal term buffer.
	// The returned slice may be modified until the next token.
	Buffer() []byte

	// Bytes returns a copy of the current term as a byte slice.
	Bytes() []byte

	// String returns the current term as a string.
	String() string

	// Length returns the length of the term in bytes.
	Length() int

	// ResizeBuffer resizes the internal buffer to at least the given capacity.
	// Returns the resized buffer.
	ResizeBuffer(newSize int) []byte

	// SetLength sets the length of the term.
	SetLength(length int)

	// Grow grows the buffer to accommodate at least minCapacity bytes.
	// Returns the grown buffer.
	Grow(minCapacity int) []byte
}

// charTermAttribute is the default implementation of CharTermAttribute.
type charTermAttribute struct {
	termBuffer []byte
	termLength int
}

// NewCharTermAttribute creates a new empty CharTermAttribute.
func NewCharTermAttribute() CharTermAttribute {
	return &charTermAttribute{
		termBuffer: make([]byte, 0, 16),
		termLength: 0,
	}
}

// Clear clears this attribute.
func (a *charTermAttribute) Clear() {
	a.termLength = 0
}

// CopyTo copies this attribute to another implementation.
func (a *charTermAttribute) CopyTo(target AttributeImpl) {
	if t, ok := target.(CharTermAttribute); ok {
		t.SetValue(a.String())
	}
}

// SetEmpty clears the term buffer.
func (a *charTermAttribute) SetEmpty() {
	a.termLength = 0
}

// Append adds bytes to the term buffer.
func (a *charTermAttribute) Append(bytes []byte) CharTermAttribute {
	a.Grow(a.termLength + len(bytes))
	copy(a.termBuffer[a.termLength:], bytes)
	a.termLength += len(bytes)
	return a
}

// AppendString adds a string to the term buffer.
func (a *charTermAttribute) AppendString(str string) CharTermAttribute {
	return a.Append([]byte(str))
}

// AppendChar adds a single byte to the term buffer.
func (a *charTermAttribute) AppendChar(b byte) CharTermAttribute {
	a.Grow(a.termLength + 1)
	a.termBuffer[a.termLength] = b
	a.termLength++
	return a
}

// AppendChars adds a byte slice to the term buffer.
func (a *charTermAttribute) AppendChars(chars []byte) CharTermAttribute {
	return a.Append(chars)
}

// SetValue sets the term text from a string.
func (a *charTermAttribute) SetValue(str string) CharTermAttribute {
	a.termBuffer = []byte(str)
	a.termLength = len(a.termBuffer)
	return a
}

// SetEmptyAndGet returns the internal buffer after clearing it.
func (a *charTermAttribute) SetEmptyAndGet() []byte {
	a.termLength = 0
	return a.termBuffer
}

// Buffer returns the internal term buffer.
func (a *charTermAttribute) Buffer() []byte {
	return a.termBuffer
}

// Bytes returns a copy of the current term.
func (a *charTermAttribute) Bytes() []byte {
	result := make([]byte, a.termLength)
	copy(result, a.termBuffer[:a.termLength])
	return result
}

// String returns the current term as a string.
func (a *charTermAttribute) String() string {
	return string(a.termBuffer[:a.termLength])
}

// Length returns the length of the term.
func (a *charTermAttribute) Length() int {
	return a.termLength
}

// ResizeBuffer resizes the internal buffer.
func (a *charTermAttribute) ResizeBuffer(newSize int) []byte {
	if newSize > len(a.termBuffer) {
		newBuffer := make([]byte, newSize)
		copy(newBuffer, a.termBuffer)
		a.termBuffer = newBuffer
	}
	return a.termBuffer
}

// SetLength sets the length of the term.
func (a *charTermAttribute) SetLength(length int) {
	if length > len(a.termBuffer) {
		panic("length exceeds buffer size")
	}
	a.termLength = length
}

// Grow grows the buffer to accommodate at least minCapacity bytes.
func (a *charTermAttribute) Grow(minCapacity int) []byte {
	if minCapacity > len(a.termBuffer) {
		// Grow by 50% or to minCapacity, whichever is larger
		newCapacity := len(a.termBuffer) * 3 / 2
		if newCapacity < minCapacity {
			newCapacity = minCapacity
		}
		if newCapacity < 16 {
			newCapacity = 16
		}
		newBuffer := make([]byte, newCapacity)
		copy(newBuffer, a.termBuffer)
		a.termBuffer = newBuffer
	}
	return a.termBuffer
}
