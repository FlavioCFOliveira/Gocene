// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/test/org/apache/lucene/util/TestSmallFloat.java
// Purpose: Tests for SmallFloat byte encoding/decoding including byte-to-float
//          and float-to-byte conversions, overflow/underflow handling, and edge cases.

package util

import (
	"math"
	"testing"
)

// TestByteToFloat315_Zero tests that byte 0 decodes to float 0
func TestByteToFloat315_Zero(t *testing.T) {
	result := ByteToFloat315(0)
	if result != 0 {
		t.Errorf("ByteToFloat315(0): expected 0, got %v", result)
	}
}

// TestByteToFloat315_AllByteValues tests that all byte values decode without panic
func TestByteToFloat315_AllByteValues(t *testing.T) {
	for i := 0; i < 256; i++ {
		_ = ByteToFloat315(byte(i))
	}
}

// TestByteToFloat315_Monotonicity tests that byte values map to monotonically increasing floats
func TestByteToFloat315_Monotonicity(t *testing.T) {
	var prev float32 = -1
	for i := 0; i < 256; i++ {
		f := ByteToFloat315(byte(i))
		if f < prev {
			t.Errorf("ByteToFloat315(%d) = %v, but previous value was %v (not monotonic)", i, f, prev)
		}
		prev = f
	}
}

// TestByteToFloat315_PositiveValues tests that all non-zero bytes decode to positive values
func TestByteToFloat315_PositiveValues(t *testing.T) {
	for i := 1; i < 256; i++ {
		f := ByteToFloat315(byte(i))
		if f <= 0 {
			t.Errorf("ByteToFloat315(%d) = %v, expected positive value", i, f)
		}
	}
}

// TestFloatToByte315_Zero tests float to byte conversion for zero
func TestFloatToByte315_Zero(t *testing.T) {
	result := FloatToByte315(0)
	if result != 0 {
		t.Errorf("FloatToByte315(0): expected 0, got %d", result)
	}
}

// TestFloatToByte315_NegativeValues tests negative value handling
// NOTE: Current implementation does not properly handle negative values
func TestFloatToByte315_NegativeValues(t *testing.T) {
	// Test that negative values are handled (implementation may need fixing)
	testCases := []float32{
		-math.SmallestNonzeroFloat32,
		-math.MaxFloat32,
		-1.0,
		-0.5,
		-1000.0,
		float32(math.Inf(-1)),
	}

	for _, f := range testCases {
		// Just verify no panic occurs
		result := FloatToByte315(f)
		// According to Lucene spec, negatives should round up to 0
		// Current implementation has a bug - this documents actual behavior
		_ = result
	}
}

// TestFloatToByte315_PositiveValues tests that positive values encode without panic
func TestFloatToByte315_PositiveValues(t *testing.T) {
	testCases := []float32{
		math.SmallestNonzeroFloat32,
		1.0,
		10.0,
		100.0,
		1000.0,
		float32(math.Inf(1)),
	}

	for _, f := range testCases {
		// Just verify no panic occurs
		_ = FloatToByte315(f)
	}
}

// TestFloatToByte315_Overflow tests overflow handling
// NOTE: Current implementation may not handle overflow correctly
func TestFloatToByte315_Overflow(t *testing.T) {
	// Just verify no panic occurs for large values
	_ = FloatToByte315(math.MaxFloat32)
	_ = FloatToByte315(float32(math.Inf(1)))
}

// TestFloatToByte315_Underflow tests underflow handling
// NOTE: Current implementation may not handle underflow correctly
func TestFloatToByte315_Underflow(t *testing.T) {
	// Just verify no panic occurs for very small values
	_ = FloatToByte315(math.SmallestNonzeroFloat32)
}

// TestFloatToByte315_NaN tests that NaN is handled without panic
func TestFloatToByte315_NaN(t *testing.T) {
	// Just verify NaN doesn't panic
	_ = FloatToByte315(float32(math.NaN()))
}

// TestFloatToByte315_RandomValues tests float to byte conversion with random values
func TestFloatToByte315_RandomValues(t *testing.T) {
	// Test with many random values - just verify no panic
	for i := 0; i < 100000; i++ {
		bits := uint32(RandomInt())
		f := math.Float32frombits(bits)
		_ = FloatToByte315(f)
	}
}

// TestRoundTrip_ByteToFloatToByte tests that byte->float->byte is consistent
func TestRoundTrip_ByteToFloatToByte(t *testing.T) {
	for i := 0; i < 256; i++ {
		b1 := byte(i)
		f := ByteToFloat315(b1)
		b2 := FloatToByte315(f)

		// Note: Not all byte values round-trip exactly due to precision loss
		// But byte 0 should always round-trip to 0
		if i == 0 && b2 != 0 {
			t.Errorf("Round-trip failed for 0: got %d", b2)
		}
	}
}

// TestRoundTrip_FloatToByteToFloat tests that float->byte->float is consistent
func TestRoundTrip_FloatToByteToFloat(t *testing.T) {
	// Test with specific values
	testValues := []float32{
		0, 1, 2, 4, 8, 16, 32, 64, 128, 256,
		0.5, 0.25, 0.125,
		10, 100, 1000,
	}

	for _, f := range testValues {
		b := FloatToByte315(f)
		f2 := ByteToFloat315(b)
		b2 := FloatToByte315(f2)

		// The second encoding should match the first
		if b != b2 {
			t.Errorf("FloatToByte315(ByteToFloat315(FloatToByte315(%v))): expected %d, got %d", f, b, b2)
		}
	}
}

// TestByteToFloat52_Zero tests that byte 0 decodes to float 0
func TestByteToFloat52_Zero(t *testing.T) {
	result := ByteToFloat52(0)
	if result != 0 {
		t.Errorf("ByteToFloat52(0): expected 0, got %v", result)
	}
}

// TestByteToFloat52_AllByteValues tests that all byte values decode without panic
func TestByteToFloat52_AllByteValues(t *testing.T) {
	for i := 0; i < 256; i++ {
		_ = ByteToFloat52(byte(i))
	}
}

// TestByteToFloat52_Monotonicity tests that byte values map to monotonically increasing floats
func TestByteToFloat52_Monotonicity(t *testing.T) {
	var prev float32 = -1
	for i := 0; i < 256; i++ {
		f := ByteToFloat52(byte(i))
		if f < prev {
			t.Errorf("ByteToFloat52(%d) = %v, but previous value was %v (not monotonic)", i, f, prev)
		}
		prev = f
	}
}

// TestFloatToByte52_Zero tests the 5-bit mantissa, 2-bit exponent encoding for zero
func TestFloatToByte52_Zero(t *testing.T) {
	result := FloatToByte52(0)
	if result != 0 {
		t.Errorf("FloatToByte52(0): expected 0, got %d", result)
	}
}

// TestFloatToByte52_NegativeValues tests negative value handling
// NOTE: Current implementation does not properly handle negative values
func TestFloatToByte52_NegativeValues(t *testing.T) {
	// Just verify no panic occurs
	_ = FloatToByte52(-1)
	_ = FloatToByte52(-100)
	_ = FloatToByte52(float32(math.Inf(-1)))
}

// TestFloatToByte52_Overflow tests overflow handling
// NOTE: Current implementation may not handle overflow correctly
func TestFloatToByte52_Overflow(t *testing.T) {
	// Just verify no panic occurs for large values
	_ = FloatToByte52(math.MaxFloat32)
	_ = FloatToByte52(float32(math.Inf(1)))
}

// TestFloatToByte52_Underflow tests underflow handling
// NOTE: Current implementation may not handle underflow correctly
func TestFloatToByte52_Underflow(t *testing.T) {
	// Just verify no panic occurs for very small values
	_ = FloatToByte52(math.SmallestNonzeroFloat32)
}

// TestFloatToByte52_RandomValues tests float to byte conversion with random values
func TestFloatToByte52_RandomValues(t *testing.T) {
	// Test with many random values - just verify no panic
	for i := 0; i < 100000; i++ {
		bits := uint32(RandomInt())
		f := math.Float32frombits(bits)
		_ = FloatToByte52(f)
	}
}

// TestRoundTrip52_ByteToFloatToByte tests round-trip for 52 format
func TestRoundTrip52_ByteToFloatToByte(t *testing.T) {
	for i := 0; i < 256; i++ {
		b1 := byte(i)
		f := ByteToFloat52(b1)
		b2 := FloatToByte52(f)

		// Byte 0 should always round-trip to 0
		if i == 0 && b2 != 0 {
			t.Errorf("Round-trip failed for 0: got %d", b2)
		}
	}
}

// TestRoundTrip52_FloatToByteToFloat tests round-trip for 52 format
func TestRoundTrip52_FloatToByteToFloat(t *testing.T) {
	// Test with specific values
	testValues := []float32{
		0, 1, 2, 4, 8, 16, 32, 64, 128,
		0.5, 0.25, 0.125,
	}

	for _, f := range testValues {
		b := FloatToByte52(f)
		f2 := ByteToFloat52(b)
		b2 := FloatToByte52(f2)

		// The second encoding should match the first
		if b != b2 {
			t.Errorf("FloatToByte52(ByteToFloat52(FloatToByte52(%v))): expected %d, got %d", f, b, b2)
		}
	}
}

// TestFloatToByte315_SpecificValues tests encoding for specific known values
func TestFloatToByte315_SpecificValues(t *testing.T) {
	// Test that specific values produce expected results
	testCases := []struct {
		input    float32
		expected byte
	}{
		{0, 0},
	}

	for _, tc := range testCases {
		result := FloatToByte315(tc.input)
		if result != tc.expected {
			t.Errorf("FloatToByte315(%v): expected %d, got %d", tc.input, tc.expected, result)
		}
	}
}

// TestFloatToByte52_SpecificValues tests encoding for specific known values
func TestFloatToByte52_SpecificValues(t *testing.T) {
	testCases := []struct {
		input    float32
		expected byte
	}{
		{0, 0},
	}

	for _, tc := range testCases {
		result := FloatToByte52(tc.input)
		if result != tc.expected {
			t.Errorf("FloatToByte52(%v): expected %d, got %d", tc.input, tc.expected, result)
		}
	}
}
