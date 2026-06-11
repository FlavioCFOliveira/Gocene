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
//	lucene99/ForUtil.java  (BLOCK_SIZE=128, long-based)
//
// Purpose: 128-wide Frame-of-Reference encode/decode with prefix-sum for
// the Lucene 9.9 backward-codecs postings format. This is the long
// (int64) variant, used by the Lucene99 postings format, distinct from
// the 128-wide int32 variant in lucene103_for_util.go and the 256-wide
// int32 variant in for_util.go.
//
// The pre-computed int32 mask tables (masks8/masks16/masks32) from
// for_util.go are not suitable because this file operates on int64
// longs which require full 64-bit mask expansion. We define separate
// int64 mask tables (l99masks8/l99masks16/l99masks32) with the same
// replication logic extended to 64 bits.

package codecs

import (
	"encoding/binary"
	"errors"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// lucene99BlockSize is the block size (128 integers per Encode/Decode cycle).
// Mirrors backward_codecs.lucene99.ForUtil.BLOCK_SIZE.
const lucene99BlockSize = 128

// lucene99BlockSizeLog2 is log2(BLOCK_SIZE) = 7.
const lucene99BlockSizeLog2 = 7

// int64 mask tables for long-based operations.
// These replicate the same pattern as masks8/masks16/masks32 in for_util.go
// but expanded to 64-bit (int64) instead of 32-bit (int32).
var l99masks8 [8]int64
var l99masks16 [16]int64
var l99masks32 [32]int64

func init() {
	for i := 0; i < 8; i++ {
		l99masks8[i] = l99mask8(i)
	}
	for i := 0; i < 16; i++ {
		l99masks16[i] = l99mask16(i)
	}
	for i := 0; i < 32; i++ {
		l99masks32[i] = l99mask32(i)
	}
}

// ---- int64 mask helpers ----

func l99expandMask32(mask32 int64) int64 {
	return mask32 | (mask32 << 32)
}

func l99expandMask16(mask16 int64) int64 {
	return l99expandMask32(mask16 | (mask16 << 16))
}

func l99expandMask8(mask8 int64) int64 {
	return l99expandMask16(mask8 | (mask8 << 8))
}

func l99mask32(bitsPerValue int) int64 {
	if bitsPerValue <= 0 {
		return 0
	}
	if bitsPerValue >= 32 {
		return -1
	}
	return l99expandMask32((int64(1) << bitsPerValue) - 1)
}

func l99mask16(bitsPerValue int) int64 {
	if bitsPerValue <= 0 {
		return 0
	}
	if bitsPerValue >= 16 {
		return -1
	}
	return l99expandMask16((int64(1) << bitsPerValue) - 1)
}

func l99mask8(bitsPerValue int) int64 {
	if bitsPerValue <= 0 {
		return 0
	}
	if bitsPerValue >= 8 {
		return -1
	}
	return l99expandMask8((int64(1) << bitsPerValue) - 1)
}

// lucene99ForUtil provides 128-wide long-based Frame-of-Reference
// encode/decode. Mirrors backward_codecs.lucene99.ForUtil.
type lucene99ForUtil struct {
	// tmp is a scratch buffer of BLOCK_SIZE/2 = 64 longs, used by
	// both encode and decode. Matches Java's final long[] tmp.
	tmp []int64
}

// newLucene99ForUtil creates a new instance with pre-allocated scratch buffer.
func newLucene99ForUtil() *lucene99ForUtil {
	return &lucene99ForUtil{
		tmp: make([]int64, lucene99BlockSize/2),
	}
}

// l99forNumBytes returns the number of bytes required to encode 128 integers
// of bitsPerValue bits per value. Mirrors ForUtil.numBytes(int).
func (f *lucene99ForUtil) l99forNumBytes(bitsPerValue int) int {
	return bitsPerValue << (lucene99BlockSizeLog2 - 3) // bitsPerValue << 4
}

// ---- collapse / expand (128-wide long-based) ----

// l99expand8 unpacks 16 packed longs into 128 longs.
// Each packed long has 8 byte-sized values at offsets 0,16,32,48,64,80,96,112.
// Mirrors ForUtil.expand8.
func l99expand8(arr []int64) {
	for i := 0; i < 16; i++ {
		l := arr[i]
		arr[i] = int64(uint64(l)>>56) & 0xFF
		arr[16+i] = int64(uint64(l)>>48) & 0xFF
		arr[32+i] = int64(uint64(l)>>40) & 0xFF
		arr[48+i] = int64(uint64(l)>>32) & 0xFF
		arr[64+i] = int64(uint64(l)>>24) & 0xFF
		arr[80+i] = int64(uint64(l)>>16) & 0xFF
		arr[96+i] = int64(uint64(l)>>8) & 0xFF
		arr[112+i] = l & 0xFF
	}
}

// l99collapse8 packs 128 longs into 16 packed longs (8 values per long).
// Mirrors ForUtil.collapse8.
func l99collapse8(arr []int64) {
	for i := 0; i < 16; i++ {
		arr[i] = (arr[i] << 56) |
			(arr[16+i] << 48) |
			(arr[32+i] << 40) |
			(arr[48+i] << 32) |
			(arr[64+i] << 24) |
			(arr[80+i] << 16) |
			(arr[96+i] << 8) |
			arr[112+i]
	}
}

// l99expand16 unpacks 32 packed longs into 128 longs.
// Each packed long has 4 short-sized values at offsets 0,32,64,96.
// Mirrors ForUtil.expand16.
func l99expand16(arr []int64) {
	for i := 0; i < 32; i++ {
		l := arr[i]
		arr[i] = int64(uint64(l)>>48) & 0xFFFF
		arr[32+i] = int64(uint64(l)>>32) & 0xFFFF
		arr[64+i] = int64(uint64(l)>>16) & 0xFFFF
		arr[96+i] = l & 0xFFFF
	}
}

// l99collapse16 packs 128 longs into 32 packed longs (4 values per long).
// Mirrors ForUtil.collapse16.
func l99collapse16(arr []int64) {
	for i := 0; i < 32; i++ {
		arr[i] = (arr[i] << 48) | (arr[32+i] << 32) | (arr[64+i] << 16) | arr[96+i]
	}
}

// l99expand32 unpacks 64 packed longs into 128 longs.
// Each packed long has 2 int-sized values (high/low 32 bits each).
// Mirrors ForUtil.expand32.
func l99expand32(arr []int64) {
	for i := 0; i < 64; i++ {
		l := arr[i]
		arr[i] = int64(uint64(l) >> 32)
		arr[64+i] = l & 0xFFFFFFFF
	}
}

// l99collapse32 packs 128 longs into 64 packed longs (2 values per long).
// Mirrors ForUtil.collapse32.
func l99collapse32(arr []int64) {
	for i := 0; i < 64; i++ {
		arr[i] = (arr[i] << 32) | arr[64+i]
	}
}

// ---- prefix-sum helpers ----

// l99expand8To32 widens from 8-per-long to 4-per-long packing.
// Mirrors ForUtil.expand8To32.
func l99expand8To32(arr []int64) {
	for i := 0; i < 16; i++ {
		l := arr[i]
		arr[i] = int64(uint64(l)>>24) & 0x000000FF000000FF
		arr[16+i] = int64(uint64(l)>>16) & 0x000000FF000000FF
		arr[32+i] = int64(uint64(l)>>8) & 0x000000FF000000FF
		arr[48+i] = l & 0x000000FF000000FF
	}
}

// l99expand16To32 widens from 16-per-long to 32-per-long packing.
// Mirrors ForUtil.expand16To32.
func l99expand16To32(arr []int64) {
	for i := 0; i < 32; i++ {
		l := arr[i]
		arr[i] = int64(uint64(l)>>16) & 0x0000FFFF0000FFFF
		arr[32+i] = l & 0x0000FFFF0000FFFF
	}
}

// l99innerPrefixSum32 is the hand-unrolled 64-element prefix sum on
// the "split" representation (32-bit fields within 64-bit longs).
// Mirrors ForUtil.innerPrefixSum32.
func l99innerPrefixSum32(arr []int64) {
	arr[1] += arr[0]
	arr[2] += arr[1]
	arr[3] += arr[2]
	arr[4] += arr[3]
	arr[5] += arr[4]
	arr[6] += arr[5]
	arr[7] += arr[6]
	arr[8] += arr[7]
	arr[9] += arr[8]
	arr[10] += arr[9]
	arr[11] += arr[10]
	arr[12] += arr[11]
	arr[13] += arr[12]
	arr[14] += arr[13]
	arr[15] += arr[14]
	arr[16] += arr[15]
	arr[17] += arr[16]
	arr[18] += arr[17]
	arr[19] += arr[18]
	arr[20] += arr[19]
	arr[21] += arr[20]
	arr[22] += arr[21]
	arr[23] += arr[22]
	arr[24] += arr[23]
	arr[25] += arr[24]
	arr[26] += arr[25]
	arr[27] += arr[26]
	arr[28] += arr[27]
	arr[29] += arr[28]
	arr[30] += arr[29]
	arr[31] += arr[30]
	arr[32] += arr[31]
	arr[33] += arr[32]
	arr[34] += arr[33]
	arr[35] += arr[34]
	arr[36] += arr[35]
	arr[37] += arr[36]
	arr[38] += arr[37]
	arr[39] += arr[38]
	arr[40] += arr[39]
	arr[41] += arr[40]
	arr[42] += arr[41]
	arr[43] += arr[42]
	arr[44] += arr[43]
	arr[45] += arr[44]
	arr[46] += arr[45]
	arr[47] += arr[46]
	arr[48] += arr[47]
	arr[49] += arr[48]
	arr[50] += arr[49]
	arr[51] += arr[50]
	arr[52] += arr[51]
	arr[53] += arr[52]
	arr[54] += arr[53]
	arr[55] += arr[54]
	arr[56] += arr[55]
	arr[57] += arr[56]
	arr[58] += arr[57]
	arr[59] += arr[58]
	arr[60] += arr[59]
	arr[61] += arr[60]
	arr[62] += arr[61]
	arr[63] += arr[62]
}

// l99prefixSum32 computes a prefix sum on the "split" representation
// (64 longs, each with 2 int-sized fields). Mirrors ForUtil.prefixSum32.
func l99prefixSum32(arr []int64, base int64) {
	arr[0] += base << 32
	l99innerPrefixSum32(arr)
	l99expand32(arr)
	l := arr[lucene99BlockSize/2-1]
	for i := lucene99BlockSize / 2; i < lucene99BlockSize; i++ {
		arr[i] += l
	}
}

// l99prefixSum8 first widens from 8→32-bit packing then delegates to
// prefixSum32. Mirrors ForUtil.prefixSum8.
func l99prefixSum8(arr []int64, base int64) {
	l99expand8To32(arr)
	l99prefixSum32(arr, base)
}

// l99prefixSum16 first widens from 16→32-bit packing then delegates to
// prefixSum32. Mirrors ForUtil.prefixSum16.
func l99prefixSum16(arr []int64, base int64) {
	l99expand16To32(arr)
	l99prefixSum32(arr, base)
}

// ---- long I/O helpers ----

// l99readLongs reads count int64 values from in (big-endian) into longs[0..count-1].
func l99readLongs(in store.IndexInput, count int, longs []int64) error {
	buf := make([]byte, count*8)
	if err := in.ReadBytes(buf); err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		longs[i] = int64(binary.BigEndian.Uint64(buf[i*8:]))
	}
	return nil
}

// ---- shiftLongs ----

// l99shiftLongs mirrors ForUtil.shiftLongs: for each i in [0,count),
//
//	b[bi+i] = (a[i] >>> shift) & mask
//
// This pattern is recognized by the Java C2 compiler for SIMD; in Go it
// simply iterates.
func l99shiftLongs(a []int64, count int, b []int64, bi int, shift int, mask int64) {
	for i := 0; i < count; i++ {
		b[bi+i] = int64(uint64(a[i])>>uint(shift)) & mask
	}
}

// ---- Encode ----

// encode encodes 128 integers from ints into out using the specified
// bitsPerValue. Mirrors ForUtil.encode.
func (f *lucene99ForUtil) encode(ints []int64, bitsPerValue int, out store.IndexOutput) error {
	if len(ints) < lucene99BlockSize {
		return errors.New("lucene99 ForUtil.encode: ints must have at least 128 elements")
	}

	// Work on a local copy to avoid mutating the caller's slice.
	buf := make([]int64, lucene99BlockSize)
	copy(buf, ints[:lucene99BlockSize])

	var nextPrimitive int
	var numLongs int
	if bitsPerValue <= 8 {
		nextPrimitive = 8
		numLongs = lucene99BlockSize / 8 // 16
		l99collapse8(buf)
	} else if bitsPerValue <= 16 {
		nextPrimitive = 16
		numLongs = lucene99BlockSize / 4 // 32
		l99collapse16(buf)
	} else {
		nextPrimitive = 32
		numLongs = lucene99BlockSize / 2 // 64
	}

	numLongsPerShift := bitsPerValue * 2
	tmp := f.tmp // reuse struct scratch buffer (size 64)

	idx := 0
	shift := nextPrimitive - bitsPerValue
	for i := 0; i < numLongsPerShift; i++ {
		tmp[i] = buf[idx] << uint(shift)
		idx++
	}
	for shift -= bitsPerValue; shift >= 0; shift -= bitsPerValue {
		for i := 0; i < numLongsPerShift; i++ {
			tmp[i] |= buf[idx] << uint(shift)
			idx++
		}
	}

	remainingBitsPerLong := shift + bitsPerValue // shift is now negative
	var maskRemaining int64
	switch nextPrimitive {
	case 8:
		maskRemaining = l99masks8[remainingBitsPerLong]
	case 16:
		maskRemaining = l99masks16[remainingBitsPerLong]
	default:
		maskRemaining = l99masks32[remainingBitsPerLong]
	}

	tmpIdx := 0
	remainingBitsPerValue := bitsPerValue
	for idx < numLongs {
		if remainingBitsPerValue >= remainingBitsPerLong {
			remainingBitsPerValue -= remainingBitsPerLong
			tmp[tmpIdx] |= int64(uint64(buf[idx])>>uint(remainingBitsPerValue)) & maskRemaining
			tmpIdx++
			if remainingBitsPerValue == 0 {
				idx++
				remainingBitsPerValue = bitsPerValue
			}
		} else {
			var mask1, mask2 int64
			switch nextPrimitive {
			case 8:
				mask1 = l99masks8[remainingBitsPerValue]
				mask2 = l99masks8[remainingBitsPerLong-remainingBitsPerValue]
			case 16:
				mask1 = l99masks16[remainingBitsPerValue]
				mask2 = l99masks16[remainingBitsPerLong-remainingBitsPerValue]
			default:
				mask1 = l99masks32[remainingBitsPerValue]
				mask2 = l99masks32[remainingBitsPerLong-remainingBitsPerValue]
			}
			tmp[tmpIdx] |= (buf[idx] & mask1) << uint(remainingBitsPerLong-remainingBitsPerValue)
			idx++
			remainingBitsPerValue = bitsPerValue - remainingBitsPerLong + remainingBitsPerValue
			tmp[tmpIdx] |= int64(uint64(buf[idx])>>uint(remainingBitsPerValue)) & mask2
			tmpIdx++
		}
	}

	// Write numLongsPerShift packed longs (big-endian).
	var b [8]byte
	for i := 0; i < numLongsPerShift; i++ {
		binary.BigEndian.PutUint64(b[:], uint64(tmp[i]))
		if err := out.WriteBytes(b[:]); err != nil {
			return err
		}
	}
	return nil
}

// ---- Decode ----

// decode decodes 128 integers from in into longs using bitsPerValue.
// Mirrors ForUtil.decode.
func (f *lucene99ForUtil) decode(bitsPerValue int, in store.IndexInput, longs []int64) error {
	if len(longs) < lucene99BlockSize {
		return errors.New("lucene99 ForUtil.decode: longs must have at least 128 elements")
	}

	tmp := f.tmp // scratch buffer (64 elements)

	var err error
	switch bitsPerValue {
	case 1:
		err = l99decode1(in, tmp, longs)
		if err == nil {
			l99expand8(longs)
		}
	case 2:
		err = l99decode2(in, tmp, longs)
		if err == nil {
			l99expand8(longs)
		}
	case 3:
		err = l99decode3(in, tmp, longs)
		if err == nil {
			l99expand8(longs)
		}
	case 4:
		err = l99decode4(in, tmp, longs)
		if err == nil {
			l99expand8(longs)
		}
	case 5:
		err = l99decode5(in, tmp, longs)
		if err == nil {
			l99expand8(longs)
		}
	case 6:
		err = l99decode6(in, tmp, longs)
		if err == nil {
			l99expand8(longs)
		}
	case 7:
		err = l99decode7(in, tmp, longs)
		if err == nil {
			l99expand8(longs)
		}
	case 8:
		err = l99decode8(in, tmp, longs)
		if err == nil {
			l99expand8(longs)
		}
	case 9:
		err = l99decode9(in, tmp, longs)
		if err == nil {
			l99expand16(longs)
		}
	case 10:
		err = l99decode10(in, tmp, longs)
		if err == nil {
			l99expand16(longs)
		}
	case 11:
		err = l99decode11(in, tmp, longs)
		if err == nil {
			l99expand16(longs)
		}
	case 12:
		err = l99decode12(in, tmp, longs)
		if err == nil {
			l99expand16(longs)
		}
	case 13:
		err = l99decode13(in, tmp, longs)
		if err == nil {
			l99expand16(longs)
		}
	case 14:
		err = l99decode14(in, tmp, longs)
		if err == nil {
			l99expand16(longs)
		}
	case 15:
		err = l99decode15(in, tmp, longs)
		if err == nil {
			l99expand16(longs)
		}
	case 16:
		err = l99decode16(in, tmp, longs)
		if err == nil {
			l99expand16(longs)
		}
	case 17:
		err = l99decode17(in, tmp, longs)
		if err == nil {
			l99expand32(longs)
		}
	case 18:
		err = l99decode18(in, tmp, longs)
		if err == nil {
			l99expand32(longs)
		}
	case 19:
		err = l99decode19(in, tmp, longs)
		if err == nil {
			l99expand32(longs)
		}
	case 20:
		err = l99decode20(in, tmp, longs)
		if err == nil {
			l99expand32(longs)
		}
	case 21:
		err = l99decode21(in, tmp, longs)
		if err == nil {
			l99expand32(longs)
		}
	case 22:
		err = l99decode22(in, tmp, longs)
		if err == nil {
			l99expand32(longs)
		}
	case 23:
		err = l99decode23(in, tmp, longs)
		if err == nil {
			l99expand32(longs)
		}
	case 24:
		err = l99decode24(in, tmp, longs)
		if err == nil {
			l99expand32(longs)
		}
	default:
		err = l99decodeSlow(bitsPerValue, in, tmp, longs)
		if err == nil {
			l99expand32(longs)
		}
	}
	return err
}

// decodeAndPrefixSum decodes 128 delta-encoded integers from in into longs,
// then computes the prefix sum. Mirrors ForUtil.decodeAndPrefixSum.
func (f *lucene99ForUtil) decodeAndPrefixSum(bitsPerValue int, in store.IndexInput, base int64, longs []int64) error {
	if len(longs) < lucene99BlockSize {
		return errors.New("lucene99 ForUtil.decodeAndPrefixSum: longs must have at least 128 elements")
	}

	tmp := f.tmp

	var err error
	switch bitsPerValue {
	case 1:
		err = l99decode1(in, tmp, longs)
		if err == nil {
			l99prefixSum8(longs, base)
		}
	case 2:
		err = l99decode2(in, tmp, longs)
		if err == nil {
			l99prefixSum8(longs, base)
		}
	case 3:
		err = l99decode3(in, tmp, longs)
		if err == nil {
			l99prefixSum8(longs, base)
		}
	case 4:
		err = l99decode4(in, tmp, longs)
		if err == nil {
			l99prefixSum8(longs, base)
		}
	case 5:
		err = l99decode5(in, tmp, longs)
		if err == nil {
			l99prefixSum8(longs, base)
		}
	case 6:
		err = l99decode6(in, tmp, longs)
		if err == nil {
			l99prefixSum8(longs, base)
		}
	case 7:
		err = l99decode7(in, tmp, longs)
		if err == nil {
			l99prefixSum8(longs, base)
		}
	case 8:
		err = l99decode8(in, tmp, longs)
		if err == nil {
			l99prefixSum8(longs, base)
		}
	case 9:
		err = l99decode9(in, tmp, longs)
		if err == nil {
			l99prefixSum16(longs, base)
		}
	case 10:
		err = l99decode10(in, tmp, longs)
		if err == nil {
			l99prefixSum16(longs, base)
		}
	case 11:
		err = l99decode11(in, tmp, longs)
		if err == nil {
			l99prefixSum16(longs, base)
		}
	case 12:
		err = l99decode12(in, tmp, longs)
		if err == nil {
			l99prefixSum16(longs, base)
		}
	case 13:
		err = l99decode13(in, tmp, longs)
		if err == nil {
			l99prefixSum16(longs, base)
		}
	case 14:
		err = l99decode14(in, tmp, longs)
		if err == nil {
			l99prefixSum16(longs, base)
		}
	case 15:
		err = l99decode15(in, tmp, longs)
		if err == nil {
			l99prefixSum16(longs, base)
		}
	case 16:
		err = l99decode16(in, tmp, longs)
		if err == nil {
			l99prefixSum16(longs, base)
		}
	case 17:
		err = l99decode17(in, tmp, longs)
		if err == nil {
			l99prefixSum32(longs, base)
		}
	case 18:
		err = l99decode18(in, tmp, longs)
		if err == nil {
			l99prefixSum32(longs, base)
		}
	case 19:
		err = l99decode19(in, tmp, longs)
		if err == nil {
			l99prefixSum32(longs, base)
		}
	case 20:
		err = l99decode20(in, tmp, longs)
		if err == nil {
			l99prefixSum32(longs, base)
		}
	case 21:
		err = l99decode21(in, tmp, longs)
		if err == nil {
			l99prefixSum32(longs, base)
		}
	case 22:
		err = l99decode22(in, tmp, longs)
		if err == nil {
			l99prefixSum32(longs, base)
		}
	case 23:
		err = l99decode23(in, tmp, longs)
		if err == nil {
			l99prefixSum32(longs, base)
		}
	case 24:
		err = l99decode24(in, tmp, longs)
		if err == nil {
			l99prefixSum32(longs, base)
		}
	default:
		err = l99decodeSlow(bitsPerValue, in, tmp, longs)
		if err == nil {
			l99prefixSum32(longs, base)
		}
	}
	return err
}

// ---- decodeSlow ----

// l99decodeSlow handles bitsPerValue > 24 (up to 31).
// Mirrors ForUtil.decodeSlow.
func l99decodeSlow(bitsPerValue int, in store.IndexInput, tmp []int64, longs []int64) error {
	numLongs := bitsPerValue << 1 // bitsPerValue * 2
	if err := l99readLongs(in, numLongs, tmp); err != nil {
		return err
	}
	mask := l99masks32[bitsPerValue]
	longsIdx := 0
	shift := 32 - bitsPerValue
	for ; shift >= 0; shift -= bitsPerValue {
		l99shiftLongs(tmp, numLongs, longs, longsIdx, shift, mask)
		longsIdx += numLongs
	}
	remainingBitsPerLong := shift + bitsPerValue
	mask32Remaining := l99masks32[remainingBitsPerLong]
	tmpIdx := 0
	remainingBits := remainingBitsPerLong
	for ; longsIdx < lucene99BlockSize/2; longsIdx++ {
		b := bitsPerValue - remainingBits
		l := (tmp[tmpIdx] & l99masks32[remainingBits]) << uint(b)
		tmpIdx++
		for b >= remainingBitsPerLong {
			b -= remainingBitsPerLong
			l |= (tmp[tmpIdx] & mask32Remaining) << uint(b)
			tmpIdx++
		}
		if b > 0 {
			l |= int64(uint64(tmp[tmpIdx])>>uint(remainingBitsPerLong-b)) & l99masks32[b]
			remainingBits = remainingBitsPerLong - b
		} else {
			remainingBits = remainingBitsPerLong
		}
		longs[longsIdx] = l
	}
	return nil
}

// ---- decode1 .. decode8 (8-bit packing) ----

// l99decode1: bpv=1. Reads 2 longs, shiftLongs × 8 into longs[0..15].
func l99decode1(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := l99readLongs(in, 2, tmp); err != nil {
		return err
	}
	l99shiftLongs(tmp, 2, longs, 0, 7, l99masks8[1])
	l99shiftLongs(tmp, 2, longs, 2, 6, l99masks8[1])
	l99shiftLongs(tmp, 2, longs, 4, 5, l99masks8[1])
	l99shiftLongs(tmp, 2, longs, 6, 4, l99masks8[1])
	l99shiftLongs(tmp, 2, longs, 8, 3, l99masks8[1])
	l99shiftLongs(tmp, 2, longs, 10, 2, l99masks8[1])
	l99shiftLongs(tmp, 2, longs, 12, 1, l99masks8[1])
	l99shiftLongs(tmp, 2, longs, 14, 0, l99masks8[1])
	return nil
}

// l99decode2: bpv=2. Reads 4 longs, shiftLongs × 4 into longs[0..15].
func l99decode2(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := l99readLongs(in, 4, tmp); err != nil {
		return err
	}
	l99shiftLongs(tmp, 4, longs, 0, 6, l99masks8[2])
	l99shiftLongs(tmp, 4, longs, 4, 4, l99masks8[2])
	l99shiftLongs(tmp, 4, longs, 8, 2, l99masks8[2])
	l99shiftLongs(tmp, 4, longs, 12, 0, l99masks8[2])
	return nil
}

// l99decode3: bpv=3. Reads 6 longs, shiftLongs × 2 + recombination.
func l99decode3(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := l99readLongs(in, 6, tmp); err != nil {
		return err
	}
	l99shiftLongs(tmp, 6, longs, 0, 5, l99masks8[3])
	l99shiftLongs(tmp, 6, longs, 6, 2, l99masks8[3])
	for iter, tmpIdx, longsIdx := 0, 0, 12; iter < 2; iter, tmpIdx, longsIdx = iter+1, tmpIdx+3, longsIdx+2 {
		l0 := (tmp[tmpIdx+0] & l99masks8[2]) << 1
		l0 |= int64(uint64(tmp[tmpIdx+1])>>1) & l99masks8[1]
		longs[longsIdx+0] = l0

		l1 := (tmp[tmpIdx+1] & l99masks8[1]) << 2
		l1 |= tmp[tmpIdx+2] & l99masks8[2]
		longs[longsIdx+1] = l1
	}
	return nil
}

// l99decode4: bpv=4. Reads 8 longs into longs, shiftLongs × 2 within longs.
func l99decode4(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := l99readLongs(in, 8, longs); err != nil {
		return err
	}
	l99shiftLongs(longs, 8, longs, 0, 4, l99masks8[4])
	l99shiftLongs(longs, 8, longs, 8, 0, l99masks8[4])
	return nil
}

// l99decode5: bpv=5. Reads 10 longs, shiftLongs × 1 + recombination.
func l99decode5(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := l99readLongs(in, 10, tmp); err != nil {
		return err
	}
	l99shiftLongs(tmp, 10, longs, 0, 3, l99masks8[5])
	for iter, tmpIdx, longsIdx := 0, 0, 10; iter < 2; iter, tmpIdx, longsIdx = iter+1, tmpIdx+5, longsIdx+3 {
		l0 := (tmp[tmpIdx+0] & l99masks8[3]) << 2
		l0 |= int64(uint64(tmp[tmpIdx+1])>>1) & l99masks8[2]
		longs[longsIdx+0] = l0

		l1 := (tmp[tmpIdx+1] & l99masks8[1]) << 4
		l1 |= (tmp[tmpIdx+2] & l99masks8[3]) << 1
		l1 |= int64(uint64(tmp[tmpIdx+3])>>2) & l99masks8[1]
		longs[longsIdx+1] = l1

		l2 := (tmp[tmpIdx+3] & l99masks8[2]) << 3
		l2 |= tmp[tmpIdx+4] & l99masks8[3]
		longs[longsIdx+2] = l2
	}
	return nil
}

// l99decode6: bpv=6. Reads 12 longs, shiftLongs × 2 + recombination.
func l99decode6(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := l99readLongs(in, 12, tmp); err != nil {
		return err
	}
	l99shiftLongs(tmp, 12, longs, 0, 2, l99masks8[6])
	l99shiftLongs(tmp, 12, tmp, 0, 0, l99masks8[2])
	for iter, tmpIdx, longsIdx := 0, 0, 12; iter < 4; iter, tmpIdx, longsIdx = iter+1, tmpIdx+3, longsIdx+1 {
		l0 := tmp[tmpIdx+0] << 4
		l0 |= tmp[tmpIdx+1] << 2
		l0 |= tmp[tmpIdx+2]
		longs[longsIdx+0] = l0
	}
	return nil
}

// l99decode7: bpv=7. Reads 14 longs, shiftLongs × 2 + recombination.
func l99decode7(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := l99readLongs(in, 14, tmp); err != nil {
		return err
	}
	l99shiftLongs(tmp, 14, longs, 0, 1, l99masks8[7])
	l99shiftLongs(tmp, 14, tmp, 0, 0, l99masks8[1])
	for iter, tmpIdx, longsIdx := 0, 0, 14; iter < 2; iter, tmpIdx, longsIdx = iter+1, tmpIdx+7, longsIdx+1 {
		l0 := tmp[tmpIdx+0] << 6
		l0 |= tmp[tmpIdx+1] << 5
		l0 |= tmp[tmpIdx+2] << 4
		l0 |= tmp[tmpIdx+3] << 3
		l0 |= tmp[tmpIdx+4] << 2
		l0 |= tmp[tmpIdx+5] << 1
		l0 |= tmp[tmpIdx+6]
		longs[longsIdx+0] = l0
	}
	return nil
}

// l99decode8: bpv=8. Reads 16 longs directly into longs (raw packed data).
func l99decode8(in store.IndexInput, tmp []int64, longs []int64) error {
	return l99readLongs(in, 16, longs)
}

// ---- decode9 .. decode16 (16-bit packing) ----

// l99decode9: bpv=9. Reads 18 longs, shiftLongs × 1 + recombination.
func l99decode9(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := l99readLongs(in, 18, tmp); err != nil {
		return err
	}
	l99shiftLongs(tmp, 18, longs, 0, 7, l99masks16[9])
	for iter, tmpIdx, longsIdx := 0, 0, 18; iter < 2; iter, tmpIdx, longsIdx = iter+1, tmpIdx+9, longsIdx+7 {
		l0 := (tmp[tmpIdx+0] & l99masks16[7]) << 2
		l0 |= int64(uint64(tmp[tmpIdx+1])>>5) & l99masks16[2]
		longs[longsIdx+0] = l0

		l1 := (tmp[tmpIdx+1] & l99masks16[5]) << 4
		l1 |= int64(uint64(tmp[tmpIdx+2])>>3) & l99masks16[4]
		longs[longsIdx+1] = l1

		l2 := (tmp[tmpIdx+2] & l99masks16[3]) << 6
		l2 |= int64(uint64(tmp[tmpIdx+3])>>1) & l99masks16[6]
		longs[longsIdx+2] = l2

		l3 := (tmp[tmpIdx+3] & l99masks16[1]) << 8
		l3 |= (tmp[tmpIdx+4] & l99masks16[7]) << 1
		l3 |= int64(uint64(tmp[tmpIdx+5])>>6) & l99masks16[1]
		longs[longsIdx+3] = l3

		l4 := (tmp[tmpIdx+5] & l99masks16[6]) << 3
		l4 |= int64(uint64(tmp[tmpIdx+6])>>4) & l99masks16[3]
		longs[longsIdx+4] = l4

		l5 := (tmp[tmpIdx+6] & l99masks16[4]) << 5
		l5 |= int64(uint64(tmp[tmpIdx+7])>>2) & l99masks16[5]
		longs[longsIdx+5] = l5

		l6 := (tmp[tmpIdx+7] & l99masks16[2]) << 7
		l6 |= tmp[tmpIdx+8] & l99masks16[7]
		longs[longsIdx+6] = l6
	}
	return nil
}

// l99decode10: bpv=10. Reads 20 longs, shiftLongs × 1 + recombination.
func l99decode10(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := l99readLongs(in, 20, tmp); err != nil {
		return err
	}
	l99shiftLongs(tmp, 20, longs, 0, 6, l99masks16[10])
	for iter, tmpIdx, longsIdx := 0, 0, 20; iter < 4; iter, tmpIdx, longsIdx = iter+1, tmpIdx+5, longsIdx+3 {
		l0 := (tmp[tmpIdx+0] & l99masks16[6]) << 4
		l0 |= int64(uint64(tmp[tmpIdx+1])>>2) & l99masks16[4]
		longs[longsIdx+0] = l0

		l1 := (tmp[tmpIdx+1] & l99masks16[2]) << 8
		l1 |= (tmp[tmpIdx+2] & l99masks16[6]) << 2
		l1 |= int64(uint64(tmp[tmpIdx+3])>>4) & l99masks16[2]
		longs[longsIdx+1] = l1

		l2 := (tmp[tmpIdx+3] & l99masks16[4]) << 6
		l2 |= tmp[tmpIdx+4] & l99masks16[6]
		longs[longsIdx+2] = l2
	}
	return nil
}

// l99decode11: bpv=11. Reads 22 longs, shiftLongs × 1 + recombination.
func l99decode11(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := l99readLongs(in, 22, tmp); err != nil {
		return err
	}
	l99shiftLongs(tmp, 22, longs, 0, 5, l99masks16[11])
	for iter, tmpIdx, longsIdx := 0, 0, 22; iter < 2; iter, tmpIdx, longsIdx = iter+1, tmpIdx+11, longsIdx+5 {
		l0 := (tmp[tmpIdx+0] & l99masks16[5]) << 6
		l0 |= (tmp[tmpIdx+1] & l99masks16[5]) << 1
		l0 |= int64(uint64(tmp[tmpIdx+2])>>4) & l99masks16[1]
		longs[longsIdx+0] = l0

		l1 := (tmp[tmpIdx+2] & l99masks16[4]) << 7
		l1 |= (tmp[tmpIdx+3] & l99masks16[5]) << 2
		l1 |= int64(uint64(tmp[tmpIdx+4])>>3) & l99masks16[2]
		longs[longsIdx+1] = l1

		l2 := (tmp[tmpIdx+4] & l99masks16[3]) << 8
		l2 |= (tmp[tmpIdx+5] & l99masks16[5]) << 3
		l2 |= int64(uint64(tmp[tmpIdx+6])>>2) & l99masks16[3]
		longs[longsIdx+2] = l2

		l3 := (tmp[tmpIdx+6] & l99masks16[2]) << 9
		l3 |= (tmp[tmpIdx+7] & l99masks16[5]) << 4
		l3 |= int64(uint64(tmp[tmpIdx+8])>>1) & l99masks16[4]
		longs[longsIdx+3] = l3

		l4 := (tmp[tmpIdx+8] & l99masks16[1]) << 10
		l4 |= (tmp[tmpIdx+9] & l99masks16[5]) << 5
		l4 |= tmp[tmpIdx+10] & l99masks16[5]
		longs[longsIdx+4] = l4
	}
	return nil
}

// l99decode12: bpv=12. Reads 24 longs, shiftLongs × 2 + recombination.
func l99decode12(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := l99readLongs(in, 24, tmp); err != nil {
		return err
	}
	l99shiftLongs(tmp, 24, longs, 0, 4, l99masks16[12])
	l99shiftLongs(tmp, 24, tmp, 0, 0, l99masks16[4])
	for iter, tmpIdx, longsIdx := 0, 0, 24; iter < 8; iter, tmpIdx, longsIdx = iter+1, tmpIdx+3, longsIdx+1 {
		l0 := tmp[tmpIdx+0] << 8
		l0 |= tmp[tmpIdx+1] << 4
		l0 |= tmp[tmpIdx+2]
		longs[longsIdx+0] = l0
	}
	return nil
}

// l99decode13: bpv=13. Reads 26 longs, shiftLongs × 1 + recombination.
func l99decode13(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := l99readLongs(in, 26, tmp); err != nil {
		return err
	}
	l99shiftLongs(tmp, 26, longs, 0, 3, l99masks16[13])
	for iter, tmpIdx, longsIdx := 0, 0, 26; iter < 2; iter, tmpIdx, longsIdx = iter+1, tmpIdx+13, longsIdx+3 {
		l0 := (tmp[tmpIdx+0] & l99masks16[3]) << 10
		l0 |= (tmp[tmpIdx+1] & l99masks16[3]) << 7
		l0 |= (tmp[tmpIdx+2] & l99masks16[3]) << 4
		l0 |= (tmp[tmpIdx+3] & l99masks16[3]) << 1
		l0 |= int64(uint64(tmp[tmpIdx+4])>>2) & l99masks16[1]
		longs[longsIdx+0] = l0

		l1 := (tmp[tmpIdx+4] & l99masks16[2]) << 11
		l1 |= (tmp[tmpIdx+5] & l99masks16[3]) << 8
		l1 |= (tmp[tmpIdx+6] & l99masks16[3]) << 5
		l1 |= (tmp[tmpIdx+7] & l99masks16[3]) << 2
		l1 |= int64(uint64(tmp[tmpIdx+8])>>1) & l99masks16[2]
		longs[longsIdx+1] = l1

		l2 := (tmp[tmpIdx+8] & l99masks16[1]) << 12
		l2 |= (tmp[tmpIdx+9] & l99masks16[3]) << 9
		l2 |= (tmp[tmpIdx+10] & l99masks16[3]) << 6
		l2 |= (tmp[tmpIdx+11] & l99masks16[3]) << 3
		l2 |= tmp[tmpIdx+12] & l99masks16[3]
		longs[longsIdx+2] = l2
	}
	return nil
}

// l99decode14: bpv=14. Reads 28 longs, shiftLongs × 2 + recombination.
func l99decode14(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := l99readLongs(in, 28, tmp); err != nil {
		return err
	}
	l99shiftLongs(tmp, 28, longs, 0, 2, l99masks16[14])
	l99shiftLongs(tmp, 28, tmp, 0, 0, l99masks16[2])
	for iter, tmpIdx, longsIdx := 0, 0, 28; iter < 4; iter, tmpIdx, longsIdx = iter+1, tmpIdx+7, longsIdx+1 {
		l0 := tmp[tmpIdx+0] << 12
		l0 |= tmp[tmpIdx+1] << 10
		l0 |= tmp[tmpIdx+2] << 8
		l0 |= tmp[tmpIdx+3] << 6
		l0 |= tmp[tmpIdx+4] << 4
		l0 |= tmp[tmpIdx+5] << 2
		l0 |= tmp[tmpIdx+6]
		longs[longsIdx+0] = l0
	}
	return nil
}

// l99decode15: bpv=15. Reads 30 longs, shiftLongs × 2 + recombination.
func l99decode15(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := l99readLongs(in, 30, tmp); err != nil {
		return err
	}
	l99shiftLongs(tmp, 30, longs, 0, 1, l99masks16[15])
	l99shiftLongs(tmp, 30, tmp, 0, 0, l99masks16[1])
	for iter, tmpIdx, longsIdx := 0, 0, 30; iter < 2; iter, tmpIdx, longsIdx = iter+1, tmpIdx+15, longsIdx+1 {
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
		longs[longsIdx+0] = l0
	}
	return nil
}

// l99decode16: bpv=16. Reads 32 longs directly into longs (raw packed data).
func l99decode16(in store.IndexInput, tmp []int64, longs []int64) error {
	return l99readLongs(in, 32, longs)
}

// ---- decode17 .. decode24 (32-bit packing) ----

// l99decode17: bpv=17. Reads 34 longs, shiftLongs × 1 + recombination.
func l99decode17(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := l99readLongs(in, 34, tmp); err != nil {
		return err
	}
	l99shiftLongs(tmp, 34, longs, 0, 15, l99masks32[17])
	for iter, tmpIdx, longsIdx := 0, 0, 34; iter < 2; iter, tmpIdx, longsIdx = iter+1, tmpIdx+17, longsIdx+15 {
		l0 := (tmp[tmpIdx+0] & l99masks32[15]) << 2
		l0 |= int64(uint64(tmp[tmpIdx+1])>>13) & l99masks32[2]
		longs[longsIdx+0] = l0

		l1 := (tmp[tmpIdx+1] & l99masks32[13]) << 4
		l1 |= int64(uint64(tmp[tmpIdx+2])>>11) & l99masks32[4]
		longs[longsIdx+1] = l1

		l2 := (tmp[tmpIdx+2] & l99masks32[11]) << 6
		l2 |= int64(uint64(tmp[tmpIdx+3])>>9) & l99masks32[6]
		longs[longsIdx+2] = l2

		l3 := (tmp[tmpIdx+3] & l99masks32[9]) << 8
		l3 |= int64(uint64(tmp[tmpIdx+4])>>7) & l99masks32[8]
		longs[longsIdx+3] = l3

		l4 := (tmp[tmpIdx+4] & l99masks32[7]) << 10
		l4 |= int64(uint64(tmp[tmpIdx+5])>>5) & l99masks32[10]
		longs[longsIdx+4] = l4

		l5 := (tmp[tmpIdx+5] & l99masks32[5]) << 12
		l5 |= int64(uint64(tmp[tmpIdx+6])>>3) & l99masks32[12]
		longs[longsIdx+5] = l5

		l6 := (tmp[tmpIdx+6] & l99masks32[3]) << 14
		l6 |= int64(uint64(tmp[tmpIdx+7])>>1) & l99masks32[14]
		longs[longsIdx+6] = l6

		l7 := (tmp[tmpIdx+7] & l99masks32[1]) << 16
		l7 |= (tmp[tmpIdx+8] & l99masks32[15]) << 1
		l7 |= int64(uint64(tmp[tmpIdx+9])>>14) & l99masks32[1]
		longs[longsIdx+7] = l7

		l8 := (tmp[tmpIdx+9] & l99masks32[14]) << 3
		l8 |= int64(uint64(tmp[tmpIdx+10])>>12) & l99masks32[3]
		longs[longsIdx+8] = l8

		l9 := (tmp[tmpIdx+10] & l99masks32[12]) << 5
		l9 |= int64(uint64(tmp[tmpIdx+11])>>10) & l99masks32[5]
		longs[longsIdx+9] = l9

		l10 := (tmp[tmpIdx+11] & l99masks32[10]) << 7
		l10 |= int64(uint64(tmp[tmpIdx+12])>>8) & l99masks32[7]
		longs[longsIdx+10] = l10

		l11 := (tmp[tmpIdx+12] & l99masks32[8]) << 9
		l11 |= int64(uint64(tmp[tmpIdx+13])>>6) & l99masks32[9]
		longs[longsIdx+11] = l11

		l12 := (tmp[tmpIdx+13] & l99masks32[6]) << 11
		l12 |= int64(uint64(tmp[tmpIdx+14])>>4) & l99masks32[11]
		longs[longsIdx+12] = l12

		l13 := (tmp[tmpIdx+14] & l99masks32[4]) << 13
		l13 |= int64(uint64(tmp[tmpIdx+15])>>2) & l99masks32[13]
		longs[longsIdx+13] = l13

		l14 := (tmp[tmpIdx+15] & l99masks32[2]) << 15
		l14 |= tmp[tmpIdx+16] & l99masks32[15]
		longs[longsIdx+14] = l14
	}
	return nil
}

// l99decode18: bpv=18. Reads 36 longs, shiftLongs × 1 + recombination.
func l99decode18(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := l99readLongs(in, 36, tmp); err != nil {
		return err
	}
	l99shiftLongs(tmp, 36, longs, 0, 14, l99masks32[18])
	for iter, tmpIdx, longsIdx := 0, 0, 36; iter < 4; iter, tmpIdx, longsIdx = iter+1, tmpIdx+9, longsIdx+7 {
		l0 := (tmp[tmpIdx+0] & l99masks32[14]) << 4
		l0 |= int64(uint64(tmp[tmpIdx+1])>>10) & l99masks32[4]
		longs[longsIdx+0] = l0

		l1 := (tmp[tmpIdx+1] & l99masks32[10]) << 8
		l1 |= int64(uint64(tmp[tmpIdx+2])>>6) & l99masks32[8]
		longs[longsIdx+1] = l1

		l2 := (tmp[tmpIdx+2] & l99masks32[6]) << 12
		l2 |= int64(uint64(tmp[tmpIdx+3])>>2) & l99masks32[12]
		longs[longsIdx+2] = l2

		l3 := (tmp[tmpIdx+3] & l99masks32[2]) << 16
		l3 |= (tmp[tmpIdx+4] & l99masks32[14]) << 2
		l3 |= int64(uint64(tmp[tmpIdx+5])>>12) & l99masks32[2]
		longs[longsIdx+3] = l3

		l4 := (tmp[tmpIdx+5] & l99masks32[12]) << 6
		l4 |= int64(uint64(tmp[tmpIdx+6])>>8) & l99masks32[6]
		longs[longsIdx+4] = l4

		l5 := (tmp[tmpIdx+6] & l99masks32[8]) << 10
		l5 |= int64(uint64(tmp[tmpIdx+7])>>4) & l99masks32[10]
		longs[longsIdx+5] = l5

		l6 := (tmp[tmpIdx+7] & l99masks32[4]) << 14
		l6 |= tmp[tmpIdx+8] & l99masks32[14]
		longs[longsIdx+6] = l6
	}
	return nil
}

// l99decode19: bpv=19. Reads 38 longs, shiftLongs × 1 + recombination.
func l99decode19(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := l99readLongs(in, 38, tmp); err != nil {
		return err
	}
	l99shiftLongs(tmp, 38, longs, 0, 13, l99masks32[19])
	for iter, tmpIdx, longsIdx := 0, 0, 38; iter < 2; iter, tmpIdx, longsIdx = iter+1, tmpIdx+19, longsIdx+13 {
		l0 := (tmp[tmpIdx+0] & l99masks32[13]) << 6
		l0 |= int64(uint64(tmp[tmpIdx+1])>>7) & l99masks32[6]
		longs[longsIdx+0] = l0

		l1 := (tmp[tmpIdx+1] & l99masks32[7]) << 12
		l1 |= int64(uint64(tmp[tmpIdx+2])>>1) & l99masks32[12]
		longs[longsIdx+1] = l1

		l2 := (tmp[tmpIdx+2] & l99masks32[1]) << 18
		l2 |= (tmp[tmpIdx+3] & l99masks32[13]) << 5
		l2 |= int64(uint64(tmp[tmpIdx+4])>>8) & l99masks32[5]
		longs[longsIdx+2] = l2

		l3 := (tmp[tmpIdx+4] & l99masks32[8]) << 11
		l3 |= int64(uint64(tmp[tmpIdx+5])>>2) & l99masks32[11]
		longs[longsIdx+3] = l3

		l4 := (tmp[tmpIdx+5] & l99masks32[2]) << 17
		l4 |= (tmp[tmpIdx+6] & l99masks32[13]) << 4
		l4 |= int64(uint64(tmp[tmpIdx+7])>>9) & l99masks32[4]
		longs[longsIdx+4] = l4

		l5 := (tmp[tmpIdx+7] & l99masks32[9]) << 10
		l5 |= int64(uint64(tmp[tmpIdx+8])>>3) & l99masks32[10]
		longs[longsIdx+5] = l5

		l6 := (tmp[tmpIdx+8] & l99masks32[3]) << 16
		l6 |= (tmp[tmpIdx+9] & l99masks32[13]) << 3
		l6 |= int64(uint64(tmp[tmpIdx+10])>>10) & l99masks32[3]
		longs[longsIdx+6] = l6

		l7 := (tmp[tmpIdx+10] & l99masks32[10]) << 9
		l7 |= int64(uint64(tmp[tmpIdx+11])>>4) & l99masks32[9]
		longs[longsIdx+7] = l7

		l8 := (tmp[tmpIdx+11] & l99masks32[4]) << 15
		l8 |= (tmp[tmpIdx+12] & l99masks32[13]) << 2
		l8 |= int64(uint64(tmp[tmpIdx+13])>>11) & l99masks32[2]
		longs[longsIdx+8] = l8

		l9 := (tmp[tmpIdx+13] & l99masks32[11]) << 8
		l9 |= int64(uint64(tmp[tmpIdx+14])>>5) & l99masks32[8]
		longs[longsIdx+9] = l9

		l10 := (tmp[tmpIdx+14] & l99masks32[5]) << 14
		l10 |= (tmp[tmpIdx+15] & l99masks32[13]) << 1
		l10 |= int64(uint64(tmp[tmpIdx+16])>>12) & l99masks32[1]
		longs[longsIdx+10] = l10

		l11 := (tmp[tmpIdx+16] & l99masks32[12]) << 7
		l11 |= int64(uint64(tmp[tmpIdx+17])>>6) & l99masks32[7]
		longs[longsIdx+11] = l11

		l12 := (tmp[tmpIdx+17] & l99masks32[6]) << 13
		l12 |= tmp[tmpIdx+18] & l99masks32[13]
		longs[longsIdx+12] = l12
	}
	return nil
}

// l99decode20: bpv=20. Reads 40 longs, shiftLongs × 1 + recombination.
func l99decode20(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := l99readLongs(in, 40, tmp); err != nil {
		return err
	}
	l99shiftLongs(tmp, 40, longs, 0, 12, l99masks32[20])
	for iter, tmpIdx, longsIdx := 0, 0, 40; iter < 8; iter, tmpIdx, longsIdx = iter+1, tmpIdx+5, longsIdx+3 {
		l0 := (tmp[tmpIdx+0] & l99masks32[12]) << 8
		l0 |= int64(uint64(tmp[tmpIdx+1])>>4) & l99masks32[8]
		longs[longsIdx+0] = l0

		l1 := (tmp[tmpIdx+1] & l99masks32[4]) << 16
		l1 |= (tmp[tmpIdx+2] & l99masks32[12]) << 4
		l1 |= int64(uint64(tmp[tmpIdx+3])>>8) & l99masks32[4]
		longs[longsIdx+1] = l1

		l2 := (tmp[tmpIdx+3] & l99masks32[8]) << 12
		l2 |= tmp[tmpIdx+4] & l99masks32[12]
		longs[longsIdx+2] = l2
	}
	return nil
}

// l99decode21: bpv=21. Reads 42 longs, shiftLongs × 1 + recombination.
func l99decode21(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := l99readLongs(in, 42, tmp); err != nil {
		return err
	}
	l99shiftLongs(tmp, 42, longs, 0, 11, l99masks32[21])
	for iter, tmpIdx, longsIdx := 0, 0, 42; iter < 2; iter, tmpIdx, longsIdx = iter+1, tmpIdx+21, longsIdx+11 {
		l0 := (tmp[tmpIdx+0] & l99masks32[11]) << 10
		l0 |= int64(uint64(tmp[tmpIdx+1])>>1) & l99masks32[10]
		longs[longsIdx+0] = l0

		l1 := (tmp[tmpIdx+1] & l99masks32[1]) << 20
		l1 |= (tmp[tmpIdx+2] & l99masks32[11]) << 9
		l1 |= int64(uint64(tmp[tmpIdx+3])>>2) & l99masks32[9]
		longs[longsIdx+1] = l1

		l2 := (tmp[tmpIdx+3] & l99masks32[2]) << 19
		l2 |= (tmp[tmpIdx+4] & l99masks32[11]) << 8
		l2 |= int64(uint64(tmp[tmpIdx+5])>>3) & l99masks32[8]
		longs[longsIdx+2] = l2

		l3 := (tmp[tmpIdx+5] & l99masks32[3]) << 18
		l3 |= (tmp[tmpIdx+6] & l99masks32[11]) << 7
		l3 |= int64(uint64(tmp[tmpIdx+7])>>4) & l99masks32[7]
		longs[longsIdx+3] = l3

		l4 := (tmp[tmpIdx+7] & l99masks32[4]) << 17
		l4 |= (tmp[tmpIdx+8] & l99masks32[11]) << 6
		l4 |= int64(uint64(tmp[tmpIdx+9])>>5) & l99masks32[6]
		longs[longsIdx+4] = l4

		l5 := (tmp[tmpIdx+9] & l99masks32[5]) << 16
		l5 |= (tmp[tmpIdx+10] & l99masks32[11]) << 5
		l5 |= int64(uint64(tmp[tmpIdx+11])>>6) & l99masks32[5]
		longs[longsIdx+5] = l5

		l6 := (tmp[tmpIdx+11] & l99masks32[6]) << 15
		l6 |= (tmp[tmpIdx+12] & l99masks32[11]) << 4
		l6 |= int64(uint64(tmp[tmpIdx+13])>>7) & l99masks32[4]
		longs[longsIdx+6] = l6

		l7 := (tmp[tmpIdx+13] & l99masks32[7]) << 14
		l7 |= (tmp[tmpIdx+14] & l99masks32[11]) << 3
		l7 |= int64(uint64(tmp[tmpIdx+15])>>8) & l99masks32[3]
		longs[longsIdx+7] = l7

		l8 := (tmp[tmpIdx+15] & l99masks32[8]) << 13
		l8 |= (tmp[tmpIdx+16] & l99masks32[11]) << 2
		l8 |= int64(uint64(tmp[tmpIdx+17])>>9) & l99masks32[2]
		longs[longsIdx+8] = l8

		l9 := (tmp[tmpIdx+17] & l99masks32[9]) << 12
		l9 |= (tmp[tmpIdx+18] & l99masks32[11]) << 1
		l9 |= int64(uint64(tmp[tmpIdx+19])>>10) & l99masks32[1]
		longs[longsIdx+9] = l9

		l10 := (tmp[tmpIdx+19] & l99masks32[10]) << 11
		l10 |= tmp[tmpIdx+20] & l99masks32[11]
		longs[longsIdx+10] = l10
	}
	return nil
}

// l99decode22: bpv=22. Reads 44 longs, shiftLongs × 1 + recombination.
func l99decode22(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := l99readLongs(in, 44, tmp); err != nil {
		return err
	}
	l99shiftLongs(tmp, 44, longs, 0, 10, l99masks32[22])
	for iter, tmpIdx, longsIdx := 0, 0, 44; iter < 4; iter, tmpIdx, longsIdx = iter+1, tmpIdx+11, longsIdx+5 {
		l0 := (tmp[tmpIdx+0] & l99masks32[10]) << 12
		l0 |= (tmp[tmpIdx+1] & l99masks32[10]) << 2
		l0 |= int64(uint64(tmp[tmpIdx+2])>>8) & l99masks32[2]
		longs[longsIdx+0] = l0

		l1 := (tmp[tmpIdx+2] & l99masks32[8]) << 14
		l1 |= (tmp[tmpIdx+3] & l99masks32[10]) << 4
		l1 |= int64(uint64(tmp[tmpIdx+4])>>6) & l99masks32[4]
		longs[longsIdx+1] = l1

		l2 := (tmp[tmpIdx+4] & l99masks32[6]) << 16
		l2 |= (tmp[tmpIdx+5] & l99masks32[10]) << 6
		l2 |= int64(uint64(tmp[tmpIdx+6])>>4) & l99masks32[6]
		longs[longsIdx+2] = l2

		l3 := (tmp[tmpIdx+6] & l99masks32[4]) << 18
		l3 |= (tmp[tmpIdx+7] & l99masks32[10]) << 8
		l3 |= int64(uint64(tmp[tmpIdx+8])>>2) & l99masks32[8]
		longs[longsIdx+3] = l3

		l4 := (tmp[tmpIdx+8] & l99masks32[2]) << 20
		l4 |= (tmp[tmpIdx+9] & l99masks32[10]) << 10
		l4 |= tmp[tmpIdx+10] & l99masks32[10]
		longs[longsIdx+4] = l4
	}
	return nil
}

// l99decode23: bpv=23. Reads 46 longs, shiftLongs × 1 + recombination.
func l99decode23(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := l99readLongs(in, 46, tmp); err != nil {
		return err
	}
	l99shiftLongs(tmp, 46, longs, 0, 9, l99masks32[23])
	for iter, tmpIdx, longsIdx := 0, 0, 46; iter < 2; iter, tmpIdx, longsIdx = iter+1, tmpIdx+23, longsIdx+9 {
		l0 := (tmp[tmpIdx+0] & l99masks32[9]) << 14
		l0 |= (tmp[tmpIdx+1] & l99masks32[9]) << 5
		l0 |= int64(uint64(tmp[tmpIdx+2])>>4) & l99masks32[5]
		longs[longsIdx+0] = l0

		l1 := (tmp[tmpIdx+2] & l99masks32[4]) << 19
		l1 |= (tmp[tmpIdx+3] & l99masks32[9]) << 10
		l1 |= (tmp[tmpIdx+4] & l99masks32[9]) << 1
		l1 |= int64(uint64(tmp[tmpIdx+5])>>8) & l99masks32[1]
		longs[longsIdx+1] = l1

		l2 := (tmp[tmpIdx+5] & l99masks32[8]) << 15
		l2 |= (tmp[tmpIdx+6] & l99masks32[9]) << 6
		l2 |= int64(uint64(tmp[tmpIdx+7])>>3) & l99masks32[6]
		longs[longsIdx+2] = l2

		l3 := (tmp[tmpIdx+7] & l99masks32[3]) << 20
		l3 |= (tmp[tmpIdx+8] & l99masks32[9]) << 11
		l3 |= (tmp[tmpIdx+9] & l99masks32[9]) << 2
		l3 |= int64(uint64(tmp[tmpIdx+10])>>7) & l99masks32[2]
		longs[longsIdx+3] = l3

		l4 := (tmp[tmpIdx+10] & l99masks32[7]) << 16
		l4 |= (tmp[tmpIdx+11] & l99masks32[9]) << 7
		l4 |= int64(uint64(tmp[tmpIdx+12])>>2) & l99masks32[7]
		longs[longsIdx+4] = l4

		l5 := (tmp[tmpIdx+12] & l99masks32[2]) << 21
		l5 |= (tmp[tmpIdx+13] & l99masks32[9]) << 12
		l5 |= (tmp[tmpIdx+14] & l99masks32[9]) << 3
		l5 |= int64(uint64(tmp[tmpIdx+15])>>6) & l99masks32[3]
		longs[longsIdx+5] = l5

		l6 := (tmp[tmpIdx+15] & l99masks32[6]) << 17
		l6 |= (tmp[tmpIdx+16] & l99masks32[9]) << 8
		l6 |= int64(uint64(tmp[tmpIdx+17])>>1) & l99masks32[8]
		longs[longsIdx+6] = l6

		l7 := (tmp[tmpIdx+17] & l99masks32[1]) << 22
		l7 |= (tmp[tmpIdx+18] & l99masks32[9]) << 13
		l7 |= (tmp[tmpIdx+19] & l99masks32[9]) << 4
		l7 |= int64(uint64(tmp[tmpIdx+20])>>5) & l99masks32[4]
		longs[longsIdx+7] = l7

		l8 := (tmp[tmpIdx+20] & l99masks32[5]) << 18
		l8 |= (tmp[tmpIdx+21] & l99masks32[9]) << 9
		l8 |= tmp[tmpIdx+22] & l99masks32[9]
		longs[longsIdx+8] = l8
	}
	return nil
}

// l99decode24: bpv=24. Reads 48 longs, shiftLongs × 2 + recombination.
func l99decode24(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := l99readLongs(in, 48, tmp); err != nil {
		return err
	}
	l99shiftLongs(tmp, 48, longs, 0, 8, l99masks32[24])
	l99shiftLongs(tmp, 48, tmp, 0, 0, l99masks32[8])
	for iter, tmpIdx, longsIdx := 0, 0, 48; iter < 16; iter, tmpIdx, longsIdx = iter+1, tmpIdx+3, longsIdx+1 {
		l0 := tmp[tmpIdx+0] << 16
		l0 |= tmp[tmpIdx+1] << 8
		l0 |= tmp[tmpIdx+2]
		longs[longsIdx+0] = l0
	}
	return nil
}
