// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

const (
	// ByteBlockShift is the shift used to calculate buffer index.
	ByteBlockShift = 15
	// ByteBlockSize is the size of each buffer in the pool.
	ByteBlockSize = 1 << ByteBlockShift
	// ByteBlockMask is the mask used to find the position in the current buffer.
	ByteBlockMask = ByteBlockSize - 1
)

// Allocator is an interface for allocating and freeing byte blocks.
type Allocator interface {
	// GetByteBlock allocates a new byte block of size ByteBlockSize.
	GetByteBlock() []byte
	// RecycleByteBlocks allows the allocator to recycle old buffers.
	RecycleByteBlocks(blocks [][]byte, start, end int)
	// GetBlockSize returns the size of the blocks allocated by this allocator.
	GetBlockSize() int
}

// DirectAllocator is a simple Allocator that never recycles.
type DirectAllocator struct{}

func NewDirectAllocator() *DirectAllocator {
	return &DirectAllocator{}
}

func (a *DirectAllocator) GetByteBlock() []byte {
	return make([]byte, ByteBlockSize)
}

func (a *DirectAllocator) RecycleByteBlocks(blocks [][]byte, start, end int) {
	// No-op
}

// DirectTrackingAllocator is a simple Allocator that never recycles,
// but tracks how much total RAM is in use.
//
// Mirrors org.apache.lucene.util.ByteBlockPool.DirectTrackingAllocator,
// whose single constructor takes the org.apache.lucene.util.Counter the
// allocation is charged against. CounterAPI is the Go rendering of that
// abstract Counter type, so both the atomic *Counter and the serial
// *SerialCounter variant can be supplied, exactly as Counter.newCounter
// allows in Java.
type DirectTrackingAllocator struct {
	bytesUsed CounterAPI
}

func NewDirectTrackingAllocator(bytesUsed CounterAPI) *DirectTrackingAllocator {
	return &DirectTrackingAllocator{
		bytesUsed: bytesUsed,
	}
}

func (a *DirectTrackingAllocator) GetByteBlock() []byte {
	a.bytesUsed.AddAndGet(int64(ByteBlockSize))
	return make([]byte, ByteBlockSize)
}

func (a *DirectTrackingAllocator) RecycleByteBlocks(blocks [][]byte, start, end int) {
	a.bytesUsed.AddAndGet(-int64((end - start) * ByteBlockSize))
	for i := start; i < end; i++ {
		blocks[i] = nil
	}
}

// ByteBlockPool enables the allocation of fixed-size buffers and their management as part of a buffer array.
// This is the Go port of Lucene's org.apache.lucene.util.ByteBlockPool.
type ByteBlockPool struct {
	// buffers is the array of buffers currently used in the pool.
	buffers [][]byte
	// bufferUpto is the index into the buffers array pointing to the current buffer used as the head.
	bufferUpto int
	// ByteUpto is where we are in the head buffer.
	ByteUpto int
	// Buffer is the current head buffer.
	Buffer []byte
	// ByteOffset is the offset from the start of the first buffer to the start of the current buffer.
	ByteOffset int
	// allocator is the strategy for allocating and recycling blocks.
	allocator Allocator
}

// NewByteBlockPool creates a new ByteBlockPool with the given allocator.
func NewByteBlockPool(allocator Allocator) *ByteBlockPool {
	return &ByteBlockPool{
		buffers:    make([][]byte, 10),
		bufferUpto: -1,
		ByteUpto:   ByteBlockSize,
		ByteOffset: -ByteBlockSize,
		allocator:  allocator,
	}
}

// Reset resets the pool to its initial state, while optionally reusing the first buffer.
func (p *ByteBlockPool) Reset(zeroFillBuffers, reuseFirst bool) {
	if p.bufferUpto != -1 {
		if zeroFillBuffers {
			for i := 0; i < p.bufferUpto; i++ {
				for j := range p.buffers[i] {
					p.buffers[i][j] = 0
				}
			}
			for j := 0; j < p.ByteUpto; j++ {
				p.buffers[p.bufferUpto][j] = 0
			}
		}

		if p.bufferUpto > 0 || !reuseFirst {
			offset := 0
			if reuseFirst {
				offset = 1
			}
			p.allocator.RecycleByteBlocks(p.buffers, offset, 1+p.bufferUpto)
			for i := offset; i < 1+p.bufferUpto; i++ {
				p.buffers[i] = nil
			}
		}

		if reuseFirst {
			p.bufferUpto = 0
			p.ByteUpto = 0
			p.ByteOffset = 0
			p.Buffer = p.buffers[0]
		} else {
			p.bufferUpto = -1
			p.ByteUpto = ByteBlockSize
			p.ByteOffset = -ByteBlockSize
			p.Buffer = nil
		}
	}
}

// NextBuffer allocates a new buffer and advances the pool to it.
func (p *ByteBlockPool) NextBuffer() {
	if 1+p.bufferUpto == len(p.buffers) {
		newSize := len(p.buffers) * 2
		if newSize == 0 {
			newSize = 10
		}
		newBuffers := make([][]byte, newSize)
		copy(newBuffers, p.buffers)
		p.buffers = newBuffers
	}
	p.buffers[1+p.bufferUpto] = p.allocator.GetByteBlock()
	p.Buffer = p.buffers[1+p.bufferUpto]
	p.bufferUpto++
	p.ByteUpto = 0
	p.ByteOffset += ByteBlockSize
}

// SetBytesRef fills the provided BytesRef with the bytes at the specified offset and length.
func (p *ByteBlockPool) SetBytesRef(builder *BytesRefBuilder, result *BytesRef, offset int64, length int) {
	result.Length = length

	bufferIndex := int(offset >> ByteBlockShift)
	buffer := p.buffers[bufferIndex]
	pos := int(offset & ByteBlockMask)

	if pos+length <= ByteBlockSize {
		result.Bytes = buffer
		result.Offset = pos
	} else {
		builder.GrowNoCopy(length)
		result.Bytes = builder.Bytes()
		result.Offset = 0
		p.ReadBytes(offset, result.Bytes, 0, length)
	}
}

// AppendBytesRef appends the bytes in the provided BytesRef at the current position.
func (p *ByteBlockPool) AppendBytesRef(bytes *BytesRef) {
	p.AppendBytesWithOffset(bytes.Bytes, bytes.Offset, bytes.Length)
}

// AppendByteBlockPool appends the bytes from a source ByteBlockPool at a given offset and length.
func (p *ByteBlockPool) AppendByteBlockPool(srcPool *ByteBlockPool, srcOffset int64, length int) {
	bytesLeft := length
	for bytesLeft > 0 {
		bufferLeft := ByteBlockSize - p.ByteUpto
		if bytesLeft < bufferLeft {
			p.appendBytesSingleBuffer(srcPool, srcOffset, bytesLeft)
			break
		} else {
			if bufferLeft > 0 {
				p.appendBytesSingleBuffer(srcPool, srcOffset, bufferLeft)
				bytesLeft -= bufferLeft
				srcOffset += int64(bufferLeft)
			}
			p.NextBuffer()
		}
	}
}

func (p *ByteBlockPool) appendBytesSingleBuffer(srcPool *ByteBlockPool, srcOffset int64, length int) {
	for length > 0 {
		srcBufferIndex := int(srcOffset >> ByteBlockShift)
		srcBytes := srcPool.buffers[srcBufferIndex]
		srcPos := int(srcOffset & ByteBlockMask)
		bytesToCopy := length
		if ByteBlockSize-srcPos < bytesToCopy {
			bytesToCopy = ByteBlockSize - srcPos
		}
		copy(p.Buffer[p.ByteUpto:], srcBytes[srcPos:srcPos+bytesToCopy])
		length -= bytesToCopy
		srcOffset += int64(bytesToCopy)
		p.ByteUpto += bytesToCopy
	}
}

// Append appends the provided byte array at the current position.
func (p *ByteBlockPool) Append(bytes []byte) {
	p.AppendBytesWithOffset(bytes, 0, len(bytes))
}

// AppendBytes is a wrapper for AppendBytesWithOffset for consistency with other APIs.
func (p *ByteBlockPool) AppendBytes(bytes []byte) {
	p.Append(bytes)
}

// AppendBytesWithOffset appends some portion of the provided byte array at the current position.
func (p *ByteBlockPool) AppendBytesWithOffset(bytes []byte, offset, length int) {
	bytesLeft := length
	for bytesLeft > 0 {
		bufferLeft := ByteBlockSize - p.ByteUpto
		if bytesLeft < bufferLeft {
			copy(p.Buffer[p.ByteUpto:], bytes[offset:offset+bytesLeft])
			p.ByteUpto += bytesLeft
			break
		} else {
			if bufferLeft > 0 {
				copy(p.Buffer[p.ByteUpto:], bytes[offset:offset+bufferLeft])
			}
			p.NextBuffer()
			bytesLeft -= bufferLeft
			offset += bufferLeft
		}
	}
}

// ReadBytes reads bytes out of the pool starting at the given offset with the given length into the given
// byte array at offset bytesOffset.
func (p *ByteBlockPool) ReadBytes(offset int64, bytes []byte, bytesOffset, bytesLength int) {
	bytesLeft := bytesLength
	bufferIndex := int(offset >> ByteBlockShift)
	pos := int(offset & ByteBlockMask)
	for bytesLeft > 0 {
		buffer := p.buffers[bufferIndex]
		bufferIndex++
		chunk := bytesLeft
		if ByteBlockSize-pos < chunk {
			chunk = ByteBlockSize - pos
		}
		copy(bytes[bytesOffset:], buffer[pos:pos+chunk])
		bytesOffset += chunk
		bytesLeft -= chunk
		pos = 0
	}
}

// ReadByteAt reads a single byte at the given offset. Mirrors
// ByteBlockPool.readByte(long) of Apache Lucene 10.5.0; the Go name carries
// the At suffix because the method name ReadByte is reserved for the
// io.ByteReader signature ReadByte() (byte, error), which go vet enforces.
func (p *ByteBlockPool) ReadByteAt(offset int64) byte {
	bufferIndex := int(offset >> ByteBlockShift)
	pos := int(offset & ByteBlockMask)
	return p.buffers[bufferIndex][pos]
}

// RamBytesUsed returns the total amount of RAM, in bytes, consumed by this object.
func (p *ByteBlockPool) RamBytesUsed() int64 {
	// Approximate size of the ByteBlockPool struct itself
	var size int64 = 64
	size += int64(len(p.buffers) * 8)
	for _, buf := range p.buffers {
		if buf != nil {
			size += int64(len(buf)) + 24
		}
	}
	return size
}

// GetPosition returns the current position (in absolute value) of this byte pool.
func (p *ByteBlockPool) GetPosition() int64 {
	return int64(p.bufferUpto)*int64(p.allocator.GetBlockSize()) + int64(p.ByteUpto)
}

// GetBuffer retrieves the buffer at the specified index from the buffer pool.
func (p *ByteBlockPool) GetBuffer(bufferIndex int) []byte {
	return p.buffers[bufferIndex]
}

// GetByteBlockSize is a helper for the allocator size.
func (a *DirectAllocator) GetBlockSize() int {
	return ByteBlockSize
}

func (a *DirectTrackingAllocator) GetBlockSize() int {
	return ByteBlockSize
}
