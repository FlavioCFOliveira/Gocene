package util

import (
	"fmt"
)

// BytesRefBuilder is a builder for BytesRef instances.
// This is the Go port of Lucene's org.apache.lucene.util.BytesRefBuilder.
type BytesRefBuilder struct {
	ref *BytesRef
}

// NewBytesRefBuilder creates a new BytesRefBuilder.
func NewBytesRefBuilder() *BytesRefBuilder {
	return &BytesRefBuilder{
		ref: NewBytesRefEmpty(),
	}
}

// Bytes returns the underlying bytes of the builder.
func (b *BytesRefBuilder) Bytes() []byte {
	return b.ref.Bytes
}

// Length returns the current length of the builder.
func (b *BytesRefBuilder) Length() int {
	return b.ref.Length
}

// SetLength sets the length of the builder.
func (b *BytesRefBuilder) SetLength(length int) {
	b.ref.Length = length
}

// ByteAt returns the byte at the given offset.
func (b *BytesRefBuilder) ByteAt(offset int) byte {
	return b.ref.Bytes[offset]
}

// SetByteAt sets a byte at the given offset.
func (b *BytesRefBuilder) SetByteAt(offset int, val byte) {
	b.ref.Bytes[offset] = val
}

// Grow ensures that this builder can hold at least capacity bytes without resizing.
func (b *BytesRefBuilder) Grow(capacity int) {
	b.ref.Grow(capacity)
}

// Append appends a single byte to this builder.
func (b *BytesRefBuilder) Append(val byte) {
	b.ref.Append([]byte{val})
}

// AppendBytes appends the provided bytes to this builder.
func (b *BytesRefBuilder) AppendBytes(p []byte) {
	b.ref.Append(p)
}

// AppendBytesRef appends the provided BytesRef to this builder.
func (b *BytesRefBuilder) AppendBytesRef(ref *BytesRef) {
	b.ref.AppendBytesRef(ref)
}

// AppendBuilder appends the provided BytesRefBuilder to this builder.
func (b *BytesRefBuilder) AppendBuilder(builder *BytesRefBuilder) {
	b.ref.AppendBytesRef(builder.Get())
}

// Clear resets this builder to the empty state.
func (b *BytesRefBuilder) Clear() {
	b.SetLength(0)
}

// CopyBytes replaces the content of this builder with the provided bytes.
func (b *BytesRefBuilder) CopyBytes(p []byte) {
	b.Clear()
	b.ref.Copy(NewBytesRef(p))
}

// CopyBytesRef replaces the content of this builder with the provided BytesRef.
func (b *BytesRefBuilder) CopyBytesRef(ref *BytesRef) {
	b.Clear()
	b.ref.Copy(ref)
}

// CopyBytesBuilder replaces the content of this builder with the provided BytesRefBuilder.
func (b *BytesRefBuilder) CopyBytesBuilder(builder *BytesRefBuilder) {
	b.Clear()
	b.ref.Copy(builder.Get())
}

// CopyChars replaces the content of this buffer with UTF-8 encoded bytes.
func (b *BytesRefBuilder) CopyChars(text string) {
	b.Clear()
	b.ref.Append([]byte(text))
}

// Get returns a BytesRef that points to the internal content of this builder.
func (b *BytesRefBuilder) Get() *BytesRef {
	return b.ref
}

// ToBytesRef builds a new BytesRef that has the same content as this buffer.
func (b *BytesRefBuilder) ToBytesRef() *BytesRef {
	return b.ref.Clone()
}

func (b *BytesRefBuilder) String() string {
	return b.ref.String()
}
