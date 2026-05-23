// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

// OpenStringBuilder is a rune-slice builder that exposes its internal array,
// allowing callers to read and write individual code points efficiently.
//
// Go port of org.apache.lucene.analysis.util.OpenStringBuilder (Apache Lucene
// 10.4.0). The Java original operates on UTF-16 chars; this port operates on
// Go runes (Unicode code points) to match Gocene conventions.
type OpenStringBuilder struct {
	buf []rune
	len int
}

// NewOpenStringBuilder creates an OpenStringBuilder with an initial capacity of 32.
func NewOpenStringBuilder() *OpenStringBuilder {
	return &OpenStringBuilder{buf: make([]rune, 32)}
}

// NewOpenStringBuilderWithSize creates an OpenStringBuilder with the given capacity.
func NewOpenStringBuilderWithSize(size int) *OpenStringBuilder {
	return &OpenStringBuilder{buf: make([]rune, size)}
}

// NewOpenStringBuilderFromArray creates an OpenStringBuilder backed by arr with
// length len.
func NewOpenStringBuilderFromArray(arr []rune, length int) *OpenStringBuilder {
	return &OpenStringBuilder{buf: arr, len: length}
}

// SetLength sets the logical length (does not clear or zero the array).
func (b *OpenStringBuilder) SetLength(n int) { b.len = n }

// Set replaces the backing array and sets the logical length.
func (b *OpenStringBuilder) Set(arr []rune, end int) {
	b.buf = arr
	b.len = end
}

// GetArray returns the backing rune slice (may be longer than Len).
func (b *OpenStringBuilder) GetArray() []rune { return b.buf }

// Size returns the logical length.
func (b *OpenStringBuilder) Size() int { return b.len }

// Len returns the logical length (alias for Size, mirrors len() convention).
func (b *OpenStringBuilder) Len() int { return b.len }

// Capacity returns the capacity of the backing slice.
func (b *OpenStringBuilder) Capacity() int { return cap(b.buf) }

// Append appends a rune.
func (b *OpenStringBuilder) Append(r rune) *OpenStringBuilder {
	b.Write(r)
	return b
}

// AppendString appends all runes in s.
func (b *OpenStringBuilder) AppendString(s string) *OpenStringBuilder {
	b.WriteString(s)
	return b
}

// AppendSlice appends runes from src[start:end].
func (b *OpenStringBuilder) AppendSlice(src []rune, start, end int) *OpenStringBuilder {
	b.WriteSlice(src, start, end)
	return b
}

// RuneAt returns the rune at index i.
func (b *OpenStringBuilder) RuneAt(i int) rune { return b.buf[i] }

// SetRuneAt sets the rune at index i.
func (b *OpenStringBuilder) SetRuneAt(i int, r rune) { b.buf[i] = r }

// UnsafeWrite appends r without a bounds check.  Reserve must have been called.
func (b *OpenStringBuilder) UnsafeWrite(r rune) { b.buf[b.len] = r; b.len++ }

// UnsafeWriteSlice appends src[off:off+n] without bounds checks.
func (b *OpenStringBuilder) UnsafeWriteSlice(src []rune, off, n int) {
	copy(b.buf[b.len:], src[off:off+n])
	b.len += n
}

// resize grows the buffer to at least newLen.
func (b *OpenStringBuilder) resize(newLen int) {
	newCap := cap(b.buf) * 2
	if newCap < newLen {
		newCap = newLen
	}
	newBuf := make([]rune, newCap)
	copy(newBuf, b.buf[:b.len])
	b.buf = newBuf
}

// Reserve ensures at least num additional slots are available.
func (b *OpenStringBuilder) Reserve(num int) {
	if b.len+num > len(b.buf) {
		b.resize(b.len + num)
	}
}

// Write appends a single rune, growing the buffer if necessary.
func (b *OpenStringBuilder) Write(r rune) {
	if b.len >= len(b.buf) {
		b.resize(b.len + 1)
	}
	b.UnsafeWrite(r)
}

// WriteSlice appends src[off:off+n], growing the buffer if necessary.
func (b *OpenStringBuilder) WriteSlice(src []rune, off, n int) {
	b.Reserve(n)
	b.UnsafeWriteSlice(src, off, n)
}

// WriteString appends all runes in s.
func (b *OpenStringBuilder) WriteString(s string) {
	runes := []rune(s)
	b.WriteSlice(runes, 0, len(runes))
}

// WriteOpenStringBuilder appends all runes from another OpenStringBuilder.
func (b *OpenStringBuilder) WriteOpenStringBuilder(o *OpenStringBuilder) {
	b.WriteSlice(o.buf, 0, o.len)
}

// Flush is a no-op; provided for interface parity with Java's Appendable.
func (b *OpenStringBuilder) Flush() {}

// Reset sets the logical length to zero (does not release the backing array).
func (b *OpenStringBuilder) Reset() { b.len = 0 }

// ToRuneSlice returns a copy of the logical content as a rune slice.
func (b *OpenStringBuilder) ToRuneSlice() []rune {
	out := make([]rune, b.len)
	copy(out, b.buf[:b.len])
	return out
}

// String returns the logical content as a UTF-8 string.
func (b *OpenStringBuilder) String() string { return string(b.buf[:b.len]) }
