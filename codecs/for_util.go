// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/java/org/apache/lucene/codecs/lucene104/ForUtil.java
// Purpose: Frame of Reference encoding/decoding for 256 integers.

package codecs

import (
	"encoding/binary"
	"errors"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// ForUtilBlockSize is the number of integers per block (256).
const ForUtilBlockSize = 256

// ForUtilBlockSizeLog2 is log2 of the block size (8).
const ForUtilBlockSizeLog2 = 8

// ForUtil provides Frame of Reference encoding/decoding.
// Encodes multiple integers using bit packing for SIMD-like speedups.
// Buffers are pre-allocated to avoid heap allocations during encode/decode.
type ForUtil struct {
	// ints32 is the primary decode output buffer (256 int32 collapsed values).
	ints32 []int32
	// scratch is a secondary buffer used by recombination loops in decode*.
	scratch []int32
}

// NewForUtil creates a new ForUtil instance with pre-allocated buffers.
func NewForUtil() *ForUtil {
	return &ForUtil{
		ints32:  make([]int32, ForUtilBlockSize),
		scratch: make([]int32, ForUtilBlockSize),
	}
}

// ForUtilNumBytes returns the number of bytes required to encode 256 integers
// of the given bits per value.
func ForUtilNumBytes(bitsPerValue int) int {
	return bitsPerValue << (ForUtilBlockSizeLog2 - 3)
}

// ---------- mask helpers (matching Java static methods) ----------

// expandMask16 expands a 16-bit mask pattern to fill both halves of a 32-bit int.
func expandMask16(m int32) int32 {
	return m | (m << 16)
}

// expandMask8 expands an 8-bit mask pattern to fill all four bytes of a 32-bit int.
func expandMask8(m int32) int32 {
	return expandMask16(m | (m << 8))
}

// mask32 returns a mask of the lowest bitsPerValue bits (single 32-bit word).
func mask32(bitsPerValue int) int32 {
	if bitsPerValue <= 0 {
		return 0
	}
	if bitsPerValue >= 32 {
		return -1
	}
	return int32((1 << bitsPerValue) - 1)
}

// mask16 returns an expanded 16-bit mask (both shorts populated).
func mask16(bitsPerValue int) int32 {
	if bitsPerValue <= 0 {
		return 0
	}
	if bitsPerValue >= 16 {
		return -1
	}
	return expandMask16(int32((1 << bitsPerValue) - 1))
}

// mask8 returns an expanded 8-bit mask (all four bytes populated).
func mask8(bitsPerValue int) int32 {
	if bitsPerValue <= 0 {
		return 0
	}
	if bitsPerValue >= 8 {
		return -1
	}
	return expandMask8(int32((1 << bitsPerValue) - 1))
}

// Pre-computed mask tables (indices 0..N-1).
var masks8 [8]int32
var masks16 [16]int32
var masks32 [32]int32

func init() {
	for i := 0; i < 8; i++ {
		masks8[i] = mask8(i)
	}
	for i := 0; i < 16; i++ {
		masks16[i] = mask16(i)
	}
	for i := 0; i < 32; i++ {
		masks32[i] = mask32(i)
	}
}

// ---------- collapse / expand ----------

// collapse8 packs four 8-bit values per slot: (256 int32) → (64 int32).
// Each group of four consecutive ints at offsets [i, 64+i, 128+i, 192+i] is
// merged into arr[i] via byte shifting.
func collapse8(arr []int32) {
	for i := 0; i < 64; i++ {
		arr[i] = (arr[i] << 24) | (arr[64+i] << 16) | (arr[128+i] << 8) | arr[192+i]
	}
}

// expand8 unpacks four 8-bit values from each slot: (64 int32) → (256 int32).
func expand8(arr []int32) {
	for i := 0; i < 64; i++ {
		v := arr[i]
		arr[i] = (v >> 24) & 0xFF
		arr[64+i] = (v >> 16) & 0xFF
		arr[128+i] = (v >> 8) & 0xFF
		arr[192+i] = v & 0xFF
	}
}

// collapse16 packs two 16-bit values per slot: (256 int32) → (128 int32).
func collapse16(arr []int32) {
	for i := 0; i < 128; i++ {
		arr[i] = (arr[i] << 16) | (arr[128+i] & 0xFFFF)
	}
}

// expand16 unpacks two 16-bit values from each slot: (128 int32) → (256 int32).
func expand16(arr []int32) {
	for i := 0; i < 128; i++ {
		v := arr[i]
		arr[i] = (v >> 16) & 0xFFFF
		arr[128+i] = v & 0xFFFF
	}
}

// ---------- Encode ----------

// Encode encodes 256 integers from ints into out using the specified bits per value.
// The caller must supply a slice of at least ForUtilBlockSize elements.
func (f *ForUtil) Encode(ints []int32, bitsPerValue int, out store.IndexOutput) error {
	if len(ints) < ForUtilBlockSize {
		return errors.New("ints array must have at least 256 elements")
	}

	// Work on a copy in ints32 to avoid mutating the caller's slice.
	copy(f.ints32, ints[:ForUtilBlockSize])

	var nextPrimitive int
	if bitsPerValue <= 8 {
		nextPrimitive = 8
		collapse8(f.ints32)
	} else if bitsPerValue <= 16 {
		nextPrimitive = 16
		collapse16(f.ints32)
	} else {
		nextPrimitive = 32
	}

	return encodeInternalForUtil(f.ints32, bitsPerValue, nextPrimitive, out)
}

// encodeInternalForUtil performs the actual bit-packing and writes packed ints to out.
// Matches ForUtil.encode(int[], int, int, DataOutput, int[]) in Java.
func encodeInternalForUtil(ints []int32, bitsPerValue, primitiveSize int, out store.IndexOutput) error {
	numInts := ForUtilBlockSize * primitiveSize / 32
	numIntsPerShift := bitsPerValue * 8

	// Scratch buffer sized to numIntsPerShift (≤ bpv*8 ≤ 31*8 = 248 ≤ 256).
	var scratch [ForUtilBlockSize]int32

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
			scratch[tmpIdx] |= (ints[idx] >> uint(remainingBitsPerValue)) & maskRemainingBitsPerInt
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
			scratch[tmpIdx] |= (ints[idx] >> uint(remainingBitsPerValue)) & mask2
			tmpIdx++
		}
	}

	// Write numIntsPerShift big-endian 32-bit words.
	var buf [4]byte
	for i := 0; i < numIntsPerShift; i++ {
		binary.BigEndian.PutUint32(buf[:], uint32(scratch[i]))
		if err := out.WriteBytes(buf[:]); err != nil {
			return err
		}
	}
	return nil
}

// ---------- splitInts (core decode primitive) ----------

// splitInts reads count ints from in, then for each shift level j in [0, maxIter]
// writes (c[cIdx+i] >>> (bShift - j*dec)) & bMask into b[count*j + i].
// Finally applies cMask to c[cIdx..cIdx+count-1].
//
// This is a direct port of PostingDecodingUtil.splitInts.
func splitInts(
	in store.IndexInput,
	count int,
	b []int32, bShift, dec int, bMask int32,
	c []int32, cIdx int, cMask int32,
) error {
	// Read count ints (big-endian 4-byte words) into c[cIdx..].
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

// ---------- Decode ----------

// Decode decodes 256 integers from in into ints using the specified bits per value.
// ints must have at least ForUtilBlockSize elements.
// Decoded values are placed as int64 (non-negative).
func (f *ForUtil) Decode(bitsPerValue int, in store.IndexInput, ints []int64) error {
	if len(ints) < ForUtilBlockSize {
		return errors.New("ints array must have at least 256 elements")
	}

	// ints32 is the primary output buffer (collapsed form);
	// scratch is the secondary scratch used by recombination loops.
	buf := f.ints32 // len 256
	sc := f.scratch // len 256

	// Each decode* call fills buf (and optionally sc) with decoded int32 values,
	// then an expand* call expands buf into all 256 slots.
	// Finally we promote to int64.

	var err error
	switch bitsPerValue {
	case 1:
		err = decode1(in, buf)
		if err == nil {
			expand8(buf)
		}
	case 2:
		err = decode2(in, buf)
		if err == nil {
			expand8(buf)
		}
	case 3:
		err = decode3(in, buf, sc)
		if err == nil {
			expand8(buf)
		}
	case 4:
		err = decode4(in, buf)
		if err == nil {
			expand8(buf)
		}
	case 5:
		err = decode5(in, buf, sc)
		if err == nil {
			expand8(buf)
		}
	case 6:
		err = decode6(in, buf, sc)
		if err == nil {
			expand8(buf)
		}
	case 7:
		err = decode7(in, buf, sc)
		if err == nil {
			expand8(buf)
		}
	case 8:
		err = decode8(in, buf)
		if err == nil {
			expand8(buf)
		}
	case 9:
		err = decode9(in, buf, sc)
		if err == nil {
			expand16(buf)
		}
	case 10:
		err = decode10(in, buf, sc)
		if err == nil {
			expand16(buf)
		}
	case 11:
		err = decode11(in, buf, sc)
		if err == nil {
			expand16(buf)
		}
	case 12:
		err = decode12(in, buf, sc)
		if err == nil {
			expand16(buf)
		}
	case 13:
		err = decode13(in, buf, sc)
		if err == nil {
			expand16(buf)
		}
	case 14:
		err = decode14(in, buf, sc)
		if err == nil {
			expand16(buf)
		}
	case 15:
		err = decode15(in, buf, sc)
		if err == nil {
			expand16(buf)
		}
	case 16:
		err = decode16(in, buf)
		if err == nil {
			expand16(buf)
		}
	default:
		err = decodeSlow(bitsPerValue, in, buf)
	}
	if err != nil {
		return err
	}

	for i := 0; i < ForUtilBlockSize; i++ {
		ints[i] = int64(uint32(buf[i])) // zero-extend to int64
	}
	return nil
}

// ---------- decodeSlow (bitsPerValue 17-31) ----------

// decodeSlow handles bits-per-value values that exceed 16.
// Matches ForUtil.decodeSlow in Java.
func decodeSlow(bitsPerValue int, in store.IndexInput, ints []int32) error {
	numInts := bitsPerValue << 3 // bitsPerValue * 8
	mask := masks32[bitsPerValue]

	// scratch holds the raw ints read from the stream.
	var scratch [256]int32

	// splitInts with dec=32 → maxIter=(bShift-1)/32=(31-bpv)/32=0 for bpv≥1.
	// So only j=0: b[i] = (c[i] >>> (32-bpv)) & mask; c remains unchanged (cMask=-1).
	if err := splitInts(in, numInts, ints, 32-bitsPerValue, 32, mask, scratch[:], 0, -1); err != nil {
		return err
	}

	remainingBitsPerInt := 32 - bitsPerValue
	mask32Remaining := masks32[remainingBitsPerInt]

	tmpIdx := 0
	remainingBits := remainingBitsPerInt
	for intsIdx := numInts; intsIdx < ForUtilBlockSize; intsIdx++ {
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

// ---------- decode1-16 ----------
//
// Each function uses splitInts to fill the "upper" slots of ints (the b array)
// from the compressed int32 data, and stores the remainder bits in a separate
// scratch buffer (c), which is then recombined via explicit OR operations.
//
// The implementation faithfully mirrors ForUtil.java, replacing Java's
// PostingDecodingUtil.splitInts with the splitInts function above.
//
// Naming convention:
//   - The first argument is the IndexInput.
//   - The second argument is the primary output buffer (ints, length 256).
//   - Where a separate scratch (tmp) buffer is needed it is the third argument.
//   - All buffers must have length >= 256.

func decode1(in store.IndexInput, ints []int32) error {
	// splitInts(8, ints, 7, 1, MASK8_1, ints, 56, MASK8_1)
	// Reads 8 ints into ints[56..63]; extracts 7 shifts of 1 bit into ints[0..55].
	return splitInts(in, 8, ints, 7, 1, masks8[1], ints, 56, masks8[1])
}

func decode2(in store.IndexInput, ints []int32) error {
	// splitInts(16, ints, 6, 2, MASK8_2, ints, 48, MASK8_2)
	return splitInts(in, 16, ints, 6, 2, masks8[2], ints, 48, masks8[2])
}

func decode3(in store.IndexInput, ints []int32, tmp []int32) error {
	// splitInts(24, ints, 5, 3, MASK8_3, tmp, 0, MASK8_2)
	if err := splitInts(in, 24, ints, 5, 3, masks8[3], tmp, 0, masks8[2]); err != nil {
		return err
	}
	for iter, tmpIdx, intsIdx := 0, 0, 48; iter < 8; iter, tmpIdx, intsIdx = iter+1, tmpIdx+3, intsIdx+2 {
		l0 := tmp[tmpIdx+0] << 1
		l0 |= int32(uint32(tmp[tmpIdx+1])>>1) & masks8[1]
		ints[intsIdx+0] = l0
		l1 := (tmp[tmpIdx+1] & masks8[1]) << 2
		l1 |= tmp[tmpIdx+2]
		ints[intsIdx+1] = l1
	}
	return nil
}

func decode4(in store.IndexInput, ints []int32) error {
	// splitInts(32, ints, 4, 4, MASK8_4, ints, 32, MASK8_4)
	return splitInts(in, 32, ints, 4, 4, masks8[4], ints, 32, masks8[4])
}

func decode5(in store.IndexInput, ints []int32, tmp []int32) error {
	// splitInts(40, ints, 3, 5, MASK8_5, tmp, 0, MASK8_3)
	if err := splitInts(in, 40, ints, 3, 5, masks8[5], tmp, 0, masks8[3]); err != nil {
		return err
	}
	for iter, tmpIdx, intsIdx := 0, 0, 40; iter < 8; iter, tmpIdx, intsIdx = iter+1, tmpIdx+5, intsIdx+3 {
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

func decode6(in store.IndexInput, ints []int32, tmp []int32) error {
	// splitInts(48, ints, 2, 6, MASK8_6, tmp, 0, MASK8_2)
	if err := splitInts(in, 48, ints, 2, 6, masks8[6], tmp, 0, masks8[2]); err != nil {
		return err
	}
	for iter, tmpIdx, intsIdx := 0, 0, 48; iter < 16; iter, tmpIdx, intsIdx = iter+1, tmpIdx+3, intsIdx+1 {
		l0 := tmp[tmpIdx+0] << 4
		l0 |= tmp[tmpIdx+1] << 2
		l0 |= tmp[tmpIdx+2]
		ints[intsIdx+0] = l0
	}
	return nil
}

func decode7(in store.IndexInput, ints []int32, tmp []int32) error {
	// splitInts(56, ints, 1, 7, MASK8_7, tmp, 0, MASK8_1)
	if err := splitInts(in, 56, ints, 1, 7, masks8[7], tmp, 0, masks8[1]); err != nil {
		return err
	}
	for iter, tmpIdx, intsIdx := 0, 0, 56; iter < 8; iter, tmpIdx, intsIdx = iter+1, tmpIdx+7, intsIdx+1 {
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

func decode8(in store.IndexInput, ints []int32) error {
	// splitInts(64, ints, 0, 8, MASK8_8(=−1), ints, 0, MASK8_8)
	// Equivalent to: pdu.in.readInts(ints, 0, 64) — just read 64 ints.
	var buf [4]byte
	for i := 0; i < 64; i++ {
		if err := in.ReadBytes(buf[:]); err != nil {
			return err
		}
		ints[i] = int32(binary.BigEndian.Uint32(buf[:]))
	}
	return nil
}

func decode9(in store.IndexInput, ints []int32, tmp []int32) error {
	// splitInts(72, ints, 7, 9, MASK16_9, tmp, 0, MASK16_7)
	if err := splitInts(in, 72, ints, 7, 9, masks16[9], tmp, 0, masks16[7]); err != nil {
		return err
	}
	for iter, tmpIdx, intsIdx := 0, 0, 72; iter < 8; iter, tmpIdx, intsIdx = iter+1, tmpIdx+9, intsIdx+7 {
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

func decode10(in store.IndexInput, ints []int32, tmp []int32) error {
	// splitInts(80, ints, 6, 10, MASK16_10, tmp, 0, MASK16_6)
	if err := splitInts(in, 80, ints, 6, 10, masks16[10], tmp, 0, masks16[6]); err != nil {
		return err
	}
	for iter, tmpIdx, intsIdx := 0, 0, 80; iter < 16; iter, tmpIdx, intsIdx = iter+1, tmpIdx+5, intsIdx+3 {
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

func decode11(in store.IndexInput, ints []int32, tmp []int32) error {
	// splitInts(88, ints, 5, 11, MASK16_11, tmp, 0, MASK16_5)
	if err := splitInts(in, 88, ints, 5, 11, masks16[11], tmp, 0, masks16[5]); err != nil {
		return err
	}
	for iter, tmpIdx, intsIdx := 0, 0, 88; iter < 8; iter, tmpIdx, intsIdx = iter+1, tmpIdx+11, intsIdx+5 {
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

func decode12(in store.IndexInput, ints []int32, tmp []int32) error {
	// splitInts(96, ints, 4, 12, MASK16_12, tmp, 0, MASK16_4)
	if err := splitInts(in, 96, ints, 4, 12, masks16[12], tmp, 0, masks16[4]); err != nil {
		return err
	}
	for iter, tmpIdx, intsIdx := 0, 0, 96; iter < 32; iter, tmpIdx, intsIdx = iter+1, tmpIdx+3, intsIdx+1 {
		l0 := tmp[tmpIdx+0] << 8
		l0 |= tmp[tmpIdx+1] << 4
		l0 |= tmp[tmpIdx+2]
		ints[intsIdx+0] = l0
	}
	return nil
}

func decode13(in store.IndexInput, ints []int32, tmp []int32) error {
	// splitInts(104, ints, 3, 13, MASK16_13, tmp, 0, MASK16_3)
	if err := splitInts(in, 104, ints, 3, 13, masks16[13], tmp, 0, masks16[3]); err != nil {
		return err
	}
	for iter, tmpIdx, intsIdx := 0, 0, 104; iter < 8; iter, tmpIdx, intsIdx = iter+1, tmpIdx+13, intsIdx+3 {
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

func decode14(in store.IndexInput, ints []int32, tmp []int32) error {
	// splitInts(112, ints, 2, 14, MASK16_14, tmp, 0, MASK16_2)
	if err := splitInts(in, 112, ints, 2, 14, masks16[14], tmp, 0, masks16[2]); err != nil {
		return err
	}
	for iter, tmpIdx, intsIdx := 0, 0, 112; iter < 16; iter, tmpIdx, intsIdx = iter+1, tmpIdx+7, intsIdx+1 {
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

func decode15(in store.IndexInput, ints []int32, tmp []int32) error {
	// splitInts(120, ints, 1, 15, MASK16_15, tmp, 0, MASK16_1)
	if err := splitInts(in, 120, ints, 1, 15, masks16[15], tmp, 0, masks16[1]); err != nil {
		return err
	}
	for iter, tmpIdx, intsIdx := 0, 0, 120; iter < 8; iter, tmpIdx, intsIdx = iter+1, tmpIdx+15, intsIdx+1 {
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

func decode16(in store.IndexInput, ints []int32) error {
	// pdu.in.readInts(ints, 0, 128)
	var buf [4]byte
	for i := 0; i < 128; i++ {
		if err := in.ReadBytes(buf[:]); err != nil {
			return err
		}
		ints[i] = int32(binary.BigEndian.Uint32(buf[:]))
	}
	return nil
}
