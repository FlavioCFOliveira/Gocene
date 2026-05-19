// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"errors"
	"fmt"
	"math/bits"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// GroupVIntUtil ports org.apache.lucene.util.GroupVIntUtil from Lucene 10.4.0
// (lucene/core/src/java/org/apache/lucene/util/GroupVIntUtil.java).
//
// Group-varint packs four 32-bit unsigned integers into a "group". The group
// begins with a single control byte that stores the byte-length minus one of
// each of the four packed integers in 2-bit fields (so each length is in
// [1, 4]). The four little-endian payloads follow concatenated. Counts that
// are not multiples of four are encoded by emitting complete groups via the
// group format and falling back to a regular VInt for the trailing
// 1, 2 or 3 values.
//
// The control byte layout matches the upstream encoding (bits 0-1 hold the
// length of value[3] - 1, bits 2-3 hold length of value[2] - 1, and so on).

// GroupVIntMaxLengthPerGroup is the maximum number of bytes a single
// group-varint can occupy: 1 control byte plus 4 * 4-byte payloads.
//
// Mirrors GroupVIntUtil.MAX_LENGTH_PER_GROUP.
const GroupVIntMaxLengthPerGroup = 1 + 4*4

// intMasks is the table of masks indexed by numBytesMinus1 used by the
// branch-less random-access fast path. Mirrors GroupVIntUtil.INT_MASKS.
var intMasks = [4]uint32{0xFF, 0xFFFF, 0xFFFFFF, 0xFFFFFFFF}

// ErrGroupVIntOverflow is returned by ToInt32 when a uint32 view of the
// supplied uint64 value would lose information (i.e. value > 0xFFFFFFFF).
// Mirrors Lucene's ArithmeticException("integer overflow").
var ErrGroupVIntOverflow = errors.New("group vint: integer overflow")

// ReadGroupVInts reads exactly limit 32-bit values from in into dst using the
// group-varint format, including the tail values that do not form a complete
// group of four (those are encoded as regular VInts).
//
// Ports GroupVIntUtil.readGroupVInts(DataInput, int[], int).
func ReadGroupVInts(in store.DataInput, dst []int32, limit int) error {
	if limit < 0 {
		return fmt.Errorf("group vint: negative limit %d", limit)
	}
	if limit > len(dst) {
		return fmt.Errorf("group vint: dst length %d shorter than limit %d", len(dst), limit)
	}
	i := 0
	for ; i <= limit-4; i += 4 {
		if err := readGroupVInt32(in, dst, i); err != nil {
			return err
		}
	}
	for ; i < limit; i++ {
		v, err := readVInt(in)
		if err != nil {
			return err
		}
		dst[i] = v
	}
	return nil
}

// ReadGroupVInt reads a single group of four 32-bit values starting at the
// given offset in dst. For optimal performance prefer ReadGroupVInts, which
// decodes the trailing partial group via the regular VInt format.
//
// Ports the deprecated GroupVIntUtil.readGroupVInt(DataInput, int[], int).
//
// Deprecated: use ReadGroupVInts to decode a full group including tails.
func ReadGroupVInt(in store.DataInput, dst []int32, offset int) error {
	return readGroupVInt32(in, dst, offset)
}

// readGroupVInt32 is the shared implementation that decodes a single group
// of four 32-bit values. It mirrors the Java implementation: if the input
// implements both RandomAccessInput and IndexInput and there are at least
// 16 bytes left, the four payloads are decoded branch-lessly via absolute
// reads; otherwise the slow path is used.
func readGroupVInt32(in store.DataInput, dst []int32, offset int) error {
	if offset+4 > len(dst) {
		return fmt.Errorf("group vint: dst overflow at offset %d (len=%d)", offset, len(dst))
	}
	flag, err := in.ReadByte()
	if err != nil {
		return err
	}
	n1m1 := int(flag >> 6)
	n2m1 := int((flag >> 4) & 0x03)
	n3m1 := int((flag >> 2) & 0x03)
	n4m1 := int(flag & 0x03)

	// Fast path: random access + seekable input with enough trailing bytes.
	if iin, ok := in.(store.IndexInput); ok {
		if rin, ok2 := in.(store.RandomAccessInput); ok2 {
			pos := iin.GetFilePointer()
			if rin.Length()-pos >= 4*4 {
				v1, err := rin.ReadIntAt(pos)
				if err != nil {
					return err
				}
				dst[offset] = int32(uint32(v1) & intMasks[n1m1])
				pos += int64(1 + n1m1)

				v2, err := rin.ReadIntAt(pos)
				if err != nil {
					return err
				}
				dst[offset+1] = int32(uint32(v2) & intMasks[n2m1])
				pos += int64(1 + n2m1)

				v3, err := rin.ReadIntAt(pos)
				if err != nil {
					return err
				}
				dst[offset+2] = int32(uint32(v3) & intMasks[n3m1])
				pos += int64(1 + n3m1)

				v4, err := rin.ReadIntAt(pos)
				if err != nil {
					return err
				}
				dst[offset+3] = int32(uint32(v4) & intMasks[n4m1])
				pos += int64(1 + n4m1)

				return iin.SetPosition(pos)
			}
		}
	}

	// Slow path: sequential per-integer reads.
	v1, err := readIntInGroup(in, n1m1)
	if err != nil {
		return err
	}
	dst[offset] = int32(v1)
	v2, err := readIntInGroup(in, n2m1)
	if err != nil {
		return err
	}
	dst[offset+1] = int32(v2)
	v3, err := readIntInGroup(in, n3m1)
	if err != nil {
		return err
	}
	dst[offset+2] = int32(v3)
	v4, err := readIntInGroup(in, n4m1)
	if err != nil {
		return err
	}
	dst[offset+3] = int32(v4)
	return nil
}

// ReadGroupVIntsBaseline mirrors the GroupVIntUtil.readGroupVInts$Baseline
// method that exists solely for benchmarking the random-access fast path
// against the per-byte slow path. Production code should call ReadGroupVInts.
func ReadGroupVIntsBaseline(in store.DataInput, dst []int32, limit int) error {
	if limit < 0 {
		return fmt.Errorf("group vint: negative limit %d", limit)
	}
	if limit > len(dst) {
		return fmt.Errorf("group vint: dst length %d shorter than limit %d", len(dst), limit)
	}
	i := 0
	for ; i <= limit-4; i += 4 {
		if err := readGroupVInt32Baseline(in, dst, i); err != nil {
			return err
		}
	}
	for ; i < limit; i++ {
		v, err := readVInt(in)
		if err != nil {
			return err
		}
		dst[i] = v
	}
	return nil
}

// readGroupVInt32Baseline is the slow-path-only decode used by the baseline
// benchmark variant. It deliberately avoids the RandomAccessInput fast path.
func readGroupVInt32Baseline(in store.DataInput, dst []int32, offset int) error {
	if offset+4 > len(dst) {
		return fmt.Errorf("group vint: dst overflow at offset %d (len=%d)", offset, len(dst))
	}
	flag, err := in.ReadByte()
	if err != nil {
		return err
	}
	n1m1 := int(flag >> 6)
	n2m1 := int((flag >> 4) & 0x03)
	n3m1 := int((flag >> 2) & 0x03)
	n4m1 := int(flag & 0x03)

	v1, err := readIntInGroup(in, n1m1)
	if err != nil {
		return err
	}
	dst[offset] = int32(v1)
	v2, err := readIntInGroup(in, n2m1)
	if err != nil {
		return err
	}
	dst[offset+1] = int32(v2)
	v3, err := readIntInGroup(in, n3m1)
	if err != nil {
		return err
	}
	dst[offset+2] = int32(v3)
	v4, err := readIntInGroup(in, n4m1)
	if err != nil {
		return err
	}
	dst[offset+3] = int32(v4)
	return nil
}

// readIntInGroup decodes a single payload of (numBytesMinus1 + 1) bytes
// from in, returned as a zero-extended uint32. Byte order is little-endian
// to match the upstream on-wire format (VH_LE_INT on write).
//
// Note on endianness: the Java version uses DataInput.readShort()/readInt()
// which are big-endian on the abstract class, but the BBDI overrides used
// in practice read little-endian. To remain wire-format-correct regardless
// of the concrete DataInput implementation, this Go port assembles the
// payload from raw bytes rather than calling ReadShort/ReadInt (which in
// Gocene return little-endian, but reading raw bytes avoids any dependency
// on per-implementation endianness choices).
func readIntInGroup(in store.DataInput, numBytesMinus1 int) (uint32, error) {
	n := numBytesMinus1 + 1
	var v uint32
	for b := 0; b < n; b++ {
		x, err := in.ReadByte()
		if err != nil {
			return 0, err
		}
		v |= uint32(x) << uint(8*b)
	}
	return v, nil
}

// IntReader is a deprecated functional alias used by deprecated overloads
// of GroupVIntUtil. It is included for API completeness but is not used
// anywhere in Gocene.
//
// Deprecated: no longer used.
type IntReader func(pos int64) int32

// ErrReadGroupVIntCustom is returned by ReadGroupVIntCustom to mirror the
// UnsupportedOperationException thrown by the deprecated overload in the
// Java reference. The Java method exists only so legacy subclasses can be
// detected and removed; Gocene exposes it for symmetry.
var ErrReadGroupVIntCustom = errors.New("group vint: no longer implemented")

// ReadGroupVIntCustom mirrors the deprecated
// GroupVIntUtil.readGroupVInt(DataInput, long, IntReader, long, int[], int)
// overload that throws UnsupportedOperationException unconditionally.
//
// Deprecated: never call this; it always returns ErrReadGroupVIntCustom.
// Provided only so callers detect that the custom RandomAccess override is
// no longer supported and can be removed from the caller.
func ReadGroupVIntCustom(
	_ store.DataInput,
	_ int64,
	_ IntReader,
	_ int64,
	_ []int32,
	_ int,
) (int, error) {
	return 0, ErrReadGroupVIntCustom
}

// numBytes returns the number of bytes required to represent v in 1..4 bytes
// using the group-varint length convention (a zero value still occupies one
// byte, matching the Java implementation that or-folds 1 into v).
func numBytes(v uint32) int {
	// bits.LeadingZeros32(0) == 32, so v|1 makes 0 collapse to 1 (a single byte).
	return 4 - (bits.LeadingZeros32(v|1) >> 3)
}

// ToInt32 narrows v to int32 (uint32 width) or returns ErrGroupVIntOverflow
// if the high 32 bits are set. Mirrors GroupVIntUtil.toInt(long).
func ToInt32(v uint64) (int32, error) {
	if v > 0xFFFFFFFF {
		return 0, ErrGroupVIntOverflow
	}
	return int32(uint32(v)), nil
}

// WriteGroupVInts encodes the first limit values from values into out using
// the group-varint format, with the trailing 1, 2 or 3 values that do not
// form a full group emitted as regular VInts. The scratch slice must hold at
// least GroupVIntMaxLengthPerGroup bytes; it is overwritten on each group.
//
// Ports GroupVIntUtil.writeGroupVInts(DataOutput, byte[], int[], int).
func WriteGroupVInts(out store.DataOutput, scratch []byte, values []int32, limit int) error {
	if limit < 0 {
		return fmt.Errorf("group vint: negative limit %d", limit)
	}
	if limit > len(values) {
		return fmt.Errorf("group vint: values length %d shorter than limit %d", len(values), limit)
	}
	if len(scratch) < GroupVIntMaxLengthPerGroup {
		return fmt.Errorf("group vint: scratch length %d below minimum %d", len(scratch), GroupVIntMaxLengthPerGroup)
	}
	readPos := 0
	for limit-readPos >= 4 {
		writePos := 0
		v1 := uint32(values[readPos])
		v2 := uint32(values[readPos+1])
		v3 := uint32(values[readPos+2])
		v4 := uint32(values[readPos+3])
		n1m1 := numBytes(v1) - 1
		n2m1 := numBytes(v2) - 1
		n3m1 := numBytes(v3) - 1
		n4m1 := numBytes(v4) - 1
		flag := byte((n1m1 << 6) | (n2m1 << 4) | (n3m1 << 2) | n4m1)
		scratch[writePos] = flag
		writePos++
		writePos += writeLE32(scratch[writePos:], v1, n1m1+1)
		writePos += writeLE32(scratch[writePos:], v2, n2m1+1)
		writePos += writeLE32(scratch[writePos:], v3, n3m1+1)
		writePos += writeLE32(scratch[writePos:], v4, n4m1+1)
		if err := out.WriteBytesN(scratch[:writePos], writePos); err != nil {
			return err
		}
		readPos += 4
	}
	// Tail vints.
	for ; readPos < limit; readPos++ {
		if err := writeVInt(out, values[readPos]); err != nil {
			return err
		}
	}
	return nil
}

// ReadGroupVIntsInt64 reads exactly limit 32-bit values from in into dst as
// zero-extended int64, including the tail values encoded as regular VInts.
//
// Ports the deprecated GroupVIntUtil.readGroupVInts(DataInput, long[], int).
//
// Deprecated: only kept for backwards-compatible codecs.
func ReadGroupVIntsInt64(in store.DataInput, dst []int64, limit int) error {
	if limit < 0 {
		return fmt.Errorf("group vint: negative limit %d", limit)
	}
	if limit > len(dst) {
		return fmt.Errorf("group vint: dst length %d shorter than limit %d", len(dst), limit)
	}
	i := 0
	for ; i <= limit-4; i += 4 {
		if err := readGroupVInt64(in, dst, i); err != nil {
			return err
		}
	}
	for ; i < limit; i++ {
		v, err := readVInt(in)
		if err != nil {
			return err
		}
		dst[i] = int64(uint32(v))
	}
	return nil
}

// ReadGroupVIntInt64 decodes a single group of four 32-bit payloads as
// zero-extended int64 values into dst[offset:offset+4].
//
// Ports the deprecated GroupVIntUtil.readGroupVInt(DataInput, long[], int).
//
// Deprecated: only kept for backwards-compatible codecs.
func ReadGroupVIntInt64(in store.DataInput, dst []int64, offset int) error {
	return readGroupVInt64(in, dst, offset)
}

func readGroupVInt64(in store.DataInput, dst []int64, offset int) error {
	if offset+4 > len(dst) {
		return fmt.Errorf("group vint: dst overflow at offset %d (len=%d)", offset, len(dst))
	}
	flag, err := in.ReadByte()
	if err != nil {
		return err
	}
	n1m1 := int(flag >> 6)
	n2m1 := int((flag >> 4) & 0x03)
	n3m1 := int((flag >> 2) & 0x03)
	n4m1 := int(flag & 0x03)

	v1, err := readIntInGroup(in, n1m1)
	if err != nil {
		return err
	}
	dst[offset] = int64(v1)
	v2, err := readIntInGroup(in, n2m1)
	if err != nil {
		return err
	}
	dst[offset+1] = int64(v2)
	v3, err := readIntInGroup(in, n3m1)
	if err != nil {
		return err
	}
	dst[offset+2] = int64(v3)
	v4, err := readIntInGroup(in, n4m1)
	if err != nil {
		return err
	}
	dst[offset+3] = int64(v4)
	return nil
}

// WriteGroupVIntsInt64 encodes the first limit int64 values from values
// using the group-varint format. Each value must fit in a uint32; values
// outside that range produce ErrGroupVIntOverflow.
//
// Ports the deprecated GroupVIntUtil.writeGroupVInts(DataOutput, byte[], long[], int).
//
// Deprecated: only kept for backwards-compatible codecs.
func WriteGroupVIntsInt64(out store.DataOutput, scratch []byte, values []int64, limit int) error {
	if limit < 0 {
		return fmt.Errorf("group vint: negative limit %d", limit)
	}
	if limit > len(values) {
		return fmt.Errorf("group vint: values length %d shorter than limit %d", len(values), limit)
	}
	if len(scratch) < GroupVIntMaxLengthPerGroup {
		return fmt.Errorf("group vint: scratch length %d below minimum %d", len(scratch), GroupVIntMaxLengthPerGroup)
	}
	readPos := 0
	for limit-readPos >= 4 {
		v1, err := ToInt32(uint64(values[readPos]))
		if err != nil {
			return err
		}
		v2, err := ToInt32(uint64(values[readPos+1]))
		if err != nil {
			return err
		}
		v3, err := ToInt32(uint64(values[readPos+2]))
		if err != nil {
			return err
		}
		v4, err := ToInt32(uint64(values[readPos+3]))
		if err != nil {
			return err
		}
		u1 := uint32(v1)
		u2 := uint32(v2)
		u3 := uint32(v3)
		u4 := uint32(v4)
		n1m1 := numBytes(u1) - 1
		n2m1 := numBytes(u2) - 1
		n3m1 := numBytes(u3) - 1
		n4m1 := numBytes(u4) - 1
		flag := byte((n1m1 << 6) | (n2m1 << 4) | (n3m1 << 2) | n4m1)
		writePos := 0
		scratch[writePos] = flag
		writePos++
		writePos += writeLE32(scratch[writePos:], u1, n1m1+1)
		writePos += writeLE32(scratch[writePos:], u2, n2m1+1)
		writePos += writeLE32(scratch[writePos:], u3, n3m1+1)
		writePos += writeLE32(scratch[writePos:], u4, n4m1+1)
		if err := out.WriteBytesN(scratch[:writePos], writePos); err != nil {
			return err
		}
		readPos += 4
	}
	for ; readPos < limit; readPos++ {
		v, err := ToInt32(uint64(values[readPos]))
		if err != nil {
			return err
		}
		if err := writeVInt(out, v); err != nil {
			return err
		}
	}
	return nil
}

// writeLE32 writes the low n bytes of v (little-endian) into dst starting at
// index 0 and returns n. n must be in [1, 4]; the caller is responsible for
// ensuring dst has at least 4 bytes of capacity so that the optimised path
// can run without bounds checks even for n == 4. Mirrors VH_LE_INT.set
// followed by writePos += n.
func writeLE32(dst []byte, v uint32, n int) int {
	// Always write 4 bytes into dst (group payload area always has room),
	// then advance writePos by only n. This matches the Java behavior of
	// VH_LE_INT.set writing 4 bytes regardless of payload length.
	_ = dst[3] // bounds check elimination hint
	dst[0] = byte(v)
	dst[1] = byte(v >> 8)
	dst[2] = byte(v >> 16)
	dst[3] = byte(v >> 24)
	return n
}

// readVInt reads a Lucene VInt directly from a DataInput. We do not use the
// VariableLengthInput interface here because GroupVIntUtil's callers may
// pass DataInput implementations that do not also implement
// VariableLengthInput (the Java DataInput abstract class declares
// readVInt() itself).
func readVInt(in store.DataInput) (int32, error) {
	b, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	v := int32(b) & 0x7F
	for shift := 7; b&0x80 != 0; shift += 7 {
		if shift >= 32 {
			return 0, errors.New("group vint: corrupted VInt")
		}
		b, err = in.ReadByte()
		if err != nil {
			return 0, err
		}
		v |= int32(b&0x7F) << shift
	}
	return v, nil
}

// writeVInt writes a Lucene VInt directly to a DataOutput, for the same
// reasons that readVInt bypasses VariableLengthOutput.
func writeVInt(out store.DataOutput, i int32) error {
	for i&^0x7F != 0 {
		if err := out.WriteByte(byte((i & 0x7F) | 0x80)); err != nil {
			return err
		}
		i = int32(uint32(i) >> 7)
	}
	return out.WriteByte(byte(i))
}
