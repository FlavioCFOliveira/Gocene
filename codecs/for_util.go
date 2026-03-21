// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/java/org/apache/lucene/codecs/lucene104/ForUtil.java
// Purpose: Frame of Reference encoding/decoding for 256 integers

package codecs

import (
	"encoding/binary"
	"errors"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// ForUtilBlockSize is the number of integers per block (256)
const ForUtilBlockSize = 256

// ForUtilBlockSizeLog2 is log2 of the block size (8)
const ForUtilBlockSizeLog2 = 8

// ForUtil provides Frame of Reference encoding/decoding.
// It encodes multiple integers using bit packing for SIMD-like speedups.
// Buffers are pre-allocated to avoid heap allocations during encode/decode.
type ForUtil struct {
	// tmp is a reusable buffer for encoding/decoding operations
	// Sized for the largest tmp usage (120 ints for decode15)
	tmp []int32

	// encodeBuf is a reusable buffer for encoding (256 ints)
	encodeBuf []int32

	// byteBuf is a reusable 4-byte buffer for reading/writing
	byteBuf []byte

	// decodeSlowBuf is a reusable buffer for decodeSlow (max 256 ints)
	decodeSlowBuf []int32
}

// NewForUtil creates a new ForUtil instance with pre-allocated buffers
// to eliminate heap allocations during encode/decode operations.
func NewForUtil() *ForUtil {
	return &ForUtil{
		tmp:           make([]int32, 256), // Largest tmp usage
		encodeBuf:     make([]int32, 256), // For block encoding
		byteBuf:       make([]byte, 4),    // Reusable byte buffer
		decodeSlowBuf: make([]int32, 256), // For decodeSlow
	}
}

// ForUtilNumBytes returns the number of bytes required to encode 256 integers
// of the given bits per value
func ForUtilNumBytes(bitsPerValue int) int {
	return bitsPerValue << (ForUtilBlockSizeLog2 - 3)
}

// Encode encodes 256 integers from ints into out using the specified bits per value
func (f *ForUtil) Encode(ints []int32, bitsPerValue int, out store.IndexOutput) error {
	if len(ints) < ForUtilBlockSize {
		return errors.New("ints array must have at least 256 elements")
	}

	// Use pre-allocated buffer instead of allocating new one
	data := f.encodeBuf
	copy(data, ints)

	var nextPrimitive int
	if bitsPerValue <= 8 {
		nextPrimitive = 8
		f.collapse8(data)
	} else if bitsPerValue <= 16 {
		nextPrimitive = 16
		f.collapse16(data)
	} else {
		nextPrimitive = 32
	}

	return f.encodeInternal(data, bitsPerValue, nextPrimitive, out)
}

// Decode decodes 256 integers from in into ints using the specified bits per value
func (f *ForUtil) Decode(bitsPerValue int, in store.IndexInput, ints []int64) error {
	if len(ints) < ForUtilBlockSize {
		return errors.New("ints array must have at least 256 elements")
	}

	switch bitsPerValue {
	case 1:
		if err := f.decode1(in, ints); err != nil {
			return err
		}
		f.expand8(ints)
	case 2:
		if err := f.decode2(in, ints); err != nil {
			return err
		}
		f.expand8(ints)
	case 3:
		if err := f.decode3(in, ints); err != nil {
			return err
		}
		f.expand8(ints)
	case 4:
		if err := f.decode4(in, ints); err != nil {
			return err
		}
		f.expand8(ints)
	case 5:
		if err := f.decode5(in, ints); err != nil {
			return err
		}
		f.expand8(ints)
	case 6:
		if err := f.decode6(in, ints); err != nil {
			return err
		}
		f.expand8(ints)
	case 7:
		if err := f.decode7(in, ints); err != nil {
			return err
		}
		f.expand8(ints)
	case 8:
		if err := f.decode8(in, ints); err != nil {
			return err
		}
		f.expand8(ints)
	case 9:
		if err := f.decode9(in, ints); err != nil {
			return err
		}
		f.expand16(ints)
	case 10:
		if err := f.decode10(in, ints); err != nil {
			return err
		}
		f.expand16(ints)
	case 11:
		if err := f.decode11(in, ints); err != nil {
			return err
		}
		f.expand16(ints)
	case 12:
		if err := f.decode12(in, ints); err != nil {
			return err
		}
		f.expand16(ints)
	case 13:
		if err := f.decode13(in, ints); err != nil {
			return err
		}
		f.expand16(ints)
	case 14:
		if err := f.decode14(in, ints); err != nil {
			return err
		}
		f.expand16(ints)
	case 15:
		if err := f.decode15(in, ints); err != nil {
			return err
		}
		f.expand16(ints)
	case 16:
		if err := f.decode16(in, ints); err != nil {
			return err
		}
		f.expand16(ints)
	default:
		if err := f.decodeSlow(bitsPerValue, in, ints); err != nil {
			return err
		}
	}

	return nil
}

// collapse8 collapses 4 bytes into 1 int (256 ints -> 64 ints)
func (f *ForUtil) collapse8(arr []int32) {
	for i := 0; i < 64; i++ {
		arr[i] = (arr[i] << 24) | (arr[64+i] << 16) | (arr[128+i] << 8) | arr[192+i]
	}
}

// expand8 expands 1 int into 4 bytes (64 ints -> 256 ints)
func (f *ForUtil) expand8(ints []int64) {
	// Process in reverse to avoid overwriting
	for i := 63; i >= 0; i-- {
		v := ints[i]
		ints[192+i] = v & 0xFF
		ints[128+i] = (v >> 8) & 0xFF
		ints[64+i] = (v >> 16) & 0xFF
		ints[i] = (v >> 24) & 0xFF
	}
}

// collapse16 collapses 2 shorts into 1 int (256 ints -> 128 ints)
func (f *ForUtil) collapse16(arr []int32) {
	for i := 0; i < 128; i++ {
		arr[i] = (arr[i] << 16) | (arr[128+i] & 0xFFFF)
	}
}

// expand16 expands 1 int into 2 shorts (128 ints -> 256 ints)
func (f *ForUtil) expand16(ints []int64) {
	// Process in reverse to avoid overwriting
	for i := 127; i >= 0; i-- {
		v := ints[i]
		ints[128+i] = v & 0xFFFF
		ints[i] = (v >> 16) & 0xFFFF
	}
}

// encodeInternal performs the actual bit packing encoding
func (f *ForUtil) encodeInternal(ints []int32, bitsPerValue, primitiveSize int, out store.IndexOutput) error {
	numInts := ForUtilBlockSize * primitiveSize / 32
	numIntsPerShift := bitsPerValue * 8

	// Clear tmp array
	for i := range f.tmp {
		f.tmp[i] = 0
	}

	idx := 0
	shift := primitiveSize - bitsPerValue
	for i := 0; i < numIntsPerShift; i++ {
		f.tmp[i] = ints[idx] << shift
		idx++
	}
	for shift = shift - bitsPerValue; shift >= 0; shift -= bitsPerValue {
		for i := 0; i < numIntsPerShift; i++ {
			f.tmp[i] |= ints[idx] << shift
			idx++
		}
	}

	remainingBitsPerInt := shift + bitsPerValue
	var maskRemainingBitsPerInt int32
	if primitiveSize == 8 {
		maskRemainingBitsPerInt = mask8(remainingBitsPerInt)
	} else if primitiveSize == 16 {
		maskRemainingBitsPerInt = mask16(remainingBitsPerInt)
	} else {
		maskRemainingBitsPerInt = mask32(remainingBitsPerInt)
	}

	tmpIdx := 0
	remainingBitsPerValue := bitsPerValue
	for idx < numInts {
		if remainingBitsPerValue >= remainingBitsPerInt {
			remainingBitsPerValue -= remainingBitsPerInt
			f.tmp[tmpIdx] |= (ints[idx] >> uint(remainingBitsPerValue)) & maskRemainingBitsPerInt
			if remainingBitsPerValue == 0 {
				idx++
				remainingBitsPerValue = bitsPerValue
			}
		} else {
			var mask1, mask2 int32
			if primitiveSize == 8 {
				mask1 = mask8(remainingBitsPerValue)
				mask2 = mask8(remainingBitsPerInt - remainingBitsPerValue)
			} else if primitiveSize == 16 {
				mask1 = mask16(remainingBitsPerValue)
				mask2 = mask16(remainingBitsPerInt - remainingBitsPerValue)
			} else {
				mask1 = mask32(remainingBitsPerValue)
				mask2 = mask32(remainingBitsPerInt - remainingBitsPerValue)
			}
			f.tmp[tmpIdx] |= (ints[idx] & mask1) << uint(remainingBitsPerInt-remainingBitsPerValue)
			idx++
			remainingBitsPerValue = bitsPerValue - remainingBitsPerInt + remainingBitsPerValue
			f.tmp[tmpIdx] |= (ints[idx] >> uint(remainingBitsPerValue)) & mask2
			tmpIdx++
		}
	}

	// Write the packed data using pre-allocated buffer
	for i := 0; i < numIntsPerShift; i++ {
		binary.BigEndian.PutUint32(f.byteBuf, uint32(f.tmp[i]))
		if err := out.WriteBytes(f.byteBuf); err != nil {
			return err
		}
	}

	return nil
}

// decodeSlow handles bits per value > 16
func (f *ForUtil) decodeSlow(bitsPerValue int, in store.IndexInput, ints []int64) error {
	numInts := bitsPerValue << 3
	mask := mask32(bitsPerValue)

	// Read packed integers using pre-allocated buffers
	tmp := f.decodeSlowBuf[:numInts]
	for i := 0; i < numInts; i++ {
		if err := in.ReadBytes(f.byteBuf); err != nil {
			return err
		}
		tmp[i] = int32(binary.BigEndian.Uint32(f.byteBuf))
	}

	// Unpack first numInts values
	for i := 0; i < numInts; i++ {
		ints[i] = int64(tmp[i]>>uint(32-bitsPerValue)) & int64(mask)
	}

	// Handle remaining values
	remainingBitsPerInt := 32 - bitsPerValue
	maskRemaining := mask32(remainingBitsPerInt)

	tmpIdx := 0
	remainingBits := remainingBitsPerInt
	for intsIdx := numInts; intsIdx < ForUtilBlockSize; intsIdx++ {
		b := bitsPerValue - remainingBits
		l := int64(tmp[tmpIdx]&mask32(remainingBits)) << uint(b)
		tmpIdx++
		for b >= remainingBitsPerInt {
			b -= remainingBitsPerInt
			l |= int64(tmp[tmpIdx]&maskRemaining) << uint(b)
			tmpIdx++
		}
		if b > 0 {
			l |= int64((tmp[tmpIdx] >> uint(remainingBitsPerInt-b)) & mask32(b))
			remainingBits = remainingBitsPerInt - b
		} else {
			remainingBits = remainingBitsPerInt
		}
		ints[intsIdx] = l
	}

	return nil
}

// decode1-16 handle specific bits per value cases
func (f *ForUtil) decode1(in store.IndexInput, ints []int64) error {
	for i := 0; i < 8; i++ {
		if err := in.ReadBytes(f.byteBuf); err != nil {
			return err
		}
		v := binary.BigEndian.Uint32(f.byteBuf)
		ints[i] = int64(v >> 7)
	}
	return nil
}

func (f *ForUtil) decode2(in store.IndexInput, ints []int64) error {
	for i := 0; i < 16; i++ {
		if err := in.ReadBytes(f.byteBuf); err != nil {
			return err
		}
		v := binary.BigEndian.Uint32(f.byteBuf)
		ints[i] = int64(v >> 6)
	}
	return nil
}

func (f *ForUtil) decode3(in store.IndexInput, ints []int64) error {
	// Read 24 ints (72 bytes)
	for i := 0; i < 24; i++ {
		if err := in.ReadBytes(f.byteBuf); err != nil {
			return err
		}
		f.tmp[i] = int32(binary.BigEndian.Uint32(f.byteBuf))
	}

	// Process 8 iterations
	for iter := 0; iter < 8; iter++ {
		tmpIdx := iter * 3
		intsIdx := iter * 2
		l0 := (f.tmp[tmpIdx] >> 5) & 0x7
		l0 |= ((f.tmp[tmpIdx+1] >> 1) & 0x1)
		ints[48+intsIdx] = int64(l0)
		l1 := (f.tmp[tmpIdx+1] & 0x1) << 2
		l1 |= (f.tmp[tmpIdx+2] >> 0) & 0x3
		ints[48+intsIdx+1] = int64(l1)
	}

	// First 48 values
	for i := 0; i < 48; i++ {
		ints[i] = int64((f.tmp[i] >> 5) & 0x7)
	}

	return nil
}

func (f *ForUtil) decode4(in store.IndexInput, ints []int64) error {
	for i := 0; i < 32; i++ {
		if err := in.ReadBytes(f.byteBuf); err != nil {
			return err
		}
		v := binary.BigEndian.Uint32(f.byteBuf)
		ints[i] = int64(v >> 4)
	}
	return nil
}

func (f *ForUtil) decode5(in store.IndexInput, ints []int64) error {
	for i := 0; i < 40; i++ {
		if err := in.ReadBytes(f.byteBuf); err != nil {
			return err
		}
		f.tmp[i] = int32(binary.BigEndian.Uint32(f.byteBuf))
	}

	for iter := 0; iter < 8; iter++ {
		tmpIdx := iter * 5
		intsIdx := iter * 3
		l0 := (f.tmp[tmpIdx] >> 3) & 0x1F
		l0 |= ((f.tmp[tmpIdx+1] >> 1) & 0x3)
		ints[40+intsIdx] = int64(l0)
		l1 := (f.tmp[tmpIdx+1] & 0x1) << 4
		l1 |= (f.tmp[tmpIdx+2] >> 0) & 0xF
		l1 |= ((f.tmp[tmpIdx+3] >> 2) & 0x1)
		ints[40+intsIdx+1] = int64(l1)
		l2 := (f.tmp[tmpIdx+3] & 0x3) << 3
		l2 |= (f.tmp[tmpIdx+4] >> 0) & 0x7
		ints[40+intsIdx+2] = int64(l2)
	}

	for i := 0; i < 40; i++ {
		ints[i] = int64((f.tmp[i] >> 3) & 0x1F)
	}

	return nil
}

func (f *ForUtil) decode6(in store.IndexInput, ints []int64) error {
	for i := 0; i < 48; i++ {
		if err := in.ReadBytes(f.byteBuf); err != nil {
			return err
		}
		f.tmp[i] = int32(binary.BigEndian.Uint32(f.byteBuf))
	}

	for iter := 0; iter < 16; iter++ {
		tmpIdx := iter * 3
		intsIdx := iter
		l0 := (f.tmp[tmpIdx] >> 2) & 0x3F
		l0 |= ((f.tmp[tmpIdx+1] >> 2) & 0x3) << 4
		l0 |= ((f.tmp[tmpIdx+2] >> 2) & 0x3) << 2
		ints[48+intsIdx] = int64(l0)
	}

	for i := 0; i < 48; i++ {
		ints[i] = int64((f.tmp[i] >> 2) & 0x3F)
	}

	return nil
}

func (f *ForUtil) decode7(in store.IndexInput, ints []int64) error {
	for i := 0; i < 56; i++ {
		if err := in.ReadBytes(f.byteBuf); err != nil {
			return err
		}
		f.tmp[i] = int32(binary.BigEndian.Uint32(f.byteBuf))
	}

	for iter := 0; iter < 8; iter++ {
		tmpIdx := iter * 7
		intsIdx := iter
		l0 := (f.tmp[tmpIdx] >> 1) & 0x7F
		l0 |= ((f.tmp[tmpIdx+1] >> 1) & 0x1) << 6
		l0 |= ((f.tmp[tmpIdx+2] >> 1) & 0x1) << 5
		l0 |= ((f.tmp[tmpIdx+3] >> 1) & 0x1) << 4
		l0 |= ((f.tmp[tmpIdx+4] >> 1) & 0x1) << 3
		l0 |= ((f.tmp[tmpIdx+5] >> 1) & 0x1) << 2
		l0 |= ((f.tmp[tmpIdx+6] >> 1) & 0x1) << 1
		ints[56+intsIdx] = int64(l0)
	}

	for i := 0; i < 56; i++ {
		ints[i] = int64((f.tmp[i] >> 1) & 0x7F)
	}

	return nil
}

func (f *ForUtil) decode8(in store.IndexInput, ints []int64) error {
	for i := 0; i < 64; i++ {
		if err := in.ReadBytes(f.byteBuf); err != nil {
			return err
		}
		ints[i] = int64(binary.BigEndian.Uint32(f.byteBuf))
	}
	return nil
}

func (f *ForUtil) decode9(in store.IndexInput, ints []int64) error {
	for i := 0; i < 72; i++ {
		if err := in.ReadBytes(f.byteBuf); err != nil {
			return err
		}
		f.tmp[i] = int32(binary.BigEndian.Uint32(f.byteBuf))
	}

	for iter := 0; iter < 8; iter++ {
		tmpIdx := iter * 9
		intsIdx := iter * 7
		l0 := (f.tmp[tmpIdx] >> 7) & 0x1FF
		l0 |= ((f.tmp[tmpIdx+1] >> 5) & 0x3) << 2
		ints[72+intsIdx] = int64(l0)
		l1 := (f.tmp[tmpIdx+1] & 0x1F) << 4
		l1 |= ((f.tmp[tmpIdx+2] >> 3) & 0xF)
		ints[72+intsIdx+1] = int64(l1)
		l2 := (f.tmp[tmpIdx+2] & 0x7) << 6
		l2 |= ((f.tmp[tmpIdx+3] >> 1) & 0x3F)
		ints[72+intsIdx+2] = int64(l2)
		l3 := ((f.tmp[tmpIdx+3] & 0x1) << 8)
		l3 |= (f.tmp[tmpIdx+4] >> 0) & 0xFF
		l3 |= ((f.tmp[tmpIdx+5] >> 6) & 0x1)
		ints[72+intsIdx+3] = int64(l3)
		l4 := (f.tmp[tmpIdx+5] & 0x3F) << 3
		l4 |= ((f.tmp[tmpIdx+6] >> 4) & 0x7)
		ints[72+intsIdx+4] = int64(l4)
		l5 := (f.tmp[tmpIdx+6] & 0xF) << 5
		l5 |= ((f.tmp[tmpIdx+7] >> 2) & 0x1F)
		ints[72+intsIdx+5] = int64(l5)
		l6 := (f.tmp[tmpIdx+7] & 0x3) << 7
		l6 |= (f.tmp[tmpIdx+8] >> 0) & 0x7F
		ints[72+intsIdx+6] = int64(l6)
	}

	for i := 0; i < 72; i++ {
		ints[i] = int64((f.tmp[i] >> 7) & 0x1FF)
	}

	return nil
}

func (f *ForUtil) decode10(in store.IndexInput, ints []int64) error {
	for i := 0; i < 80; i++ {
		if err := in.ReadBytes(f.byteBuf); err != nil {
			return err
		}
		f.tmp[i] = int32(binary.BigEndian.Uint32(f.byteBuf))
	}

	for iter := 0; iter < 16; iter++ {
		tmpIdx := iter * 5
		intsIdx := iter * 3
		l0 := (f.tmp[tmpIdx] >> 6) & 0x3FF
		l0 |= ((f.tmp[tmpIdx+1] >> 2) & 0xF) << 4
		ints[80+intsIdx] = int64(l0)
		l1 := (f.tmp[tmpIdx+1] & 0x3) << 8
		l1 |= (f.tmp[tmpIdx+2] >> 0) & 0xFF
		l1 |= ((f.tmp[tmpIdx+3] >> 4) & 0x3)
		ints[80+intsIdx+1] = int64(l1)
		l2 := (f.tmp[tmpIdx+3] & 0xF) << 6
		l2 |= (f.tmp[tmpIdx+4] >> 0) & 0x3F
		ints[80+intsIdx+2] = int64(l2)
	}

	for i := 0; i < 80; i++ {
		ints[i] = int64((f.tmp[i] >> 6) & 0x3FF)
	}

	return nil
}

func (f *ForUtil) decode11(in store.IndexInput, ints []int64) error {
	for i := 0; i < 88; i++ {
		if err := in.ReadBytes(f.byteBuf); err != nil {
			return err
		}
		f.tmp[i] = int32(binary.BigEndian.Uint32(f.byteBuf))
	}

	for iter := 0; iter < 8; iter++ {
		tmpIdx := iter * 11
		intsIdx := iter * 5
		l0 := (f.tmp[tmpIdx] >> 5) & 0x7FF
		l0 |= (f.tmp[tmpIdx+1] & 0x1F) << 6
		l0 |= ((f.tmp[tmpIdx+2] >> 4) & 0x1)
		ints[88+intsIdx] = int64(l0)
		l1 := (f.tmp[tmpIdx+2] & 0xF) << 7
		l1 |= (f.tmp[tmpIdx+3] & 0x7F) << 2
		l1 |= ((f.tmp[tmpIdx+4] >> 3) & 0x3)
		ints[88+intsIdx+1] = int64(l1)
		l2 := (f.tmp[tmpIdx+4] & 0x7) << 8
		l2 |= (f.tmp[tmpIdx+5] & 0xFF) << 3
		l2 |= ((f.tmp[tmpIdx+6] >> 2) & 0x7)
		ints[88+intsIdx+2] = int64(l2)
		l3 := (f.tmp[tmpIdx+6] & 0x3) << 9
		l3 |= (f.tmp[tmpIdx+7] & 0xFF) << 4
		l3 |= ((f.tmp[tmpIdx+8] >> 1) & 0xF)
		ints[88+intsIdx+3] = int64(l3)
		l4 := (f.tmp[tmpIdx+8] & 0x1) << 10
		l4 |= (f.tmp[tmpIdx+9] & 0xFF) << 5
		l4 |= (f.tmp[tmpIdx+10] & 0x1F)
		ints[88+intsIdx+4] = int64(l4)
	}

	for i := 0; i < 88; i++ {
		ints[i] = int64((f.tmp[i] >> 5) & 0x7FF)
	}

	return nil
}

func (f *ForUtil) decode12(in store.IndexInput, ints []int64) error {
	for i := 0; i < 96; i++ {
		if err := in.ReadBytes(f.byteBuf); err != nil {
			return err
		}
		f.tmp[i] = int32(binary.BigEndian.Uint32(f.byteBuf))
	}

	for iter := 0; iter < 32; iter++ {
		tmpIdx := iter * 3
		intsIdx := iter
		l0 := (f.tmp[tmpIdx] >> 4) & 0xFFF
		l0 |= (f.tmp[tmpIdx+1] & 0xF) << 8
		l0 |= (f.tmp[tmpIdx+2] & 0xF) << 4
		ints[96+intsIdx] = int64(l0)
	}

	for i := 0; i < 96; i++ {
		ints[i] = int64((f.tmp[i] >> 4) & 0xFFF)
	}

	return nil
}

func (f *ForUtil) decode13(in store.IndexInput, ints []int64) error {
	for i := 0; i < 104; i++ {
		if err := in.ReadBytes(f.byteBuf); err != nil {
			return err
		}
		f.tmp[i] = int32(binary.BigEndian.Uint32(f.byteBuf))
	}

	for iter := 0; iter < 8; iter++ {
		tmpIdx := iter * 13
		intsIdx := iter * 3
		l0 := (f.tmp[tmpIdx] >> 3) & 0x1FFF
		l0 |= (f.tmp[tmpIdx+1] & 0x7) << 10
		l0 |= (f.tmp[tmpIdx+2] & 0x7) << 7
		l0 |= (f.tmp[tmpIdx+3] & 0x7) << 4
		l0 |= ((f.tmp[tmpIdx+4] >> 2) & 0x1)
		ints[104+intsIdx] = int64(l0)
		l1 := (f.tmp[tmpIdx+4] & 0x3) << 11
		l1 |= (f.tmp[tmpIdx+5] & 0xFF) << 8
		l1 |= (f.tmp[tmpIdx+6] & 0xFF) << 5
		l1 |= (f.tmp[tmpIdx+7] & 0xFF) << 2
		l1 |= ((f.tmp[tmpIdx+8] >> 1) & 0x3)
		ints[104+intsIdx+1] = int64(l1)
		l2 := (f.tmp[tmpIdx+8] & 0x1) << 12
		l2 |= (f.tmp[tmpIdx+9] & 0xFF) << 9
		l2 |= (f.tmp[tmpIdx+10] & 0xFF) << 6
		l2 |= (f.tmp[tmpIdx+11] & 0xFF) << 3
		l2 |= (f.tmp[tmpIdx+12] & 0x7)
		ints[104+intsIdx+2] = int64(l2)
	}

	for i := 0; i < 104; i++ {
		ints[i] = int64((f.tmp[i] >> 3) & 0x1FFF)
	}

	return nil
}

func (f *ForUtil) decode14(in store.IndexInput, ints []int64) error {
	for i := 0; i < 112; i++ {
		if err := in.ReadBytes(f.byteBuf); err != nil {
			return err
		}
		f.tmp[i] = int32(binary.BigEndian.Uint32(f.byteBuf))
	}

	for iter := 0; iter < 16; iter++ {
		tmpIdx := iter * 7
		intsIdx := iter
		l0 := (f.tmp[tmpIdx] >> 2) & 0x3FFF
		l0 |= (f.tmp[tmpIdx+1] & 0x3) << 12
		l0 |= (f.tmp[tmpIdx+2] & 0x3) << 10
		l0 |= (f.tmp[tmpIdx+3] & 0x3) << 8
		l0 |= (f.tmp[tmpIdx+4] & 0x3) << 6
		l0 |= (f.tmp[tmpIdx+5] & 0x3) << 4
		l0 |= (f.tmp[tmpIdx+6] & 0x3) << 2
		ints[112+intsIdx] = int64(l0)
	}

	for i := 0; i < 112; i++ {
		ints[i] = int64((f.tmp[i] >> 2) & 0x3FFF)
	}

	return nil
}

func (f *ForUtil) decode15(in store.IndexInput, ints []int64) error {
	for i := 0; i < 120; i++ {
		if err := in.ReadBytes(f.byteBuf); err != nil {
			return err
		}
		f.tmp[i] = int32(binary.BigEndian.Uint32(f.byteBuf))
	}

	for iter := 0; iter < 8; iter++ {
		tmpIdx := iter * 15
		intsIdx := iter
		l0 := (f.tmp[tmpIdx] >> 1) & 0x7FFF
		l0 |= (f.tmp[tmpIdx+1] & 0x1) << 14
		l0 |= (f.tmp[tmpIdx+2] & 0x1) << 13
		l0 |= (f.tmp[tmpIdx+3] & 0x1) << 12
		l0 |= (f.tmp[tmpIdx+4] & 0x1) << 11
		l0 |= (f.tmp[tmpIdx+5] & 0x1) << 10
		l0 |= (f.tmp[tmpIdx+6] & 0x1) << 9
		l0 |= (f.tmp[tmpIdx+7] & 0x1) << 8
		l0 |= (f.tmp[tmpIdx+8] & 0x1) << 7
		l0 |= (f.tmp[tmpIdx+9] & 0x1) << 6
		l0 |= (f.tmp[tmpIdx+10] & 0x1) << 5
		l0 |= (f.tmp[tmpIdx+11] & 0x1) << 4
		l0 |= (f.tmp[tmpIdx+12] & 0x1) << 3
		l0 |= (f.tmp[tmpIdx+13] & 0x1) << 2
		l0 |= (f.tmp[tmpIdx+14] & 0x1) << 1
		ints[120+intsIdx] = int64(l0)
	}

	for i := 0; i < 120; i++ {
		ints[i] = int64((f.tmp[i] >> 1) & 0x7FFF)
	}

	return nil
}

func (f *ForUtil) decode16(in store.IndexInput, ints []int64) error {
	for i := 0; i < 128; i++ {
		if err := in.ReadBytes(f.byteBuf); err != nil {
			return err
		}
		ints[i] = int64(binary.BigEndian.Uint32(f.byteBuf))
	}
	return nil
}

// Mask functions
func mask8(bits int) int32 {
	if bits <= 0 {
		return 0
	}
	if bits >= 8 {
		return -1 // 0xFFFFFFFF as signed int32
	}
	m := (1 << bits) - 1
	// Expand to all bytes
	return int32(m | (m << 8) | (m << 16) | (m << 24))
}

func mask16(bits int) int32 {
	if bits <= 0 {
		return 0
	}
	if bits >= 16 {
		return -1 // 0xFFFFFFFF as signed int32
	}
	m := (1 << bits) - 1
	// Expand to both shorts
	return int32(m | (m << 16))
}

func mask32(bits int) int32 {
	if bits <= 0 {
		return 0
	}
	if bits >= 32 {
		return -1 // 0xFFFFFFFF as signed int32
	}
	return int32((1 << bits) - 1)
}

// expandMask16 expands a 16-bit mask to 32 bits
func expandMask16(mask16 int) int {
	return mask16 | (mask16 << 16)
}

// expandMask8 expands an 8-bit mask to 32 bits
func expandMask8(mask8 int) int {
	return expandMask16(mask8 | (mask8 << 8))
}
