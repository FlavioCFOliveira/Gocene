// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"fmt"
	"math"
	"unsafe"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// PagedBytes represents a logical byte[] as a series of pages.
// You can write-once into the logical byte[] (append only), using copy,
// and then retrieve slices (BytesRef) into it using fill.
//
// This is the Go port of Lucene's org.apache.lucene.util.PagedBytes.
type PagedBytes struct {
	blocks            [][]byte
	numBlocks         int
	blockSize         int
	blockBits         int
	blockMask         int
	didSkipBytes      bool
	frozen            bool
	upto              int
	currentBlock      []byte
	bytesUsedPerBlock int64
}

var emptyBytes = make([]byte, 0)

// Reader provides methods to read BytesRefs from a frozen PagedBytes.
type Reader struct {
	blocks            [][]byte
	blockBits         int
	blockMask         int
	blockSize         int
	bytesUsedPerBlock int64
}

// NewPagedBytes creates a new PagedBytes with the given block bits.
// 1<<blockBits must be bigger than biggest single BytesRef slice that will be pulled.
func NewPagedBytes(blockBits int) (*PagedBytes, error) {
	if blockBits <= 0 || blockBits > 31 {
		return nil, fmt.Errorf("blockBits must be between 1 and 31, got %d", blockBits)
	}
	blockSize := 1 << blockBits
	return &PagedBytes{
		blocks:            make([][]byte, 16),
		blockSize:         blockSize,
		blockBits:         blockBits,
		blockMask:         blockSize - 1,
		upto:              blockSize,
		bytesUsedPerBlock: int64(blockSize + 24), // Approximate RAM usage per block (array header + alignment)
		numBlocks:         0,
	}, nil
}

// addBlock adds a block to the blocks array
func (p *PagedBytes) addBlock(block []byte) {
	if p.numBlocks == len(p.blocks) {
		// Grow blocks array
		newBlocks := make([][]byte, len(p.blocks)*2)
		copy(newBlocks, p.blocks)
		p.blocks = newBlocks
	}
	p.blocks[p.numBlocks] = block
	p.numBlocks++
}

// Copy copies bytes from the given IndexInput.
func (p *PagedBytes) Copy(input store.IndexInput, byteCount int64) error {
	if p.frozen {
		return fmt.Errorf("cannot copy after freeze")
	}

	for byteCount > 0 {
		left := p.blockSize - p.upto
		if left == 0 {
			if p.currentBlock != nil {
				p.addBlock(p.currentBlock)
			}
			p.currentBlock = make([]byte, p.blockSize)
			p.upto = 0
			left = p.blockSize
		}

		if int64(left) < byteCount {
			// Read partial block
			if err := input.ReadBytes(p.currentBlock[p.upto : p.upto+left]); err != nil {
				return err
			}
			p.upto = p.blockSize
			byteCount -= int64(left)
		} else {
			// Read remaining bytes
			if err := input.ReadBytes(p.currentBlock[p.upto : p.upto+int(byteCount)]); err != nil {
				return err
			}
			p.upto += int(byteCount)
			break
		}
	}
	return nil
}

// CopyBytesRef copies a BytesRef into this PagedBytes and sets the out BytesRef to the result.
// Do not use this if you will use Freeze(true). This only supports bytes.Length <= blockSize.
func (p *PagedBytes) CopyBytesRef(bytes *BytesRef, out *BytesRef) error {
	if p.frozen {
		return fmt.Errorf("cannot copy after freeze")
	}
	if bytes == nil {
		return nil
	}

	left := p.blockSize - p.upto
	if bytes.Length > left || p.currentBlock == nil {
		if p.currentBlock != nil {
			p.addBlock(p.currentBlock)
			p.didSkipBytes = true
		}
		p.currentBlock = make([]byte, p.blockSize)
		p.upto = 0
		left = p.blockSize
		if bytes.Length > p.blockSize {
			return fmt.Errorf("bytes length %d exceeds block size %d", bytes.Length, p.blockSize)
		}
	}

	out.Bytes = p.currentBlock
	out.Offset = p.upto
	out.Length = bytes.Length

	copy(p.currentBlock[p.upto:], bytes.Bytes[bytes.Offset:bytes.Offset+bytes.Length])
	p.upto += bytes.Length
	return nil
}

// Freeze commits the final byte[], trimming it if necessary and if trim=true.
// Returns a Reader for reading the frozen data.
func (p *PagedBytes) Freeze(trim bool) (*Reader, error) {
	if p.frozen {
		return nil, fmt.Errorf("already frozen")
	}
	if p.didSkipBytes {
		return nil, fmt.Errorf("cannot freeze when CopyBytesRef was used")
	}

	if trim && p.upto < p.blockSize && p.currentBlock != nil {
		newBlock := make([]byte, p.upto)
		copy(newBlock, p.currentBlock[:p.upto])
		p.currentBlock = newBlock
	}

	if p.currentBlock == nil {
		p.currentBlock = emptyBytes
	}

	p.addBlock(p.currentBlock)
	p.frozen = true
	p.currentBlock = nil

	// Copy blocks for the reader
	readerBlocks := make([][]byte, p.numBlocks)
	copy(readerBlocks, p.blocks[:p.numBlocks])

	return &Reader{
		blocks:            readerBlocks,
		blockBits:         p.blockBits,
		blockMask:         p.blockMask,
		blockSize:         p.blockSize,
		bytesUsedPerBlock: p.bytesUsedPerBlock,
	}, nil
}

// GetPointer returns the current position.
func (p *PagedBytes) GetPointer() int64 {
	if p.currentBlock == nil {
		return 0
	}
	return int64(p.numBlocks*p.blockSize) + int64(p.upto)
}

// RamBytesUsed returns the RAM usage in bytes.
func (p *PagedBytes) RamBytesUsed() int64 {
	size := int64(24) + int64(len(p.blocks))*8 // Base size + blocks array overhead
	if p.numBlocks > 0 {
		size += int64(p.numBlocks-1) * p.bytesUsedPerBlock
		size += int64(len(p.blocks[p.numBlocks-1])) + 24 // Last block
	}
	if p.currentBlock != nil {
		size += int64(len(p.currentBlock)) + 24
	}
	return size
}

// CopyUsingLengthPrefix copies bytes in, writing the length as a 1 or 2 byte vInt prefix.
// Returns the pointer where the data was written.
func (p *PagedBytes) CopyUsingLengthPrefix(bytes *BytesRef) (int64, error) {
	if p.frozen {
		return 0, fmt.Errorf("cannot copy after freeze")
	}
	if bytes.Length >= 32768 {
		return 0, fmt.Errorf("max length is 32767, got %d", bytes.Length)
	}

	if p.upto+bytes.Length+2 > p.blockSize {
		if bytes.Length+2 > p.blockSize {
			return 0, fmt.Errorf("block size %d is too small to store length %d bytes", p.blockSize, bytes.Length)
		}
		if p.currentBlock != nil {
			p.addBlock(p.currentBlock)
		}
		p.currentBlock = make([]byte, p.blockSize)
		p.upto = 0
	}

	pointer := p.GetPointer()

	if bytes.Length < 128 {
		p.currentBlock[p.upto] = byte(bytes.Length)
		p.upto++
	} else {
		// Write 2-byte length with high bit set
		lengthWithFlag := uint16(bytes.Length) | 0x8000
		p.currentBlock[p.upto] = byte(lengthWithFlag >> 8)
		p.currentBlock[p.upto+1] = byte(lengthWithFlag & 0xFF)
		p.upto += 2
	}
	copy(p.currentBlock[p.upto:], bytes.Bytes[bytes.Offset:bytes.Offset+bytes.Length])
	p.upto += bytes.Length

	return pointer, nil
}

// GetDataInput returns a DataInput to read values from this PagedBytes instance.
// Must call Freeze() before calling this method.
func (p *PagedBytes) GetDataInput() (*PagedBytesDataInput, error) {
	if !p.frozen {
		return nil, fmt.Errorf("must call Freeze() before GetDataInput")
	}
	if p.numBlocks == 0 {
		return &PagedBytesDataInput{
			pagedBytes: p,
			blocks:     p.blocks,
			blockBits:  p.blockBits,
			blockMask:  p.blockMask,
			blockSize:  p.blockSize,
		}, nil
	}
	return &PagedBytesDataInput{
		pagedBytes:        p,
		blocks:            p.blocks,
		currentBlock:      p.blocks[0],
		blockBits:         p.blockBits,
		blockMask:         p.blockMask,
		blockSize:         p.blockSize,
		currentBlockIndex: 0,
		currentBlockUpto:  0,
	}, nil
}

// GetDataOutput returns a DataOutput that you may use to write into this PagedBytes instance.
// If you do this, you should not call the other writing methods (eg, Copy).
func (p *PagedBytes) GetDataOutput() *PagedBytesDataOutput {
	return &PagedBytesDataOutput{pagedBytes: p}
}

// FillSlice fills the given BytesRef with a slice starting at start with the given length.
// Slices spanning more than two blocks are not supported.
func (r *Reader) FillSlice(b *BytesRef, start int64, length int) error {
	if length < 0 {
		return fmt.Errorf("length must be non-negative, got %d", length)
	}
	if length > r.blockSize+1 {
		return fmt.Errorf("length %d exceeds max allowed %d", length, r.blockSize+1)
	}

	b.Length = length
	if length == 0 {
		return nil
	}

	index := int(start >> r.blockBits)
	offset := int(start & int64(r.blockMask))

	if r.blockSize-offset >= length {
		// Within single block
		b.Bytes = r.blocks[index]
		b.Offset = offset
	} else {
		// Split across blocks - need to copy
		b.Bytes = make([]byte, length)
		b.Offset = 0
		firstPart := r.blockSize - offset
		copy(b.Bytes[:firstPart], r.blocks[index][offset:])
		copy(b.Bytes[firstPart:], r.blocks[index+1][:length-firstPart])
	}
	return nil
}

// GetByte gets the byte at the given offset.
func (r *Reader) GetByte(offset int64) byte {
	index := int(offset >> r.blockBits)
	pos := int(offset & int64(r.blockMask))
	return r.blocks[index][pos]
}

// Fill reads length as 1 or 2 byte vInt prefix, starting at start.
// Note: this method does not support slices spanning across block borders.
func (r *Reader) Fill(b *BytesRef, start int64) error {
	index := int(start >> r.blockBits)
	offset := int(start & int64(r.blockMask))
	block := r.blocks[index]
	b.Bytes = block

	if (block[offset] & 128) == 0 {
		b.Length = int(block[offset])
		b.Offset = offset + 1
	} else {
		// 2-byte length with high bit set
		b.Length = int(((uint16(block[offset]) << 8) | uint16(block[offset+1])) & 0x7FFF)
		b.Offset = offset + 2
	}
	return nil
}

// RamBytesUsed returns the RAM usage in bytes.
func (r *Reader) RamBytesUsed() int64 {
	size := int64(24) + int64(len(r.blocks))*8 // Base + blocks array
	if len(r.blocks) > 0 {
		size += int64(len(r.blocks)-1) * r.bytesUsedPerBlock
		size += int64(len(r.blocks[len(r.blocks)-1])) + 24 // Last block
	}
	return size
}

// PagedBytesDataInput implements DataInput for reading from PagedBytes.
type PagedBytesDataInput struct {
	pagedBytes        *PagedBytes
	blocks            [][]byte
	currentBlockIndex int
	currentBlockUpto  int
	currentBlock      []byte
	blockBits         int
	blockMask         int
	blockSize         int
}

// ReadByte reads a single byte.
func (in *PagedBytesDataInput) ReadByte() (byte, error) {
	if in.currentBlockUpto == in.blockSize {
		if err := in.nextBlock(); err != nil {
			return 0, err
		}
	}
	b := in.currentBlock[in.currentBlockUpto]
	in.currentBlockUpto++
	return b, nil
}

// ReadBytes reads len(b) bytes into b.
func (in *PagedBytesDataInput) ReadBytes(b []byte) error {
	if len(b) == 0 {
		return nil
	}

	offset := 0
	offsetEnd := len(b)

	for {
		blockLeft := in.blockSize - in.currentBlockUpto
		left := offsetEnd - offset

		if blockLeft < left {
			copy(b[offset:], in.currentBlock[in.currentBlockUpto:in.currentBlockUpto+blockLeft])
			if err := in.nextBlock(); err != nil {
				return err
			}
			offset += blockLeft
		} else {
			// Last block
			copy(b[offset:], in.currentBlock[in.currentBlockUpto:in.currentBlockUpto+left])
			in.currentBlockUpto += left
			break
		}
	}
	return nil
}

// ReadBytesN reads exactly n bytes and returns them.
func (in *PagedBytesDataInput) ReadBytesN(n int) ([]byte, error) {
	buf := make([]byte, n)
	if err := in.ReadBytes(buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// SkipBytes skips n bytes.
func (in *PagedBytesDataInput) SkipBytes(n int64) error {
	if n < 0 {
		return fmt.Errorf("numBytes must be >= 0, got %d", n)
	}
	skipTo := in.GetPosition() + n
	in.SetPosition(skipTo)
	return nil
}

// GetPosition returns the current byte position.
func (in *PagedBytesDataInput) GetPosition() int64 {
	return int64(in.currentBlockIndex)*int64(in.blockSize) + int64(in.currentBlockUpto)
}

// SetPosition seeks to a position previously obtained from GetPosition.
func (in *PagedBytesDataInput) SetPosition(pos int64) {
	in.currentBlockIndex = int(pos >> int64(in.blockBits))
	if in.currentBlockIndex < len(in.blocks) {
		in.currentBlock = in.blocks[in.currentBlockIndex]
	} else {
		in.currentBlock = nil
	}
	in.currentBlockUpto = int(pos & int64(in.blockMask))
}

// nextBlock advances to the next block.
func (in *PagedBytesDataInput) nextBlock() error {
	in.currentBlockIndex++
	if in.currentBlockIndex >= len(in.blocks) {
		return fmt.Errorf("attempted to read past end of data")
	}
	in.currentBlockUpto = 0
	in.currentBlock = in.blocks[in.currentBlockIndex]
	return nil
}

// Clone returns a clone of this PagedBytesDataInput.
func (in *PagedBytesDataInput) Clone() *PagedBytesDataInput {
	clone := &PagedBytesDataInput{
		pagedBytes:        in.pagedBytes,
		blocks:            in.blocks,
		currentBlockIndex: in.currentBlockIndex,
		currentBlockUpto:  in.currentBlockUpto,
		currentBlock:      in.currentBlock,
		blockBits:         in.blockBits,
		blockMask:         in.blockMask,
		blockSize:         in.blockSize,
	}
	return clone
}

// PagedBytesDataOutput implements DataOutput for writing to PagedBytes.
type PagedBytesDataOutput struct {
	pagedBytes *PagedBytes
}

// WriteByte writes a single byte.
func (out *PagedBytesDataOutput) WriteByte(b byte) error {
	p := out.pagedBytes
	if p.frozen {
		return fmt.Errorf("cannot write after freeze")
	}

	if p.upto == p.blockSize {
		if p.currentBlock != nil {
			p.addBlock(p.currentBlock)
		}
		p.currentBlock = make([]byte, p.blockSize)
		p.upto = 0
	}
	p.currentBlock[p.upto] = b
	p.upto++
	return nil
}

// WriteBytes writes bytes from b.
func (out *PagedBytesDataOutput) WriteBytes(b []byte) error {
	if len(b) == 0 {
		return nil
	}

	p := out.pagedBytes
	if p.frozen {
		return fmt.Errorf("cannot write after freeze")
	}

	if p.upto == p.blockSize {
		if p.currentBlock != nil {
			p.addBlock(p.currentBlock)
		}
		p.currentBlock = make([]byte, p.blockSize)
		p.upto = 0
	}

	offset := 0
	offsetEnd := len(b)

	for {
		left := offsetEnd - offset
		blockLeft := p.blockSize - p.upto

		if blockLeft < left {
			copy(p.currentBlock[p.upto:], b[offset:offset+blockLeft])
			p.addBlock(p.currentBlock)
			p.currentBlock = make([]byte, p.blockSize)
			p.upto = 0
			offset += blockLeft
		} else {
			// Last block
			copy(p.currentBlock[p.upto:], b[offset:offset+left])
			p.upto += left
			break
		}
	}
	return nil
}

// WriteBytesN writes exactly length bytes from b.
func (out *PagedBytesDataOutput) WriteBytesN(b []byte, length int) error {
	if length > len(b) {
		return fmt.Errorf("length %d exceeds buffer size %d", length, len(b))
	}
	return out.WriteBytes(b[:length])
}

// GetPosition returns the current byte position.
func (out *PagedBytesDataOutput) GetPosition() int64 {
	return out.pagedBytes.GetPointer()
}

// Helper functions for tests

// GetBlockSize returns the block size (for testing).
func (p *PagedBytes) GetBlockSize() int {
	return p.blockSize
}

// GetNumBlocks returns the number of blocks (for testing).
func (p *PagedBytes) GetNumBlocks() int {
	return p.numBlocks
}

// IsFrozen returns whether this PagedBytes is frozen (for testing).
func (p *PagedBytes) IsFrozen() bool {
	return p.frozen
}

// GetBlockBits returns the block bits (for testing).
func (p *PagedBytes) GetBlockBits() int {
	return p.blockBits
}

// GetBlockSize returns the block size (for testing).
func (r *Reader) GetBlockSize() int {
	return r.blockSize
}

// ensure we implement the interfaces
var _ store.DataInput = (*PagedBytesDataInput)(nil)
var _ store.DataOutput = (*PagedBytesDataOutput)(nil)

// Additional DataInput methods to satisfy interface

// ReadShort reads a 16-bit value.
func (in *PagedBytesDataInput) ReadShort() (int16, error) {
	b, err := in.ReadBytesN(2)
	if err != nil {
		return 0, err
	}
	return int16(b[0])<<8 | int16(b[1]), nil
}

// ReadInt reads a 32-bit value.
func (in *PagedBytesDataInput) ReadInt() (int32, error) {
	b, err := in.ReadBytesN(4)
	if err != nil {
		return 0, err
	}
	return int32(b[0])<<24 | int32(b[1])<<16 | int32(b[2])<<8 | int32(b[3]), nil
}

// ReadLong reads a 64-bit value.
func (in *PagedBytesDataInput) ReadLong() (int64, error) {
	b, err := in.ReadBytesN(8)
	if err != nil {
		return 0, err
	}
	return int64(b[0])<<56 | int64(b[1])<<48 | int64(b[2])<<40 | int64(b[3])<<32 |
		int64(b[4])<<24 | int64(b[5])<<16 | int64(b[6])<<8 | int64(b[7]), nil
}

// ReadString reads a string.
func (in *PagedBytesDataInput) ReadString() (string, error) {
	length, err := in.ReadInt()
	if err != nil {
		return "", err
	}
	b, err := in.ReadBytesN(int(length))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Additional DataOutput methods to satisfy interface

// WriteShort writes a 16-bit value.
func (out *PagedBytesDataOutput) WriteShort(v int16) error {
	buf := []byte{byte(v >> 8), byte(v)}
	return out.WriteBytes(buf)
}

// WriteInt writes a 32-bit value.
func (out *PagedBytesDataOutput) WriteInt(v int32) error {
	buf := []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	return out.WriteBytes(buf)
}

// WriteLong writes a 64-bit value.
func (out *PagedBytesDataOutput) WriteLong(v int64) error {
	buf := make([]byte, 8)
	buf[0] = byte(v >> 56)
	buf[1] = byte(v >> 48)
	buf[2] = byte(v >> 40)
	buf[3] = byte(v >> 32)
	buf[4] = byte(v >> 24)
	buf[5] = byte(v >> 16)
	buf[6] = byte(v >> 8)
	buf[7] = byte(v)
	return out.WriteBytes(buf)
}

// WriteVInt writes a variable-length integer.
func (out *PagedBytesDataOutput) WriteVInt(v int32) error {
	uv := uint32(v)
	for uv >= 0x80 {
		if err := out.WriteByte(byte(uv | 0x80)); err != nil {
			return err
		}
		uv >>= 7
	}
	return out.WriteByte(byte(uv))
}

// WriteVLong writes a variable-length long.
func (out *PagedBytesDataOutput) WriteVLong(v int64) error {
	uv := uint64(v)
	for uv >= 0x80 {
		if err := out.WriteByte(byte(uv | 0x80)); err != nil {
			return err
		}
		uv >>= 7
	}
	return out.WriteByte(byte(uv))
}

// WriteZInt writes a zig-zag encoded integer.
func (out *PagedBytesDataOutput) WriteZInt(v int32) error {
	return out.WriteVInt((v << 1) ^ (v >> 31))
}

// WriteZLong writes a zig-zag encoded long.
func (out *PagedBytesDataOutput) WriteZLong(v int64) error {
	return out.WriteVLong((v << 1) ^ (v >> 63))
}

// WriteString writes a string.
// Uses unsafe conversion to avoid heap allocation.
func (out *PagedBytesDataOutput) WriteString(s string) error {
	if err := out.WriteVInt(int32(len(s))); err != nil {
		return err
	}
	// Unsafe conversion: string -> []byte without allocation
	// Safe because WriteBytes only reads the data
	if len(s) > 0 {
		data := unsafe.Slice(unsafe.StringData(s), len(s))
		return out.WriteBytes(data)
	}
	return nil
}

// CopyBytes copies bytes from a DataInput.
func (out *PagedBytesDataOutput) CopyBytes(input store.DataInput, numBytes int64) error {
	buf := make([]byte, int(math.Min(float64(numBytes), 8192)))
	remaining := numBytes
	for remaining > 0 {
		toRead := int64(len(buf))
		if remaining < toRead {
			toRead = remaining
		}
		n, err := input.ReadBytesN(int(toRead))
		if err != nil {
			return err
		}
		if err := out.WriteBytes(n); err != nil {
			return err
		}
		remaining -= toRead
	}
	return nil
}
