// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.5.0:
//
//	Licensed to the Apache Software Foundation (ASF) under one or more
//	contributor license agreements. See the NOTICE file distributed with
//	this work for additional information regarding copyright ownership.
//	The ASF licenses this file to You under the Apache License, Version
//	2.0 (the "License"); you may not use this file except in compliance
//	with the License. You may obtain a copy of the License at
//
//	    http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS,
//	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
//	implied. See the License for the specific language governing
//	permissions and limitations under the License.

package compressing

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// This file is the Go port of
// org.apache.lucene.codecs.lucene90.compressing.StoredFieldsInts
// (Lucene 10.5.0, StoredFieldsInts.java, 196 lines).
//
// The Java artefact is a package-private final class with a private
// constructor and nothing but static methods, so it has no Go struct: its
// two package-private entry points become the package-level functions
// storedFieldsIntsWriteInts and storedFieldsIntsReadInts. The prefix stands
// in for the Java class name, which is what disambiguates writeInts/readInts
// from the same-named private helpers of Lucene90CompressingStoredFieldsWriter
// (saveInts) and Lucene90CompressingStoredFieldsReader.

const (
	// storedFieldsIntsBlockSize is StoredFieldsInts.BLOCK_SIZE
	// (StoredFieldsInts.java:26).
	storedFieldsIntsBlockSize = 128

	// storedFieldsIntsBlockSizeMinusOne is
	// StoredFieldsInts.BLOCK_SIZE_MINUS_ONE (StoredFieldsInts.java:27).
	storedFieldsIntsBlockSizeMinusOne = storedFieldsIntsBlockSize - 1
)

// storedFieldsIntsWriteInts is StoredFieldsInts.writeInts(int[], int, int,
// DataOutput) (StoredFieldsInts.java:31-57).
//
// A single leading byte selects the encoding: 0 when every value in the
// window is equal (followed by a VInt holding values[0] — note that Java
// writes values[0], not values[start], and that quirk is part of the wire
// contract), 8, 16 or 32 for the transposed fixed-width block layouts.
func storedFieldsIntsWriteInts(values []int32, start, count int, out store.DataOutput) error {
	allEqual := true
	for i := 1; i < count; i++ {
		if values[start+i] != values[start] {
			allEqual = false
			break
		}
	}
	if allEqual {
		if err := out.WriteByte(0); err != nil {
			return fmt.Errorf("lucene90/compressing: StoredFieldsInts write bpv 0: %w", err)
		}
		// Java writes values[0] here, not values[start].
		if err := out.WriteVInt(values[0]); err != nil {
			return fmt.Errorf("lucene90/compressing: StoredFieldsInts write all-equal value: %w", err)
		}
		return nil
	}

	var max uint64
	for i := 0; i < count; i++ {
		// Integer.toUnsignedLong(values[start + i])
		max |= uint64(uint32(values[start+i]))
	}
	switch {
	case max <= 0xff:
		if err := out.WriteByte(8); err != nil {
			return fmt.Errorf("lucene90/compressing: StoredFieldsInts write bpv 8: %w", err)
		}
		return storedFieldsIntsWriteInts8(out, count, values, start)
	case max <= 0xffff:
		if err := out.WriteByte(16); err != nil {
			return fmt.Errorf("lucene90/compressing: StoredFieldsInts write bpv 16: %w", err)
		}
		return storedFieldsIntsWriteInts16(out, count, values, start)
	default:
		if err := out.WriteByte(32); err != nil {
			return fmt.Errorf("lucene90/compressing: StoredFieldsInts write bpv 32: %w", err)
		}
		return storedFieldsIntsWriteInts32(out, count, values, start)
	}
}

// storedFieldsIntsWriteInts8 is StoredFieldsInts.writeInts8
// (StoredFieldsInts.java:59-76).
func storedFieldsIntsWriteInts8(out store.DataOutput, count int, values []int32, offset int) error {
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
				return fmt.Errorf("lucene90/compressing: StoredFieldsInts writeInts8 long: %w", err)
			}
		}
	}
	for ; k < count; k++ {
		if err := out.WriteByte(byte(values[offset+k])); err != nil {
			return fmt.Errorf("lucene90/compressing: StoredFieldsInts writeInts8 tail: %w", err)
		}
	}
	return nil
}

// storedFieldsIntsWriteInts16 is StoredFieldsInts.writeInts16
// (StoredFieldsInts.java:78-95).
func storedFieldsIntsWriteInts16(out store.DataOutput, count int, values []int32, offset int) error {
	k := 0
	for ; k < count-storedFieldsIntsBlockSizeMinusOne; k += storedFieldsIntsBlockSize {
		step := offset + k
		for i := 0; i < 32; i++ {
			l := uint64(uint32(values[step+i]))<<48 |
				uint64(uint32(values[step+32+i]))<<32 |
				uint64(uint32(values[step+64+i]))<<16 |
				uint64(uint32(values[step+96+i]))
			if err := out.WriteLong(int64(l)); err != nil {
				return fmt.Errorf("lucene90/compressing: StoredFieldsInts writeInts16 long: %w", err)
			}
		}
	}
	for ; k < count; k++ {
		if err := out.WriteShort(int16(values[offset+k])); err != nil {
			return fmt.Errorf("lucene90/compressing: StoredFieldsInts writeInts16 tail: %w", err)
		}
	}
	return nil
}

// storedFieldsIntsWriteInts32 is StoredFieldsInts.writeInts32
// (StoredFieldsInts.java:97-112):
//
//	long l = ((long) values[step + i] << 32) | (long) values[step + 64 + i];
//
// Both operands are Java int values widened by a SIGN-extending (long) cast.
// The widening of the high operand is immaterial once it is shifted left by
// 32, but the low operand is OR'd in unshifted, so a negative value there
// sets every one of the 64 bits. Masking it to 32 bits would set only the
// low half and emit different bytes; the port therefore widens through int64,
// exactly as Java does.
func storedFieldsIntsWriteInts32(out store.DataOutput, count int, values []int32, offset int) error {
	k := 0
	for ; k < count-storedFieldsIntsBlockSizeMinusOne; k += storedFieldsIntsBlockSize {
		step := offset + k
		for i := 0; i < 64; i++ {
			l := (int64(values[step+i]) << 32) | int64(values[step+64+i])
			if err := out.WriteLong(l); err != nil {
				return fmt.Errorf("lucene90/compressing: StoredFieldsInts writeInts32 long: %w", err)
			}
		}
	}
	for ; k < count; k++ {
		if err := out.WriteInt(values[offset+k]); err != nil {
			return fmt.Errorf("lucene90/compressing: StoredFieldsInts writeInts32 tail: %w", err)
		}
	}
	return nil
}

// storedFieldsIntsReadInts is StoredFieldsInts.readInts(IndexInput, int,
// long[], int) (StoredFieldsInts.java:114-131): "Read count integers into
// values".
func storedFieldsIntsReadInts(in store.DataInput, count int, values []int64, offset int) error {
	bpv, err := in.ReadByte()
	if err != nil {
		return fmt.Errorf("lucene90/compressing: StoredFieldsInts read bpv: %w", err)
	}
	switch bpv {
	case 0:
		v, err := store.ReadVInt(in)
		if err != nil {
			return fmt.Errorf("lucene90/compressing: StoredFieldsInts read all-equal value: %w", err)
		}
		// Arrays.fill(values, offset, offset + count, in.readVInt())
		for i := offset; i < offset+count; i++ {
			values[i] = int64(v)
		}
		return nil
	case 8:
		return storedFieldsIntsReadInts8(in, count, values, offset)
	case 16:
		return storedFieldsIntsReadInts16(in, count, values, offset)
	case 32:
		return storedFieldsIntsReadInts32(in, count, values, offset)
	default:
		// Java: throw new IOException("Unsupported number of bits per value: " + bpv)
		// where bpv came from IndexInput.readByte(), i.e. a signed Java byte.
		return fmt.Errorf("lucene90/compressing: unsupported number of bits per value: %d", int8(bpv))
	}
}

// storedFieldsIntsReadInts8 is StoredFieldsInts.readInts8
// (StoredFieldsInts.java:133-154).
func storedFieldsIntsReadInts8(in store.DataInput, count int, values []int64, offset int) error {
	k := 0
	for ; k < count-storedFieldsIntsBlockSizeMinusOne; k += storedFieldsIntsBlockSize {
		step := offset + k
		if err := in.ReadLongs(values, step, 16); err != nil {
			return fmt.Errorf("lucene90/compressing: StoredFieldsInts readInts8 longs: %w", err)
		}
		for i := 0; i < 16; i++ {
			l := uint64(values[step+i])
			values[step+i] = int64((l >> 56) & 0xFF)
			values[step+16+i] = int64((l >> 48) & 0xFF)
			values[step+32+i] = int64((l >> 40) & 0xFF)
			values[step+48+i] = int64((l >> 32) & 0xFF)
			values[step+64+i] = int64((l >> 24) & 0xFF)
			values[step+80+i] = int64((l >> 16) & 0xFF)
			values[step+96+i] = int64((l >> 8) & 0xFF)
			values[step+112+i] = int64(l & 0xFF)
		}
	}
	for ; k < count; k++ {
		b, err := in.ReadByte()
		if err != nil {
			return fmt.Errorf("lucene90/compressing: StoredFieldsInts readInts8 tail: %w", err)
		}
		// Byte.toUnsignedInt(in.readByte())
		values[offset+k] = int64(b)
	}
	return nil
}

// storedFieldsIntsReadInts16 is StoredFieldsInts.readInts16
// (StoredFieldsInts.java:156-173).
func storedFieldsIntsReadInts16(in store.DataInput, count int, values []int64, offset int) error {
	k := 0
	for ; k < count-storedFieldsIntsBlockSizeMinusOne; k += storedFieldsIntsBlockSize {
		step := offset + k
		if err := in.ReadLongs(values, step, 32); err != nil {
			return fmt.Errorf("lucene90/compressing: StoredFieldsInts readInts16 longs: %w", err)
		}
		for i := 0; i < 32; i++ {
			l := uint64(values[step+i])
			values[step+i] = int64((l >> 48) & 0xFFFF)
			values[step+32+i] = int64((l >> 32) & 0xFFFF)
			values[step+64+i] = int64((l >> 16) & 0xFFFF)
			values[step+96+i] = int64(l & 0xFFFF)
		}
	}
	for ; k < count; k++ {
		s, err := in.ReadShort()
		if err != nil {
			return fmt.Errorf("lucene90/compressing: StoredFieldsInts readInts16 tail: %w", err)
		}
		// Short.toUnsignedInt(in.readShort())
		values[offset+k] = int64(uint16(s))
	}
	return nil
}

// storedFieldsIntsReadInts32 is StoredFieldsInts.readInts32
// (StoredFieldsInts.java:175-194).
func storedFieldsIntsReadInts32(in store.DataInput, count int, values []int64, offset int) error {
	k := 0
	for ; k < count-storedFieldsIntsBlockSizeMinusOne; k += storedFieldsIntsBlockSize {
		step := offset + k
		if err := in.ReadLongs(values, step, 64); err != nil {
			return fmt.Errorf("lucene90/compressing: StoredFieldsInts readInts32 longs: %w", err)
		}
		for i := 0; i < 64; i++ {
			l := uint64(values[step+i])
			values[step+i] = int64(l >> 32)
			values[step+64+i] = int64(l & 0xFFFFFFFF)
		}
	}
	for ; k < count; k++ {
		v, err := in.ReadInt()
		if err != nil {
			return fmt.Errorf("lucene90/compressing: StoredFieldsInts readInts32 tail: %w", err)
		}
		// Java assigns the signed int straight into the long slot.
		values[offset+k] = int64(v)
	}
	return nil
}
