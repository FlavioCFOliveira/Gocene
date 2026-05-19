// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"github.com/FlavioCFOliveira/Gocene/store"
)

// DirectPackedReader provides random access over an on-disk PackedInts
// stream encoded with FormatPacked, by seeking and reading only the
// bytes that overlap the requested value on every Get. It holds no
// decoded buffer, so it trades CPU and I/O cost for zero heap
// residency.
//
// This is the Go port of org.apache.lucene.util.packed.DirectPackedReader
// in Apache Lucene 10.4.0. The class is kept solely for back-compat;
// new code should prefer DirectReader / DirectWriter, which produce a
// more efficient layout.
//
// Wire format / endianness
//
// The byte stream consumed here is the one PackedWriter emits via the
// PACKED encoder, which lays the most significant byte of each
// 64-bit conceptual block first. The Java reference therefore
// reassembles multi-byte words via IndexInput.readShort/readInt/
// readLong, all of which return big-endian. Gocene's
// store.IndexInput.ReadShort/Int/Long are little-endian
// (see store/index_input.go), so calling them here would mis-decode
// the stream. To stay byte-for-byte compatible with the Lucene
// reference and with PackedWriter's output, we bypass ReadShort/Int/
// Long entirely and assemble the rawValue from successive ReadByte
// calls in big-endian order.
type DirectPackedReader struct {
	readerImpl
	in           store.IndexInput
	bitsPerValue int
	startPointer int64
	valueMask    uint64
}

// NewDirectPackedReader builds a reader over valueCount values of
// bitsPerValue width, starting at the current file pointer of in. The
// reader retains in and seeks within it on every Get.
func NewDirectPackedReader(bitsPerValue, valueCount int, in store.IndexInput) *DirectPackedReader {
	var mask uint64
	if bitsPerValue == 64 {
		mask = ^uint64(0)
	} else {
		mask = (uint64(1) << uint(bitsPerValue)) - 1
	}
	return &DirectPackedReader{
		readerImpl:   readerImpl{valueCount: valueCount},
		in:           in,
		bitsPerValue: bitsPerValue,
		startPointer: in.GetFilePointer(),
		valueMask:    mask,
	}
}

// Get returns the value at the given index. The underlying input is
// seeked on every call and the minimal number of bytes covering the
// value is read. Any I/O failure surfaces as a panic, matching the
// reference behavior of the Java original
// (RuntimeException(IOException)).
func (r *DirectPackedReader) Get(index int) int64 {
	majorBitPos := int64(index) * int64(r.bitsPerValue)
	elementPos := majorBitPos >> 3
	if err := r.in.SetPosition(r.startPointer + elementPos); err != nil {
		panic(err)
	}

	bitPos := int(majorBitPos & 7)
	// Round up bits to a multiple of 8 to find total bytes needed
	// to read.
	roundedBits := (bitPos + r.bitsPerValue + 7) &^ 7
	// The number of extra bits read at the end to shift out.
	shiftRightBits := roundedBits - bitPos - r.bitsPerValue

	// Assemble rawValue from ReadByte in big-endian order. We
	// deliberately do NOT use ReadShort/ReadInt/ReadLong here: the
	// Gocene store.IndexInput emits little-endian, while the Lucene
	// PackedWriter stream this reader consumes is big-endian inside
	// each 64-bit conceptual block. See the package doc on this
	// type for the full rationale.
	var rawValue uint64
	switch roundedBits >> 3 {
	case 1:
		b, err := r.in.ReadByte()
		if err != nil {
			panic(err)
		}
		rawValue = uint64(b)
	case 2:
		rawValue = r.readBytesBE(2)
	case 3:
		rawValue = r.readBytesBE(3)
	case 4:
		rawValue = r.readBytesBE(4)
	case 5:
		rawValue = r.readBytesBE(5)
	case 6:
		rawValue = r.readBytesBE(6)
	case 7:
		rawValue = r.readBytesBE(7)
	case 8:
		rawValue = r.readBytesBE(8)
	case 9:
		// We must be very careful not to shift out relevant bits.
		// Mirror the Java path: shift the first 8 bytes left by
		// (8 - shiftRightBits), OR in the 9th byte shifted right
		// by shiftRightBits, then suppress the final shift.
		hi := r.readBytesBE(8)
		lo, err := r.in.ReadByte()
		if err != nil {
			panic(err)
		}
		rawValue = (hi << uint(8-shiftRightBits)) | (uint64(lo) >> uint(shiftRightBits))
		shiftRightBits = 0
	default:
		panic("packed: bitsPerValue too large")
	}
	return int64((rawValue >> uint(shiftRightBits)) & r.valueMask)
}

// readBytesBE reads exactly n bytes (1 <= n <= 8) from the underlying
// input and assembles them into a uint64 in big-endian order. Used to
// replace ReadShort/Int/Long, which would yield little-endian on
// Gocene's store.IndexInput. Any read failure panics, matching Get.
func (r *DirectPackedReader) readBytesBE(n int) uint64 {
	var v uint64
	for i := 0; i < n; i++ {
		b, err := r.in.ReadByte()
		if err != nil {
			panic(err)
		}
		v = (v << 8) | uint64(b)
	}
	return v
}

// GetBulk fills arr[off:off+length] with up to length values starting
// at index using the generic sequential bulk-get.
func (r *DirectPackedReader) GetBulk(index int, arr []int64, off, length int) int {
	return readerBulkGet(r, index, arr, off, length)
}

// RamBytesUsed mirrors the Java reference which reports zero heap
// usage because the reader holds no decoded buffer.
func (r *DirectPackedReader) RamBytesUsed() int64 { return 0 }
