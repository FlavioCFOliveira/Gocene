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
//	http://www.apache.org/licenses/LICENSE-2.0

package fst

import "math/bits"

// bitTableUtil ports the package-private static helpers from
// org.apache.lucene.util.fst.BitTableUtil. The bit-table is the
// presence array that prefixes the arcs of a direct-addressing node;
// every helper assumes the reader is positioned at the start of that
// table. Lucene's reverse readers expose bytes in the "natural"
// reading order produced by the forward writer, so byte indexing
// inside the table proceeds with positive skips.

// bitTableIsBitSet returns whether the bit at the given zero-based
// index is set. Mirrors BitTableUtil.isBitSet.
func bitTableIsBitSet(bitIndex int, reader BytesReader) (bool, error) {
	if err := reader.SkipBytes(int64(bitIndex >> 3)); err != nil {
		return false, err
	}
	b, err := reader.ReadByte()
	if err != nil {
		return false, err
	}
	return (uint64(b) & (uint64(1) << uint(bitIndex&7))) != 0, nil
}

// bitTableCountBits returns the number of bits set in the entire
// bit-table whose byte length is bitTableBytes. Mirrors
// BitTableUtil.countBits.
func bitTableCountBits(bitTableBytes int, reader BytesReader) (int, error) {
	bitCount := 0
	for i := bitTableBytes >> 3; i > 0; i-- {
		v, err := reader.ReadLong()
		if err != nil {
			return 0, err
		}
		bitCount += bits.OnesCount64(uint64(v))
	}
	if numRemainingBytes := bitTableBytes & 7; numRemainingBytes != 0 {
		v, err := readUpTo8Bytes(numRemainingBytes, reader)
		if err != nil {
			return 0, err
		}
		bitCount += bits.OnesCount64(v)
	}
	return bitCount, nil
}

// bitTableCountBitsUpTo returns the count of bits set strictly before
// the given zero-based index. Mirrors BitTableUtil.countBitsUpTo.
func bitTableCountBitsUpTo(bitIndex int, reader BytesReader) (int, error) {
	bitCount := 0
	for i := bitIndex >> 6; i > 0; i-- {
		v, err := reader.ReadLong()
		if err != nil {
			return 0, err
		}
		bitCount += bits.OnesCount64(uint64(v))
	}
	if remainingBits := bitIndex & 63; remainingBits != 0 {
		numRemainingBytes := (remainingBits + 7) >> 3
		// Mask with 1s on the right up to bitIndex exclusive.
		// Shifts are mod 64 in both Java and Go, matching the original
		// "(1L << bitIndex) - 1L" expression. For Go, the shift count
		// must be reduced modulo 64 explicitly because Go panics on
		// shift counts beyond the type width when they are untyped
		// constants; using uint(...) keeps the runtime mod-64
		// behaviour consistent with the JVM's "<<" on long.
		mask := (uint64(1) << uint(bitIndex&63)) - 1
		v, err := readUpTo8Bytes(numRemainingBytes, reader)
		if err != nil {
			return 0, err
		}
		bitCount += bits.OnesCount64(v & mask)
	}
	return bitCount, nil
}

// bitTableNextBitSet returns the zero-based index of the next bit set
// strictly after bitIndex, or -1 if there is none. bitIndex must be
// in [-1, bitTableBytes*8). Mirrors BitTableUtil.nextBitSet.
func bitTableNextBitSet(bitIndex, bitTableBytes int, reader BytesReader) (int, error) {
	// Java truncates toward zero, so -1/8 == 0 (matched by Go).
	byteIndex := bitIndex / 8
	// mask = -1 << ((bitIndex+1) & 7)  in 32-bit two's complement
	mask := int32(-1) << uint((bitIndex+1)&7)
	var i int
	if mask == -1 && bitIndex != -1 {
		if err := reader.SkipBytes(int64(byteIndex + 1)); err != nil {
			return 0, err
		}
		i = 0
	} else {
		if err := reader.SkipBytes(int64(byteIndex)); err != nil {
			return 0, err
		}
		b, err := reader.ReadByte()
		if err != nil {
			return 0, err
		}
		// Match Java: (reader.readByte() & 0xFF) & mask
		i = (int(b) & 0xFF) & int(mask)
	}
	for i == 0 {
		byteIndex++
		if byteIndex == bitTableBytes {
			return -1, nil
		}
		b, err := reader.ReadByte()
		if err != nil {
			return 0, err
		}
		i = int(b) & 0xFF
	}
	return bits.TrailingZeros32(uint32(i)) + (byteIndex << 3), nil
}

// bitTablePreviousBitSet returns the zero-based index of the previous
// bit set strictly before bitIndex, or -1 if there is none.
// Mirrors BitTableUtil.previousBitSet, including the reliance on
// negative skipBytes to walk the table backward.
func bitTablePreviousBitSet(bitIndex int, reader BytesReader) (int, error) {
	byteIndex := bitIndex >> 3
	if err := reader.SkipBytes(int64(byteIndex)); err != nil {
		return 0, err
	}
	mask := (1 << uint(bitIndex&7)) - 1
	b, err := reader.ReadByte()
	if err != nil {
		return 0, err
	}
	i := int(b) & 0xFF & mask
	for i == 0 {
		if byteIndex == 0 {
			return -1, nil
		}
		byteIndex--
		// Same trick as Java: skip -2 so the next ReadByte returns the
		// previous byte. This relies on BytesReader implementations
		// supporting negative skips (reverse readers do).
		if err := reader.SkipBytes(-2); err != nil {
			return 0, err
		}
		b, err = reader.ReadByte()
		if err != nil {
			return 0, err
		}
		i = int(b) & 0xFF
	}
	// (Integer.SIZE - 1) - Integer.numberOfLeadingZeros(i) + (byteIndex << 3)
	return 31 - bits.LeadingZeros32(uint32(i)) + (byteIndex << 3), nil
}

// readUpTo8Bytes reads numBytes bytes (1..8) and assembles them as a
// little-endian uint64 — the high (8-numBytes) bytes are zero. Mirrors
// BitTableUtil.readUpTo8Bytes.
func readUpTo8Bytes(numBytes int, reader BytesReader) (uint64, error) {
	b, err := reader.ReadByte()
	if err != nil {
		return 0, err
	}
	l := uint64(b) & 0xFF
	shift := 0
	for numBytes--; numBytes != 0; numBytes-- {
		b, err = reader.ReadByte()
		if err != nil {
			return 0, err
		}
		shift += 8
		l |= (uint64(b) & 0xFF) << uint(shift)
	}
	return l, nil
}
