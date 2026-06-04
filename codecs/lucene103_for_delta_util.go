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
//	lucene103/ForDeltaUtil.java
//
// Purpose: 128-wide FOR-delta encode/decode for the read-only Lucene 10.3
// postings format. decodeAndPrefixSum fuses the FOR decode of doc-delta
// blocks with the prefix-sum that turns deltas back into absolute doc IDs.

package codecs

import (
	"errors"
	"fmt"
	"math/bits"

	"github.com/FlavioCFOliveira/Gocene/store"
)

const (
	l103HalfBlockSize         = lucene103BlockSizeConst / 2     // 64
	l103OneBlockSizeFourth    = lucene103BlockSizeConst / 4     // 32
	l103TwoBlockSizeFourths   = lucene103BlockSizeConst / 2     // 64
	l103ThreeBlockSizeFourths = 3 * lucene103BlockSizeConst / 4 // 96
)

// lucene103ForDeltaUtil provides 128-wide FOR-delta encode/decode.
// Mirrors backward_codecs.lucene103.ForDeltaUtil.
type lucene103ForDeltaUtil struct {
	forUtil *lucene103ForUtil
	// tmp is the scratch buffer used by the To32 / To16 recombination loops.
	tmp []int32
}

// newLucene103ForDeltaUtil allocates the delta-coder and its scratch buffer.
func newLucene103ForDeltaUtil() *lucene103ForDeltaUtil {
	return &lucene103ForDeltaUtil{
		forUtil: newLucene103ForUtil(),
		tmp:     make([]int32, lucene103BlockSizeConst),
	}
}

// ---------- prefix-sum fan-out helpers ----------

// l103prefixSumScalar computes the in-place running sum of arr[0:len] from base.
// Mirrors ForDeltaUtil.prefixSum(int[], int, int).
func l103prefixSumScalar(arr []int32, length int, base int32) {
	sum := base
	for i := 0; i < length; i++ {
		sum += arr[i]
		arr[i] = sum
	}
}

// l103prefixSum8 is the 4-lane prefix-sum used when bitsPerValue <= 3.
// Mirrors ForDeltaUtil.prefixSum8.
func l103prefixSum8(arr []int32, base int32) {
	l103prefixSumScalar(arr, l103OneBlockSizeFourth, 0)
	l103expand8(arr)
	l0 := base
	l1 := l0 + arr[l103OneBlockSizeFourth-1]
	l2 := l1 + arr[l103TwoBlockSizeFourths-1]
	l3 := l2 + arr[l103ThreeBlockSizeFourths-1]
	for i := 0; i < l103OneBlockSizeFourth; i++ {
		arr[i] += l0
		arr[l103OneBlockSizeFourth+i] += l1
		arr[l103TwoBlockSizeFourths+i] += l2
		arr[l103ThreeBlockSizeFourths+i] += l3
	}
}

// l103prefixSum16 is the 2-lane prefix-sum used when bitsPerValue <= 10.
// Mirrors ForDeltaUtil.prefixSum16.
func l103prefixSum16(arr []int32, base int32) {
	l103prefixSumScalar(arr, l103HalfBlockSize, 0)
	l103expand16(arr)
	l0 := base
	l1 := base + arr[l103HalfBlockSize-1]
	for i := 0; i < l103HalfBlockSize; i++ {
		arr[i] += l0
		arr[l103HalfBlockSize+i] += l1
	}
}

// l103prefixSum32 is the scalar prefix-sum used when bitsPerValue >= 11.
// Mirrors ForDeltaUtil.prefixSum32.
func l103prefixSum32(arr []int32, base int32) {
	l103prefixSumScalar(arr, lucene103BlockSizeConst, base)
}

// ---------- bitsRequired / encode ----------

// bitsRequired returns the number of bits needed to store the strictly positive
// deltas in ints. Mirrors ForDeltaUtil.bitsRequired.
func (f *lucene103ForDeltaUtil) bitsRequired(ints []int32) (int, error) {
	var or int32
	for _, l := range ints[:lucene103BlockSizeConst] {
		or |= l
	}
	if or == 0 {
		// Deltas should be strictly positive (consecutive doc IDs differ by >= 1).
		return 0, errors.New("lucene103 ForDeltaUtil: zero OR across deltas (delta must be > 0)")
	}
	return bits.Len32(uint32(or)), nil
}

// encodeDeltas encodes the strictly-monotonic delta sequence in ints with
// bitsPerValue bits per value. Mirrors ForDeltaUtil.encodeDeltas.
func (f *lucene103ForDeltaUtil) encodeDeltas(bitsPerValue int, ints []int32, out store.IndexOutput) error {
	if len(ints) < lucene103BlockSizeConst {
		return errors.New("lucene103 ForDeltaUtil.encodeDeltas: ints must have at least 128 elements")
	}
	var primitiveSize int
	if bitsPerValue <= 3 {
		primitiveSize = 8
		l103collapse8(ints)
	} else if bitsPerValue <= 10 {
		primitiveSize = 16
		l103collapse16(ints)
	} else {
		primitiveSize = 32
	}
	return l103encodeInternal(ints, bitsPerValue, primitiveSize, out)
}

// ---------- decodeAndPrefixSum ----------

// decodeAndPrefixSum delta-decodes 128 integers into ints, fusing the FOR
// decode with the prefix-sum so that ints holds absolute doc IDs on return.
// Mirrors ForDeltaUtil.decodeAndPrefixSum.
func (f *lucene103ForDeltaUtil) decodeAndPrefixSum(bitsPerValue int, in store.IndexInput, base int32, ints []int64) error {
	if len(ints) < lucene103BlockSizeConst {
		return errors.New("lucene103 ForDeltaUtil.decodeAndPrefixSum: ints must have at least 128 elements")
	}
	buf := f.forUtil.ints32 // primary collapsed buffer
	tmp := f.tmp

	var err error
	switch bitsPerValue {
	case 1:
		err = l103decode1(in, buf)
		if err == nil {
			l103prefixSum8(buf, base)
		}
	case 2:
		err = l103decode2(in, buf)
		if err == nil {
			l103prefixSum8(buf, base)
		}
	case 3:
		err = l103decode3(in, buf, tmp)
		if err == nil {
			l103prefixSum8(buf, base)
		}
	case 4:
		err = l103decode4To16(in, buf)
		if err == nil {
			l103prefixSum16(buf, base)
		}
	case 5:
		err = l103decode5To16(in, buf, tmp)
		if err == nil {
			l103prefixSum16(buf, base)
		}
	case 6:
		err = l103decode6To16(in, buf, tmp)
		if err == nil {
			l103prefixSum16(buf, base)
		}
	case 7:
		err = l103decode7To16(in, buf, tmp)
		if err == nil {
			l103prefixSum16(buf, base)
		}
	case 8:
		err = l103decode8To16(in, buf)
		if err == nil {
			l103prefixSum16(buf, base)
		}
	case 9:
		err = l103decode9(in, buf, tmp)
		if err == nil {
			l103prefixSum16(buf, base)
		}
	case 10:
		err = l103decode10(in, buf, tmp)
		if err == nil {
			l103prefixSum16(buf, base)
		}
	case 11:
		err = l103decode11To32(in, buf, tmp)
		if err == nil {
			l103prefixSum32(buf, base)
		}
	case 12:
		err = l103decode12To32(in, buf, tmp)
		if err == nil {
			l103prefixSum32(buf, base)
		}
	case 13:
		err = l103decode13To32(in, buf, tmp)
		if err == nil {
			l103prefixSum32(buf, base)
		}
	case 14:
		err = l103decode14To32(in, buf, tmp)
		if err == nil {
			l103prefixSum32(buf, base)
		}
	case 15:
		err = l103decode15To32(in, buf, tmp)
		if err == nil {
			l103prefixSum32(buf, base)
		}
	case 16:
		err = l103decode16To32(in, buf)
		if err == nil {
			l103prefixSum32(buf, base)
		}
	default:
		if bitsPerValue < 1 || bitsPerValue > 32 {
			return fmt.Errorf("lucene103 ForDeltaUtil: illegal number of bits per value: %d", bitsPerValue)
		}
		err = l103decodeSlow(bitsPerValue, in, buf)
		if err == nil {
			l103prefixSum32(buf, base)
		}
	}
	if err != nil {
		return err
	}

	for i := 0; i < lucene103BlockSizeConst; i++ {
		ints[i] = int64(uint32(buf[i]))
	}
	return nil
}

// ---------- specialised decode-to-wider-primitive functions ----------
//
// These differ from the plain decode* functions in for_util / lucene103_for_util:
// they decode directly into the wider (16-bit or 32-bit) primitive layout that
// the prefix-sum fan-out (prefixSum16 / prefixSum32) then consumes. Direct
// transcription of backward_codecs.lucene103.ForDeltaUtil.

func l103decode4To16(in store.IndexInput, ints []int32) error {
	// splitInts(16, ints, 12, 4, MASK16_4, ints, 48, MASK16_4)
	return l103splitInts(in, 16, ints, 12, 4, masks16[4], ints, 48, masks16[4])
}

func l103decode5To16(in store.IndexInput, ints []int32, tmp []int32) error {
	// splitInts(20, ints, 11, 5, MASK16_5, tmp, 0, MASK16_1)
	if err := l103splitInts(in, 20, ints, 11, 5, masks16[5], tmp, 0, masks16[1]); err != nil {
		return err
	}
	for iter, tmpIdx, intsIdx := 0, 0, 60; iter < 4; iter, tmpIdx, intsIdx = iter+1, tmpIdx+5, intsIdx+1 {
		l0 := tmp[tmpIdx+0] << 4
		l0 |= tmp[tmpIdx+1] << 3
		l0 |= tmp[tmpIdx+2] << 2
		l0 |= tmp[tmpIdx+3] << 1
		l0 |= tmp[tmpIdx+4]
		ints[intsIdx+0] = l0
	}
	return nil
}

func l103decode6To16(in store.IndexInput, ints []int32, tmp []int32) error {
	// splitInts(24, ints, 10, 6, MASK16_6, tmp, 0, MASK16_4)
	if err := l103splitInts(in, 24, ints, 10, 6, masks16[6], tmp, 0, masks16[4]); err != nil {
		return err
	}
	for iter, tmpIdx, intsIdx := 0, 0, 48; iter < 8; iter, tmpIdx, intsIdx = iter+1, tmpIdx+3, intsIdx+2 {
		l0 := tmp[tmpIdx+0] << 2
		l0 |= int32(uint32(tmp[tmpIdx+1])>>2) & masks16[2]
		ints[intsIdx+0] = l0
		l1 := (tmp[tmpIdx+1] & masks16[2]) << 4
		l1 |= tmp[tmpIdx+2]
		ints[intsIdx+1] = l1
	}
	return nil
}

func l103decode7To16(in store.IndexInput, ints []int32, tmp []int32) error {
	// splitInts(28, ints, 9, 7, MASK16_7, tmp, 0, MASK16_2)
	if err := l103splitInts(in, 28, ints, 9, 7, masks16[7], tmp, 0, masks16[2]); err != nil {
		return err
	}
	for iter, tmpIdx, intsIdx := 0, 0, 56; iter < 4; iter, tmpIdx, intsIdx = iter+1, tmpIdx+7, intsIdx+2 {
		l0 := tmp[tmpIdx+0] << 5
		l0 |= tmp[tmpIdx+1] << 3
		l0 |= tmp[tmpIdx+2] << 1
		l0 |= int32(uint32(tmp[tmpIdx+3])>>1) & masks16[1]
		ints[intsIdx+0] = l0
		l1 := (tmp[tmpIdx+3] & masks16[1]) << 6
		l1 |= tmp[tmpIdx+4] << 4
		l1 |= tmp[tmpIdx+5] << 2
		l1 |= tmp[tmpIdx+6]
		ints[intsIdx+1] = l1
	}
	return nil
}

func l103decode8To16(in store.IndexInput, ints []int32) error {
	// splitInts(32, ints, 8, 8, MASK16_8, ints, 32, MASK16_8)
	return l103splitInts(in, 32, ints, 8, 8, masks16[8], ints, 32, masks16[8])
}

func l103decode11To32(in store.IndexInput, ints []int32, tmp []int32) error {
	// splitInts(44, ints, 21, 11, MASK32_11, tmp, 0, MASK32_10)
	if err := l103splitInts(in, 44, ints, 21, 11, masks32[11], tmp, 0, masks32[10]); err != nil {
		return err
	}
	for iter, tmpIdx, intsIdx := 0, 0, 88; iter < 4; iter, tmpIdx, intsIdx = iter+1, tmpIdx+11, intsIdx+10 {
		l0 := tmp[tmpIdx+0] << 1
		l0 |= int32(uint32(tmp[tmpIdx+1])>>9) & masks32[1]
		ints[intsIdx+0] = l0
		l1 := (tmp[tmpIdx+1] & masks32[9]) << 2
		l1 |= int32(uint32(tmp[tmpIdx+2])>>8) & masks32[2]
		ints[intsIdx+1] = l1
		l2 := (tmp[tmpIdx+2] & masks32[8]) << 3
		l2 |= int32(uint32(tmp[tmpIdx+3])>>7) & masks32[3]
		ints[intsIdx+2] = l2
		l3 := (tmp[tmpIdx+3] & masks32[7]) << 4
		l3 |= int32(uint32(tmp[tmpIdx+4])>>6) & masks32[4]
		ints[intsIdx+3] = l3
		l4 := (tmp[tmpIdx+4] & masks32[6]) << 5
		l4 |= int32(uint32(tmp[tmpIdx+5])>>5) & masks32[5]
		ints[intsIdx+4] = l4
		l5 := (tmp[tmpIdx+5] & masks32[5]) << 6
		l5 |= int32(uint32(tmp[tmpIdx+6])>>4) & masks32[6]
		ints[intsIdx+5] = l5
		l6 := (tmp[tmpIdx+6] & masks32[4]) << 7
		l6 |= int32(uint32(tmp[tmpIdx+7])>>3) & masks32[7]
		ints[intsIdx+6] = l6
		l7 := (tmp[tmpIdx+7] & masks32[3]) << 8
		l7 |= int32(uint32(tmp[tmpIdx+8])>>2) & masks32[8]
		ints[intsIdx+7] = l7
		l8 := (tmp[tmpIdx+8] & masks32[2]) << 9
		l8 |= int32(uint32(tmp[tmpIdx+9])>>1) & masks32[9]
		ints[intsIdx+8] = l8
		l9 := (tmp[tmpIdx+9] & masks32[1]) << 10
		l9 |= tmp[tmpIdx+10]
		ints[intsIdx+9] = l9
	}
	return nil
}

func l103decode12To32(in store.IndexInput, ints []int32, tmp []int32) error {
	// splitInts(48, ints, 20, 12, MASK32_12, tmp, 0, MASK32_8)
	if err := l103splitInts(in, 48, ints, 20, 12, masks32[12], tmp, 0, masks32[8]); err != nil {
		return err
	}
	for iter, tmpIdx, intsIdx := 0, 0, 96; iter < 16; iter, tmpIdx, intsIdx = iter+1, tmpIdx+3, intsIdx+2 {
		l0 := tmp[tmpIdx+0] << 4
		l0 |= int32(uint32(tmp[tmpIdx+1])>>4) & masks32[4]
		ints[intsIdx+0] = l0
		l1 := (tmp[tmpIdx+1] & masks32[4]) << 8
		l1 |= tmp[tmpIdx+2]
		ints[intsIdx+1] = l1
	}
	return nil
}

func l103decode13To32(in store.IndexInput, ints []int32, tmp []int32) error {
	// splitInts(52, ints, 19, 13, MASK32_13, tmp, 0, MASK32_6)
	if err := l103splitInts(in, 52, ints, 19, 13, masks32[13], tmp, 0, masks32[6]); err != nil {
		return err
	}
	for iter, tmpIdx, intsIdx := 0, 0, 104; iter < 4; iter, tmpIdx, intsIdx = iter+1, tmpIdx+13, intsIdx+6 {
		l0 := tmp[tmpIdx+0] << 7
		l0 |= tmp[tmpIdx+1] << 1
		l0 |= int32(uint32(tmp[tmpIdx+2])>>5) & masks32[1]
		ints[intsIdx+0] = l0
		l1 := (tmp[tmpIdx+2] & masks32[5]) << 8
		l1 |= tmp[tmpIdx+3] << 2
		l1 |= int32(uint32(tmp[tmpIdx+4])>>4) & masks32[2]
		ints[intsIdx+1] = l1
		l2 := (tmp[tmpIdx+4] & masks32[4]) << 9
		l2 |= tmp[tmpIdx+5] << 3
		l2 |= int32(uint32(tmp[tmpIdx+6])>>3) & masks32[3]
		ints[intsIdx+2] = l2
		l3 := (tmp[tmpIdx+6] & masks32[3]) << 10
		l3 |= tmp[tmpIdx+7] << 4
		l3 |= int32(uint32(tmp[tmpIdx+8])>>2) & masks32[4]
		ints[intsIdx+3] = l3
		l4 := (tmp[tmpIdx+8] & masks32[2]) << 11
		l4 |= tmp[tmpIdx+9] << 5
		l4 |= int32(uint32(tmp[tmpIdx+10])>>1) & masks32[5]
		ints[intsIdx+4] = l4
		l5 := (tmp[tmpIdx+10] & masks32[1]) << 12
		l5 |= tmp[tmpIdx+11] << 6
		l5 |= tmp[tmpIdx+12]
		ints[intsIdx+5] = l5
	}
	return nil
}

func l103decode14To32(in store.IndexInput, ints []int32, tmp []int32) error {
	// splitInts(56, ints, 18, 14, MASK32_14, tmp, 0, MASK32_4)
	if err := l103splitInts(in, 56, ints, 18, 14, masks32[14], tmp, 0, masks32[4]); err != nil {
		return err
	}
	for iter, tmpIdx, intsIdx := 0, 0, 112; iter < 8; iter, tmpIdx, intsIdx = iter+1, tmpIdx+7, intsIdx+2 {
		l0 := tmp[tmpIdx+0] << 10
		l0 |= tmp[tmpIdx+1] << 6
		l0 |= tmp[tmpIdx+2] << 2
		l0 |= int32(uint32(tmp[tmpIdx+3])>>2) & masks32[2]
		ints[intsIdx+0] = l0
		l1 := (tmp[tmpIdx+3] & masks32[2]) << 12
		l1 |= tmp[tmpIdx+4] << 8
		l1 |= tmp[tmpIdx+5] << 4
		l1 |= tmp[tmpIdx+6]
		ints[intsIdx+1] = l1
	}
	return nil
}

func l103decode15To32(in store.IndexInput, ints []int32, tmp []int32) error {
	// splitInts(60, ints, 17, 15, MASK32_15, tmp, 0, MASK32_2)
	if err := l103splitInts(in, 60, ints, 17, 15, masks32[15], tmp, 0, masks32[2]); err != nil {
		return err
	}
	for iter, tmpIdx, intsIdx := 0, 0, 120; iter < 4; iter, tmpIdx, intsIdx = iter+1, tmpIdx+15, intsIdx+2 {
		l0 := tmp[tmpIdx+0] << 13
		l0 |= tmp[tmpIdx+1] << 11
		l0 |= tmp[tmpIdx+2] << 9
		l0 |= tmp[tmpIdx+3] << 7
		l0 |= tmp[tmpIdx+4] << 5
		l0 |= tmp[tmpIdx+5] << 3
		l0 |= tmp[tmpIdx+6] << 1
		l0 |= int32(uint32(tmp[tmpIdx+7])>>1) & masks32[1]
		ints[intsIdx+0] = l0
		l1 := (tmp[tmpIdx+7] & masks32[1]) << 14
		l1 |= tmp[tmpIdx+8] << 12
		l1 |= tmp[tmpIdx+9] << 10
		l1 |= tmp[tmpIdx+10] << 8
		l1 |= tmp[tmpIdx+11] << 6
		l1 |= tmp[tmpIdx+12] << 4
		l1 |= tmp[tmpIdx+13] << 2
		l1 |= tmp[tmpIdx+14]
		ints[intsIdx+1] = l1
	}
	return nil
}

func l103decode16To32(in store.IndexInput, ints []int32) error {
	// splitInts(64, ints, 16, 16, MASK32_16, ints, 64, MASK32_16)
	return l103splitInts(in, 64, ints, 16, 16, masks32[16], ints, 64, masks32[16])
}
