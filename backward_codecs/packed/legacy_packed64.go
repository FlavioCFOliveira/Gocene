// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// LegacyPacked64 is an immutable packed-integer reader loaded directly from a
// DataInput stream that was written without a header
// (PackedInts.getWriterNoHeader / Format.PACKED). It mirrors
// org.apache.lucene.backward_codecs.packed.LegacyPacked64 (Lucene 10.4.0).
//
// The bit layout is identical to util/packed.Packed64; the only difference
// is that this variant is constructed by consuming bytes from a stream rather
// than being built incrementally.
type LegacyPacked64 struct {
	valueCount        int
	bitsPerValue      int
	blocks            []int64
	maskRight         uint64
	bpvMinusBlockSize int
}

const (
	legacyBlockSize = 64 // bits per backing long
	legacyBlockBits = 6  // log2(blockSize)
	legacyModMask   = legacyBlockSize - 1
)

// newLegacyPacked64 constructs a LegacyPacked64 by reading packed data from in.
//
// packedIntsVersion is the PackedInts format version used when the data was
// written (controls how many bytes are on disk via Format.ByteCount).
// valueCount is the number of packed integers; bitsPerValue their width.
//
// Port of LegacyPacked64(int, DataInput, int, int).
func newLegacyPacked64(packedIntsVersion int, in store.DataInput, valueCount, bitsPerValue int) (*LegacyPacked64, error) {
	byteCount := packed.FormatPacked.ByteCount(packedIntsVersion, valueCount, bitsPerValue)
	longCount := packed.FormatPacked.LongCount(packed.VersionCurrent, valueCount, bitsPerValue)
	blocks := make([]int64, longCount)

	// Read all bytes and assemble blocks in big-endian order (MSB first),
	// matching the PACKED encoder's byte-by-byte output layout.
	// We do NOT use in.ReadLong() because Gocene's native ReadLong is
	// little-endian, whereas the PACKED format requires big-endian byte
	// order within each 64-bit block.
	totalBytes := int(byteCount)
	blockIdx := 0
	for i := 0; i < totalBytes; {
		var block int64
		for bit := 56; bit >= 0 && i < totalBytes; bit -= 8 {
			b, err := in.ReadByte()
			if err != nil {
				return nil, fmt.Errorf("legacyPacked64: read byte[%d]: %w", i, err)
			}
			block |= int64(b&0xFF) << uint(bit)
			i++
		}
		blocks[blockIdx] = block
		blockIdx++
	}

	mask := (^uint64(0) << uint(legacyBlockSize-bitsPerValue)) >> uint(legacyBlockSize-bitsPerValue)
	return &LegacyPacked64{
		valueCount:        valueCount,
		bitsPerValue:      bitsPerValue,
		blocks:            blocks,
		maskRight:         mask,
		bpvMinusBlockSize: bitsPerValue - legacyBlockSize,
	}, nil
}

// Get returns the packed integer at the given index.
//
// Port of LegacyPacked64.get(int).
func (p *LegacyPacked64) Get(index int) int64 {
	majorBitPos := int64(index) * int64(p.bitsPerValue)
	elementPos := int(uint64(majorBitPos) >> uint(legacyBlockBits))
	endBits := int(majorBitPos&legacyModMask) + p.bpvMinusBlockSize
	if endBits <= 0 {
		return int64((uint64(p.blocks[elementPos]) >> uint(-endBits)) & p.maskRight)
	}
	high := uint64(p.blocks[elementPos]) << uint(endBits)
	low := uint64(p.blocks[elementPos+1]) >> uint(legacyBlockSize-endBits)
	return int64((high | low) & p.maskRight)
}

// GetBulk reads up to length values starting at index into arr[off:].
// Returns the number of values actually read.
func (p *LegacyPacked64) GetBulk(index int, arr []int64, off, length int) int {
	if remaining := p.valueCount - index; remaining < length {
		length = remaining
	}
	for i := 0; i < length; i++ {
		arr[off+i] = p.Get(index + i)
	}
	return length
}

// Size returns the number of values.
func (p *LegacyPacked64) Size() int { return p.valueCount }

// RamBytesUsed returns approximate heap usage.
func (p *LegacyPacked64) RamBytesUsed() int64 {
	return int64(len(p.blocks)) * 8
}

// String returns a debug-friendly representation.
func (p *LegacyPacked64) String() string {
	return fmt.Sprintf("LegacyPacked64(bitsPerValue=%d,size=%d,blocks=%d)",
		p.bitsPerValue, p.valueCount, len(p.blocks))
}

// GetReaderNoHeader reads a packed.Reader using Format.PACKED (no header).
// This is the format used by LegacyFieldsIndexReader.
//
// Port of LegacyPackedInts.getReaderNoHeader(DataInput, Format.PACKED, int, int, int).
func GetReaderNoHeader(in store.DataInput, version, valueCount, bitsPerValue int) (packed.Reader, error) {
	return GetReaderNoHeaderFormat(in, packed.FormatPacked, version, valueCount, bitsPerValue)
}

// GetReaderNoHeaderFormat reads a packed.Reader from in using the given legacy
// format (no header). Supports both FormatPacked and FormatPackedSingleBlock.
//
// Port of LegacyPackedInts.getReaderNoHeader(DataInput, Format, int, int, int).
func GetReaderNoHeaderFormat(in store.DataInput, format packed.Format, version, valueCount, bitsPerValue int) (packed.Reader, error) {
	if err := packed.CheckVersion(version); err != nil {
		return nil, err
	}
	switch format {
	case packed.FormatPackedSingleBlock:
		return CreateLegacyPacked64SingleBlock(in, valueCount, bitsPerValue)
	case packed.FormatPacked:
		if bitsPerValue == 0 {
			// Zero-width reader: all Get() calls return 0.
			return &zeroReader{size: valueCount}, nil
		}
		return newLegacyPacked64(version, in, valueCount, bitsPerValue)
	default:
		return nil, fmt.Errorf("legacyPackedInts: unknown format %d", format)
	}
}

// zeroReader is a Reader where every value is 0. Used when bitsPerValue == 0.
type zeroReader struct{ size int }

func (z *zeroReader) Get(_ int) int64                            { return 0 }
func (z *zeroReader) GetBulk(_ int, arr []int64, off, n int) int { clear(arr[off : off+n]); return n }
func (z *zeroReader) Size() int                                  { return z.size }
func (z *zeroReader) RamBytesUsed() int64                        { return 0 }
