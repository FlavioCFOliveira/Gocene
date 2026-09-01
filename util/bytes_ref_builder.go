package util

// BytesRefBuilder is a builder for BytesRef instances.
// This is the Go port of Lucene's org.apache.lucene.util.BytesRefBuilder.
type BytesRefBuilder struct {
	bytes  []byte
	length int
}

// NewBytesRefBuilder creates a new BytesRefBuilder.
func NewBytesRefBuilder() *BytesRefBuilder {
	return &BytesRefBuilder{}
}

// GrowNoCopy grows the internal buffer to accommodate at least minSize bytes without copying.
func (b *BytesRefBuilder) GrowNoCopy(minSize int) {
	if cap(b.bytes) >= minSize {
		b.bytes = b.bytes[:minSize]
		return
	}
	b.bytes = make([]byte, minSize)
}

// Bytes returns the underlying byte slice.
func (b *BytesRefBuilder) Bytes() []byte {
	return b.bytes
}

// Get returns a BytesRef view of the builder's contents.
func (b *BytesRefBuilder) Get() *BytesRef {
	return &BytesRef{
		Bytes:  b.bytes,
		Offset: 0,
		Length: b.length,
	}
}

// CopyChars copies characters from a string into the builder.
func (b *BytesRefBuilder) CopyChars(s string) {
	b.length = len(s)
	if cap(b.bytes) < b.length {
		b.bytes = make([]byte, b.length)
	} else {
		b.bytes = b.bytes[:b.length]
	}
	copy(b.bytes, s)
}

// Grow grows the internal buffer to accommodate at least minSize bytes.
// Existing content up to min(b.length, len(b.bytes)) is preserved.
func (b *BytesRefBuilder) Grow(minSize int) {
	if cap(b.bytes) >= minSize {
		return
	}
	newBytes := make([]byte, minSize)
	copyLen := b.length
	if copyLen > len(b.bytes) {
		copyLen = len(b.bytes)
	}
	if copyLen > 0 {
		copy(newBytes, b.bytes[:copyLen])
	}
	b.bytes = newBytes
}

// SetLength sets the length of the builder.
func (b *BytesRefBuilder) SetLength(length int) {
	b.length = length
	if b.bytes != nil && length <= cap(b.bytes) {
		b.bytes = b.bytes[:length]
	}
}
