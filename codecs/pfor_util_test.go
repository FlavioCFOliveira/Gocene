// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/test/org/apache/lucene/codecs/lucene104/TestPForUtil.java
// Purpose: Tests Patched Frame of Reference encoding/decoding for 256 integers

package codecs_test

import (
	"math"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestPForUtil_EncodeDecode tests encoding and decoding of 256 integers
// using Patched Frame of Reference compression
func TestPForUtil_EncodeDecode(t *testing.T) {
	iterations := randomIntBetween(50, 1000)
	values := createPForTestData(iterations, 31)

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	endPointer := encodePForTestData(iterations, values, dir, t)

	in, err := dir.OpenInput("test.bin", store.IOContextReadOnce)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	pforUtil := codecs.NewPForUtil(codecs.NewForUtil())

	for i := 0; i < iterations; i++ {
		// Randomly skip some iterations (20% chance)
		if rand.Intn(5) == 0 {
			if err := codecs.PForUtilSkip(in); err != nil {
				t.Fatalf("Failed to skip at iteration %d: %v", i, err)
			}
			continue
		}

		restored := make([]int64, codecs.ForUtilBlockSize)
		if err := pforUtil.Decode(in, restored); err != nil {
			t.Fatalf("Failed to decode at iteration %d: %v", i, err)
		}

		ints := make([]int32, codecs.ForUtilBlockSize)
		for j := 0; j < codecs.ForUtilBlockSize; j++ {
			ints[j] = int32(restored[j])
		}

		expected := values[i*codecs.ForUtilBlockSize : (i+1)*codecs.ForUtilBlockSize]
		for j := 0; j < codecs.ForUtilBlockSize; j++ {
			if ints[j] != expected[j] {
				t.Errorf("Iteration %d, position %d: expected %d, got %d", i, j, expected[j], ints[j])
				break
			}
		}
	}

	if in.GetFilePointer() != endPointer {
		t.Errorf("Expected file pointer %d, got %d", endPointer, in.GetFilePointer())
	}
}

// TestPForUtil_Skip tests the skip functionality
func TestPForUtil_Skip(t *testing.T) {
	iterations := 100
	values := createPForTestData(iterations, 20)

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	encodePForTestData(iterations, values, dir, t)

	in, err := dir.OpenInput("test.bin", store.IOContextReadOnce)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	// Skip all blocks
	for i := 0; i < iterations; i++ {
		if err := codecs.PForUtilSkip(in); err != nil {
			t.Fatalf("Failed to skip at iteration %d: %v", i, err)
		}
	}
}

// TestPForUtil_AllEqual tests encoding when all values are equal
func TestPForUtil_AllEqual(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	out, err := dir.CreateOutput("test.bin", store.IOContextWrite)
	if err != nil {
		t.Fatal(err)
	}

	pforUtil := codecs.NewPForUtil(codecs.NewForUtil())

	// All equal values
	source := make([]int32, codecs.ForUtilBlockSize)
	for i := 0; i < codecs.ForUtilBlockSize; i++ {
		source[i] = 42
	}

	if err := pforUtil.Encode(source, out); err != nil {
		t.Fatal(err)
	}
	out.Close()

	in, err := dir.OpenInput("test.bin", store.IOContextReadOnce)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	restored := make([]int64, codecs.ForUtilBlockSize)
	if err := pforUtil.Decode(in, restored); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < codecs.ForUtilBlockSize; i++ {
		if int32(restored[i]) != 42 {
			t.Errorf("Position %d: expected 42, got %d", i, restored[i])
		}
	}
}

// TestPForUtil_ZeroValues tests encoding of all zeros
func TestPForUtil_ZeroValues(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	out, err := dir.CreateOutput("test.bin", store.IOContextWrite)
	if err != nil {
		t.Fatal(err)
	}

	pforUtil := codecs.NewPForUtil(codecs.NewForUtil())

	// All zeros
	source := make([]int32, codecs.ForUtilBlockSize)

	if err := pforUtil.Encode(source, out); err != nil {
		t.Fatal(err)
	}
	out.Close()

	in, err := dir.OpenInput("test.bin", store.IOContextReadOnce)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	restored := make([]int64, codecs.ForUtilBlockSize)
	if err := pforUtil.Decode(in, restored); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < codecs.ForUtilBlockSize; i++ {
		if restored[i] != 0 {
			t.Errorf("Position %d: expected 0, got %d", i, restored[i])
		}
	}
}

// TestPForUtil_WithExceptions tests encoding with exceptions
func TestPForUtil_WithExceptions(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	out, err := dir.CreateOutput("test.bin", store.IOContextWrite)
	if err != nil {
		t.Fatal(err)
	}

	pforUtil := codecs.NewPForUtil(codecs.NewForUtil())

	// Create data with exceptions (values requiring more bits)
	source := make([]int32, codecs.ForUtilBlockSize)
	for i := 0; i < codecs.ForUtilBlockSize; i++ {
		if i%50 == 0 {
			// Exception value requiring more bits
			source[i] = 100000
		} else {
			source[i] = int32(randomIntBetween(0, 100))
		}
	}

	if err := pforUtil.Encode(source, out); err != nil {
		t.Fatal(err)
	}
	out.Close()

	in, err := dir.OpenInput("test.bin", store.IOContextReadOnce)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	restored := make([]int64, codecs.ForUtilBlockSize)
	if err := pforUtil.Decode(in, restored); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < codecs.ForUtilBlockSize; i++ {
		if int32(restored[i]) != source[i] {
			t.Errorf("Position %d: expected %d, got %d", i, source[i], restored[i])
		}
	}
}

// TestPForUtil_MaxExceptions tests encoding with maximum exceptions
func TestPForUtil_MaxExceptions(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	out, err := dir.CreateOutput("test.bin", store.IOContextWrite)
	if err != nil {
		t.Fatal(err)
	}

	pforUtil := codecs.NewPForUtil(codecs.NewForUtil())

	// Create data with many exceptions
	source := make([]int32, codecs.ForUtilBlockSize)
	for i := 0; i < codecs.ForUtilBlockSize; i++ {
		if i%40 == 0 {
			// Exception value
			source[i] = 500000
		} else {
			source[i] = int32(randomIntBetween(0, 50))
		}
	}

	if err := pforUtil.Encode(source, out); err != nil {
		t.Fatal(err)
	}
	out.Close()

	in, err := dir.OpenInput("test.bin", store.IOContextReadOnce)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	restored := make([]int64, codecs.ForUtilBlockSize)
	if err := pforUtil.Decode(in, restored); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < codecs.ForUtilBlockSize; i++ {
		if int32(restored[i]) != source[i] {
			t.Errorf("Position %d: expected %d, got %d", i, source[i], restored[i])
		}
	}
}

// TestPForUtil_RandomData tests with various random data patterns
func TestPForUtil_RandomData(t *testing.T) {
	for run := 0; run < 10; run++ {
		dir := store.NewByteBuffersDirectory()

		out, err := dir.CreateOutput("test.bin", store.IOContextWrite)
		if err != nil {
			t.Fatal(err)
		}

		pforUtil := codecs.NewPForUtil(codecs.NewForUtil())

		// Create random test data
		source := make([]int32, codecs.ForUtilBlockSize)
		maxBpv := randomIntBetween(5, 25)
		for i := 0; i < codecs.ForUtilBlockSize; i++ {
			source[i] = int32(randomIntBetween(0, int(maxValue(maxBpv))))
		}

		if err := pforUtil.Encode(source, out); err != nil {
			t.Fatal(err)
		}
		out.Close()

		in, err := dir.OpenInput("test.bin", store.IOContextReadOnce)
		if err != nil {
			t.Fatal(err)
		}

		restored := make([]int64, codecs.ForUtilBlockSize)
		if err := pforUtil.Decode(in, restored); err != nil {
			t.Fatal(err)
		}

		for i := 0; i < codecs.ForUtilBlockSize; i++ {
			if int32(restored[i]) != source[i] {
				t.Errorf("Run %d, position %d: expected %d, got %d", run, i, source[i], restored[i])
				break
			}
		}

		in.Close()
		dir.Close()
	}
}

// Helper functions

func createPForTestData(iterations, maxBpv int) []int32 {
	values := make([]int32, iterations*codecs.ForUtilBlockSize)

	for i := 0; i < iterations; i++ {
		bpv := randomIntBetween(0, maxBpv)
		for j := 0; j < codecs.ForUtilBlockSize; j++ {
			values[i*codecs.ForUtilBlockSize+j] = int32(randomIntBetween(0, int(maxValue(bpv))))

			// Occasionally add exceptions (1% chance)
			if rand.Intn(100) == 0 {
				exceptionBpv := bpv + randomIntBetween(1, 8)
				if exceptionBpv > maxBpv {
					exceptionBpv = maxBpv
				}
				if exceptionBpv > bpv {
					extraBits := exceptionBpv - bpv
					extra := rand.Intn(1 << extraBits)
					values[i*codecs.ForUtilBlockSize+j] |= int32(extra << bpv)
				}
			}
		}
	}

	return values
}

func encodePForTestData(iterations int, values []int32, dir store.Directory, t *testing.T) int64 {
	out, err := dir.CreateOutput("test.bin", store.IOContextWrite)
	if err != nil {
		t.Fatal(err)
	}

	pforUtil := codecs.NewPForUtil(codecs.NewForUtil())

	for i := 0; i < iterations; i++ {
		source := make([]int32, codecs.ForUtilBlockSize)
		for j := 0; j < codecs.ForUtilBlockSize; j++ {
			source[j] = values[i*codecs.ForUtilBlockSize+j]
		}
		if err := pforUtil.Encode(source, out); err != nil {
			t.Fatalf("Failed to encode at iteration %d: %v", i, err)
		}
	}

	endPointer := out.GetFilePointer()
	out.Close()

	return endPointer
}

func bitsRequiredPFor(v int64) int {
	if v < 0 {
		return 64
	}
	if v == 0 {
		return 0
	}
	return int(math.Floor(math.Log2(float64(v)))) + 1
}
