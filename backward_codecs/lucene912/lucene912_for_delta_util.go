// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// This file has been automatically ported from a generated Java source.
// DO NOT EDIT the decode*/prefixSum* methods by hand.

package lucene912

import (
	"github.com/FlavioCFOliveira/Gocene/store"
)

// forDeltaUtil encodes deltas for strictly monotonically increasing sequences
// of integers.  It is the direct Go port of the auto-generated
// org.apache.lucene.backward_codecs.lucene912.ForDeltaUtil (Lucene 10.4.0).
type forDeltaUtil struct {
	tmp [BlockSize / 2]int64 // scratch buffer
}

// ---------- constants ----------

const (
	fdOneBlockSizeFourth  = BlockSize / 4
	fdTwoBlockSizeFourths = BlockSize / 2
	fdThreeBlockFourths   = 3 * BlockSize / 4

	fdOneBlockSizeEighth   = BlockSize / 8
	fdTwoBlockSizeEighths  = BlockSize / 4
	fdThreeBlockSizeEights = 3 * BlockSize / 8
	fdFourBlockSizeEighths = BlockSize / 2
	fdFiveBlockSizeEighths = 5 * BlockSize / 8
	fdSixBlockSizeEighths  = 3 * BlockSize / 4
	fdSevenBlockEighths    = 7 * BlockSize / 8
)

// IDENTITY_PLUS_ONE[i] == i+1.  Used by prefixSumOfOnes.
var fdIdentityPlusOne [BlockSize]int64

func init() {
	for i := 0; i < BlockSize; i++ {
		fdIdentityPlusOne[i] = int64(i + 1)
	}
}

// ---------- prefix-sum helpers ----------

func fdPrefixSumOfOnes(arr []int64, base int64) {
	copy(arr[:BlockSize], fdIdentityPlusOne[:])
	for i := 0; i < BlockSize; i++ {
		arr[i] += base
	}
}

func fdPrefixSum8(arr []int64, base int64) {
	fdInnerPrefixSum8(arr)
	forUtilExpand8(arr)
	l0 := base
	l1 := l0 + arr[fdOneBlockSizeEighth-1]
	l2 := l1 + arr[fdTwoBlockSizeEighths-1]
	l3 := l2 + arr[fdThreeBlockSizeEights-1]
	l4 := l3 + arr[fdFourBlockSizeEighths-1]
	l5 := l4 + arr[fdFiveBlockSizeEighths-1]
	l6 := l5 + arr[fdSixBlockSizeEighths-1]
	l7 := l6 + arr[fdSevenBlockEighths-1]
	for i := 0; i < fdOneBlockSizeEighth; i++ {
		arr[i] += l0
		arr[fdOneBlockSizeEighth+i] += l1
		arr[fdTwoBlockSizeEighths+i] += l2
		arr[fdThreeBlockSizeEights+i] += l3
		arr[fdFourBlockSizeEighths+i] += l4
		arr[fdFiveBlockSizeEighths+i] += l5
		arr[fdSixBlockSizeEighths+i] += l6
		arr[fdSevenBlockEighths+i] += l7
	}
}

func fdPrefixSum16(arr []int64, base int64) {
	fdInnerPrefixSum16(arr)
	forUtilExpand16(arr)
	l0 := base
	l1 := l0 + arr[fdOneBlockSizeFourth-1]
	l2 := l1 + arr[fdTwoBlockSizeFourths-1]
	l3 := l2 + arr[fdThreeBlockFourths-1]
	for i := 0; i < fdOneBlockSizeFourth; i++ {
		arr[i] += l0
		arr[fdOneBlockSizeFourth+i] += l1
		arr[fdTwoBlockSizeFourths+i] += l2
		arr[fdThreeBlockFourths+i] += l3
	}
}

func fdPrefixSum32(arr []int64, base int64) {
	arr[0] += base << 32
	fdInnerPrefixSum32(arr)
	forUtilExpand32(arr)
	l := arr[BlockSize/2-1]
	for i := BlockSize / 2; i < BlockSize; i++ {
		arr[i] += l
	}
}

// ---------- inner prefix-sum helpers (unrolled for performance) ----------

func fdInnerPrefixSum8(arr []int64) {
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
}

func fdInnerPrefixSum16(arr []int64) {
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
}

func fdInnerPrefixSum32(arr []int64) {
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

// ---------- encode deltas ----------

// encodeDeltas encodes deltas of a strictly monotonically increasing sequence.
// The longs slice must contain deltas between consecutive values.
//
//lint:ignore U1000 write-path entry point; called by Lucene912PostingsWriter (PostingsWriter sprint).
func (f *forDeltaUtil) encodeDeltas(longs []int64, out store.DataOutput) error {
	if longs[0] == 1 && pforUtilAllEqual(longs) {
		return out.WriteByte(0)
	}
	var or int64
	for _, l := range longs[:BlockSize] {
		or |= l
	}
	bitsPerValue := bitsRequired(or)
	if err := out.WriteByte(byte(bitsPerValue)); err != nil {
		return err
	}
	var primitiveSize int
	if bitsPerValue <= 4 {
		primitiveSize = 8
		forUtilCollapse8(longs)
	} else if bitsPerValue <= 11 {
		primitiveSize = 16
		forUtilCollapse16(longs)
	} else {
		primitiveSize = 32
		forUtilCollapse32(longs)
	}
	return forUtilEncodeInternal(longs, bitsPerValue, primitiveSize, out, f.tmp[:])
}

// decodeAndPrefixSum decodes deltas, computes the prefix sum, and adds base
// to all decoded longs.
func (f *forDeltaUtil) decodeAndPrefixSum(in store.IndexInput, base int64, longs []int64) error {
	b, err := in.ReadByte()
	if err != nil {
		return err
	}
	bitsPerValue := int(b)
	if bitsPerValue == 0 {
		fdPrefixSumOfOnes(longs, base)
		return nil
	}
	return f.decodeAndPrefixSumWithBPV(bitsPerValue, in, base, longs)
}

// decodeAndPrefixSumWithBPV decodes 128 delta-encoded integers and applies prefix sum.
func (f *forDeltaUtil) decodeAndPrefixSumWithBPV(bitsPerValue int, in store.IndexInput, base int64, longs []int64) error {
	switch bitsPerValue {
	case 1:
		if err := forUtilDecode1(in, longs); err != nil {
			return err
		}
		fdPrefixSum8(longs, base)
	case 2:
		if err := forUtilDecode2(in, longs); err != nil {
			return err
		}
		fdPrefixSum8(longs, base)
	case 3:
		if err := forUtilDecode3(in, f.tmp[:], longs); err != nil {
			return err
		}
		fdPrefixSum8(longs, base)
	case 4:
		if err := forUtilDecode4(in, longs); err != nil {
			return err
		}
		fdPrefixSum8(longs, base)
	case 5:
		if err := fdDecode5To16(in, f.tmp[:], longs); err != nil {
			return err
		}
		fdPrefixSum16(longs, base)
	case 6:
		if err := fdDecode6To16(in, f.tmp[:], longs); err != nil {
			return err
		}
		fdPrefixSum16(longs, base)
	case 7:
		if err := fdDecode7To16(in, f.tmp[:], longs); err != nil {
			return err
		}
		fdPrefixSum16(longs, base)
	case 8:
		if err := fdDecode8To16(in, longs); err != nil {
			return err
		}
		fdPrefixSum16(longs, base)
	case 9:
		if err := forUtilDecode9(in, f.tmp[:], longs); err != nil {
			return err
		}
		fdPrefixSum16(longs, base)
	case 10:
		if err := forUtilDecode10(in, f.tmp[:], longs); err != nil {
			return err
		}
		fdPrefixSum16(longs, base)
	case 11:
		if err := forUtilDecode11(in, f.tmp[:], longs); err != nil {
			return err
		}
		fdPrefixSum16(longs, base)
	case 12:
		if err := fdDecode12To32(in, f.tmp[:], longs); err != nil {
			return err
		}
		fdPrefixSum32(longs, base)
	case 13:
		if err := fdDecode13To32(in, f.tmp[:], longs); err != nil {
			return err
		}
		fdPrefixSum32(longs, base)
	case 14:
		if err := fdDecode14To32(in, f.tmp[:], longs); err != nil {
			return err
		}
		fdPrefixSum32(longs, base)
	case 15:
		if err := fdDecode15To32(in, f.tmp[:], longs); err != nil {
			return err
		}
		fdPrefixSum32(longs, base)
	case 16:
		if err := fdDecode16To32(in, longs); err != nil {
			return err
		}
		fdPrefixSum32(longs, base)
	case 17:
		if err := forUtilDecode17(in, f.tmp[:], longs); err != nil {
			return err
		}
		fdPrefixSum32(longs, base)
	case 18:
		if err := forUtilDecode18(in, f.tmp[:], longs); err != nil {
			return err
		}
		fdPrefixSum32(longs, base)
	case 19:
		if err := forUtilDecode19(in, f.tmp[:], longs); err != nil {
			return err
		}
		fdPrefixSum32(longs, base)
	case 20:
		if err := forUtilDecode20(in, f.tmp[:], longs); err != nil {
			return err
		}
		fdPrefixSum32(longs, base)
	case 21:
		if err := forUtilDecode21(in, f.tmp[:], longs); err != nil {
			return err
		}
		fdPrefixSum32(longs, base)
	case 22:
		if err := forUtilDecode22(in, f.tmp[:], longs); err != nil {
			return err
		}
		fdPrefixSum32(longs, base)
	case 23:
		if err := forUtilDecode23(in, f.tmp[:], longs); err != nil {
			return err
		}
		fdPrefixSum32(longs, base)
	case 24:
		if err := forUtilDecode24(in, f.tmp[:], longs); err != nil {
			return err
		}
		fdPrefixSum32(longs, base)
	default:
		if err := forUtilDecodeSlow(bitsPerValue, in, f.tmp[:], longs); err != nil {
			return err
		}
		fdPrefixSum32(longs, base)
	}
	return nil
}

// ---------- ForDeltaUtil-specific decode functions ----------
// These pack into 16 or 32 bit targets differently from the ForUtil
// equivalents (used by PForUtil).

func fdDecode5To16(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := forUtilSplitLongs(in, 10, longs, 11, 5, fuMASK16_5, tmp, 0, fuMASK16_1); err != nil {
		return err
	}
	for iter, tmpIdx, longsIdx := 0, 0, 30; iter < 2; iter, tmpIdx, longsIdx = iter+1, tmpIdx+5, longsIdx+1 {
		l0 := tmp[tmpIdx+0] << 4
		l0 |= tmp[tmpIdx+1] << 3
		l0 |= tmp[tmpIdx+2] << 2
		l0 |= tmp[tmpIdx+3] << 1
		l0 |= tmp[tmpIdx+4]
		longs[longsIdx+0] = l0
	}
	return nil
}

func fdDecode6To16(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := forUtilSplitLongs(in, 12, longs, 10, 6, fuMASK16_6, tmp, 0, fuMASK16_4); err != nil {
		return err
	}
	for iter, tmpIdx, longsIdx := 0, 0, 24; iter < 4; iter, tmpIdx, longsIdx = iter+1, tmpIdx+3, longsIdx+2 {
		l0 := tmp[tmpIdx+0] << 2
		l0 |= (tmp[tmpIdx+1] >> 2) & fuMASK16_2
		longs[longsIdx+0] = l0
		l1 := (tmp[tmpIdx+1] & fuMASK16_2) << 4
		l1 |= tmp[tmpIdx+2]
		longs[longsIdx+1] = l1
	}
	return nil
}

func fdDecode7To16(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := forUtilSplitLongs(in, 14, longs, 9, 7, fuMASK16_7, tmp, 0, fuMASK16_2); err != nil {
		return err
	}
	for iter, tmpIdx, longsIdx := 0, 0, 28; iter < 2; iter, tmpIdx, longsIdx = iter+1, tmpIdx+7, longsIdx+2 {
		l0 := tmp[tmpIdx+0] << 5
		l0 |= tmp[tmpIdx+1] << 3
		l0 |= tmp[tmpIdx+2] << 1
		l0 |= (tmp[tmpIdx+3] >> 1) & fuMASK16_1
		longs[longsIdx+0] = l0
		l1 := (tmp[tmpIdx+3] & fuMASK16_1) << 6
		l1 |= tmp[tmpIdx+4] << 4
		l1 |= tmp[tmpIdx+5] << 2
		l1 |= tmp[tmpIdx+6]
		longs[longsIdx+1] = l1
	}
	return nil
}

func fdDecode8To16(in store.IndexInput, longs []int64) error {
	return forUtilSplitLongs(in, 16, longs, 8, 8, fuMASK16_8, longs, 16, fuMASK16_8)
}

func fdDecode12To32(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := forUtilSplitLongs(in, 24, longs, 20, 12, fuMASK32_12, tmp, 0, fuMASK32_8); err != nil {
		return err
	}
	for iter, tmpIdx, longsIdx := 0, 0, 48; iter < 8; iter, tmpIdx, longsIdx = iter+1, tmpIdx+3, longsIdx+2 {
		l0 := tmp[tmpIdx+0] << 4
		l0 |= (tmp[tmpIdx+1] >> 4) & fuMASK32_4
		longs[longsIdx+0] = l0
		l1 := (tmp[tmpIdx+1] & fuMASK32_4) << 8
		l1 |= tmp[tmpIdx+2]
		longs[longsIdx+1] = l1
	}
	return nil
}

func fdDecode13To32(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := forUtilSplitLongs(in, 26, longs, 19, 13, fuMASK32_13, tmp, 0, fuMASK32_6); err != nil {
		return err
	}
	for iter, tmpIdx, longsIdx := 0, 0, 52; iter < 2; iter, tmpIdx, longsIdx = iter+1, tmpIdx+13, longsIdx+6 {
		l0 := tmp[tmpIdx+0] << 7
		l0 |= tmp[tmpIdx+1] << 1
		l0 |= (tmp[tmpIdx+2] >> 5) & fuMASK32_1
		longs[longsIdx+0] = l0
		l1 := (tmp[tmpIdx+2] & fuMASK32_5) << 8
		l1 |= tmp[tmpIdx+3] << 2
		l1 |= (tmp[tmpIdx+4] >> 4) & fuMASK32_2
		longs[longsIdx+1] = l1
		l2 := (tmp[tmpIdx+4] & fuMASK32_4) << 9
		l2 |= tmp[tmpIdx+5] << 3
		l2 |= (tmp[tmpIdx+6] >> 3) & fuMASK32_3
		longs[longsIdx+2] = l2
		l3 := (tmp[tmpIdx+6] & fuMASK32_3) << 10
		l3 |= tmp[tmpIdx+7] << 4
		l3 |= (tmp[tmpIdx+8] >> 2) & fuMASK32_4
		longs[longsIdx+3] = l3
		l4 := (tmp[tmpIdx+8] & fuMASK32_2) << 11
		l4 |= tmp[tmpIdx+9] << 5
		l4 |= (tmp[tmpIdx+10] >> 1) & fuMASK32_5
		longs[longsIdx+4] = l4
		l5 := (tmp[tmpIdx+10] & fuMASK32_1) << 12
		l5 |= tmp[tmpIdx+11] << 6
		l5 |= tmp[tmpIdx+12]
		longs[longsIdx+5] = l5
	}
	return nil
}

func fdDecode14To32(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := forUtilSplitLongs(in, 28, longs, 18, 14, fuMASK32_14, tmp, 0, fuMASK32_4); err != nil {
		return err
	}
	for iter, tmpIdx, longsIdx := 0, 0, 56; iter < 4; iter, tmpIdx, longsIdx = iter+1, tmpIdx+7, longsIdx+2 {
		l0 := tmp[tmpIdx+0] << 10
		l0 |= tmp[tmpIdx+1] << 6
		l0 |= tmp[tmpIdx+2] << 2
		l0 |= (tmp[tmpIdx+3] >> 2) & fuMASK32_2
		longs[longsIdx+0] = l0
		l1 := (tmp[tmpIdx+3] & fuMASK32_2) << 12
		l1 |= tmp[tmpIdx+4] << 8
		l1 |= tmp[tmpIdx+5] << 4
		l1 |= tmp[tmpIdx+6]
		longs[longsIdx+1] = l1
	}
	return nil
}

func fdDecode15To32(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := forUtilSplitLongs(in, 30, longs, 17, 15, fuMASK32_15, tmp, 0, fuMASK32_2); err != nil {
		return err
	}
	for iter, tmpIdx, longsIdx := 0, 0, 60; iter < 2; iter, tmpIdx, longsIdx = iter+1, tmpIdx+15, longsIdx+2 {
		l0 := tmp[tmpIdx+0] << 13
		l0 |= tmp[tmpIdx+1] << 11
		l0 |= tmp[tmpIdx+2] << 9
		l0 |= tmp[tmpIdx+3] << 7
		l0 |= tmp[tmpIdx+4] << 5
		l0 |= tmp[tmpIdx+5] << 3
		l0 |= tmp[tmpIdx+6] << 1
		l0 |= (tmp[tmpIdx+7] >> 1) & fuMASK32_1
		longs[longsIdx+0] = l0
		l1 := (tmp[tmpIdx+7] & fuMASK32_1) << 14
		l1 |= tmp[tmpIdx+8] << 12
		l1 |= tmp[tmpIdx+9] << 10
		l1 |= tmp[tmpIdx+10] << 8
		l1 |= tmp[tmpIdx+11] << 6
		l1 |= tmp[tmpIdx+12] << 4
		l1 |= tmp[tmpIdx+13] << 2
		l1 |= tmp[tmpIdx+14]
		longs[longsIdx+1] = l1
	}
	return nil
}

func fdDecode16To32(in store.IndexInput, longs []int64) error {
	return forUtilSplitLongs(in, 32, longs, 16, 16, fuMASK32_16, longs, 32, fuMASK32_16)
}

// bitsRequired returns the number of bits needed to represent the value v.
// Equivalent to Java PackedInts.bitsRequired(long).
//
//lint:ignore U1000 write-path helper; used by forDeltaUtil.encodeDeltas and pforUtil.encode (PostingsWriter sprint).
func bitsRequired(v int64) int {
	if v == 0 {
		return 1
	}
	bits := 0
	uv := uint64(v)
	for uv > 0 {
		bits++
		uv >>= 1
	}
	return bits
}
