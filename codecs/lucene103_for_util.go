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

// Source: lucene/backward-codecs/src/java/org/apache/lucene/backward_codecs/
//
//	lucene103/{ForUtil,ForDeltaUtil,PForUtil,PostingDecodingUtil}.java
//
// Purpose: 128-wide Frame-of-Reference / Patched-FOR / FOR-delta block
// primitives for the read-only Lucene 10.3 postings format. These are
// deliberately kept separate from the 256-wide ForUtil/PForUtil used by the
// current Lucene 10.4 postings format (see for_util.go / pfor_util.go); the
// two block sizes use different splitInts counts, offsets and prefix-sum
// fan-out, so they cannot share code. The pre-computed mask tables
// (masks8/masks16/masks32) and the mask helpers (expandMask*/mask*) ARE
// block-size independent and are reused from for_util.go.

package codecs

import (
	"encoding/binary"
	"errors"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// lucene103BlockSizeConst is the number of integers per packed block (128).
// Mirrors backward_codecs.lucene103.ForUtil.BLOCK_SIZE.
const lucene103BlockSizeConst = 128

// lucene103BlockSizeLog2 is log2(BLOCK_SIZE) = 7.
// Mirrors backward_codecs.lucene103.ForUtil.BLOCK_SIZE_LOG2.
const lucene103BlockSizeLog2 = 7

// lucene103ForUtil provides 128-wide Frame-of-Reference encode/decode.
// Mirrors backward_codecs.lucene103.ForUtil.
type lucene103ForUtil struct {
	// ints32 is the primary decode output buffer (collapsed values).
	ints32 []int32
	// scratch is the secondary buffer used by recombination loops.
	scratch []int32
}

// newLucene103ForUtil allocates the encode/decode scratch buffers.
func newLucene103ForUtil() *lucene103ForUtil {
	return &lucene103ForUtil{
		ints32:  make([]int32, lucene103BlockSizeConst),
		scratch: make([]int32, lucene103BlockSizeConst),
	}
}

// lucene103ForNumBytes returns the number of bytes required to encode 128
// integers of bitsPerValue bits per value.
// Mirrors ForUtil.numBytes(int).
func lucene103ForNumBytes(bitsPerValue int) int {
	return bitsPerValue << (lucene103BlockSizeLog2 - 3)
}

// ---------- collapse / expand (128-wide) ----------

// l103collapse8 packs four 8-bit values per slot: (128 int32) -> (32 int32).
// Mirrors ForUtil.collapse8.
func l103collapse8(arr []int32) {
	for i := 0; i < 32; i++ {
		arr[i] = (arr[i] << 24) | (arr[32+i] << 16) | (arr[64+i] << 8) | arr[96+i]
	}
}

// l103expand8 unpacks four 8-bit values from each slot: (32 int32) -> (128 int32).
// Mirrors ForUtil.expand8.
func l103expand8(arr []int32) {
	for i := 0; i < 32; i++ {
		l := arr[i]
		arr[i] = int32(uint32(l)>>24) & 0xFF
		arr[32+i] = int32(uint32(l)>>16) & 0xFF
		arr[64+i] = int32(uint32(l)>>8) & 0xFF
		arr[96+i] = l & 0xFF
	}
}

// l103collapse16 packs two 16-bit values per slot: (128 int32) -> (64 int32).
// Mirrors ForUtil.collapse16.
func l103collapse16(arr []int32) {
	for i := 0; i < 64; i++ {
		arr[i] = (arr[i] << 16) | (arr[64+i] & 0xFFFF)
	}
}

// l103expand16 unpacks two 16-bit values from each slot: (64 int32) -> (128 int32).
// Mirrors ForUtil.expand16.
func l103expand16(arr []int32) {
	for i := 0; i < 64; i++ {
		l := arr[i]
		arr[i] = int32(uint32(l)>>16) & 0xFFFF
		arr[64+i] = l & 0xFFFF
	}
}

// ---------- Encode ----------

// encode encodes 128 integers from ints into out using bitsPerValue.
// Mirrors ForUtil.encode(int[], int, DataOutput).
func (f *lucene103ForUtil) encode(ints []int32, bitsPerValue int, out store.IndexOutput) error {
	if len(ints) < lucene103BlockSizeConst {
		return errors.New("lucene103 ForUtil.encode: ints must have at least 128 elements")
	}
	copy(f.ints32, ints[:lucene103BlockSizeConst])

	var nextPrimitive int
	if bitsPerValue <= 8 {
		nextPrimitive = 8
		l103collapse8(f.ints32)
	} else if bitsPerValue <= 16 {
		nextPrimitive = 16
		l103collapse16(f.ints32)
	} else {
		nextPrimitive = 32
	}
	return l103encodeInternal(f.ints32, bitsPerValue, nextPrimitive, out)
}

// l103encodeInternal performs the bit-packing for a 128-wide block and writes
// numIntsPerShift big-endian 32-bit words. Mirrors ForUtil.encode(int[], int,
// int, DataOutput, int[]).
func l103encodeInternal(ints []int32, bitsPerValue, primitiveSize int, out store.IndexOutput) error {
	numInts := lucene103BlockSizeConst * primitiveSize / 32
	numIntsPerShift := bitsPerValue * 4

	var scratch [lucene103BlockSizeConst]int32

	idx := 0
	shift := primitiveSize - bitsPerValue
	for i := 0; i < numIntsPerShift; i++ {
		scratch[i] = ints[idx] << uint(shift)
		idx++
	}
	for shift = shift - bitsPerValue; shift >= 0; shift -= bitsPerValue {
		for i := 0; i < numIntsPerShift; i++ {
			scratch[i] |= ints[idx] << uint(shift)
			idx++
		}
	}

	remainingBitsPerInt := shift + bitsPerValue
	var maskRemainingBitsPerInt int32
	switch primitiveSize {
	case 8:
		maskRemainingBitsPerInt = masks8[remainingBitsPerInt]
	case 16:
		maskRemainingBitsPerInt = masks16[remainingBitsPerInt]
	default:
		maskRemainingBitsPerInt = masks32[remainingBitsPerInt]
	}

	tmpIdx := 0
	remainingBitsPerValue := bitsPerValue
	for idx < numInts {
		if remainingBitsPerValue >= remainingBitsPerInt {
			remainingBitsPerValue -= remainingBitsPerInt
			scratch[tmpIdx] |= int32(uint32(ints[idx])>>uint(remainingBitsPerValue)) & maskRemainingBitsPerInt
			tmpIdx++
			if remainingBitsPerValue == 0 {
				idx++
				remainingBitsPerValue = bitsPerValue
			}
		} else {
			var mask1, mask2 int32
			switch primitiveSize {
			case 8:
				mask1 = masks8[remainingBitsPerValue]
				mask2 = masks8[remainingBitsPerInt-remainingBitsPerValue]
			case 16:
				mask1 = masks16[remainingBitsPerValue]
				mask2 = masks16[remainingBitsPerInt-remainingBitsPerValue]
			default:
				mask1 = masks32[remainingBitsPerValue]
				mask2 = masks32[remainingBitsPerInt-remainingBitsPerValue]
			}
			scratch[tmpIdx] |= (ints[idx] & mask1) << uint(remainingBitsPerInt-remainingBitsPerValue)
			idx++
			remainingBitsPerValue = bitsPerValue - remainingBitsPerInt + remainingBitsPerValue
			scratch[tmpIdx] |= int32(uint32(ints[idx])>>uint(remainingBitsPerValue)) & mask2
			tmpIdx++
		}
	}

	var buf [4]byte
	for i := 0; i < numIntsPerShift; i++ {
		binary.BigEndian.PutUint32(buf[:], uint32(scratch[i]))
		if err := out.WriteBytes(buf[:]); err != nil {
			return err
		}
	}
	return nil
}

// ---------- splitInts (128-wide decode primitive) ----------

// l103splitInts is a direct port of PostingDecodingUtil.splitInts. It reads
// count big-endian 32-bit words into c[cIdx..], applies the descending shift
// levels into b, and finally masks c with cMask.
func l103splitInts(
	in store.IndexInput,
	count int,
	b []int32, bShift, dec int, bMask int32,
	c []int32, cIdx int, cMask int32,
) error {
	var buf [4]byte
	for i := 0; i < count; i++ {
		if err := in.ReadBytes(buf[:]); err != nil {
			return err
		}
		c[cIdx+i] = int32(binary.BigEndian.Uint32(buf[:]))
	}

	maxIter := (bShift - 1) / dec
	for j := 0; j <= maxIter; j++ {
		shift := bShift - j*dec
		bOffset := count * j
		for i := 0; i < count; i++ {
			b[bOffset+i] = int32(uint32(c[cIdx+i])>>uint(shift)) & bMask
		}
	}
	for i := 0; i < count; i++ {
		c[cIdx+i] &= cMask
	}
	return nil
}

// ---------- Decode (128-wide) ----------

// decode decodes 128 integers from in into ints using bitsPerValue.
// Decoded values are zero-extended into int64 (non-negative).
// Mirrors ForUtil.decode(int, PostingDecodingUtil, int[]).
func (f *lucene103ForUtil) decode(bitsPerValue int, in store.IndexInput, ints []int64) error {
	if len(ints) < lucene103BlockSizeConst {
		return errors.New("lucene103 ForUtil.decode: ints must have at least 128 elements")
	}
	buf := f.ints32
	sc := f.scratch

	var err error
	switch bitsPerValue {
	case 1:
		err = l103decode1(in, buf)
		if err == nil {
			l103expand8(buf)
		}
	case 2:
		err = l103decode2(in, buf)
		if err == nil {
			l103expand8(buf)
		}
	case 3:
		err = l103decode3(in, buf, sc)
		if err == nil {
			l103expand8(buf)
		}
	case 4:
		err = l103decode4(in, buf)
		if err == nil {
			l103expand8(buf)
		}
	case 5:
		err = l103decode5(in, buf, sc)
		if err == nil {
			l103expand8(buf)
		}
	case 6:
		err = l103decode6(in, buf, sc)
		if err == nil {
			l103expand8(buf)
		}
	case 7:
		err = l103decode7(in, buf, sc)
		if err == nil {
			l103expand8(buf)
		}
	case 8:
		err = l103decode8(in, buf)
		if err == nil {
			l103expand8(buf)
		}
	case 9:
		err = l103decode9(in, buf, sc)
		if err == nil {
			l103expand16(buf)
		}
	case 10:
		err = l103decode10(in, buf, sc)
		if err == nil {
			l103expand16(buf)
		}
	case 11:
		err = l103decode11(in, buf, sc)
		if err == nil {
			l103expand16(buf)
		}
	case 12:
		err = l103decode12(in, buf, sc)
		if err == nil {
			l103expand16(buf)
		}
	case 13:
		err = l103decode13(in, buf, sc)
		if err == nil {
			l103expand16(buf)
		}
	case 14:
		err = l103decode14(in, buf, sc)
		if err == nil {
			l103expand16(buf)
		}
	case 15:
		err = l103decode15(in, buf, sc)
		if err == nil {
			l103expand16(buf)
		}
	case 16:
		err = l103decode16(in, buf)
		if err == nil {
			l103expand16(buf)
		}
	default:
		err = l103decodeSlow(bitsPerValue, in, buf)
	}
	if err != nil {
		return err
	}

	for i := 0; i < lucene103BlockSizeConst; i++ {
		ints[i] = int64(uint32(buf[i]))
	}
	return nil
}

// l103decodeSlow handles bitsPerValue values 17..31. Mirrors ForUtil.decodeSlow.
func l103decodeSlow(bitsPerValue int, in store.IndexInput, ints []int32) error {
	numInts := bitsPerValue << 2 // bitsPerValue * 4
	mask := masks32[bitsPerValue]

	var scratch [lucene103BlockSizeConst]int32

	if err := l103splitInts(in, numInts, ints, 32-bitsPerValue, 32, mask, scratch[:], 0, -1); err != nil {
		return err
	}

	remainingBitsPerInt := 32 - bitsPerValue
	mask32Remaining := masks32[remainingBitsPerInt]

	tmpIdx := 0
	remainingBits := remainingBitsPerInt
	for intsIdx := numInts; intsIdx < lucene103BlockSizeConst; intsIdx++ {
		b := bitsPerValue - remainingBits
		l := (scratch[tmpIdx] & masks32[remainingBits]) << uint(b)
		tmpIdx++
		for b >= remainingBitsPerInt {
			b -= remainingBitsPerInt
			l |= (scratch[tmpIdx] & mask32Remaining) << uint(b)
			tmpIdx++
		}
		if b > 0 {
			l |= int32(uint32(scratch[tmpIdx])>>uint(remainingBitsPerInt-b)) & masks32[b]
			remainingBits = remainingBitsPerInt - b
		} else {
			remainingBits = remainingBitsPerInt
		}
		ints[intsIdx] = l
	}
	return nil
}

// ---------- decode1..16 (128-wide) ----------
//
// These are direct transcriptions of backward_codecs.lucene103.ForUtil's
// generated decode methods. The splitInts counts, shifts and intsIdx offsets
// differ from the 256-wide variants (for_util.go) because the block is half
// as wide.

func l103decode1(in store.IndexInput, ints []int32) error {
	// splitInts(4, ints, 7, 1, MASK8_1, ints, 28, MASK8_1)
	return l103splitInts(in, 4, ints, 7, 1, masks8[1], ints, 28, masks8[1])
}

func l103decode2(in store.IndexInput, ints []int32) error {
	// splitInts(8, ints, 6, 2, MASK8_2, ints, 24, MASK8_2)
	return l103splitInts(in, 8, ints, 6, 2, masks8[2], ints, 24, masks8[2])
}

func l103decode3(in store.IndexInput, ints []int32, tmp []int32) error {
	// splitInts(12, ints, 5, 3, MASK8_3, tmp, 0, MASK8_2)
	if err := l103splitInts(in, 12, ints, 5, 3, masks8[3], tmp, 0, masks8[2]); err != nil {
		return err
	}
	for iter, tmpIdx, intsIdx := 0, 0, 24; iter < 4; iter, tmpIdx, intsIdx = iter+1, tmpIdx+3, intsIdx+2 {
		l0 := tmp[tmpIdx+0] << 1
		l0 |= int32(uint32(tmp[tmpIdx+1])>>1) & masks8[1]
		ints[intsIdx+0] = l0
		l1 := (tmp[tmpIdx+1] & masks8[1]) << 2
		l1 |= tmp[tmpIdx+2]
		ints[intsIdx+1] = l1
	}
	return nil
}

func l103decode4(in store.IndexInput, ints []int32) error {
	// splitInts(16, ints, 4, 4, MASK8_4, ints, 16, MASK8_4)
	return l103splitInts(in, 16, ints, 4, 4, masks8[4], ints, 16, masks8[4])
}

func l103decode5(in store.IndexInput, ints []int32, tmp []int32) error {
	// splitInts(20, ints, 3, 5, MASK8_5, tmp, 0, MASK8_3)
	if err := l103splitInts(in, 20, ints, 3, 5, masks8[5], tmp, 0, masks8[3]); err != nil {
		return err
	}
	for iter, tmpIdx, intsIdx := 0, 0, 20; iter < 4; iter, tmpIdx, intsIdx = iter+1, tmpIdx+5, intsIdx+3 {
		l0 := tmp[tmpIdx+0] << 2
		l0 |= int32(uint32(tmp[tmpIdx+1])>>1) & masks8[2]
		ints[intsIdx+0] = l0
		l1 := (tmp[tmpIdx+1] & masks8[1]) << 4
		l1 |= tmp[tmpIdx+2] << 1
		l1 |= int32(uint32(tmp[tmpIdx+3])>>2) & masks8[1]
		ints[intsIdx+1] = l1
		l2 := (tmp[tmpIdx+3] & masks8[2]) << 3
		l2 |= tmp[tmpIdx+4]
		ints[intsIdx+2] = l2
	}
	return nil
}

func l103decode6(in store.IndexInput, ints []int32, tmp []int32) error {
	// splitInts(24, ints, 2, 6, MASK8_6, tmp, 0, MASK8_2)
	if err := l103splitInts(in, 24, ints, 2, 6, masks8[6], tmp, 0, masks8[2]); err != nil {
		return err
	}
	for iter, tmpIdx, intsIdx := 0, 0, 24; iter < 8; iter, tmpIdx, intsIdx = iter+1, tmpIdx+3, intsIdx+1 {
		l0 := tmp[tmpIdx+0] << 4
		l0 |= tmp[tmpIdx+1] << 2
		l0 |= tmp[tmpIdx+2]
		ints[intsIdx+0] = l0
	}
	return nil
}

func l103decode7(in store.IndexInput, ints []int32, tmp []int32) error {
	// splitInts(28, ints, 1, 7, MASK8_7, tmp, 0, MASK8_1)
	if err := l103splitInts(in, 28, ints, 1, 7, masks8[7], tmp, 0, masks8[1]); err != nil {
		return err
	}
	for iter, tmpIdx, intsIdx := 0, 0, 28; iter < 4; iter, tmpIdx, intsIdx = iter+1, tmpIdx+7, intsIdx+1 {
		l0 := tmp[tmpIdx+0] << 6
		l0 |= tmp[tmpIdx+1] << 5
		l0 |= tmp[tmpIdx+2] << 4
		l0 |= tmp[tmpIdx+3] << 3
		l0 |= tmp[tmpIdx+4] << 2
		l0 |= tmp[tmpIdx+5] << 1
		l0 |= tmp[tmpIdx+6]
		ints[intsIdx+0] = l0
	}
	return nil
}

func l103decode8(in store.IndexInput, ints []int32) error {
	// pdu.in.readInts(ints, 0, 32)
	var buf [4]byte
	for i := 0; i < 32; i++ {
		if err := in.ReadBytes(buf[:]); err != nil {
			return err
		}
		ints[i] = int32(binary.BigEndian.Uint32(buf[:]))
	}
	return nil
}

func l103decode9(in store.IndexInput, ints []int32, tmp []int32) error {
	// splitInts(36, ints, 7, 9, MASK16_9, tmp, 0, MASK16_7)
	if err := l103splitInts(in, 36, ints, 7, 9, masks16[9], tmp, 0, masks16[7]); err != nil {
		return err
	}
	for iter, tmpIdx, intsIdx := 0, 0, 36; iter < 4; iter, tmpIdx, intsIdx = iter+1, tmpIdx+9, intsIdx+7 {
		l0 := tmp[tmpIdx+0] << 2
		l0 |= int32(uint32(tmp[tmpIdx+1])>>5) & masks16[2]
		ints[intsIdx+0] = l0
		l1 := (tmp[tmpIdx+1] & masks16[5]) << 4
		l1 |= int32(uint32(tmp[tmpIdx+2])>>3) & masks16[4]
		ints[intsIdx+1] = l1
		l2 := (tmp[tmpIdx+2] & masks16[3]) << 6
		l2 |= int32(uint32(tmp[tmpIdx+3])>>1) & masks16[6]
		ints[intsIdx+2] = l2
		l3 := (tmp[tmpIdx+3] & masks16[1]) << 8
		l3 |= tmp[tmpIdx+4] << 1
		l3 |= int32(uint32(tmp[tmpIdx+5])>>6) & masks16[1]
		ints[intsIdx+3] = l3
		l4 := (tmp[tmpIdx+5] & masks16[6]) << 3
		l4 |= int32(uint32(tmp[tmpIdx+6])>>4) & masks16[3]
		ints[intsIdx+4] = l4
		l5 := (tmp[tmpIdx+6] & masks16[4]) << 5
		l5 |= int32(uint32(tmp[tmpIdx+7])>>2) & masks16[5]
		ints[intsIdx+5] = l5
		l6 := (tmp[tmpIdx+7] & masks16[2]) << 7
		l6 |= tmp[tmpIdx+8]
		ints[intsIdx+6] = l6
	}
	return nil
}

func l103decode10(in store.IndexInput, ints []int32, tmp []int32) error {
	// splitInts(40, ints, 6, 10, MASK16_10, tmp, 0, MASK16_6)
	if err := l103splitInts(in, 40, ints, 6, 10, masks16[10], tmp, 0, masks16[6]); err != nil {
		return err
	}
	for iter, tmpIdx, intsIdx := 0, 0, 40; iter < 8; iter, tmpIdx, intsIdx = iter+1, tmpIdx+5, intsIdx+3 {
		l0 := tmp[tmpIdx+0] << 4
		l0 |= int32(uint32(tmp[tmpIdx+1])>>2) & masks16[4]
		ints[intsIdx+0] = l0
		l1 := (tmp[tmpIdx+1] & masks16[2]) << 8
		l1 |= tmp[tmpIdx+2] << 2
		l1 |= int32(uint32(tmp[tmpIdx+3])>>4) & masks16[2]
		ints[intsIdx+1] = l1
		l2 := (tmp[tmpIdx+3] & masks16[4]) << 6
		l2 |= tmp[tmpIdx+4]
		ints[intsIdx+2] = l2
	}
	return nil
}

func l103decode11(in store.IndexInput, ints []int32, tmp []int32) error {
	// splitInts(44, ints, 5, 11, MASK16_11, tmp, 0, MASK16_5)
	if err := l103splitInts(in, 44, ints, 5, 11, masks16[11], tmp, 0, masks16[5]); err != nil {
		return err
	}
	for iter, tmpIdx, intsIdx := 0, 0, 44; iter < 4; iter, tmpIdx, intsIdx = iter+1, tmpIdx+11, intsIdx+5 {
		l0 := tmp[tmpIdx+0] << 6
		l0 |= tmp[tmpIdx+1] << 1
		l0 |= int32(uint32(tmp[tmpIdx+2])>>4) & masks16[1]
		ints[intsIdx+0] = l0
		l1 := (tmp[tmpIdx+2] & masks16[4]) << 7
		l1 |= tmp[tmpIdx+3] << 2
		l1 |= int32(uint32(tmp[tmpIdx+4])>>3) & masks16[2]
		ints[intsIdx+1] = l1
		l2 := (tmp[tmpIdx+4] & masks16[3]) << 8
		l2 |= tmp[tmpIdx+5] << 3
		l2 |= int32(uint32(tmp[tmpIdx+6])>>2) & masks16[3]
		ints[intsIdx+2] = l2
		l3 := (tmp[tmpIdx+6] & masks16[2]) << 9
		l3 |= tmp[tmpIdx+7] << 4
		l3 |= int32(uint32(tmp[tmpIdx+8])>>1) & masks16[4]
		ints[intsIdx+3] = l3
		l4 := (tmp[tmpIdx+8] & masks16[1]) << 10
		l4 |= tmp[tmpIdx+9] << 5
		l4 |= tmp[tmpIdx+10]
		ints[intsIdx+4] = l4
	}
	return nil
}

func l103decode12(in store.IndexInput, ints []int32, tmp []int32) error {
	// splitInts(48, ints, 4, 12, MASK16_12, tmp, 0, MASK16_4)
	if err := l103splitInts(in, 48, ints, 4, 12, masks16[12], tmp, 0, masks16[4]); err != nil {
		return err
	}
	for iter, tmpIdx, intsIdx := 0, 0, 48; iter < 16; iter, tmpIdx, intsIdx = iter+1, tmpIdx+3, intsIdx+1 {
		l0 := tmp[tmpIdx+0] << 8
		l0 |= tmp[tmpIdx+1] << 4
		l0 |= tmp[tmpIdx+2]
		ints[intsIdx+0] = l0
	}
	return nil
}

func l103decode13(in store.IndexInput, ints []int32, tmp []int32) error {
	// splitInts(52, ints, 3, 13, MASK16_13, tmp, 0, MASK16_3)
	if err := l103splitInts(in, 52, ints, 3, 13, masks16[13], tmp, 0, masks16[3]); err != nil {
		return err
	}
	for iter, tmpIdx, intsIdx := 0, 0, 52; iter < 4; iter, tmpIdx, intsIdx = iter+1, tmpIdx+13, intsIdx+3 {
		l0 := tmp[tmpIdx+0] << 10
		l0 |= tmp[tmpIdx+1] << 7
		l0 |= tmp[tmpIdx+2] << 4
		l0 |= tmp[tmpIdx+3] << 1
		l0 |= int32(uint32(tmp[tmpIdx+4])>>2) & masks16[1]
		ints[intsIdx+0] = l0
		l1 := (tmp[tmpIdx+4] & masks16[2]) << 11
		l1 |= tmp[tmpIdx+5] << 8
		l1 |= tmp[tmpIdx+6] << 5
		l1 |= tmp[tmpIdx+7] << 2
		l1 |= int32(uint32(tmp[tmpIdx+8])>>1) & masks16[2]
		ints[intsIdx+1] = l1
		l2 := (tmp[tmpIdx+8] & masks16[1]) << 12
		l2 |= tmp[tmpIdx+9] << 9
		l2 |= tmp[tmpIdx+10] << 6
		l2 |= tmp[tmpIdx+11] << 3
		l2 |= tmp[tmpIdx+12]
		ints[intsIdx+2] = l2
	}
	return nil
}

func l103decode14(in store.IndexInput, ints []int32, tmp []int32) error {
	// splitInts(56, ints, 2, 14, MASK16_14, tmp, 0, MASK16_2)
	if err := l103splitInts(in, 56, ints, 2, 14, masks16[14], tmp, 0, masks16[2]); err != nil {
		return err
	}
	for iter, tmpIdx, intsIdx := 0, 0, 56; iter < 8; iter, tmpIdx, intsIdx = iter+1, tmpIdx+7, intsIdx+1 {
		l0 := tmp[tmpIdx+0] << 12
		l0 |= tmp[tmpIdx+1] << 10
		l0 |= tmp[tmpIdx+2] << 8
		l0 |= tmp[tmpIdx+3] << 6
		l0 |= tmp[tmpIdx+4] << 4
		l0 |= tmp[tmpIdx+5] << 2
		l0 |= tmp[tmpIdx+6]
		ints[intsIdx+0] = l0
	}
	return nil
}

func l103decode15(in store.IndexInput, ints []int32, tmp []int32) error {
	// splitInts(60, ints, 1, 15, MASK16_15, tmp, 0, MASK16_1)
	if err := l103splitInts(in, 60, ints, 1, 15, masks16[15], tmp, 0, masks16[1]); err != nil {
		return err
	}
	for iter, tmpIdx, intsIdx := 0, 0, 60; iter < 4; iter, tmpIdx, intsIdx = iter+1, tmpIdx+15, intsIdx+1 {
		l0 := tmp[tmpIdx+0] << 14
		l0 |= tmp[tmpIdx+1] << 13
		l0 |= tmp[tmpIdx+2] << 12
		l0 |= tmp[tmpIdx+3] << 11
		l0 |= tmp[tmpIdx+4] << 10
		l0 |= tmp[tmpIdx+5] << 9
		l0 |= tmp[tmpIdx+6] << 8
		l0 |= tmp[tmpIdx+7] << 7
		l0 |= tmp[tmpIdx+8] << 6
		l0 |= tmp[tmpIdx+9] << 5
		l0 |= tmp[tmpIdx+10] << 4
		l0 |= tmp[tmpIdx+11] << 3
		l0 |= tmp[tmpIdx+12] << 2
		l0 |= tmp[tmpIdx+13] << 1
		l0 |= tmp[tmpIdx+14]
		ints[intsIdx+0] = l0
	}
	return nil
}

func l103decode16(in store.IndexInput, ints []int32) error {
	// pdu.in.readInts(ints, 0, 64)
	var buf [4]byte
	for i := 0; i < 64; i++ {
		if err := in.ReadBytes(buf[:]); err != nil {
			return err
		}
		ints[i] = int32(binary.BigEndian.Uint32(buf[:]))
	}
	return nil
}
