// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/java/org/apache/lucene/codecs/lucene104/PForUtil.java
// Purpose: Patched Frame of Reference encoding/decoding for 256 integers

package codecs

import (
	"errors"
	"math"

	"github.com/FlavioCFOliveira/Gocene/store"
)

const (
	// PForMaxExceptions is the maximum number of exceptions allowed
	PForMaxExceptions = 7
)

// PForUtil provides Patched Frame of Reference encoding/decoding.
// It encodes sequences of 256 small positive integers, using exceptions
// for values that don't fit in the majority bit width.
type PForUtil struct {
	forUtil *ForUtil
}

// NewPForUtil creates a new PForUtil instance
func NewPForUtil(forUtil *ForUtil) *PForUtil {
	return &PForUtil{
		forUtil: forUtil,
	}
}

// Encode encodes 256 integers from ints into out
func (p *PForUtil) Encode(ints []int32, out store.IndexOutput) error {
	if len(ints) < ForUtilBlockSize {
		return errors.New("ints array must have at least 256 elements")
	}

	// Histogram of bit widths
	histogram := make([]int, 32)
	maxBitsRequired := 0
	for i := 0; i < ForUtilBlockSize; i++ {
		v := ints[i]
		bits := bitsRequired(int64(v))
		histogram[bits]++
		if bits > maxBitsRequired {
			maxBitsRequired = bits
		}
	}

	// We store patch on a byte, so we can't decrease bits by more than 8
	minBits := max(0, maxBitsRequired-8)
	cumulativeExceptions := 0
	patchedBitsRequired := maxBitsRequired
	numExceptions := 0

	for b := maxBitsRequired; b >= minBits; b-- {
		if cumulativeExceptions > PForMaxExceptions {
			break
		}
		patchedBitsRequired = b
		numExceptions = cumulativeExceptions
		cumulativeExceptions += histogram[b]
	}

	maxUnpatchedValue := (1 << patchedBitsRequired) - 1
	exceptions := make([]byte, numExceptions*2)

	if numExceptions > 0 {
		exceptionCount := 0
		for i := 0; i < ForUtilBlockSize; i++ {
			if ints[i] > int32(maxUnpatchedValue) {
				exceptions[exceptionCount*2] = byte(i)
				exceptions[exceptionCount*2+1] = byte(ints[i] >> patchedBitsRequired)
				exceptionCount++
			}
		}
		if exceptionCount != numExceptions {
			return errors.New("exception count mismatch")
		}
	}

	// Make a copy of ints since we modify it
	intsCopy := make([]int32, ForUtilBlockSize)
	copy(intsCopy, ints)

	// Patch values to fit in patchedBitsRequired
	for i := 0; i < ForUtilBlockSize; i++ {
		if intsCopy[i] > int32(maxUnpatchedValue) {
			intsCopy[i] = int32(maxUnpatchedValue)
		}
	}

	if p.allEqual(intsCopy) && maxBitsRequired <= 8 {
		// All values are equal and small
		for i := 0; i < numExceptions; i++ {
			exceptions[2*i+1] = byte(int(exceptions[2*i+1]) << patchedBitsRequired)
		}
		if err := out.WriteByte(byte(numExceptions << 5)); err != nil {
			return err
		}
		if err := store.WriteVInt(out, intsCopy[0]); err != nil {
			return err
		}
	} else {
		token := (numExceptions << 5) | patchedBitsRequired
		if err := out.WriteByte(byte(token)); err != nil {
			return err
		}
		if err := p.forUtil.Encode(intsCopy, patchedBitsRequired, out); err != nil {
			return err
		}
	}

	// Write exceptions
	if err := out.WriteBytes(exceptions); err != nil {
		return err
	}

	return nil
}

// Decode decodes 256 integers from in into ints
func (p *PForUtil) Decode(in store.IndexInput, ints []int64) error {
	if len(ints) < ForUtilBlockSize {
		return errors.New("ints array must have at least 256 elements")
	}

	tokenByte, err := in.ReadByte()
	if err != nil {
		return err
	}
	token := int(byte(tokenByte))
	bitsPerValue := token & 0x1F
	numExceptions := token >> 5

	if bitsPerValue == 0 {
		// All values are the same
		val, err := store.ReadVInt(in)
		if err != nil {
			return err
		}
		for i := 0; i < ForUtilBlockSize; i++ {
			ints[i] = int64(val)
		}
	} else {
		// Decode using ForUtil
		if err := p.forUtil.Decode(bitsPerValue, in, ints); err != nil {
			return err
		}
	}

	// Apply exceptions
	for i := 0; i < numExceptions; i++ {
		idxByte, err := in.ReadByte()
		if err != nil {
			return err
		}
		idx := int(byte(idxByte))

		valByte, err := in.ReadByte()
		if err != nil {
			return err
		}
		val := int(byte(valByte))

		ints[idx] |= int64(val) << bitsPerValue
	}

	return nil
}

// PForUtilSkip skips 256 integers in the input
func PForUtilSkip(in store.IndexInput) error {
	tokenByte, err := in.ReadByte()
	if err != nil {
		return err
	}
	token := int(byte(tokenByte))
	bitsPerValue := token & 0x1F
	numExceptions := token >> 5

	if bitsPerValue == 0 {
		// Skip VInt for the repeated value
		_, err := store.ReadVLong(in)
		if err != nil {
			return err
		}
		// Skip exceptions
		if err := in.SetPosition(in.GetFilePointer() + int64(numExceptions<<1)); err != nil {
			return err
		}
	} else {
		// Skip ForUtil data
		numBytes := ForUtilNumBytes(bitsPerValue)
		if err := in.SetPosition(in.GetFilePointer() + int64(numExceptions<<1) + int64(numBytes)); err != nil {
			return err
		}
	}

	return nil
}

// allEqual returns true if all values in the array are equal
func (p *PForUtil) allEqual(l []int32) bool {
	if len(l) == 0 {
		return true
	}
	first := l[0]
	for i := 1; i < len(l); i++ {
		if l[i] != first {
			return false
		}
	}
	return true
}

// bitsRequired returns the number of bits required to represent v
func bitsRequired(v int64) int {
	if v < 0 {
		return 64
	}
	if v == 0 {
		return 0
	}
	return int(math.Floor(math.Log2(float64(v)))) + 1
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
