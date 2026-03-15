// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"encoding/binary"
	"fmt"
	"math"
)

// Constants for ByteBuffersDataOutput
const (
	DefaultMinBitsPerBlock        = 10 // 1024 B
	DefaultMaxBitsPerBlock          = 26 // 64 MB
	LimitMinBitsPerBlock            = 1
	LimitMaxBitsPerBlock            = 31
	MaxBlocksBeforeBlockExpansion   = 100
	NumBytesObjectRef               = 8 // On 64-bit systems
)

// ByteBuffersDataOutput is a DataOutput implementation that stores data in a list of byte buffers.
// This is the Go port of Lucene's org.apache.lucene.store.ByteBuffersDataOutput.
type ByteBuffersDataOutput struct {
	maxBitsPerBlock int
	minBitsPerBlock int
	blockBits       int
	blocks          [][]byte
	currentBlock    []byte
	ramBytesUsed    int64
	allocator       func(int) []byte
	recycler        func([]byte)
}

// NewByteBuffersDataOutput creates a new output with default settings.
func NewByteBuffersDataOutput() *ByteBuffersDataOutput {
	return NewByteBuffersDataOutputWithRecycler(
		DefaultMinBitsPerBlock,
		DefaultMaxBitsPerBlock,
		allocateBBOnHeap,
		noReuse,
	)
}

// NewByteBuffersDataOutputWithSize creates a new output optimized for the expected size.
func NewByteBuffersDataOutputWithSize(expectedSize int64) *ByteBuffersDataOutput {
	blockBits := computeBlockSizeBitsFor(expectedSize)
	return NewByteBuffersDataOutputWithRecycler(
		blockBits,
		DefaultMaxBitsPerBlock,
		allocateBBOnHeap,
		noReuse,
	)
}

// NewByteBuffersDataOutputWithRecycler creates a new output with custom allocator and recycler.
func NewByteBuffersDataOutputWithRecycler(minBits, maxBits int, allocator func(int) []byte, recycler func([]byte)) *ByteBuffersDataOutput {
	if minBits < LimitMinBitsPerBlock {
		panic("minBitsPerBlock too small")
	}
	if maxBits > LimitMaxBitsPerBlock {
		panic("maxBitsPerBlock too large")
	}
	if minBits > maxBits {
		panic("minBitsPerBlock cannot exceed maxBitsPerBlock")
	}
	if allocator == nil {
		panic("allocator must not be nil")
	}
	if recycler == nil {
		panic("recycler must not be nil")
	}

	return &ByteBuffersDataOutput{
		minBitsPerBlock: minBits,
		maxBitsPerBlock: maxBits,
		blockBits:       minBits,
		blocks:          make([][]byte, 0),
		currentBlock:    nil,
		ramBytesUsed:    0,
		allocator:       allocator,
		recycler:        recycler,
	}
}

func computeBlockSizeBitsFor(bytes int64) int {
	if bytes <= 0 {
		return DefaultMinBitsPerBlock
	}
	powerOfTwo := nextHighestPowerOfTwo(bytes / MaxBlocksBeforeBlockExpansion)
	if powerOfTwo == 0 {
		return DefaultMinBitsPerBlock
	}
	blockBits := int(math.Log2(float64(powerOfTwo)))
	if blockBits > DefaultMaxBitsPerBlock {
		blockBits = DefaultMaxBitsPerBlock
	}
	if blockBits < DefaultMinBitsPerBlock {
		blockBits = DefaultMinBitsPerBlock
	}
	return blockBits
}

func nextHighestPowerOfTwo(n int64) int64 {
	if n <= 0 {
		return 0
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n |= n >> 32
	n++
	return n
}

func allocateBBOnHeap(size int) []byte {
	return make([]byte, size)
}

func noReuse(buf []byte) {
	panic("reset() is not allowed on this buffer")
}

// WriteByte writes a single byte.
func (o *ByteBuffersDataOutput) WriteByte(b byte) error {
	if o.currentBlock == nil || len(o.currentBlock) >= o.blockSize() {
		o.appendBlock()
	}
	o.currentBlock = append(o.currentBlock, b)
	return nil
}

// WriteBytes writes all bytes from b.
func (o *ByteBuffersDataOutput) WriteBytes(b []byte) error {
	for len(b) > 0 {
		if o.currentBlock == nil || len(o.currentBlock) >= o.blockSize() {
			o.appendBlock()
		}
		space := o.blockSize() - len(o.currentBlock)
		if space > len(b) {
			space = len(b)
		}
		o.currentBlock = append(o.currentBlock, b[:space]...)
		b = b[space:]
	}
	return nil
}

// WriteShort writes a 16-bit value.
func (o *ByteBuffersDataOutput) WriteShort(v int16) error {
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, uint16(v))
	return o.WriteBytes(buf)
}

// WriteInt writes a 32-bit value.
func (o *ByteBuffersDataOutput) WriteInt(v int32) error {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(v))
	return o.WriteBytes(buf)
}

// WriteLong writes a 64-bit value.
func (o *ByteBuffersDataOutput) WriteLong(v int64) error {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, uint64(v))
	return o.WriteBytes(buf)
}

// WriteBytesN writes exactly len(b) bytes from b.
func (o *ByteBuffersDataOutput) WriteBytesN(b []byte, length int) error {
	if length > len(b) {
		return fmt.Errorf("length %d exceeds buffer size %d", length, len(b))
	}
	return o.WriteBytes(b[:length])
}

// WriteVInt writes a variable-length integer.
func (o *ByteBuffersDataOutput) WriteVInt(v int32) error {
	uv := uint32(v)
	for uv >= 0x80 {
		if err := o.WriteByte(byte(uv | 0x80)); err != nil {
			return err
		}
		uv >>= 7
	}
	return o.WriteByte(byte(uv))
}

// WriteVLong writes a variable-length long.
func (o *ByteBuffersDataOutput) WriteVLong(v int64) error {
	uv := uint64(v)
	for uv >= 0x80 {
		if err := o.WriteByte(byte(uv | 0x80)); err != nil {
			return err
		}
		uv >>= 7
	}
	return o.WriteByte(byte(uv))
}

// WriteZInt writes a zig-zag encoded integer.
func (o *ByteBuffersDataOutput) WriteZInt(v int32) error {
	return o.WriteVInt((v << 1) ^ (v >> 31))
}

// WriteZLong writes a zig-zag encoded long.
func (o *ByteBuffersDataOutput) WriteZLong(v int64) error {
	return o.WriteVLong((v << 1) ^ (v >> 63))
}

// WriteString writes a string.
func (o *ByteBuffersDataOutput) WriteString(s string) error {
	data := []byte(s)
	if err := o.WriteVInt(int32(len(data))); err != nil {
		return err
	}
	return o.WriteBytes(data)
}

// CopyBytes copies bytes from a DataInput.
func (o *ByteBuffersDataOutput) CopyBytes(input DataInput, numBytes int64) error {
	buf := make([]byte, numBytes)
	if err := input.ReadBytes(buf); err != nil {
		return err
	}
	o.WriteBytes(buf)
	return nil
}

// CopyTo copies the current content to another DataOutput.
func (o *ByteBuffersDataOutput) CopyTo(output DataOutput) error {
	for _, block := range o.blocks {
		if err := output.WriteBytes(block); err != nil {
			return err
		}
	}
	if o.currentBlock != nil {
		if err := output.WriteBytes(o.currentBlock); err != nil {
			return err
		}
	}
	return nil
}

// Size returns the number of bytes written.
func (o *ByteBuffersDataOutput) Size() int64 {
	size := int64(0)
	for _, block := range o.blocks {
		size += int64(len(block))
	}
	if o.currentBlock != nil {
		size += int64(len(o.currentBlock))
	}
	return size
}

// RamBytesUsed returns the RAM usage in bytes.
func (o *ByteBuffersDataOutput) RamBytesUsed() int64 {
	return o.ramBytesUsed
}

// Reset clears all data and enables buffer reuse.
func (o *ByteBuffersDataOutput) Reset() {
	if o.recycler != nil {
		for _, block := range o.blocks {
			o.recycler(block)
		}
	}
	o.blocks = o.blocks[:0]
	o.currentBlock = nil
	o.ramBytesUsed = 0
	o.blockBits = o.minBitsPerBlock
}

// ToArrayCopy returns a copy of all written data as a single byte slice.
func (o *ByteBuffersDataOutput) ToArrayCopy() []byte {
	result := make([]byte, o.Size())
	offset := 0
	for _, block := range o.blocks {
		copy(result[offset:], block)
		offset += len(block)
	}
	if o.currentBlock != nil {
		copy(result[offset:], o.currentBlock)
	}
	return result
}

// ToDataInput returns a ByteArrayDataInput for reading the written data.
func (o *ByteBuffersDataOutput) ToDataInput() DataInput {
	return NewByteArrayDataInput(o.ToArrayCopy())
}

// ReadOnlyBuffer wraps a byte slice with read-only semantics
type ReadOnlyBuffer struct {
	data []byte
}

// IsReadOnly always returns true for ReadOnlyBuffer
func (b *ReadOnlyBuffer) IsReadOnly() bool {
	return true
}

// Bytes returns the underlying data
func (b *ReadOnlyBuffer) Bytes() []byte {
	return b.data
}

// Len returns the length of the buffer
func (b *ReadOnlyBuffer) Len() int {
	return len(b.data)
}

// WriteableBuffer wraps a byte slice with writeable semantics
type WriteableBuffer struct {
	data []byte
}

// IsReadOnly always returns false for WriteableBuffer
func (b *WriteableBuffer) IsReadOnly() bool {
	return false
}

// Bytes returns the underlying data
func (b *WriteableBuffer) Bytes() []byte {
	return b.data
}

// Len returns the length of the buffer
func (b *WriteableBuffer) Len() int {
	return len(b.data)
}

// ToBufferList returns a list of read-only buffers.
func (o *ByteBuffersDataOutput) ToBufferList() []*ReadOnlyBuffer {
	result := make([]*ReadOnlyBuffer, 0, len(o.blocks)+1)
	for _, block := range o.blocks {
		result = append(result, &ReadOnlyBuffer{data: block})
	}
	if o.currentBlock != nil {
		result = append(result, &ReadOnlyBuffer{data: o.currentBlock})
	}
	if len(result) == 0 {
		result = append(result, &ReadOnlyBuffer{data: nil})
	}
	return result
}

// ToWriteableBufferList returns a list of writable buffers.
func (o *ByteBuffersDataOutput) ToWriteableBufferList() []*WriteableBuffer {
	result := make([]*WriteableBuffer, 0, len(o.blocks)+1)
	for _, block := range o.blocks {
		result = append(result, &WriteableBuffer{data: block})
	}
	if o.currentBlock != nil {
		result = append(result, &WriteableBuffer{data: o.currentBlock})
	}
	if len(result) == 0 {
		result = append(result, &WriteableBuffer{data: nil})
	}
	return result
}

// BufferCount returns the number of buffers.
func (o *ByteBuffersDataOutput) BufferCount() int {
	count := len(o.blocks)
	if o.currentBlock != nil {
		count++
	}
	return count
}

// BlockCapacity returns the capacity of the block at the given index.
func (o *ByteBuffersDataOutput) BlockCapacity(index int) int {
	if index < len(o.blocks) {
		return len(o.blocks[index])
	}
	if index == len(o.blocks) && o.currentBlock != nil {
		return len(o.currentBlock)
	}
	return 0
}

func (o *ByteBuffersDataOutput) blockSize() int {
	return 1 << o.blockBits
}

func (o *ByteBuffersDataOutput) appendBlock() {
	if len(o.blocks) >= MaxBlocksBeforeBlockExpansion && o.blockBits < o.maxBitsPerBlock {
		o.rewriteToBlockSize(o.blockBits + 1)
		if o.currentBlock != nil && len(o.currentBlock) < o.blockSize() {
			return
		}
	}

	if o.currentBlock != nil {
		o.blocks = append(o.blocks, o.currentBlock)
	}

	requiredSize := 1 << o.blockBits
	o.currentBlock = o.allocator(requiredSize)[:0] // Allocate but set length to 0
	o.ramBytesUsed += int64(requiredSize) + NumBytesObjectRef
}

func (o *ByteBuffersDataOutput) rewriteToBlockSize(targetBlockBits int) {
	cloned := NewByteBuffersDataOutputWithRecycler(
		targetBlockBits,
		targetBlockBits,
		o.allocator,
		func(buf []byte) {},
	)

	for _, block := range o.blocks {
		cloned.WriteBytes(block)
		if o.recycler != nil {
			o.recycler(block)
		}
	}
	if o.currentBlock != nil {
		cloned.WriteBytes(o.currentBlock)
	}

	o.blocks = cloned.blocks
	o.currentBlock = cloned.currentBlock
	o.blockBits = targetBlockBits
	o.ramBytesUsed = cloned.ramBytesUsed
}
