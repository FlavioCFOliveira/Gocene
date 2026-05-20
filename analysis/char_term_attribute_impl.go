// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
// lucene/core/src/java/org/apache/lucene/analysis/tokenattributes/CharTermAttributeImpl.java

package analysis

import (
	"bytes"
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// minCharTermBufferSize is the minimum initial buffer capacity for a
// CharTermAttributeImpl, matching Lucene's MIN_BUFFER_SIZE = 10.
const minCharTermBufferSize = 10

// CharTermAttributeImpl is the Go port of Lucene's
// org.apache.lucene.analysis.tokenattributes.CharTermAttributeImpl.
//
// It is the exported concrete implementation of [CharTermAttribute]
// (which extends [TermToBytesRefAttribute] in Gocene). The term is
// stored in a growable []byte buffer (Gocene uses UTF-8 bytes where
// Lucene uses UTF-16 char[]).
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/analysis/tokenattributes/CharTermAttributeImpl.java
type CharTermAttributeImpl struct {
	termBuffer []byte
	termLength int

	// builder is a reused BytesRef returned by GetBytesRef, avoiding
	// per-call allocations on the indexing hot path (Lucene parity).
	builder *util.BytesRef
}

// Compile-time assertions.
var (
	_ util.AttributeImpl              = (*CharTermAttributeImpl)(nil)
	_ CharTermAttribute               = (*CharTermAttributeImpl)(nil)
	_ TermToBytesRefAttribute         = (*CharTermAttributeImpl)(nil)
	_ util.AttributeInterfaceProvider = (*CharTermAttributeImpl)(nil)
)

// AttributeInterfaces satisfies [util.AttributeInterfaceProvider].
func (c *CharTermAttributeImpl) AttributeInterfaces() []reflect.Type {
	return []reflect.Type{CharTermAttributeType, TermToBytesRefAttributeType}
}

// NewCharTermAttributeImpl initialises this attribute with an empty
// term buffer, matching the Lucene no-arg constructor.
func NewCharTermAttributeImpl() *CharTermAttributeImpl {
	return &CharTermAttributeImpl{
		termBuffer: make([]byte, 0, oversizeCharBuf(minCharTermBufferSize)),
	}
}

// oversizeCharBuf returns the oversize capacity for n bytes using the
// same 3/2 growth factor as Lucene's ArrayUtil.oversize for char[].
func oversizeCharBuf(n int) int {
	cap := n * 3 / 2
	if cap < 16 {
		cap = 16
	}
	return cap
}

// --- CharTermAttribute / CharSequence-style interface ---

// Clear resets the term length to 0, matching
// {@code CharTermAttributeImpl#clear()}.
func (c *CharTermAttributeImpl) Clear() { c.termLength = 0 }

// SetEmpty clears the term buffer and returns this attribute for
// chaining, matching {@code CharTermAttributeImpl#setEmpty()}.
func (c *CharTermAttributeImpl) SetEmpty() { c.termLength = 0 }

// Length returns the current term length in bytes,
// matching {@code CharTermAttributeImpl#length()}.
func (c *CharTermAttributeImpl) Length() int { return c.termLength }

// Buffer returns the internal term buffer, matching
// {@code CharTermAttributeImpl#buffer()}.
func (c *CharTermAttributeImpl) Buffer() []byte { return c.termBuffer }

// Bytes returns a copy of the current term as a byte slice.
func (c *CharTermAttributeImpl) Bytes() []byte {
	out := make([]byte, c.termLength)
	copy(out, c.termBuffer[:c.termLength])
	return out
}

// String returns the current term as a string, matching
// {@code CharTermAttributeImpl#toString()}.
func (c *CharTermAttributeImpl) String() string {
	return string(c.termBuffer[:c.termLength])
}

// SetLength sets the term length. Panics if length exceeds the buffer,
// matching Lucene's Objects.checkFromIndexSize.
func (c *CharTermAttributeImpl) SetLength(length int) {
	if length < 0 || length > len(c.termBuffer) {
		panic("CharTermAttributeImpl.SetLength: length out of range")
	}
	c.termLength = length
}

// ResizeBuffer grows the buffer to at least newSize bytes, preserving
// existing content, matching {@code CharTermAttributeImpl#resizeBuffer}.
func (c *CharTermAttributeImpl) ResizeBuffer(newSize int) []byte {
	if len(c.termBuffer) < newSize {
		newBuf := make([]byte, oversizeCharBuf(newSize))
		copy(newBuf, c.termBuffer)
		c.termBuffer = newBuf
	}
	return c.termBuffer
}

// Grow grows the buffer to accommodate at least minCapacity bytes,
// matching the internal growTermBuffer helper.
func (c *CharTermAttributeImpl) Grow(minCapacity int) []byte {
	if len(c.termBuffer) < minCapacity {
		newBuf := make([]byte, oversizeCharBuf(minCapacity))
		copy(newBuf, c.termBuffer)
		c.termBuffer = newBuf
	}
	return c.termBuffer
}

// SetEmptyAndGet clears the term and returns the underlying buffer.
func (c *CharTermAttributeImpl) SetEmptyAndGet() []byte {
	c.termLength = 0
	return c.termBuffer
}

// SetValue sets the term from a string and returns the attribute.
func (c *CharTermAttributeImpl) SetValue(s string) CharTermAttribute {
	b := []byte(s)
	c.Grow(len(b))
	copy(c.termBuffer, b)
	c.termLength = len(b)
	return c
}

// Append appends a byte slice and returns the attribute.
func (c *CharTermAttributeImpl) Append(b []byte) CharTermAttribute {
	c.Grow(c.termLength + len(b))
	copy(c.termBuffer[c.termLength:], b)
	c.termLength += len(b)
	return c
}

// AppendString appends a string and returns the attribute.
func (c *CharTermAttributeImpl) AppendString(s string) CharTermAttribute {
	return c.Append([]byte(s))
}

// AppendChar appends a single byte and returns the attribute.
func (c *CharTermAttributeImpl) AppendChar(b byte) CharTermAttribute {
	c.Grow(c.termLength + 1)
	c.termBuffer[c.termLength] = b
	c.termLength++
	return c
}

// AppendChars appends a byte slice and returns the attribute (alias for
// Append; provided to satisfy the full CharTermAttribute contract).
func (c *CharTermAttributeImpl) AppendChars(chars []byte) CharTermAttribute {
	return c.Append(chars)
}

// --- TermToBytesRefAttribute ---

// GetBytesRef satisfies [TermToBytesRefAttribute]. Returns a reused
// BytesRef wrapping the live buffer — valid until the next mutation.
func (c *CharTermAttributeImpl) GetBytesRef() *util.BytesRef {
	if c.builder == nil {
		c.builder = &util.BytesRef{}
	}
	c.builder.Bytes = c.termBuffer
	c.builder.Offset = 0
	c.builder.Length = c.termLength
	return c.builder
}

// --- AttributeImpl ---

// End resets the attribute, matching the Lucene base default.
func (c *CharTermAttributeImpl) End() { c.Clear() }

// CloneAttribute returns a deep copy as [util.AttributeImpl].
func (c *CharTermAttributeImpl) CloneAttribute() util.AttributeImpl {
	return c.Copy()
}

// Copy returns a deep clone, matching Lucene's clone() contract.
func (c *CharTermAttributeImpl) Copy() util.AttributeImpl {
	clone := &CharTermAttributeImpl{
		termBuffer: make([]byte, c.termLength),
		termLength: c.termLength,
	}
	copy(clone.termBuffer, c.termBuffer[:c.termLength])
	return clone
}

// CopyTo deep-copies this impl's term onto target, which must
// implement [CharTermAttribute]; a panic is raised otherwise.
func (c *CharTermAttributeImpl) CopyTo(target util.AttributeImpl) {
	t, ok := target.(CharTermAttribute)
	if !ok {
		panic("CharTermAttributeImpl.CopyTo: target must implement CharTermAttribute")
	}
	t.SetValue(c.String())
}

// ReflectWith emits the two triples expected by the Lucene reference:
// (CharTermAttribute, "term", string) and
// (TermToBytesRefAttribute, "bytes", BytesRef).
func (c *CharTermAttributeImpl) ReflectWith(reflector util.AttributeReflector) {
	reflector(CharTermAttributeType, "term", c.String())
	reflector(reflect.TypeOf((*TermToBytesRefAttribute)(nil)).Elem(), "bytes", c.GetBytesRef())
}

// Equals returns true if other is a [CharTermAttributeImpl] whose term
// bytes compare equal, matching Lucene's {@code equals(Object)}.
func (c *CharTermAttributeImpl) Equals(other any) bool {
	if c == other {
		return true
	}
	o, ok := other.(*CharTermAttributeImpl)
	if !ok {
		return false
	}
	if c.termLength != o.termLength {
		return false
	}
	return bytes.Equal(c.termBuffer[:c.termLength], o.termBuffer[:o.termLength])
}

// HashCode returns the Lucene-parity hash over the term bytes:
// code = termLength; code = code*31 + byte[i] for each byte.
func (c *CharTermAttributeImpl) HashCode() int {
	code := c.termLength
	for i := 0; i < c.termLength; i++ {
		code = code*31 + int(int8(c.termBuffer[i]))
	}
	return code
}
