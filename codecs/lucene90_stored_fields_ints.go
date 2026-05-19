// Package codecs - Lucene90 StoredFieldsInts: a small bit-packed integer
// codec used internally by Lucene90 compressing stored fields and term
// vectors to encode the per-document/chunk integer streams (e.g. number of
// stored fields, lengths, offsets).
//
// This file is a Go port of
// org.apache.lucene.codecs.lucene90.compressing.StoredFieldsInts
// (Apache Lucene 10.4.0).
//
// Format overview (matches the Java reference):
//   - The stream is written in blocks of BlockSize=128 ints.
//   - For each call to WriteStoredFieldsInts a single header byte selects
//     the number of bits per value used for the count:
//     0  -> all values are equal; the value is then written as a VInt.
//     8  -> each value fits in an unsigned byte.
//     16 -> each value fits in an unsigned short.
//     32 -> each value is written as a full 32-bit int.
//   - Within a 128-value block the values are packed into 64-bit longs in
//     a SIMD-friendly transposed layout (8/16/32 bpv -> 8/4/2 lanes per
//     long). Trailing values (< BlockSize) are written one at a time using
//     the obvious single-value writer for the bpv.
package codecs

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// Errors returned by the StoredFieldsInts decoder.
var (
	// ErrStoredFieldsIntsUnsupportedBPV is returned by ReadStoredFieldsInts
	// when the header byte selects a bits-per-value value that this codec
	// does not understand. Mirrors the IOException thrown by the Java
	// reference for any value other than 0, 8, 16, or 32.
	ErrStoredFieldsIntsUnsupportedBPV = errors.New("stored fields ints: unsupported number of bits per value")
)

// Block layout constants. The 128-value block size is part of the on-disk
// format and must match Lucene's StoredFieldsInts.BLOCK_SIZE exactly.
const (
	storedFieldsIntsBlockSize         = 128
	storedFieldsIntsBlockSizeMinusOne = storedFieldsIntsBlockSize - 1
)

// WriteStoredFieldsInts encodes count integers from values[start:start+count]
// to out using the Lucene90 StoredFieldsInts bit-packed block format.
//
// The function panics-free contract follows the Java reference: callers are
// expected to provide a slice large enough for start+count. count may be 0,
// in which case a single zero header byte and a VInt(values[0]) are still
// written when count <= 1 falls through to the all-equal branch (in the
// Java reference, count == 0 makes the all-equal loop vacuously true and
// emits a 0 header followed by VInt(values[0]); we preserve that behaviour
// to keep the wire format identical).
func WriteStoredFieldsInts(values []int32, start, count int, out store.DataOutput) error {
	allEqual := true
	for i := 1; i < count; i++ {
		if values[start+i] != values[start] {
			allEqual = false
			break
		}
	}
	if allEqual {
		if err := out.WriteByte(0); err != nil {
			return fmt.Errorf("stored fields ints: write all-equal header: %w", err)
		}
		// Mirrors Java's `out.writeVInt(values[0])` — note that the
		// reference unconditionally writes values[0], not values[start].
		if err := store.WriteVInt(out, values[0]); err != nil {
			return fmt.Errorf("stored fields ints: write all-equal vint: %w", err)
		}
		return nil
	}

	// Compute the unsigned max across the [start, start+count) window. We
	// OR the unsigned 32-bit values into a 64-bit accumulator (so that the
	// sign bit of int32 values is treated as a normal high bit), then
	// pick the smallest bpv that fits.
	var max uint64
	for i := 0; i < count; i++ {
		max |= uint64(uint32(values[start+i]))
	}
	switch {
	case max <= 0xff:
		if err := out.WriteByte(8); err != nil {
			return fmt.Errorf("stored fields ints: write 8bpv header: %w", err)
		}
		return writeStoredFieldsInts8(out, count, values, start)
	case max <= 0xffff:
		if err := out.WriteByte(16); err != nil {
			return fmt.Errorf("stored fields ints: write 16bpv header: %w", err)
		}
		return writeStoredFieldsInts16(out, count, values, start)
	default:
		if err := out.WriteByte(32); err != nil {
			return fmt.Errorf("stored fields ints: write 32bpv header: %w", err)
		}
		return writeStoredFieldsInts32(out, count, values, start)
	}
}

// writeStoredFieldsInts8 packs 8-bit values 8-per-long in transposed layout
// across full 128-value blocks; the tail is written one byte per value.
func writeStoredFieldsInts8(out store.DataOutput, count int, values []int32, offset int) error {
	k := 0
	for ; k < count-storedFieldsIntsBlockSizeMinusOne; k += storedFieldsIntsBlockSize {
		step := offset + k
		for i := 0; i < 16; i++ {
			l := uint64(uint32(values[step+i]))<<56 |
				uint64(uint32(values[step+16+i]))<<48 |
				uint64(uint32(values[step+32+i]))<<40 |
				uint64(uint32(values[step+48+i]))<<32 |
				uint64(uint32(values[step+64+i]))<<24 |
				uint64(uint32(values[step+80+i]))<<16 |
				uint64(uint32(values[step+96+i]))<<8 |
				uint64(uint32(values[step+112+i]))
			if err := out.WriteLong(int64(l)); err != nil {
				return fmt.Errorf("stored fields ints 8bpv: write long: %w", err)
			}
		}
	}
	for ; k < count; k++ {
		if err := out.WriteByte(byte(values[offset+k])); err != nil {
			return fmt.Errorf("stored fields ints 8bpv: write tail byte: %w", err)
		}
	}
	return nil
}

// writeStoredFieldsInts16 packs 16-bit values 4-per-long across full blocks;
// the tail is written one short per value.
func writeStoredFieldsInts16(out store.DataOutput, count int, values []int32, offset int) error {
	k := 0
	for ; k < count-storedFieldsIntsBlockSizeMinusOne; k += storedFieldsIntsBlockSize {
		step := offset + k
		for i := 0; i < 32; i++ {
			l := uint64(uint32(values[step+i]))<<48 |
				uint64(uint32(values[step+32+i]))<<32 |
				uint64(uint32(values[step+64+i]))<<16 |
				uint64(uint32(values[step+96+i]))
			if err := out.WriteLong(int64(l)); err != nil {
				return fmt.Errorf("stored fields ints 16bpv: write long: %w", err)
			}
		}
	}
	for ; k < count; k++ {
		if err := out.WriteShort(int16(values[offset+k])); err != nil {
			return fmt.Errorf("stored fields ints 16bpv: write tail short: %w", err)
		}
	}
	return nil
}

// writeStoredFieldsInts32 packs 32-bit values 2-per-long across full blocks;
// the tail is written one int per value.
func writeStoredFieldsInts32(out store.DataOutput, count int, values []int32, offset int) error {
	k := 0
	for ; k < count-storedFieldsIntsBlockSizeMinusOne; k += storedFieldsIntsBlockSize {
		step := offset + k
		for i := 0; i < 64; i++ {
			l := uint64(uint32(values[step+i]))<<32 |
				uint64(uint32(values[step+64+i]))
			if err := out.WriteLong(int64(l)); err != nil {
				return fmt.Errorf("stored fields ints 32bpv: write long: %w", err)
			}
		}
	}
	for ; k < count; k++ {
		if err := out.WriteInt(values[offset+k]); err != nil {
			return fmt.Errorf("stored fields ints 32bpv: write tail int: %w", err)
		}
	}
	return nil
}

// ReadStoredFieldsInts reads count integers from in into values[offset:offset+count],
// inverting the layout written by WriteStoredFieldsInts. Values are stored
// as int64 (unsigned-extended) to match the Java reference's long[] target
// — this mirrors the consumer-side widening used by Lucene90 compressing
// stored fields and term vectors readers.
//
// The Java reference takes IndexInput because it uses the bulk readLongs
// primitive; the Go port reads longs one at a time, so the broader
// DataInput surface is sufficient. Any IndexInput satisfies DataInput, so
// existing call sites are source-compatible.
func ReadStoredFieldsInts(in store.DataInput, count int, values []int64, offset int) error {
	bpv, err := in.ReadByte()
	if err != nil {
		return fmt.Errorf("stored fields ints: read header byte: %w", err)
	}
	switch bpv {
	case 0:
		v, err := store.ReadVInt(in)
		if err != nil {
			return fmt.Errorf("stored fields ints: read all-equal vint: %w", err)
		}
		// Unsigned-extend the int32 into int64 to match Java's long fill,
		// in which the int is promoted to long without sign extension only
		// when the source value is non-negative. The Java reference uses
		// in.readVInt() which itself can return a negative int (VInt is
		// signed), so we mirror that behaviour exactly.
		fill := int64(v)
		for i := offset; i < offset+count; i++ {
			values[i] = fill
		}
		return nil
	case 8:
		return readStoredFieldsInts8(in, count, values, offset)
	case 16:
		return readStoredFieldsInts16(in, count, values, offset)
	case 32:
		return readStoredFieldsInts32(in, count, values, offset)
	default:
		return fmt.Errorf("%w: %d", ErrStoredFieldsIntsUnsupportedBPV, int8(bpv))
	}
}

// readStoredFieldsInts8 inverts the 8-per-long transposed layout for full
// 128-value blocks, then reads single bytes for the tail.
func readStoredFieldsInts8(in store.DataInput, count int, values []int64, offset int) error {
	k := 0
	for ; k < count-storedFieldsIntsBlockSizeMinusOne; k += storedFieldsIntsBlockSize {
		step := offset + k
		// Reference reads 16 longs in bulk via in.readLongs(values, step, 16).
		// We do not have a bulk ReadLongs primitive on IndexInput, so we
		// read longs one at a time into the destination slots, then expand
		// each long into eight values across the 16 lanes.
		for i := 0; i < 16; i++ {
			l, err := in.ReadLong()
			if err != nil {
				return fmt.Errorf("stored fields ints 8bpv: read long: %w", err)
			}
			values[step+i] = l
		}
		for i := 0; i < 16; i++ {
			l := uint64(values[step+i])
			values[step+i] = int64((l >> 56) & 0xff)
			values[step+16+i] = int64((l >> 48) & 0xff)
			values[step+32+i] = int64((l >> 40) & 0xff)
			values[step+48+i] = int64((l >> 32) & 0xff)
			values[step+64+i] = int64((l >> 24) & 0xff)
			values[step+80+i] = int64((l >> 16) & 0xff)
			values[step+96+i] = int64((l >> 8) & 0xff)
			values[step+112+i] = int64(l & 0xff)
		}
	}
	for ; k < count; k++ {
		b, err := in.ReadByte()
		if err != nil {
			return fmt.Errorf("stored fields ints 8bpv: read tail byte: %w", err)
		}
		values[offset+k] = int64(b)
	}
	return nil
}

// readStoredFieldsInts16 inverts the 4-per-long layout for full blocks then
// reads single shorts for the tail (unsigned-extended into int64).
func readStoredFieldsInts16(in store.DataInput, count int, values []int64, offset int) error {
	k := 0
	for ; k < count-storedFieldsIntsBlockSizeMinusOne; k += storedFieldsIntsBlockSize {
		step := offset + k
		for i := 0; i < 32; i++ {
			l, err := in.ReadLong()
			if err != nil {
				return fmt.Errorf("stored fields ints 16bpv: read long: %w", err)
			}
			values[step+i] = l
		}
		for i := 0; i < 32; i++ {
			l := uint64(values[step+i])
			values[step+i] = int64((l >> 48) & 0xffff)
			values[step+32+i] = int64((l >> 32) & 0xffff)
			values[step+64+i] = int64((l >> 16) & 0xffff)
			values[step+96+i] = int64(l & 0xffff)
		}
	}
	for ; k < count; k++ {
		s, err := in.ReadShort()
		if err != nil {
			return fmt.Errorf("stored fields ints 16bpv: read tail short: %w", err)
		}
		values[offset+k] = int64(uint16(s))
	}
	return nil
}

// readStoredFieldsInts32 inverts the 2-per-long layout for full blocks then
// reads single ints for the tail (unsigned-extended into int64).
func readStoredFieldsInts32(in store.DataInput, count int, values []int64, offset int) error {
	k := 0
	for ; k < count-storedFieldsIntsBlockSizeMinusOne; k += storedFieldsIntsBlockSize {
		step := offset + k
		for i := 0; i < 64; i++ {
			l, err := in.ReadLong()
			if err != nil {
				return fmt.Errorf("stored fields ints 32bpv: read long: %w", err)
			}
			values[step+i] = l
		}
		for i := 0; i < 64; i++ {
			l := uint64(values[step+i])
			values[step+i] = int64(l >> 32)
			values[step+64+i] = int64(l & 0xffffffff)
		}
	}
	for ; k < count; k++ {
		v, err := in.ReadInt()
		if err != nil {
			return fmt.Errorf("stored fields ints 32bpv: read tail int: %w", err)
		}
		values[offset+k] = int64(uint32(v))
	}
	return nil
}
