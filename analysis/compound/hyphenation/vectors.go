// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hyphenation

// ByteVector is a simple growable byte slice with a fixed growth increment.
//
// This is the Go port of
// org.apache.lucene.analysis.compound.hyphenation.ByteVector from
// Apache Lucene 10.4.0. Taken originally from the Apache FOP project.
type ByteVector struct {
	blockSize int
	array     []byte
	n         int
}

const defaultBlockSize = 2048

// NewByteVector creates a ByteVector with default capacity.
func NewByteVector() *ByteVector { return NewByteVectorWithCapacity(defaultBlockSize) }

// NewByteVectorWithCapacity creates a ByteVector with the given initial capacity.
func NewByteVectorWithCapacity(capacity int) *ByteVector {
	bs := defaultBlockSize
	if capacity > 0 {
		bs = capacity
	}
	return &ByteVector{blockSize: bs, array: make([]byte, bs), n: 0}
}

// NewByteVectorFromArray wraps an existing byte slice.
func NewByteVectorFromArray(a []byte) *ByteVector {
	return &ByteVector{blockSize: defaultBlockSize, array: a, n: 0}
}

// GetArray returns the underlying byte slice.
func (v *ByteVector) GetArray() []byte { return v.array }

// Length returns the number of items stored.
func (v *ByteVector) Length() int { return v.n }

// Capacity returns the current capacity of the underlying array.
func (v *ByteVector) Capacity() int { return len(v.array) }

// Put stores val at index.
func (v *ByteVector) Put(index int, val byte) { v.array[index] = val }

// Get returns the byte at index.
func (v *ByteVector) Get(index int) byte { return v.array[index] }

// Alloc allocates size bytes and returns the index of the first allocated byte.
func (v *ByteVector) Alloc(size int) int {
	index := v.n
	if v.n+size >= len(v.array) {
		aux := make([]byte, len(v.array)+v.blockSize)
		copy(aux, v.array)
		v.array = aux
	}
	v.n += size
	return index
}

// TrimToSize shrinks the underlying array to the number of items stored.
func (v *ByteVector) TrimToSize() {
	if v.n < len(v.array) {
		aux := make([]byte, v.n)
		copy(aux, v.array)
		v.array = aux
	}
}

// CharVector is a simple growable char (rune as uint16) slice.
//
// This is the Go port of
// org.apache.lucene.analysis.compound.hyphenation.CharVector from
// Apache Lucene 10.4.0. Taken originally from the Apache FOP project.
//
// Deviation: Java uses char (16-bit); Go uses char16 (uint16 alias).
type CharVector struct {
	blockSize int
	array     []uint16
	n         int
}

// NewCharVector creates a CharVector with default capacity.
func NewCharVector() *CharVector { return NewCharVectorWithCapacity(defaultBlockSize) }

// NewCharVectorWithCapacity creates a CharVector with the given initial capacity.
func NewCharVectorWithCapacity(capacity int) *CharVector {
	bs := defaultBlockSize
	if capacity > 0 {
		bs = capacity
	}
	return &CharVector{blockSize: bs, array: make([]uint16, bs), n: 0}
}

// NewCharVectorFromArray wraps an existing uint16 slice and sets n to len(a).
func NewCharVectorFromArray(a []uint16) *CharVector {
	return &CharVector{blockSize: defaultBlockSize, array: a, n: len(a)}
}

// Clone returns a deep copy.
func (v *CharVector) Clone() *CharVector {
	c := &CharVector{
		blockSize: v.blockSize,
		array:     make([]uint16, len(v.array)),
		n:         v.n,
	}
	copy(c.array, v.array)
	return c
}

// Clear resets the item count without modifying the array.
func (v *CharVector) Clear() { v.n = 0 }

// GetArray returns the underlying uint16 slice.
func (v *CharVector) GetArray() []uint16 { return v.array }

// Length returns the number of items stored.
func (v *CharVector) Length() int { return v.n }

// Capacity returns the current capacity of the underlying array.
func (v *CharVector) Capacity() int { return len(v.array) }

// Put stores val at index.
func (v *CharVector) Put(index int, val uint16) { v.array[index] = val }

// Get returns the uint16 at index.
func (v *CharVector) Get(index int) uint16 { return v.array[index] }

// Alloc allocates size uint16 slots and returns the starting index.
func (v *CharVector) Alloc(size int) int {
	index := v.n
	if v.n+size >= len(v.array) {
		aux := make([]uint16, len(v.array)+v.blockSize)
		copy(aux, v.array)
		v.array = aux
	}
	v.n += size
	return index
}

// TrimToSize shrinks the underlying array to the number of items stored.
func (v *CharVector) TrimToSize() {
	if v.n < len(v.array) {
		aux := make([]uint16, v.n)
		copy(aux, v.array)
		v.array = aux
	}
}
