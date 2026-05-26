// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// This file has been automatically ported from a generated Java source.
// DO NOT EDIT the decode* / expand* / collapse* methods by hand.

package lucene912

import (
	"github.com/FlavioCFOliveira/Gocene/store"
)

// forUtil encodes/decodes blocks of 128 integers using Frame of Reference
// bit-packing.  It is the direct Go port of the auto-generated
// org.apache.lucene.backward_codecs.lucene912.ForUtil (Lucene 10.4.0).
//
// All buffers are []int64 (matching Java long[]).  The BLOCK_SIZE constant
// (128) is defined in postings_util.go.
type forUtil struct {
	tmp [BlockSize / 2]int64 // scratch for encode / decodeSlow
}

// ---------- mask helpers ----------

func forUtilExpandMask32(mask32 int64) int64 {
	return mask32 | (mask32 << 32)
}

func forUtilExpandMask16(mask16 int64) int64 {
	return forUtilExpandMask32(mask16 | (mask16 << 16))
}

func forUtilExpandMask8(mask8 int64) int64 {
	return forUtilExpandMask16(mask8 | (mask8 << 8))
}

func forUtilMask32(bitsPerValue int) int64 {
	return forUtilExpandMask32((1 << uint(bitsPerValue)) - 1)
}

func forUtilMask16(bitsPerValue int) int64 {
	return forUtilExpandMask16((1 << uint(bitsPerValue)) - 1)
}

func forUtilMask8(bitsPerValue int) int64 {
	return forUtilExpandMask8((1 << uint(bitsPerValue)) - 1)
}

// Pre-computed mask tables.
var (
	fuMASKS8  [8]int64
	fuMASKS16 [16]int64
	fuMASKS32 [32]int64
)

func init() {
	for i := 0; i < 8; i++ {
		fuMASKS8[i] = forUtilMask8(i)
	}
	for i := 0; i < 16; i++ {
		fuMASKS16[i] = forUtilMask16(i)
	}
	for i := 0; i < 32; i++ {
		fuMASKS32[i] = forUtilMask32(i)
	}
}

// Frequently-used mask constants (matching Java's named final longs).
var (
	fuMASK8_1   = fuMASKS8[1]
	fuMASK8_2   = fuMASKS8[2]
	fuMASK8_3   = fuMASKS8[3]
	fuMASK8_4   = fuMASKS8[4]
	fuMASK8_5   = fuMASKS8[5]
	fuMASK8_6   = fuMASKS8[6]
	fuMASK8_7   = fuMASKS8[7]
	fuMASK16_1  = fuMASKS16[1]
	fuMASK16_2  = fuMASKS16[2]
	fuMASK16_3  = fuMASKS16[3]
	fuMASK16_4  = fuMASKS16[4]
	fuMASK16_5  = fuMASKS16[5]
	fuMASK16_6  = fuMASKS16[6]
	fuMASK16_7  = fuMASKS16[7]
	fuMASK16_8  = fuMASKS16[8]
	fuMASK16_9  = fuMASKS16[9]
	fuMASK16_10 = fuMASKS16[10]
	fuMASK16_11 = fuMASKS16[11]
	fuMASK16_12 = fuMASKS16[12]
	fuMASK16_13 = fuMASKS16[13]
	fuMASK16_14 = fuMASKS16[14]
	fuMASK16_15 = fuMASKS16[15]
	fuMASK32_1  = fuMASKS32[1]
	fuMASK32_2  = fuMASKS32[2]
	fuMASK32_3  = fuMASKS32[3]
	fuMASK32_4  = fuMASKS32[4]
	fuMASK32_5  = fuMASKS32[5]
	fuMASK32_6  = fuMASKS32[6]
	fuMASK32_7  = fuMASKS32[7]
	fuMASK32_8  = fuMASKS32[8]
	fuMASK32_9  = fuMASKS32[9]
	fuMASK32_10 = fuMASKS32[10]
	fuMASK32_11 = fuMASKS32[11]
	fuMASK32_12 = fuMASKS32[12]
	fuMASK32_13 = fuMASKS32[13]
	fuMASK32_14 = fuMASKS32[14]
	fuMASK32_15 = fuMASKS32[15]
	fuMASK32_16 = fuMASKS32[16]
	fuMASK32_17 = fuMASKS32[17]
	fuMASK32_18 = fuMASKS32[18]
	fuMASK32_19 = fuMASKS32[19]
	fuMASK32_20 = fuMASKS32[20]
	fuMASK32_21 = fuMASKS32[21]
	fuMASK32_22 = fuMASKS32[22]
	fuMASK32_23 = fuMASKS32[23]
	fuMASK32_24 = fuMASKS32[24]
)

// numBytes returns the number of bytes required to encode 128 integers of
// bitsPerValue bits per value.
func forUtilNumBytes(bitsPerValue int) int {
	return bitsPerValue << (BlockSizeLog2 - 3)
}

// BlockSizeLog2 is log2(BlockSize) = log2(128) = 7.
const BlockSizeLog2 = 7

// ---------- expand / collapse ----------

func forUtilExpand8(arr []int64) {
	for i := 0; i < 16; i++ {
		l := arr[i]
		arr[i] = (l >> 56) & 0xFF
		arr[16+i] = (l >> 48) & 0xFF
		arr[32+i] = (l >> 40) & 0xFF
		arr[48+i] = (l >> 32) & 0xFF
		arr[64+i] = (l >> 24) & 0xFF
		arr[80+i] = (l >> 16) & 0xFF
		arr[96+i] = (l >> 8) & 0xFF
		arr[112+i] = l & 0xFF
	}
}

//lint:ignore U1000 write-path helper; used by forDeltaUtil.encodeDeltas and forUtil.encode (PostingsWriter sprint).
func forUtilCollapse8(arr []int64) {
	for i := 0; i < 16; i++ {
		arr[i] = (arr[i] << 56) | (arr[16+i] << 48) | (arr[32+i] << 40) |
			(arr[48+i] << 32) | (arr[64+i] << 24) | (arr[80+i] << 16) |
			(arr[96+i] << 8) | arr[112+i]
	}
}

func forUtilExpand16(arr []int64) {
	for i := 0; i < 32; i++ {
		l := arr[i]
		arr[i] = (l >> 48) & 0xFFFF
		arr[32+i] = (l >> 32) & 0xFFFF
		arr[64+i] = (l >> 16) & 0xFFFF
		arr[96+i] = l & 0xFFFF
	}
}

//lint:ignore U1000 write-path helper; used by forDeltaUtil.encodeDeltas and forUtil.encode (PostingsWriter sprint).
func forUtilCollapse16(arr []int64) {
	for i := 0; i < 32; i++ {
		arr[i] = (arr[i] << 48) | (arr[32+i] << 32) | (arr[64+i] << 16) | arr[96+i]
	}
}

func forUtilExpand32(arr []int64) {
	for i := 0; i < 64; i++ {
		l := arr[i]
		arr[i] = int64(uint64(l) >> 32)
		arr[64+i] = l & 0xFFFFFFFF
	}
}

//lint:ignore U1000 write-path helper; used by forDeltaUtil.encodeDeltas and forUtil.encode (PostingsWriter sprint).
func forUtilCollapse32(arr []int64) {
	for i := 0; i < 64; i++ {
		arr[i] = (arr[i] << 32) | arr[64+i]
	}
}

// ---------- splitLongs ----------

// splitLongs reads count int64 values from in, then splits each value into
// two parts: a high bShift-bit field stored into b[0..count-1] (shifted right
// by bShift and masked by bMask), and a low (dec - bShift) bit field stored
// into c[cIndex..].  cMask is applied to the raw value before storing it in c.
//
// This is the direct port of ForUtil.splitLongs(IndexInput, int, long[], int,
// int, long, long[], int, long).  Java calls in.readLongs(c, cIndex, count);
// Go reads count longs sequentially with ReadLong().
func forUtilSplitLongs(
	in store.IndexInput,
	count int,
	b []int64,
	bShift int,
	dec int,
	bMask int64,
	c []int64,
	cIndex int,
	cMask int64,
) error {
	for i := 0; i < count; i++ {
		v, err := in.ReadLong()
		if err != nil {
			return err
		}
		c[cIndex+i] = v
	}
	maxIter := (bShift - 1) / dec
	for i := 0; i < count; i++ {
		for j := 0; j <= maxIter; j++ {
			b[count*j+i] = int64(uint64(c[cIndex+i])>>uint(bShift-j*dec)) & bMask
		}
		c[cIndex+i] &= cMask
	}
	return nil
}

// ---------- encode ----------

// encode encodes 128 integers from longs into out using bitsPerValue bits per value.
//
//lint:ignore U1000 write-path entry point; called by pforUtil.encode (PostingsWriter sprint).
func (f *forUtil) encode(longs []int64, bitsPerValue int, out store.DataOutput) error {
	var nextPrimitive int
	if bitsPerValue <= 8 {
		nextPrimitive = 8
		forUtilCollapse8(longs)
	} else if bitsPerValue <= 16 {
		nextPrimitive = 16
		forUtilCollapse16(longs)
	} else {
		nextPrimitive = 32
		forUtilCollapse32(longs)
	}
	return forUtilEncodeInternal(longs, bitsPerValue, nextPrimitive, out, f.tmp[:])
}

//lint:ignore U1000 write-path helper; called by forUtil.encode and forDeltaUtil.encodeDeltas (PostingsWriter sprint).
func forUtilEncodeInternal(longs []int64, bitsPerValue, primitiveSize int, out store.DataOutput, tmp []int64) error {
	numLongs := BlockSize * primitiveSize / 64
	numLongsPerShift := bitsPerValue * 2
	idx := 0
	shift := primitiveSize - bitsPerValue
	for i := 0; i < numLongsPerShift; i++ {
		tmp[i] = longs[idx] << uint(shift)
		idx++
	}
	for shift -= bitsPerValue; shift >= 0; shift -= bitsPerValue {
		for i := 0; i < numLongsPerShift; i++ {
			tmp[i] |= longs[idx] << uint(shift)
			idx++
		}
	}

	remainingBitsPerLong := shift + bitsPerValue
	var maskRemainingBitsPerLong int64
	switch primitiveSize {
	case 8:
		maskRemainingBitsPerLong = fuMASKS8[remainingBitsPerLong]
	case 16:
		maskRemainingBitsPerLong = fuMASKS16[remainingBitsPerLong]
	default:
		maskRemainingBitsPerLong = fuMASKS32[remainingBitsPerLong]
	}

	tmpIdx := 0
	remainingBitsPerValue := bitsPerValue
	for idx < numLongs {
		if remainingBitsPerValue >= remainingBitsPerLong {
			remainingBitsPerValue -= remainingBitsPerLong
			tmp[tmpIdx] |= (longs[idx] >> uint(remainingBitsPerValue)) & maskRemainingBitsPerLong
			tmpIdx++
			if remainingBitsPerValue == 0 {
				idx++
				remainingBitsPerValue = bitsPerValue
			}
		} else {
			var mask1, mask2 int64
			switch primitiveSize {
			case 8:
				mask1 = fuMASKS8[remainingBitsPerValue]
				mask2 = fuMASKS8[remainingBitsPerLong-remainingBitsPerValue]
			case 16:
				mask1 = fuMASKS16[remainingBitsPerValue]
				mask2 = fuMASKS16[remainingBitsPerLong-remainingBitsPerValue]
			default:
				mask1 = fuMASKS32[remainingBitsPerValue]
				mask2 = fuMASKS32[remainingBitsPerLong-remainingBitsPerValue]
			}
			tmp[tmpIdx] |= (longs[idx] & mask1) << uint(remainingBitsPerLong-remainingBitsPerValue)
			idx++
			remainingBitsPerValue = bitsPerValue - remainingBitsPerLong + remainingBitsPerValue
			tmp[tmpIdx] |= (longs[idx] >> uint(remainingBitsPerValue)) & mask2
			tmpIdx++
		}
	}

	for i := 0; i < numLongsPerShift; i++ {
		if err := out.WriteLong(tmp[i]); err != nil {
			return err
		}
	}
	return nil
}

// ---------- decode (dispatch) ----------

// decode decodes 128 integers from in into longs.
func (f *forUtil) decode(bitsPerValue int, in store.IndexInput, longs []int64) error {
	switch bitsPerValue {
	case 1:
		if err := forUtilDecode1(in, longs); err != nil {
			return err
		}
		forUtilExpand8(longs)
	case 2:
		if err := forUtilDecode2(in, longs); err != nil {
			return err
		}
		forUtilExpand8(longs)
	case 3:
		if err := forUtilDecode3(in, f.tmp[:], longs); err != nil {
			return err
		}
		forUtilExpand8(longs)
	case 4:
		if err := forUtilDecode4(in, longs); err != nil {
			return err
		}
		forUtilExpand8(longs)
	case 5:
		if err := forUtilDecode5(in, f.tmp[:], longs); err != nil {
			return err
		}
		forUtilExpand8(longs)
	case 6:
		if err := forUtilDecode6(in, f.tmp[:], longs); err != nil {
			return err
		}
		forUtilExpand8(longs)
	case 7:
		if err := forUtilDecode7(in, f.tmp[:], longs); err != nil {
			return err
		}
		forUtilExpand8(longs)
	case 8:
		if err := forUtilDecode8(in, longs); err != nil {
			return err
		}
		forUtilExpand8(longs)
	case 9:
		if err := forUtilDecode9(in, f.tmp[:], longs); err != nil {
			return err
		}
		forUtilExpand16(longs)
	case 10:
		if err := forUtilDecode10(in, f.tmp[:], longs); err != nil {
			return err
		}
		forUtilExpand16(longs)
	case 11:
		if err := forUtilDecode11(in, f.tmp[:], longs); err != nil {
			return err
		}
		forUtilExpand16(longs)
	case 12:
		if err := forUtilDecode12(in, f.tmp[:], longs); err != nil {
			return err
		}
		forUtilExpand16(longs)
	case 13:
		if err := forUtilDecode13(in, f.tmp[:], longs); err != nil {
			return err
		}
		forUtilExpand16(longs)
	case 14:
		if err := forUtilDecode14(in, f.tmp[:], longs); err != nil {
			return err
		}
		forUtilExpand16(longs)
	case 15:
		if err := forUtilDecode15(in, f.tmp[:], longs); err != nil {
			return err
		}
		forUtilExpand16(longs)
	case 16:
		if err := forUtilDecode16(in, longs); err != nil {
			return err
		}
		forUtilExpand16(longs)
	case 17:
		if err := forUtilDecode17(in, f.tmp[:], longs); err != nil {
			return err
		}
		forUtilExpand32(longs)
	case 18:
		if err := forUtilDecode18(in, f.tmp[:], longs); err != nil {
			return err
		}
		forUtilExpand32(longs)
	case 19:
		if err := forUtilDecode19(in, f.tmp[:], longs); err != nil {
			return err
		}
		forUtilExpand32(longs)
	case 20:
		if err := forUtilDecode20(in, f.tmp[:], longs); err != nil {
			return err
		}
		forUtilExpand32(longs)
	case 21:
		if err := forUtilDecode21(in, f.tmp[:], longs); err != nil {
			return err
		}
		forUtilExpand32(longs)
	case 22:
		if err := forUtilDecode22(in, f.tmp[:], longs); err != nil {
			return err
		}
		forUtilExpand32(longs)
	case 23:
		if err := forUtilDecode23(in, f.tmp[:], longs); err != nil {
			return err
		}
		forUtilExpand32(longs)
	case 24:
		if err := forUtilDecode24(in, f.tmp[:], longs); err != nil {
			return err
		}
		forUtilExpand32(longs)
	default:
		if err := forUtilDecodeSlow(bitsPerValue, in, f.tmp[:], longs); err != nil {
			return err
		}
		forUtilExpand32(longs)
	}
	return nil
}

// ---------- decodeSlow ----------

func forUtilDecodeSlow(bitsPerValue int, in store.IndexInput, tmp []int64, longs []int64) error {
	numLongs := bitsPerValue * 2
	mask := fuMASKS32[bitsPerValue]
	if err := forUtilSplitLongs(in, numLongs, longs, 32-bitsPerValue, 32, mask, tmp, 0, -1); err != nil {
		return err
	}
	remainingBitsPerLong := 32 - bitsPerValue
	mask32Remaining := fuMASKS32[remainingBitsPerLong]
	tmpIdx := 0
	remainingBits := remainingBitsPerLong
	for longsIdx := numLongs; longsIdx < BlockSize/2; longsIdx++ {
		b := bitsPerValue - remainingBits
		l := (tmp[tmpIdx] & fuMASKS32[remainingBits]) << uint(b)
		tmpIdx++
		for b >= remainingBitsPerLong {
			b -= remainingBitsPerLong
			l |= (tmp[tmpIdx] & mask32Remaining) << uint(b)
			tmpIdx++
		}
		if b > 0 {
			l |= int64(uint64(tmp[tmpIdx])>>uint(remainingBitsPerLong-b)) & fuMASKS32[b]
			remainingBits = remainingBitsPerLong - b
		} else {
			remainingBits = remainingBitsPerLong
		}
		longs[longsIdx] = l
	}
	return nil
}

// ---------- individual decode functions (ports of the Java auto-generated methods) ----------

func forUtilDecode1(in store.IndexInput, longs []int64) error {
	return forUtilSplitLongs(in, 2, longs, 7, 1, fuMASK8_1, longs, 14, fuMASK8_1)
}

func forUtilDecode2(in store.IndexInput, longs []int64) error {
	return forUtilSplitLongs(in, 4, longs, 6, 2, fuMASK8_2, longs, 12, fuMASK8_2)
}

func forUtilDecode3(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := forUtilSplitLongs(in, 6, longs, 5, 3, fuMASK8_3, tmp, 0, fuMASK8_2); err != nil {
		return err
	}
	for iter, tmpIdx, longsIdx := 0, 0, 12; iter < 2; iter, tmpIdx, longsIdx = iter+1, tmpIdx+3, longsIdx+2 {
		l0 := tmp[tmpIdx+0] << 1
		l0 |= (tmp[tmpIdx+1] >> 1) & fuMASK8_1
		longs[longsIdx+0] = l0
		l1 := (tmp[tmpIdx+1] & fuMASK8_1) << 2
		l1 |= tmp[tmpIdx+2]
		longs[longsIdx+1] = l1
	}
	return nil
}

func forUtilDecode4(in store.IndexInput, longs []int64) error {
	return forUtilSplitLongs(in, 8, longs, 4, 4, fuMASK8_4, longs, 8, fuMASK8_4)
}

func forUtilDecode5(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := forUtilSplitLongs(in, 10, longs, 3, 5, fuMASK8_5, tmp, 0, fuMASK8_3); err != nil {
		return err
	}
	for iter, tmpIdx, longsIdx := 0, 0, 10; iter < 2; iter, tmpIdx, longsIdx = iter+1, tmpIdx+5, longsIdx+3 {
		l0 := tmp[tmpIdx+0] << 2
		l0 |= (tmp[tmpIdx+1] >> 1) & fuMASK8_2
		longs[longsIdx+0] = l0
		l1 := (tmp[tmpIdx+1] & fuMASK8_1) << 4
		l1 |= tmp[tmpIdx+2] << 1
		l1 |= (tmp[tmpIdx+3] >> 2) & fuMASK8_1
		longs[longsIdx+1] = l1
		l2 := (tmp[tmpIdx+3] & fuMASK8_2) << 3
		l2 |= tmp[tmpIdx+4]
		longs[longsIdx+2] = l2
	}
	return nil
}

func forUtilDecode6(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := forUtilSplitLongs(in, 12, longs, 2, 6, fuMASK8_6, tmp, 0, fuMASK8_2); err != nil {
		return err
	}
	for iter, tmpIdx, longsIdx := 0, 0, 12; iter < 4; iter, tmpIdx, longsIdx = iter+1, tmpIdx+3, longsIdx+1 {
		l0 := tmp[tmpIdx+0] << 4
		l0 |= tmp[tmpIdx+1] << 2
		l0 |= tmp[tmpIdx+2]
		longs[longsIdx+0] = l0
	}
	return nil
}

func forUtilDecode7(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := forUtilSplitLongs(in, 14, longs, 1, 7, fuMASK8_7, tmp, 0, fuMASK8_1); err != nil {
		return err
	}
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

func forUtilDecode8(in store.IndexInput, longs []int64) error {
	for i := 0; i < 16; i++ {
		v, err := in.ReadLong()
		if err != nil {
			return err
		}
		longs[i] = v
	}
	return nil
}

func forUtilDecode9(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := forUtilSplitLongs(in, 18, longs, 7, 9, fuMASK16_9, tmp, 0, fuMASK16_7); err != nil {
		return err
	}
	for iter, tmpIdx, longsIdx := 0, 0, 18; iter < 2; iter, tmpIdx, longsIdx = iter+1, tmpIdx+9, longsIdx+7 {
		l0 := tmp[tmpIdx+0] << 2
		l0 |= (tmp[tmpIdx+1] >> 5) & fuMASK16_2
		longs[longsIdx+0] = l0
		l1 := (tmp[tmpIdx+1] & fuMASK16_5) << 4
		l1 |= (tmp[tmpIdx+2] >> 3) & fuMASK16_4
		longs[longsIdx+1] = l1
		l2 := (tmp[tmpIdx+2] & fuMASK16_3) << 6
		l2 |= (tmp[tmpIdx+3] >> 1) & fuMASK16_6
		longs[longsIdx+2] = l2
		l3 := (tmp[tmpIdx+3] & fuMASK16_1) << 8
		l3 |= tmp[tmpIdx+4] << 1
		l3 |= (tmp[tmpIdx+5] >> 6) & fuMASK16_1
		longs[longsIdx+3] = l3
		l4 := (tmp[tmpIdx+5] & fuMASK16_6) << 3
		l4 |= (tmp[tmpIdx+6] >> 4) & fuMASK16_3
		longs[longsIdx+4] = l4
		l5 := (tmp[tmpIdx+6] & fuMASK16_4) << 5
		l5 |= (tmp[tmpIdx+7] >> 2) & fuMASK16_5
		longs[longsIdx+5] = l5
		l6 := (tmp[tmpIdx+7] & fuMASK16_2) << 7
		l6 |= tmp[tmpIdx+8]
		longs[longsIdx+6] = l6
	}
	return nil
}

func forUtilDecode10(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := forUtilSplitLongs(in, 20, longs, 6, 10, fuMASK16_10, tmp, 0, fuMASK16_6); err != nil {
		return err
	}
	for iter, tmpIdx, longsIdx := 0, 0, 20; iter < 4; iter, tmpIdx, longsIdx = iter+1, tmpIdx+5, longsIdx+3 {
		l0 := tmp[tmpIdx+0] << 4
		l0 |= (tmp[tmpIdx+1] >> 2) & fuMASK16_4
		longs[longsIdx+0] = l0
		l1 := (tmp[tmpIdx+1] & fuMASK16_2) << 8
		l1 |= tmp[tmpIdx+2] << 2
		l1 |= (tmp[tmpIdx+3] >> 4) & fuMASK16_2
		longs[longsIdx+1] = l1
		l2 := (tmp[tmpIdx+3] & fuMASK16_4) << 6
		l2 |= tmp[tmpIdx+4]
		longs[longsIdx+2] = l2
	}
	return nil
}

func forUtilDecode11(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := forUtilSplitLongs(in, 22, longs, 5, 11, fuMASK16_11, tmp, 0, fuMASK16_5); err != nil {
		return err
	}
	for iter, tmpIdx, longsIdx := 0, 0, 22; iter < 2; iter, tmpIdx, longsIdx = iter+1, tmpIdx+11, longsIdx+5 {
		l0 := tmp[tmpIdx+0] << 6
		l0 |= tmp[tmpIdx+1] << 1
		l0 |= (tmp[tmpIdx+2] >> 4) & fuMASK16_1
		longs[longsIdx+0] = l0
		l1 := (tmp[tmpIdx+2] & fuMASK16_4) << 7
		l1 |= tmp[tmpIdx+3] << 2
		l1 |= (tmp[tmpIdx+4] >> 3) & fuMASK16_2
		longs[longsIdx+1] = l1
		l2 := (tmp[tmpIdx+4] & fuMASK16_3) << 8
		l2 |= tmp[tmpIdx+5] << 3
		l2 |= (tmp[tmpIdx+6] >> 2) & fuMASK16_3
		longs[longsIdx+2] = l2
		l3 := (tmp[tmpIdx+6] & fuMASK16_2) << 9
		l3 |= tmp[tmpIdx+7] << 4
		l3 |= (tmp[tmpIdx+8] >> 1) & fuMASK16_4
		longs[longsIdx+3] = l3
		l4 := (tmp[tmpIdx+8] & fuMASK16_1) << 10
		l4 |= tmp[tmpIdx+9] << 5
		l4 |= tmp[tmpIdx+10]
		longs[longsIdx+4] = l4
	}
	return nil
}

func forUtilDecode12(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := forUtilSplitLongs(in, 24, longs, 4, 12, fuMASK16_12, tmp, 0, fuMASK16_4); err != nil {
		return err
	}
	for iter, tmpIdx, longsIdx := 0, 0, 24; iter < 8; iter, tmpIdx, longsIdx = iter+1, tmpIdx+3, longsIdx+1 {
		l0 := tmp[tmpIdx+0] << 8
		l0 |= tmp[tmpIdx+1] << 4
		l0 |= tmp[tmpIdx+2]
		longs[longsIdx+0] = l0
	}
	return nil
}

func forUtilDecode13(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := forUtilSplitLongs(in, 26, longs, 3, 13, fuMASK16_13, tmp, 0, fuMASK16_3); err != nil {
		return err
	}
	for iter, tmpIdx, longsIdx := 0, 0, 26; iter < 2; iter, tmpIdx, longsIdx = iter+1, tmpIdx+13, longsIdx+3 {
		l0 := tmp[tmpIdx+0] << 10
		l0 |= tmp[tmpIdx+1] << 7
		l0 |= tmp[tmpIdx+2] << 4
		l0 |= tmp[tmpIdx+3] << 1
		l0 |= (tmp[tmpIdx+4] >> 2) & fuMASK16_1
		longs[longsIdx+0] = l0
		l1 := (tmp[tmpIdx+4] & fuMASK16_2) << 11
		l1 |= tmp[tmpIdx+5] << 8
		l1 |= tmp[tmpIdx+6] << 5
		l1 |= tmp[tmpIdx+7] << 2
		l1 |= (tmp[tmpIdx+8] >> 1) & fuMASK16_2
		longs[longsIdx+1] = l1
		l2 := (tmp[tmpIdx+8] & fuMASK16_1) << 12
		l2 |= tmp[tmpIdx+9] << 9
		l2 |= tmp[tmpIdx+10] << 6
		l2 |= tmp[tmpIdx+11] << 3
		l2 |= tmp[tmpIdx+12]
		longs[longsIdx+2] = l2
	}
	return nil
}

func forUtilDecode14(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := forUtilSplitLongs(in, 28, longs, 2, 14, fuMASK16_14, tmp, 0, fuMASK16_2); err != nil {
		return err
	}
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

func forUtilDecode15(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := forUtilSplitLongs(in, 30, longs, 1, 15, fuMASK16_15, tmp, 0, fuMASK16_1); err != nil {
		return err
	}
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

func forUtilDecode16(in store.IndexInput, longs []int64) error {
	for i := 0; i < 32; i++ {
		v, err := in.ReadLong()
		if err != nil {
			return err
		}
		longs[i] = v
	}
	return nil
}

func forUtilDecode17(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := forUtilSplitLongs(in, 34, longs, 15, 17, fuMASK32_17, tmp, 0, fuMASK32_15); err != nil {
		return err
	}
	for iter, tmpIdx, longsIdx := 0, 0, 34; iter < 2; iter, tmpIdx, longsIdx = iter+1, tmpIdx+17, longsIdx+15 {
		l0 := tmp[tmpIdx+0] << 2
		l0 |= (tmp[tmpIdx+1] >> 13) & fuMASK32_2
		longs[longsIdx+0] = l0
		l1 := (tmp[tmpIdx+1] & fuMASK32_13) << 4
		l1 |= (tmp[tmpIdx+2] >> 11) & fuMASK32_4
		longs[longsIdx+1] = l1
		l2 := (tmp[tmpIdx+2] & fuMASK32_11) << 6
		l2 |= (tmp[tmpIdx+3] >> 9) & fuMASK32_6
		longs[longsIdx+2] = l2
		l3 := (tmp[tmpIdx+3] & fuMASK32_9) << 8
		l3 |= (tmp[tmpIdx+4] >> 7) & fuMASK32_8
		longs[longsIdx+3] = l3
		l4 := (tmp[tmpIdx+4] & fuMASK32_7) << 10
		l4 |= (tmp[tmpIdx+5] >> 5) & fuMASK32_10
		longs[longsIdx+4] = l4
		l5 := (tmp[tmpIdx+5] & fuMASK32_5) << 12
		l5 |= (tmp[tmpIdx+6] >> 3) & fuMASK32_12
		longs[longsIdx+5] = l5
		l6 := (tmp[tmpIdx+6] & fuMASK32_3) << 14
		l6 |= (tmp[tmpIdx+7] >> 1) & fuMASK32_14
		longs[longsIdx+6] = l6
		l7 := (tmp[tmpIdx+7] & fuMASK32_1) << 16
		l7 |= tmp[tmpIdx+8] << 1
		l7 |= (tmp[tmpIdx+9] >> 14) & fuMASK32_1
		longs[longsIdx+7] = l7
		l8 := (tmp[tmpIdx+9] & fuMASK32_14) << 3
		l8 |= (tmp[tmpIdx+10] >> 12) & fuMASK32_3
		longs[longsIdx+8] = l8
		l9 := (tmp[tmpIdx+10] & fuMASK32_12) << 5
		l9 |= (tmp[tmpIdx+11] >> 10) & fuMASK32_5
		longs[longsIdx+9] = l9
		l10 := (tmp[tmpIdx+11] & fuMASK32_10) << 7
		l10 |= (tmp[tmpIdx+12] >> 8) & fuMASK32_7
		longs[longsIdx+10] = l10
		l11 := (tmp[tmpIdx+12] & fuMASK32_8) << 9
		l11 |= (tmp[tmpIdx+13] >> 6) & fuMASK32_9
		longs[longsIdx+11] = l11
		l12 := (tmp[tmpIdx+13] & fuMASK32_6) << 11
		l12 |= (tmp[tmpIdx+14] >> 4) & fuMASK32_11
		longs[longsIdx+12] = l12
		l13 := (tmp[tmpIdx+14] & fuMASK32_4) << 13
		l13 |= (tmp[tmpIdx+15] >> 2) & fuMASK32_13
		longs[longsIdx+13] = l13
		l14 := (tmp[tmpIdx+15] & fuMASK32_2) << 15
		l14 |= tmp[tmpIdx+16]
		longs[longsIdx+14] = l14
	}
	return nil
}

func forUtilDecode18(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := forUtilSplitLongs(in, 36, longs, 14, 18, fuMASK32_18, tmp, 0, fuMASK32_14); err != nil {
		return err
	}
	for iter, tmpIdx, longsIdx := 0, 0, 36; iter < 4; iter, tmpIdx, longsIdx = iter+1, tmpIdx+9, longsIdx+7 {
		l0 := tmp[tmpIdx+0] << 4
		l0 |= (tmp[tmpIdx+1] >> 10) & fuMASK32_4
		longs[longsIdx+0] = l0
		l1 := (tmp[tmpIdx+1] & fuMASK32_10) << 8
		l1 |= (tmp[tmpIdx+2] >> 6) & fuMASK32_8
		longs[longsIdx+1] = l1
		l2 := (tmp[tmpIdx+2] & fuMASK32_6) << 12
		l2 |= (tmp[tmpIdx+3] >> 2) & fuMASK32_12
		longs[longsIdx+2] = l2
		l3 := (tmp[tmpIdx+3] & fuMASK32_2) << 16
		l3 |= tmp[tmpIdx+4] << 2
		l3 |= (tmp[tmpIdx+5] >> 12) & fuMASK32_2
		longs[longsIdx+3] = l3
		l4 := (tmp[tmpIdx+5] & fuMASK32_12) << 6
		l4 |= (tmp[tmpIdx+6] >> 8) & fuMASK32_6
		longs[longsIdx+4] = l4
		l5 := (tmp[tmpIdx+6] & fuMASK32_8) << 10
		l5 |= (tmp[tmpIdx+7] >> 4) & fuMASK32_10
		longs[longsIdx+5] = l5
		l6 := (tmp[tmpIdx+7] & fuMASK32_4) << 14
		l6 |= tmp[tmpIdx+8]
		longs[longsIdx+6] = l6
	}
	return nil
}

func forUtilDecode19(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := forUtilSplitLongs(in, 38, longs, 13, 19, fuMASK32_19, tmp, 0, fuMASK32_13); err != nil {
		return err
	}
	for iter, tmpIdx, longsIdx := 0, 0, 38; iter < 2; iter, tmpIdx, longsIdx = iter+1, tmpIdx+19, longsIdx+13 {
		l0 := tmp[tmpIdx+0] << 6
		l0 |= (tmp[tmpIdx+1] >> 7) & fuMASK32_6
		longs[longsIdx+0] = l0
		l1 := (tmp[tmpIdx+1] & fuMASK32_7) << 12
		l1 |= (tmp[tmpIdx+2] >> 1) & fuMASK32_12
		longs[longsIdx+1] = l1
		l2 := (tmp[tmpIdx+2] & fuMASK32_1) << 18
		l2 |= tmp[tmpIdx+3] << 5
		l2 |= (tmp[tmpIdx+4] >> 8) & fuMASK32_5
		longs[longsIdx+2] = l2
		l3 := (tmp[tmpIdx+4] & fuMASK32_8) << 11
		l3 |= (tmp[tmpIdx+5] >> 2) & fuMASK32_11
		longs[longsIdx+3] = l3
		l4 := (tmp[tmpIdx+5] & fuMASK32_2) << 17
		l4 |= tmp[tmpIdx+6] << 4
		l4 |= (tmp[tmpIdx+7] >> 9) & fuMASK32_4
		longs[longsIdx+4] = l4
		l5 := (tmp[tmpIdx+7] & fuMASK32_9) << 10
		l5 |= (tmp[tmpIdx+8] >> 3) & fuMASK32_10
		longs[longsIdx+5] = l5
		l6 := (tmp[tmpIdx+8] & fuMASK32_3) << 16
		l6 |= tmp[tmpIdx+9] << 3
		l6 |= (tmp[tmpIdx+10] >> 10) & fuMASK32_3
		longs[longsIdx+6] = l6
		l7 := (tmp[tmpIdx+10] & fuMASK32_10) << 9
		l7 |= (tmp[tmpIdx+11] >> 4) & fuMASK32_9
		longs[longsIdx+7] = l7
		l8 := (tmp[tmpIdx+11] & fuMASK32_4) << 15
		l8 |= tmp[tmpIdx+12] << 2
		l8 |= (tmp[tmpIdx+13] >> 11) & fuMASK32_2
		longs[longsIdx+8] = l8
		l9 := (tmp[tmpIdx+13] & fuMASK32_11) << 8
		l9 |= (tmp[tmpIdx+14] >> 5) & fuMASK32_8
		longs[longsIdx+9] = l9
		l10 := (tmp[tmpIdx+14] & fuMASK32_5) << 14
		l10 |= tmp[tmpIdx+15] << 1
		l10 |= (tmp[tmpIdx+16] >> 12) & fuMASK32_1
		longs[longsIdx+10] = l10
		l11 := (tmp[tmpIdx+16] & fuMASK32_12) << 7
		l11 |= (tmp[tmpIdx+17] >> 6) & fuMASK32_7
		longs[longsIdx+11] = l11
		l12 := (tmp[tmpIdx+17] & fuMASK32_6) << 13
		l12 |= tmp[tmpIdx+18]
		longs[longsIdx+12] = l12
	}
	return nil
}

func forUtilDecode20(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := forUtilSplitLongs(in, 40, longs, 12, 20, fuMASK32_20, tmp, 0, fuMASK32_12); err != nil {
		return err
	}
	for iter, tmpIdx, longsIdx := 0, 0, 40; iter < 8; iter, tmpIdx, longsIdx = iter+1, tmpIdx+5, longsIdx+3 {
		l0 := tmp[tmpIdx+0] << 8
		l0 |= (tmp[tmpIdx+1] >> 4) & fuMASK32_8
		longs[longsIdx+0] = l0
		l1 := (tmp[tmpIdx+1] & fuMASK32_4) << 16
		l1 |= tmp[tmpIdx+2] << 4
		l1 |= (tmp[tmpIdx+3] >> 8) & fuMASK32_4
		longs[longsIdx+1] = l1
		l2 := (tmp[tmpIdx+3] & fuMASK32_8) << 12
		l2 |= tmp[tmpIdx+4]
		longs[longsIdx+2] = l2
	}
	return nil
}

func forUtilDecode21(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := forUtilSplitLongs(in, 42, longs, 11, 21, fuMASK32_21, tmp, 0, fuMASK32_11); err != nil {
		return err
	}
	for iter, tmpIdx, longsIdx := 0, 0, 42; iter < 2; iter, tmpIdx, longsIdx = iter+1, tmpIdx+21, longsIdx+11 {
		l0 := tmp[tmpIdx+0] << 10
		l0 |= (tmp[tmpIdx+1] >> 1) & fuMASK32_10
		longs[longsIdx+0] = l0
		l1 := (tmp[tmpIdx+1] & fuMASK32_1) << 20
		l1 |= tmp[tmpIdx+2] << 9
		l1 |= (tmp[tmpIdx+3] >> 2) & fuMASK32_9
		longs[longsIdx+1] = l1
		l2 := (tmp[tmpIdx+3] & fuMASK32_2) << 19
		l2 |= tmp[tmpIdx+4] << 8
		l2 |= (tmp[tmpIdx+5] >> 3) & fuMASK32_8
		longs[longsIdx+2] = l2
		l3 := (tmp[tmpIdx+5] & fuMASK32_3) << 18
		l3 |= tmp[tmpIdx+6] << 7
		l3 |= (tmp[tmpIdx+7] >> 4) & fuMASK32_7
		longs[longsIdx+3] = l3
		l4 := (tmp[tmpIdx+7] & fuMASK32_4) << 17
		l4 |= tmp[tmpIdx+8] << 6
		l4 |= (tmp[tmpIdx+9] >> 5) & fuMASK32_6
		longs[longsIdx+4] = l4
		l5 := (tmp[tmpIdx+9] & fuMASK32_5) << 16
		l5 |= tmp[tmpIdx+10] << 5
		l5 |= (tmp[tmpIdx+11] >> 6) & fuMASK32_5
		longs[longsIdx+5] = l5
		l6 := (tmp[tmpIdx+11] & fuMASK32_6) << 15
		l6 |= tmp[tmpIdx+12] << 4
		l6 |= (tmp[tmpIdx+13] >> 7) & fuMASK32_4
		longs[longsIdx+6] = l6
		l7 := (tmp[tmpIdx+13] & fuMASK32_7) << 14
		l7 |= tmp[tmpIdx+14] << 3
		l7 |= (tmp[tmpIdx+15] >> 8) & fuMASK32_3
		longs[longsIdx+7] = l7
		l8 := (tmp[tmpIdx+15] & fuMASK32_8) << 13
		l8 |= tmp[tmpIdx+16] << 2
		l8 |= (tmp[tmpIdx+17] >> 9) & fuMASK32_2
		longs[longsIdx+8] = l8
		l9 := (tmp[tmpIdx+17] & fuMASK32_9) << 12
		l9 |= tmp[tmpIdx+18] << 1
		l9 |= (tmp[tmpIdx+19] >> 10) & fuMASK32_1
		longs[longsIdx+9] = l9
		l10 := (tmp[tmpIdx+19] & fuMASK32_10) << 11
		l10 |= tmp[tmpIdx+20]
		longs[longsIdx+10] = l10
	}
	return nil
}

func forUtilDecode22(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := forUtilSplitLongs(in, 44, longs, 10, 22, fuMASK32_22, tmp, 0, fuMASK32_10); err != nil {
		return err
	}
	for iter, tmpIdx, longsIdx := 0, 0, 44; iter < 4; iter, tmpIdx, longsIdx = iter+1, tmpIdx+11, longsIdx+5 {
		l0 := tmp[tmpIdx+0] << 12
		l0 |= tmp[tmpIdx+1] << 2
		l0 |= (tmp[tmpIdx+2] >> 8) & fuMASK32_2
		longs[longsIdx+0] = l0
		l1 := (tmp[tmpIdx+2] & fuMASK32_8) << 14
		l1 |= tmp[tmpIdx+3] << 4
		l1 |= (tmp[tmpIdx+4] >> 6) & fuMASK32_4
		longs[longsIdx+1] = l1
		l2 := (tmp[tmpIdx+4] & fuMASK32_6) << 16
		l2 |= tmp[tmpIdx+5] << 6
		l2 |= (tmp[tmpIdx+6] >> 4) & fuMASK32_6
		longs[longsIdx+2] = l2
		l3 := (tmp[tmpIdx+6] & fuMASK32_4) << 18
		l3 |= tmp[tmpIdx+7] << 8
		l3 |= (tmp[tmpIdx+8] >> 2) & fuMASK32_8
		longs[longsIdx+3] = l3
		l4 := (tmp[tmpIdx+8] & fuMASK32_2) << 20
		l4 |= tmp[tmpIdx+9] << 10
		l4 |= tmp[tmpIdx+10]
		longs[longsIdx+4] = l4
	}
	return nil
}

func forUtilDecode23(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := forUtilSplitLongs(in, 46, longs, 9, 23, fuMASK32_23, tmp, 0, fuMASK32_9); err != nil {
		return err
	}
	for iter, tmpIdx, longsIdx := 0, 0, 46; iter < 2; iter, tmpIdx, longsIdx = iter+1, tmpIdx+23, longsIdx+9 {
		l0 := tmp[tmpIdx+0] << 14
		l0 |= tmp[tmpIdx+1] << 5
		l0 |= (tmp[tmpIdx+2] >> 4) & fuMASK32_5
		longs[longsIdx+0] = l0
		l1 := (tmp[tmpIdx+2] & fuMASK32_4) << 19
		l1 |= tmp[tmpIdx+3] << 10
		l1 |= tmp[tmpIdx+4] << 1
		l1 |= (tmp[tmpIdx+5] >> 8) & fuMASK32_1
		longs[longsIdx+1] = l1
		l2 := (tmp[tmpIdx+5] & fuMASK32_8) << 15
		l2 |= tmp[tmpIdx+6] << 6
		l2 |= (tmp[tmpIdx+7] >> 3) & fuMASK32_6
		longs[longsIdx+2] = l2
		l3 := (tmp[tmpIdx+7] & fuMASK32_3) << 20
		l3 |= tmp[tmpIdx+8] << 11
		l3 |= tmp[tmpIdx+9] << 2
		l3 |= (tmp[tmpIdx+10] >> 7) & fuMASK32_2
		longs[longsIdx+3] = l3
		l4 := (tmp[tmpIdx+10] & fuMASK32_7) << 16
		l4 |= tmp[tmpIdx+11] << 7
		l4 |= (tmp[tmpIdx+12] >> 2) & fuMASK32_7
		longs[longsIdx+4] = l4
		l5 := (tmp[tmpIdx+12] & fuMASK32_2) << 21
		l5 |= tmp[tmpIdx+13] << 12
		l5 |= tmp[tmpIdx+14] << 3
		l5 |= (tmp[tmpIdx+15] >> 6) & fuMASK32_3
		longs[longsIdx+5] = l5
		l6 := (tmp[tmpIdx+15] & fuMASK32_6) << 17
		l6 |= tmp[tmpIdx+16] << 8
		l6 |= (tmp[tmpIdx+17] >> 1) & fuMASK32_8
		longs[longsIdx+6] = l6
		l7 := (tmp[tmpIdx+17] & fuMASK32_1) << 22
		l7 |= tmp[tmpIdx+18] << 13
		l7 |= tmp[tmpIdx+19] << 4
		l7 |= (tmp[tmpIdx+20] >> 5) & fuMASK32_4
		longs[longsIdx+7] = l7
		l8 := (tmp[tmpIdx+20] & fuMASK32_5) << 18
		l8 |= tmp[tmpIdx+21] << 9
		l8 |= tmp[tmpIdx+22]
		longs[longsIdx+8] = l8
	}
	return nil
}

func forUtilDecode24(in store.IndexInput, tmp []int64, longs []int64) error {
	if err := forUtilSplitLongs(in, 48, longs, 8, 24, fuMASK32_24, tmp, 0, fuMASK32_8); err != nil {
		return err
	}
	for iter, tmpIdx, longsIdx := 0, 0, 48; iter < 16; iter, tmpIdx, longsIdx = iter+1, tmpIdx+3, longsIdx+1 {
		l0 := tmp[tmpIdx+0] << 16
		l0 |= tmp[tmpIdx+1] << 8
		l0 |= tmp[tmpIdx+2]
		longs[longsIdx+0] = l0
	}
	return nil
}
