// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"encoding/binary"
	"hash"
)

// BufferedChecksum wraps another hash.Hash32 with an internal buffer
// to speed up checksum calculations.
//
// This is the Go port of Lucene's org.apache.lucene.store.BufferedChecksum.
type BufferedChecksum struct {
	in     hash.Hash32
	buffer []byte
	upto   int
}

// DefaultBufferSize is the default buffer size: 1024
const DefaultBufferSize = 1024

// NewBufferedChecksum creates a new BufferedChecksum with DefaultBufferSize
func NewBufferedChecksum(in hash.Hash32) *BufferedChecksum {
	return NewBufferedChecksumWithSize(in, DefaultBufferSize)
}

// NewBufferedChecksumWithSize creates a new BufferedChecksum with the specified bufferSize
func NewBufferedChecksumWithSize(in hash.Hash32, bufferSize int) *BufferedChecksum {
	return &BufferedChecksum{
		in:     in,
		buffer: make([]byte, bufferSize),
		upto:   0,
	}
}

// Write implements hash.Hash32 by adding bytes to the buffer.
// This method is used to satisfy the hash.Hash32 interface.
func (c *BufferedChecksum) Write(p []byte) (int, error) {
	c.UpdateBytes(p)
	return len(p), nil
}

// Update adds a single byte to the checksum.
func (c *BufferedChecksum) Update(b byte) {
	if c.upto == len(c.buffer) {
		c.flush()
	}
	c.buffer[c.upto] = b
	c.upto++
}

// UpdateBytes updates the checksum with the given byte slice.
// This is the main update method that handles buffering logic.
func (c *BufferedChecksum) UpdateBytes(b []byte) {
	if len(b) >= len(c.buffer) {
		c.flush()
		c.in.Write(b)
		return
	}
	if c.upto+len(b) > len(c.buffer) {
		c.flush()
	}
	copy(c.buffer[c.upto:], b)
	c.upto += len(b)
}

// UpdateShort updates the checksum with a short value (2 bytes, little-endian).
func (c *BufferedChecksum) UpdateShort(val int16) {
	if c.upto+2 > len(c.buffer) {
		c.flush()
	}
	binary.LittleEndian.PutUint16(c.buffer[c.upto:], uint16(val))
	c.upto += 2
}

// UpdateInt updates the checksum with an int value (4 bytes, little-endian).
func (c *BufferedChecksum) UpdateInt(val int32) {
	if c.upto+4 > len(c.buffer) {
		c.flush()
	}
	binary.LittleEndian.PutUint32(c.buffer[c.upto:], uint32(val))
	c.upto += 4
}

// UpdateLong updates the checksum with a long value (8 bytes, little-endian).
func (c *BufferedChecksum) UpdateLong(val int64) {
	if c.upto+8 > len(c.buffer) {
		c.flush()
	}
	binary.LittleEndian.PutUint64(c.buffer[c.upto:], uint64(val))
	c.upto += 8
}

// UpdateLongs updates the checksum with an array of long values.
func (c *BufferedChecksum) UpdateLongs(vals []int64, offset int, length int) {
	if c.upto > 0 {
		remainingCapacityInLong := min((len(c.buffer)-c.upto)/8, length)
		for i := 0; i < remainingCapacityInLong; i++ {
			c.UpdateLong(vals[offset])
			offset++
			length--
		}
		if length == 0 {
			return
		}
	}

	capacityInLong := len(c.buffer) / 8
	for length > 0 {
		c.flush()
		l := min(capacityInLong, length)
		for i := 0; i < l; i++ {
			binary.LittleEndian.PutUint64(c.buffer[i*8:], uint64(vals[offset+i]))
		}
		c.upto += l * 8
		offset += l
		length -= l
	}
}

// Sum32 returns the current checksum value.
func (c *BufferedChecksum) Sum32() uint32 {
	c.flush()
	return c.in.Sum32()
}

// Sum appends the current hash to b and returns the resulting slice.
func (c *BufferedChecksum) Sum(b []byte) []byte {
	c.flush()
	return c.in.Sum(b)
}

// Reset resets the checksum to its initial value.
func (c *BufferedChecksum) Reset() {
	c.upto = 0
	c.in.Reset()
}

// Size returns the number of bytes Sum will return.
func (c *BufferedChecksum) Size() int {
	return c.in.Size()
}

// BlockSize returns the hash's underlying block size.
func (c *BufferedChecksum) BlockSize() int {
	return c.in.BlockSize()
}

// flush writes the buffered data to the underlying checksum.
func (c *BufferedChecksum) flush() {
	if c.upto > 0 {
		c.in.Write(c.buffer[:c.upto])
	}
	c.upto = 0
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
