// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// LegacyPacked64SingleBlock is an immutable packed-integer reader loaded from
// a DataInput stream that was written using the PACKED_SINGLE_BLOCK format
// (no header). Values never cross 64-bit block boundaries.
//
// Port of
// org.apache.lucene.backward_codecs.packed.LegacyPacked64SingleBlock
// (Lucene 10.4.0).
//
// Reading deviation: Gocene's store.DataInput.ReadLong() is little-endian,
// but the on-disk format uses big-endian longs. Bytes are therefore read
// individually and assembled in MSB-first order — identical to what Java's
// DataInput.readLong() produces.
type LegacyPacked64SingleBlock struct {
	valueCount     int
	bitsPerValue   int
	valuesPerBlock int
	mask           uint64
	blocks         []int64
}

// newLegacyPacked64SingleBlock reads valueCount packed integers from in.
// Each 64-bit block is read byte-by-byte (big-endian) to match the Java wire
// format. bitsPerValue must be one of the supported values (see IsSupported).
func newLegacyPacked64SingleBlock(in store.DataInput, valueCount, bitsPerValue int) (*LegacyPacked64SingleBlock, error) {
	if !packed.FormatPackedSingleBlock.IsSupported(bitsPerValue) {
		return nil, fmt.Errorf("legacyPacked64SingleBlock: unsupported bitsPerValue %d", bitsPerValue)
	}
	valuesPerBlock := 64 / bitsPerValue
	numBlocks := valueCount/valuesPerBlock + boolToInt(valueCount%valuesPerBlock != 0)
	blocks := make([]int64, numBlocks)

	// Read each 64-bit block as 8 big-endian bytes.
	for i := range blocks {
		var block int64
		for bit := 56; bit >= 0; bit -= 8 {
			b, err := in.ReadByte()
			if err != nil {
				return nil, fmt.Errorf("legacyPacked64SingleBlock: read byte in block[%d]: %w", i, err)
			}
			block |= int64(b&0xFF) << uint(bit)
		}
		blocks[i] = block
	}

	mask := uint64((1 << uint(bitsPerValue)) - 1)
	return &LegacyPacked64SingleBlock{
		valueCount:     valueCount,
		bitsPerValue:   bitsPerValue,
		valuesPerBlock: valuesPerBlock,
		mask:           mask,
		blocks:         blocks,
	}, nil
}

// Get returns the packed integer at the given index.
//
// Port of LegacyPacked64SingleBlock.get(int) (delegate to per-subclass get).
func (p *LegacyPacked64SingleBlock) Get(index int) int64 {
	o := index / p.valuesPerBlock
	b := index % p.valuesPerBlock
	shift := uint(b * p.bitsPerValue)
	return int64((uint64(p.blocks[o]) >> shift) & p.mask)
}

// GetBulk reads up to length values starting at index into arr[off:].
// Returns the number of values actually read.
func (p *LegacyPacked64SingleBlock) GetBulk(index int, arr []int64, off, length int) int {
	if remaining := p.valueCount - index; remaining < length {
		length = remaining
	}
	for i := 0; i < length; i++ {
		arr[off+i] = p.Get(index + i)
	}
	return length
}

// Size returns the number of values.
func (p *LegacyPacked64SingleBlock) Size() int { return p.valueCount }

// RamBytesUsed returns approximate heap usage.
func (p *LegacyPacked64SingleBlock) RamBytesUsed() int64 {
	return int64(len(p.blocks)) * 8
}

// String returns a debug-friendly representation.
func (p *LegacyPacked64SingleBlock) String() string {
	return fmt.Sprintf("LegacyPacked64SingleBlock(bitsPerValue=%d,size=%d,blocks=%d)",
		p.bitsPerValue, p.valueCount, len(p.blocks))
}

// IsSupported reports whether bitsPerValue is valid for PACKED_SINGLE_BLOCK.
// Mirrors LegacyPacked64SingleBlock.isSupported(int).
func IsSupported(bitsPerValue int) bool {
	return packed.FormatPackedSingleBlock.IsSupported(bitsPerValue)
}

// CreateLegacyPacked64SingleBlock creates a LegacyPacked64SingleBlock by
// reading from in. Exposed for use by GetReaderNoHeader and any future
// LegacyPackedInts.getReaderNoHeader PACKED_SINGLE_BLOCK path.
//
// Port of LegacyPacked64SingleBlock.create(DataInput, int, int).
func CreateLegacyPacked64SingleBlock(in store.DataInput, valueCount, bitsPerValue int) (packed.Reader, error) {
	return newLegacyPacked64SingleBlock(in, valueCount, bitsPerValue)
}

// boolToInt converts a bool to 0 or 1.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
