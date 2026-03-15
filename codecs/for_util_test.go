// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/test/org/apache/lucene/codecs/lucene104/TestForUtil.java
// Purpose: Tests Frame of Reference encoding/decoding for 256 integers

package codecs_test

import (
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// TestForUtil_EncodeDecode tests encoding and decoding of 256 integers
// using Frame of Reference compression
func TestForUtil_EncodeDecode(t *testing.T) {
	iterations := randomIntBetween(50, 1000)
	values := make([]int32, iterations*codecs.ForUtilBlockSize)

	for i := 0; i < iterations; i++ {
		bpv := randomIntBetween(1, 31)
		for j := 0; j < codecs.ForUtilBlockSize; j++ {
			values[i*codecs.ForUtilBlockSize+j] = int32(randomIntBetween(0, int(maxValue(bpv))))
		}
	}

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	var endPointer int64

	// Encode
	{
		out, err := dir.CreateOutput("test.bin", store.IOContextWrite)
		if err != nil {
			t.Fatal(err)
		}

		forUtil := codecs.NewForUtil()

		for i := 0; i < iterations; i++ {
			source := make([]int32, codecs.ForUtilBlockSize)
			var or int64
			for j := 0; j < codecs.ForUtilBlockSize; j++ {
				source[j] = values[i*codecs.ForUtilBlockSize+j]
				or |= int64(source[j])
			}
			bpv := bitsRequired(or)
			if err := out.WriteByte(byte(bpv)); err != nil {
				t.Fatal(err)
			}
			if err := forUtil.Encode(source, bpv, out); err != nil {
				t.Fatal(err)
			}
		}
		endPointer = out.GetFilePointer()
		out.Close()
	}

	// Decode
	{
		in, err := dir.OpenInput("test.bin", store.IOContextReadOnce)
		if err != nil {
			t.Fatal(err)
		}
		defer in.Close()

		forUtil := codecs.NewForUtil()

		for i := 0; i < iterations; i++ {
			bitsPerValue, err := in.ReadByte()
			if err != nil {
				t.Fatal(err)
			}
			currentFilePointer := in.GetFilePointer()

			restored := make([]int64, codecs.ForUtilBlockSize)
			if err := forUtil.Decode(int(bitsPerValue), in, restored); err != nil {
				t.Fatal(err)
			}

			ints := make([]int32, codecs.ForUtilBlockSize)
			for j := 0; j < codecs.ForUtilBlockSize; j++ {
				ints[j] = int32(restored[j])
			}

			expected := values[i*codecs.ForUtilBlockSize : (i+1)*codecs.ForUtilBlockSize]
			for j := 0; j < codecs.ForUtilBlockSize; j++ {
				if ints[j] != expected[j] {
					t.Errorf("Iteration %d, position %d: expected %d, got %d", i, j, expected[j], ints[j])
				}
			}

			expectedBytes := codecs.ForUtilNumBytes(int(bitsPerValue))
			actualBytes := in.GetFilePointer() - currentFilePointer
			if actualBytes != int64(expectedBytes) {
				t.Errorf("Expected %d bytes read, got %d", expectedBytes, actualBytes)
			}
		}

		if in.GetFilePointer() != endPointer {
			t.Errorf("Expected file pointer %d, got %d", endPointer, in.GetFilePointer())
		}
	}
}

// TestForUtil_BlockSize verifies the block size constant
func TestForUtil_BlockSize(t *testing.T) {
	if codecs.ForUtilBlockSize != 256 {
		t.Errorf("Expected BLOCK_SIZE to be 256, got %d", codecs.ForUtilBlockSize)
	}
}

// TestForUtil_NumBytes verifies the numBytes calculation
func TestForUtil_NumBytes(t *testing.T) {
	testCases := []struct {
		bpv      int
		expected int
	}{
		{1, 32},   // 1 * 256 / 8 = 32
		{8, 256},  // 8 * 256 / 8 = 256
		{16, 512}, // 16 * 256 / 8 = 512
		{32, 1024},
	}

	for _, tc := range testCases {
		result := codecs.ForUtilNumBytes(tc.bpv)
		if result != tc.expected {
			t.Errorf("bitsPerValue=%d: expected %d bytes, got %d", tc.bpv, tc.expected, result)
		}
	}
}

// TestForUtil_EncodeDecodeAllBitsPerValue tests all valid bits per value
func TestForUtil_EncodeDecodeAllBitsPerValue(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	for bpv := 1; bpv <= 31; bpv++ {
		out, err := dir.CreateOutput("test.bin", store.IOContextWrite)
		if err != nil {
			t.Fatal(err)
		}

		forUtil := codecs.NewForUtil()

		// Create test data with max value for this bits per value
		source := make([]int32, codecs.ForUtilBlockSize)
		maxVal := int32(maxValue(bpv))
		for i := 0; i < codecs.ForUtilBlockSize; i++ {
			source[i] = int32(randomIntBetween(0, int(maxVal)))
		}

		if err := out.WriteByte(byte(bpv)); err != nil {
			t.Fatal(err)
		}
		if err := forUtil.Encode(source, bpv, out); err != nil {
			t.Fatalf("Failed to encode with bpv=%d: %v", bpv, err)
		}
		out.Close()

		// Decode and verify
		in, err := dir.OpenInput("test.bin", store.IOContextReadOnce)
		if err != nil {
			t.Fatal(err)
		}

		readBpv, err := in.ReadByte()
		if err != nil {
			t.Fatal(err)
		}
		if int(readBpv) != bpv {
			t.Errorf("Expected bpv %d, got %d", bpv, readBpv)
		}

		restored := make([]int64, codecs.ForUtilBlockSize)
		if err := forUtil.Decode(bpv, in, restored); err != nil {
			t.Fatalf("Failed to decode with bpv=%d: %v", bpv, err)
		}

		for i := 0; i < codecs.ForUtilBlockSize; i++ {
			if int32(restored[i]) != source[i] {
				t.Errorf("bpv=%d, position %d: expected %d, got %d", bpv, i, source[i], restored[i])
				break
			}
		}
		in.Close()
	}
}

// TestForUtil_ZeroValues tests encoding/decoding of all zeros
func TestForUtil_ZeroValues(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	out, err := dir.CreateOutput("test.bin", store.IOContextWrite)
	if err != nil {
		t.Fatal(err)
	}

	forUtil := codecs.NewForUtil()

	// All zeros
	source := make([]int32, codecs.ForUtilBlockSize)
	bpv := 1

	if err := out.WriteByte(byte(bpv)); err != nil {
		t.Fatal(err)
	}
	if err := forUtil.Encode(source, bpv, out); err != nil {
		t.Fatal(err)
	}
	out.Close()

	in, err := dir.OpenInput("test.bin", store.IOContextReadOnce)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	_, _ = in.ReadByte() // read bpv
	restored := make([]int64, codecs.ForUtilBlockSize)
	if err := forUtil.Decode(bpv, in, restored); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < codecs.ForUtilBlockSize; i++ {
		if restored[i] != 0 {
			t.Errorf("Position %d: expected 0, got %d", i, restored[i])
		}
	}
}

// TestForUtil_MaxValues tests encoding/decoding of max values
func TestForUtil_MaxValues(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	for bpv := 1; bpv <= 31; bpv++ {
		out, err := dir.CreateOutput("test.bin", store.IOContextWrite)
		if err != nil {
			t.Fatal(err)
		}

		forUtil := codecs.NewForUtil()

		// All max values for this bits per value
		source := make([]int32, codecs.ForUtilBlockSize)
		maxVal := int32(maxValue(bpv))
		for i := 0; i < codecs.ForUtilBlockSize; i++ {
			source[i] = maxVal
		}

		if err := out.WriteByte(byte(bpv)); err != nil {
			t.Fatal(err)
		}
		if err := forUtil.Encode(source, bpv, out); err != nil {
			t.Fatalf("Failed to encode max values with bpv=%d: %v", bpv, err)
		}
		out.Close()

		in, err := dir.OpenInput("test.bin", store.IOContextReadOnce)
		if err != nil {
			t.Fatal(err)
		}

		_, _ = in.ReadByte()
		restored := make([]int64, codecs.ForUtilBlockSize)
		if err := forUtil.Decode(bpv, in, restored); err != nil {
			t.Fatalf("Failed to decode max values with bpv=%d: %v", bpv, err)
		}

		for i := 0; i < codecs.ForUtilBlockSize; i++ {
			if int32(restored[i]) != maxVal {
				t.Errorf("bpv=%d, position %d: expected %d, got %d", bpv, i, maxVal, restored[i])
				break
			}
		}
		in.Close()
	}
}

// Helper functions

func randomIntBetween(min, max int) int {
	if min >= max {
		return min
	}
	return min + rand.Intn(max-min)
}

func maxValue(bitsPerValue int) int64 {
	if bitsPerValue >= 63 {
		return math.MaxInt64
	}
	return (int64(1) << bitsPerValue) - 1
}

func bitsRequired(v int64) int {
	if v < 0 {
		return 64
	}
	if v == 0 {
		return 1
	}
	return int(math.Floor(math.Log2(float64(v)))) + 1
}
