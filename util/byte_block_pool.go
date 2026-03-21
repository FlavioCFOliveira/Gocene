// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"fmt"
	"math"
)

// ByteBlockPool enables the allocation of fixed-size buffers and their management
// as part of a buffer array. Allocation is done through the use of an Allocator
// which can be customized, e.g. to allow recycling old buffers.
//
// This is the Go port of Lucene's org.apache.lucene.util.ByteBlockPool.
type ByteBlockPool struct {
	// Array of buffers currently used in the pool. Buffers are allocated if needed.
	buffers [][]byte

	// index into the buffers array pointing to the current buffer used as the head
	bufferUpto int // Which buffer we are upto

	// Where we are in the head buffer.
	ByteUpto int

	// Current head buffer.
	Buffer []byte

	// Offset from the start of the first buffer to the start of the current buffer
	ByteOffset int

	allocator Allocator
}

const (
	// ByteBlockShift is used to find the index of the buffer containing a byte.
	// bufferUpto = globalOffset >> BYTE_BLOCK_SHIFT
	// bufferUpto = globalOffset / BYTE_BLOCK_SIZE
	ByteBlockShift = 15

	// ByteBlockSize is the size of each buffer in the pool.
	ByteBlockSize = 1 << ByteBlockShift // 32768

	// ByteBlockMask is used to find the position of a global offset in a particular buffer.
	// positionInCurrentBuffer = globalOffset & BYTE_BLOCK_MASK
	// positionInCurrentBuffer = globalOffset % BYTE_BLOCK_SIZE
	ByteBlockMask = ByteBlockSize - 1
)

// Allocator is the interface for allocating and freeing byte blocks.
type Allocator interface {
	// RecycleByteBlocks recycles byte blocks
	RecycleByteBlocks(blocks [][]byte, start, end int)
	// GetByteBlock returns a new byte block
	GetByteBlock() []byte
}

// DirectAllocator is a simple Allocator that never recycles.
type DirectAllocator struct{}

// NewDirectAllocator creates a new DirectAllocator.
func NewDirectAllocator() *DirectAllocator {
	return &DirectAllocator{}
}

// RecycleByteBlocks is a no-op for DirectAllocator.
func (d *DirectAllocator) RecycleByteBlocks(blocks [][]byte, start, end int) {}

// GetByteBlock returns a new byte block.
func (d *DirectAllocator) GetByteBlock() []byte {
	return make([]byte, ByteBlockSize)
}

// Counter is a simple atomic counter for tracking memory usage.
type Counter struct {
	value int64
}

// NewCounter creates a new Counter.
func NewCounter() *Counter {
	return &Counter{}
}

// Get returns the current value.
func (c *Counter) Get() int64 {
	return c.value
}

// AddAndGet adds the given value and returns the new value.
func (c *Counter) AddAndGet(delta int64) int64 {
	c.value += delta
	return c.value
}

// DirectTrackingAllocator is a simple Allocator that never recycles,
// but tracks how much total RAM is in use.
type DirectTrackingAllocator struct {
	bytesUsed *Counter
}

// NewDirectTrackingAllocator creates a new DirectTrackingAllocator.
func NewDirectTrackingAllocator(bytesUsed *Counter) *DirectTrackingAllocator {
	return &DirectTrackingAllocator{bytesUsed: bytesUsed}
}

// RecycleByteBlocks recycles byte blocks and updates the counter.
func (d *DirectTrackingAllocator) RecycleByteBlocks(blocks [][]byte, start, end int) {
	d.bytesUsed.AddAndGet(-int64((end - start) * ByteBlockSize))
	for i := start; i < end; i++ {
		blocks[i] = nil
	}
}

// GetByteBlock returns a new byte block and updates the counter.
func (d *DirectTrackingAllocator) GetByteBlock() []byte {
	d.bytesUsed.AddAndGet(int64(ByteBlockSize))
	return make([]byte, ByteBlockSize)
}

// DefaultInitialBufferCapacity is the default initial capacity for the buffers slice.
// Increased from previous value of 10 to reduce reallocations for typical use cases.
const DefaultInitialBufferCapacity = 64

// NewByteBlockPool creates a new ByteBlockPool with the given allocator.
// Uses DefaultInitialBufferCapacity (64) as the initial capacity.
func NewByteBlockPool(allocator Allocator) *ByteBlockPool {
	return NewByteBlockPoolWithCapacity(allocator, DefaultInitialBufferCapacity)
}

// NewByteBlockPoolWithCapacity creates a new ByteBlockPool with custom initial capacity.
// Use this when you expect a large number of buffers to avoid repeated reallocations.
//
// Parameters:
//   - allocator: the Allocator to use for byte block allocation
//   - initialCapacity: initial capacity for the buffers slice (min 16)
func NewByteBlockPoolWithCapacity(allocator Allocator, initialCapacity int) *ByteBlockPool {
	if initialCapacity < 16 {
		initialCapacity = 16
	}
	return &ByteBlockPool{
		buffers:    make([][]byte, initialCapacity),
		bufferUpto: -1,
		ByteUpto:   ByteBlockSize,
		ByteOffset: -ByteBlockSize,
		Buffer:     nil,
		allocator:  allocator,
	}
}

// NextBuffer allocates a new buffer and advances the pool to it.
// This method should be called once after the constructor to initialize the pool.
func (p *ByteBlockPool) NextBuffer() {
	if 1+p.bufferUpto == len(p.buffers) {
		// The buffer array is full - expand it
		newBuffers := make([][]byte, oversize(len(p.buffers)+1, 8)) // 8 = size of pointer
		copy(newBuffers, p.buffers)
		p.buffers = newBuffers
	}
	// Allocate new buffer and advance the pool to it
	p.Buffer = p.allocator.GetByteBlock()
	p.buffers[1+p.bufferUpto] = p.Buffer
	p.bufferUpto++
	p.ByteUpto = 0
	p.ByteOffset = addExact(p.ByteOffset, ByteBlockSize)
}

// Reset resets the pool to its initial state, while optionally reusing the first buffer.
// Buffers that are not reused are reclaimed by RecycleByteBlocks.
//
// zeroFillBuffers: if true the buffers are filled with 0. This should be
// set to true if this pool is used with slices.
// reuseFirst: if true the first buffer will be reused.
func (p *ByteBlockPool) Reset(zeroFillBuffers, reuseFirst bool) {
	if p.bufferUpto != -1 {
		// We allocated at least one buffer

		if zeroFillBuffers {
			for i := 0; i < p.bufferUpto; i++ {
				// Fully zero fill buffers that we fully used
				for j := range p.buffers[i] {
					p.buffers[i][j] = 0
				}
			}
			// Partial zero fill the final buffer
			for i := 0; i < p.ByteUpto; i++ {
				p.buffers[p.bufferUpto][i] = 0
			}
		}

		if p.bufferUpto > 0 || !reuseFirst {
			offset := 0
			if reuseFirst {
				offset = 1
			}
			// Recycle all but the first buffer
			p.allocator.RecycleByteBlocks(p.buffers, offset, 1+p.bufferUpto)
			for i := offset; i < 1+p.bufferUpto; i++ {
				p.buffers[i] = nil
			}
		}
		if reuseFirst {
			// Re-use the first buffer
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

// AppendBytesRef appends the bytes in the provided BytesRef at the current position.
func (p *ByteBlockPool) AppendBytesRef(bytes *BytesRef) {
	if bytes == nil {
		return
	}
	p.AppendBytes(bytes.Bytes, bytes.Offset, bytes.Length)
}

// Append appends the provided byte array at the current position.
func (p *ByteBlockPool) Append(bytes []byte) {
	if len(bytes) == 0 {
		return
	}
	p.AppendBytes(bytes, 0, len(bytes))
}

// AppendBytes appends some portion of the provided byte array at the current position.
func (p *ByteBlockPool) AppendBytes(bytes []byte, offset, length int) {
	bytesLeft := length
	for bytesLeft > 0 {
		bufferLeft := ByteBlockSize - p.ByteUpto
		if bytesLeft < bufferLeft {
			// fits within current buffer
			copy(p.Buffer[p.ByteUpto:], bytes[offset:offset+bytesLeft])
			p.ByteUpto += bytesLeft
			break
		} else {
			// fill up this buffer and move to next one
			if bufferLeft > 0 {
				copy(p.Buffer[p.ByteUpto:], bytes[offset:offset+bufferLeft])
			}
			p.NextBuffer()
			bytesLeft -= bufferLeft
			offset += bufferLeft
		}
	}
}

// AppendFromPool appends the bytes from a source ByteBlockPool at a given offset and length.
func (p *ByteBlockPool) AppendFromPool(srcPool *ByteBlockPool, srcOffset int64, length int) {
	bytesLeft := length
	for bytesLeft > 0 {
		bufferLeft := ByteBlockSize - p.ByteUpto
		if bytesLeft < bufferLeft { // fits within current buffer
			p.appendBytesSingleBuffer(srcPool, srcOffset, bytesLeft)
			break
		} else { // fill up this buffer and move to next one
			if bufferLeft > 0 {
				p.appendBytesSingleBuffer(srcPool, srcOffset, bufferLeft)
				bytesLeft -= bufferLeft
				srcOffset += int64(bufferLeft)
			}
			p.NextBuffer()
		}
	}
}

// appendBytesSingleBuffer copies from source pool until no bytes left.
// length must fit within the current head buffer.
// Optimized to minimize bounds checks by pre-computing slice limits.
func (p *ByteBlockPool) appendBytesSingleBuffer(srcPool *ByteBlockPool, srcOffset int64, length int) {
	if length > ByteBlockSize-p.ByteUpto {
		panic(fmt.Sprintf("length %d exceeds buffer space %d", length, ByteBlockSize-p.ByteUpto))
	}
	// doing a loop as the bytes to copy might span across multiple byte[] in srcPool
	byteUpto := p.ByteUpto
	destBuffer := p.Buffer[byteUpto : byteUpto+length] // Pre-compute destination slice
	for length > 0 {
		srcBufferIndex := int(srcOffset >> ByteBlockShift)
		srcPos := int(srcOffset & ByteBlockMask)
		// Pre-compute source slice to avoid bounds checks in inner loop
		srcBuffer := srcPool.buffers[srcBufferIndex]
		maxCopy := ByteBlockSize - srcPos
		if maxCopy > length {
			maxCopy = length
		}
		// Calculate actual bytes copied
		n := copy(destBuffer, srcBuffer[srcPos:srcPos+maxCopy])
		length -= n
		srcOffset += int64(n)
		byteUpto += n
		destBuffer = destBuffer[n:] // Advance destination slice
	}
	p.ByteUpto = byteUpto
}

// ReadBytes reads bytes out of the pool starting at the given offset with the given length
// into the given byte array at offset off.
// Note: this method allows to copy across block boundaries.
// Optimized to minimize bounds checks by using pre-computed slices.
func (p *ByteBlockPool) ReadBytes(offset int64, bytes []byte, bytesOffset, bytesLength int) {
	bytesLeft := bytesLength
	bufferIndex := int(offset >> ByteBlockShift)
	pos := int(offset & ByteBlockMask)
	// Pre-compute destination slice to avoid repeated bounds checks
	destSlice := bytes[bytesOffset : bytesOffset+bytesLength]
	for bytesLeft > 0 {
		buffer := p.buffers[bufferIndex]
		if buffer == nil {
			panic(fmt.Sprintf("buffer at index %d is nil", bufferIndex))
		}
		// Calculate max bytes we can copy from this buffer
		maxCopy := ByteBlockSize - pos
		if maxCopy > bytesLeft {
			maxCopy = bytesLeft
		}
		// Copy using pre-computed slices
		n := copy(destSlice, buffer[pos:pos+maxCopy])
		bytesLeft -= n
		destSlice = destSlice[n:] // Advance destination slice
		bufferIndex++
		pos = 0
	}
}

// ReadByte reads a single byte at the given offset.
func (p *ByteBlockPool) ReadByte(offset int64) byte {
	bufferIndex := int(offset >> ByteBlockShift)
	pos := int(offset & ByteBlockMask)
	return p.buffers[bufferIndex][pos]
}

// SetBytesRef fills the provided BytesRef with the bytes at the specified offset and length.
// This will avoid copying the bytes if the slice fits into a single block;
// otherwise, it uses the provided BytesRefBuilder to copy bytes over.
func (p *ByteBlockPool) SetBytesRef(builder *BytesRefBuilder, result *BytesRef, offset int64, length int) {
	result.Length = length

	bufferIndex := int(offset >> ByteBlockShift)
	buffer := p.buffers[bufferIndex]
	pos := int(offset & ByteBlockMask)
	if pos+length <= ByteBlockSize {
		// Common case: The slice lives in a single block. Reference the buffer directly.
		result.Bytes = buffer
		result.Offset = pos
	} else {
		// Uncommon case: The slice spans at least 2 blocks, so we must copy the bytes.
		builder.GrowNoCopy(length)
		result.Bytes = builder.Bytes()
		result.Offset = 0
		p.ReadBytes(offset, result.Bytes, 0, length)
	}
}

// GetPosition returns the current position (in absolute value) of this byte pool.
func (p *ByteBlockPool) GetPosition() int64 {
	return int64(p.bufferUpto*ByteBlockSize + p.ByteUpto)
}

// GetBuffer retrieves the buffer at the specified index from the buffer pool.
func (p *ByteBlockPool) GetBuffer(bufferIndex int) []byte {
	return p.buffers[bufferIndex]
}

// BytesRefBuilder is a builder for BytesRef that allows growing without copying.
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
func (b *BytesRefBuilder) Grow(minSize int) {
	if cap(b.bytes) >= minSize {
		return
	}
	newBytes := make([]byte, minSize)
	copy(newBytes, b.bytes[:b.length])
	b.bytes = newBytes
}

// SetLength sets the length of the builder.
func (b *BytesRefBuilder) SetLength(length int) {
	b.length = length
	if b.bytes != nil && length <= cap(b.bytes) {
		b.bytes = b.bytes[:length]
	}
}

// addExact adds two ints with overflow checking.
func addExact(x, y int) int {
	result := x + y
	if ((x ^ result) & (y ^ result)) < 0 {
		panic(fmt.Sprintf("integer overflow: %d + %d", x, y))
	}
	return result
}

// oversize returns a new size for an array that is at least minSize.
// Uses exponential growth: 2x for smaller arrays (up to 1024 elements),
// then switches to 1.5x for larger arrays to balance memory usage and performance.
func oversize(minSize, bytesPerElement int) int {
	if minSize < 0 {
		panic("minSize must be non-negative")
	}
	if bytesPerElement <= 0 {
		panic("bytesPerElement must be positive")
	}

	var newSize int
	if minSize < 1024 {
		// For smaller arrays, use 2x growth to minimize reallocations
		// This is beneficial for ByteBlockPool which often needs many small buffers
		newSize = minSize * 2
		if newSize < 16 {
			newSize = 16 // Minimum growth of 16 elements
		}
	} else {
		// For larger arrays, use 1.5x growth to avoid excessive memory usage
		newSize = minSize + minSize/2
	}

	// Ensure we don't overflow int
	if newSize < 0 || newSize > math.MaxInt32 {
		return math.MaxInt32
	}
	return newize(newSize, bytesPerElement)
}

// newize rounds up to the nearest multiple of bytesPerElement.
func newize(size, bytesPerElement int) int {
	if bytesPerElement == 1 {
		return size
	}
	return (size + bytesPerElement - 1) / bytesPerElement * bytesPerElement
}
