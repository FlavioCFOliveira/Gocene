// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"fmt"
)

// BytesRefBlockPool represents a logical list of BytesRef backed by a ByteBlockPool.
// It uses up to two bytes to record the length of the BytesRef followed by the actual bytes.
// They can be read using the start position returned when they are appended.
//
// The BytesRef is written so it never crosses the ByteBlockSize boundary.
// The limit of the largest BytesRef is therefore ByteBlockSize-2 bytes.
//
// This is the Go port of Lucene's org.apache.lucene.util.BytesRefBlockPool.
type BytesRefBlockPool struct {
	byteBlockPool *ByteBlockPool
}

const baseRamBytes = 64

// NewBytesRefBlockPool creates a new BytesRefBlockPool with a default ByteBlockPool.
func NewBytesRefBlockPool() *BytesRefBlockPool {
	return &BytesRefBlockPool{
		byteBlockPool: NewByteBlockPool(NewDirectAllocator()),
	}
}

// NewBytesRefBlockPoolWithPool creates a new BytesRefBlockPool with the given ByteBlockPool.
func NewBytesRefBlockPoolWithPool(byteBlockPool *ByteBlockPool) *BytesRefBlockPool {
	return &BytesRefBlockPool{
		byteBlockPool: byteBlockPool,
	}
}

// Reset resets this buffer to the empty state.
func (p *BytesRefBlockPool) Reset() {
	p.byteBlockPool.Reset(false, false) // we don't need to 0-fill the buffers
}

// FillBytesRef populates the given BytesRef with the term starting at start.
func (p *BytesRefBlockPool) FillBytesRef(term *BytesRef, start int) {
	bytes := p.byteBlockPool.GetBuffer(start >> ByteBlockShift)
	pos := start & ByteBlockMask
	if (bytes[pos] & 0x80) == 0 {
		// length is 1 byte
		term.Length = int(bytes[pos])
		term.Offset = pos + 1
	} else {
		// length is 2 bytes
		// Read big-endian short: (bytes[pos] << 8) | bytes[pos+1]
		// Mask with 0x7FFF to remove the high bit flag.
		length := int(uint16(bytes[pos])<<8 | uint16(bytes[pos+1]))
		term.Length = length & 0x7FFF
		term.Offset = pos + 2
	}
	term.Bytes = bytes
}

// AddBytesRef adds a term returning the start position on the underlying ByteBlockPool.
// This can be used to read back the value using FillBytesRef.
func (p *BytesRefBlockPool) AddBytesRef(bytes *BytesRef) (int, error) {
	length := bytes.Length
	len2 := 2 + length
	if len2+p.byteBlockPool.ByteUpto > ByteBlockSize {
		if len2 > ByteBlockSize {
			return 0, fmt.Errorf("bytes can be at most %d in length; got %d", ByteBlockSize-2, length)
		}
		p.byteBlockPool.NextBuffer()
	}

	buffer := p.byteBlockPool.Buffer
	bufferUpto := p.byteBlockPool.ByteUpto
	textStart := bufferUpto + p.byteBlockPool.ByteOffset

	if length < 128 {
		// 1 byte to store length
		buffer[bufferUpto] = byte(length)
		p.byteBlockPool.ByteUpto += length + 1
		copy(buffer[bufferUpto+1:], bytes.ValidBytes())
	} else {
		// 2 byte to store length
		// Set high bit (0x80) of the first byte
		buffer[bufferUpto] = byte(length>>8 | 0x80)
		buffer[bufferUpto+1] = byte(length & 0xFF)
		p.byteBlockPool.ByteUpto += length + 2
		copy(buffer[bufferUpto+2:], bytes.ValidBytes())
	}

	return textStart, nil
}

// Hash computes the hash of the BytesRef at the given start.
func (p *BytesRefBlockPool) Hash(start int) int {
	offset := start & ByteBlockMask
	bytes := p.byteBlockPool.GetBuffer(start >> ByteBlockShift)
	var length int
	var pos int
	if (bytes[offset] & 0x80) == 0 {
		// length is 1 byte
		length = int(bytes[offset])
		pos = offset + 1
	} else {
		// length is 2 bytes
		length = int(uint16(bytes[offset])<<8|uint16(bytes[offset+1])) & 0x7FFF
		pos = offset + 2
	}

	return MurmurHash3_x86_32(bytes, pos, length, GoodFastHashSeed)
}

// Equals computes the equality between the BytesRef at the start position with the provided BytesRef.
func (p *BytesRefBlockPool) Equals(start int, b *BytesRef) bool {
	bytes := p.byteBlockPool.GetBuffer(start >> ByteBlockShift)
	pos := start & ByteBlockMask
	var length int
	var offset int
	if (bytes[pos] & 0x80) == 0 {
		// length is 1 byte
		length = int(bytes[pos])
		offset = pos + 1
	} else {
		// length is 2 bytes
		length = int(uint16(bytes[pos])<<8|uint16(bytes[pos+1])) & 0x7FFF
		offset = pos + 2
	}
	return bytesEqual(bytes[offset:offset+length], b.ValidBytes())
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// RamBytesUsed returns the total amount of RAM, in bytes, consumed by this object.
func (p *BytesRefBlockPool) RamBytesUsed() int64 {
	return int64(baseRamBytes) + p.byteBlockPool.RamBytesUsed()
}
