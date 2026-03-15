// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"math"
	"testing"
)

// TestIsZeroOrPowerOfTwo tests the IsZeroOrPowerOfTwo function.
// Source: TestBitUtil.testIsZeroOrPowerOfTwo()
// Purpose: Tests that IsZeroOrPowerOfTwo correctly identifies zero and powers of two
func TestIsZeroOrPowerOfTwo(t *testing.T) {
	// Test that 0 returns true
	if !IsZeroOrPowerOfTwo(0) {
		t.Error("Expected IsZeroOrPowerOfTwo(0) to be true")
	}

	// Test all powers of two from 2^0 to 2^31
	for shift := 0; shift <= 31; shift++ {
		val := 1 << shift
		if !IsZeroOrPowerOfTwo(val) {
			t.Errorf("Expected IsZeroOrPowerOfTwo(1<<%d = %d) to be true", shift, val)
		}
	}

	// Test non-powers of two return false
	testCases := []int{3, 5, 6, 7, 9, math.MaxInt32}
	for _, val := range testCases {
		if IsZeroOrPowerOfTwo(val) {
			t.Errorf("Expected IsZeroOrPowerOfTwo(%d) to be false", val)
		}
	}

	// Test MaxInt32 + 2 (which is 0x80000001, not a power of two)
	// In Java: Integer.MAX_VALUE + 2 = -2147483647 (overflows to negative)
	// In Go, int overflow wraps around on most platforms (64-bit)
	// So we test with the equivalent bit pattern
	if IsZeroOrPowerOfTwo(math.MaxInt32 + 2) {
		t.Error("Expected IsZeroOrPowerOfTwo(MaxInt32 + 2) to be false")
	}

	// Test -1 (all bits set)
	if IsZeroOrPowerOfTwo(-1) {
		t.Error("Expected IsZeroOrPowerOfTwo(-1) to be false")
	}
}

// TestIsZeroOrPowerOfTwoInt64 tests the IsZeroOrPowerOfTwoInt64 function.
func TestIsZeroOrPowerOfTwoInt64(t *testing.T) {
	// Test that 0 returns true
	if !IsZeroOrPowerOfTwoInt64(0) {
		t.Error("Expected IsZeroOrPowerOfTwoInt64(0) to be true")
	}

	// Test all powers of two from 2^0 to 2^62
	for shift := 0; shift <= 62; shift++ {
		val := int64(1) << shift
		if !IsZeroOrPowerOfTwoInt64(val) {
			t.Errorf("Expected IsZeroOrPowerOfTwoInt64(1<<%d) to be true", shift)
		}
	}

	// Test non-powers of two return false
	testCases := []int64{3, 5, 6, 7, 9, math.MaxInt64}
	for _, val := range testCases {
		if IsZeroOrPowerOfTwoInt64(val) {
			t.Errorf("Expected IsZeroOrPowerOfTwoInt64(%d) to be false", val)
		}
	}

	// Test -1 (all bits set)
	if IsZeroOrPowerOfTwoInt64(-1) {
		t.Error("Expected IsZeroOrPowerOfTwoInt64(-1) to be false")
	}
}

// TestNextHighestPowerOfTwo tests the NextHighestPowerOfTwo function.
func TestNextHighestPowerOfTwo(t *testing.T) {
	testCases := []struct {
		input    int
		expected int
	}{
		{0, 0},       // 0 stays 0
		{1, 1},       // 1 is already a power of two
		{2, 2},       // 2 is already a power of two
		{3, 4},       // next power of two after 3 is 4
		{4, 4},       // 4 is already a power of two
		{5, 8},       // next power of two after 5 is 8
		{6, 8},       // next power of two after 6 is 8
		{7, 8},       // next power of two after 7 is 8
		{8, 8},       // 8 is already a power of two
		{9, 16},      // next power of two after 9 is 16
		{15, 16},     // next power of two after 15 is 16
		{16, 16},     // 16 is already a power of two
		{17, 32},     // next power of two after 17 is 32
		{100, 128},   // next power of two after 100 is 128
		{1023, 1024}, // next power of two after 1023 is 1024
		{1024, 1024}, // 1024 is already a power of two
		{1025, 2048}, // next power of two after 1025 is 2048
	}

	for _, tc := range testCases {
		result := NextHighestPowerOfTwo(tc.input)
		if result != tc.expected {
			t.Errorf("NextHighestPowerOfTwo(%d) = %d, expected %d", tc.input, result, tc.expected)
		}
	}
}

// TestNextHighestPowerOfTwoInt64 tests the NextHighestPowerOfTwoInt64 function.
func TestNextHighestPowerOfTwoInt64(t *testing.T) {
	testCases := []struct {
		input    int64
		expected int64
	}{
		{0, 0},                 // 0 stays 0
		{1, 1},                 // 1 is already a power of two
		{2, 2},                 // 2 is already a power of two
		{3, 4},                 // next power of two after 3 is 4
		{4, 4},                 // 4 is already a power of two
		{5, 8},                 // next power of two after 5 is 8
		{100, 128},             // next power of two after 100 is 128
		{1023, 1024},           // next power of two after 1023 is 1024
		{1 << 30, 1 << 30},   // already a power of two
		{(1 << 30) + 1, 1 << 31}, // next power of two
	}

	for _, tc := range testCases {
		result := NextHighestPowerOfTwoInt64(tc.input)
		if result != tc.expected {
			t.Errorf("NextHighestPowerOfTwoInt64(%d) = %d, expected %d", tc.input, result, tc.expected)
		}
	}
}

// TestInterleave tests the Interleave function.
func TestInterleave(t *testing.T) {
	testCases := []struct {
		even     int
		odd      int
		expected int64
	}{
		{0, 0, 0},                    // interleaving 0s gives 0
		{1, 0, 1},                    // interleave(1, 0) = 1
		{0, 1, 2},                    // interleave(0, 1) = 2
		{1, 1, 3},                    // interleave(1, 1) = 3
		{0xFFFFFFFF, 0, 0x5555555555555555}, // all even bits set
		{0, 0xFFFFFFFF, -0x5555555555555556}, // 0xAAAAAAAAAAAAAAAA as signed = -0x5555555555555556
	}

	for _, tc := range testCases {
		result := Interleave(tc.even, tc.odd)
		if result != tc.expected {
			t.Errorf("Interleave(%d, %d) = 0x%x, expected 0x%x", tc.even, tc.odd, uint64(result), uint64(tc.expected))
		}
	}
}

// TestDeinterleave tests the Deinterleave function.
func TestDeinterleave(t *testing.T) {
	testCases := []struct {
		input    int64
		expected int64
	}{
		{0, 0},
		{1, 1},
		{2, 0}, // deinterleave of 2 (binary 10) extracts even bits which is 0
		{3, 1}, // deinterleave of 3 (binary 11) extracts even bits which is 1
		{0x5555555555555555, 0xFFFFFFFF}, // all even bits set -> all 32 bits set in result
	}

	for _, tc := range testCases {
		result := Deinterleave(tc.input)
		if result != tc.expected {
			t.Errorf("Deinterleave(0x%x) = 0x%x, expected 0x%x", uint64(tc.input), uint64(result), uint64(tc.expected))
		}
	}
}

// TestInterleaveDeinterleaveRoundTrip tests that deinterleave reverses interleave.
func TestInterleaveDeinterleaveRoundTrip(t *testing.T) {
	testCases := []struct {
		even int
		odd  int
	}{
		{0, 0},
		{1, 0},
		{0, 1},
		{1, 1},
		{42, 123},
		{0x12345678, 0x9ABCDEF0},
		{0xFFFFFFFF, 0xFFFFFFFF},
		{0x00000000, 0xFFFFFFFF},
		{0xFFFFFFFF, 0x00000000},
	}

	for _, tc := range testCases {
		interleaved := Interleave(tc.even, tc.odd)
		deinterleaved := int(Deinterleave(interleaved))
		if deinterleaved != tc.even {
			t.Errorf("Deinterleave(Interleave(%d, %d)) = %d, expected %d", tc.even, tc.odd, deinterleaved, tc.even)
		}
	}
}

// TestFlipFlop tests the FlipFlop function.
func TestFlipFlop(t *testing.T) {
	testCases := []struct {
		input    int64
		expected int64
	}{
		{0, 0},
		{1, 2},  // 01 -> 10
		{2, 1},  // 10 -> 01
		{3, 3},  // 11 -> 11
		{5, 10}, // 0101 -> 1010
		{10, 5}, // 1010 -> 0101
	}

	for _, tc := range testCases {
		result := FlipFlop(tc.input)
		if result != tc.expected {
			t.Errorf("FlipFlop(0x%x) = 0x%x, expected 0x%x", uint64(tc.input), uint64(result), uint64(tc.expected))
		}
	}
}

// TestFlipFlopInvolution tests that FlipFlop is an involution (applying it twice returns the original).
func TestFlipFlopInvolution(t *testing.T) {
	testCases := []int64{
		0,
		1,
		2,
		3,
		42,
		12345,
		0x123456789ABCDEF0,
		-1, // 0xFFFFFFFFFFFFFFFF as signed
		0x5555555555555555,
		-0x5555555555555556, // 0xAAAAAAAAAAAAAAAA as signed
	}

	for _, tc := range testCases {
		result := FlipFlop(FlipFlop(tc))
		if result != tc {
			t.Errorf("FlipFlop(FlipFlop(%d)) = %d, expected %d", tc, result, tc)
		}
	}
}

// TestZigZagEncodeInt tests the ZigZagEncodeInt function.
func TestZigZagEncodeInt(t *testing.T) {
	testCases := []struct {
		input    int
		expected int
	}{
		{0, 0},                    // 0 encodes to 0
		{-1, 1},                   // -1 encodes to 1
		{1, 2},                    // 1 encodes to 2
		{-2, 3},                   // -2 encodes to 3
		{2, 4},                    // 2 encodes to 4
		{-3, 5},                   // -3 encodes to 5
		{3, 6},                    // 3 encodes to 6
		{-4, 7},                   // -4 encodes to 7
		{4, 8},                    // 4 encodes to 8
		{math.MaxInt32, -2},       // MaxInt32 encodes to -2 (0x7FFFFFFF -> 0xFFFFFFFE)
		{math.MinInt32, math.MaxInt32}, // MinInt32 encodes to MaxInt32
	}

	for _, tc := range testCases {
		result := ZigZagEncodeInt(tc.input)
		if result != tc.expected {
			t.Errorf("ZigZagEncodeInt(%d) = %d, expected %d", tc.input, result, tc.expected)
		}
	}
}

// TestZigZagDecodeInt tests the ZigZagDecodeInt function.
func TestZigZagDecodeInt(t *testing.T) {
	testCases := []struct {
		input    int
		expected int
	}{
		{0, 0},                    // 0 decodes to 0
		{1, -1},                   // 1 decodes to -1
		{2, 1},                    // 2 decodes to 1
		{3, -2},                   // 3 decodes to -2
		{4, 2},                    // 4 decodes to 2
		{5, -3},                   // 5 decodes to -3
		{6, 3},                    // 6 decodes to 3
		{7, -4},                   // 7 decodes to -4
		{8, 4},                    // 8 decodes to 4
	}

	for _, tc := range testCases {
		result := ZigZagDecodeInt(tc.input)
		if result != tc.expected {
			t.Errorf("ZigZagDecodeInt(%d) = %d, expected %d", tc.input, result, tc.expected)
		}
	}
}

// TestZigZagIntRoundTrip tests that ZigZagDecodeInt reverses ZigZagEncodeInt.
func TestZigZagIntRoundTrip(t *testing.T) {
	testCases := []int{
		0,
		1,
		-1,
		2,
		-2,
		100,
		-100,
		10000,
		-10000,
		math.MaxInt32,
		math.MinInt32,
		math.MaxInt32 - 1,
		math.MinInt32 + 1,
	}

	for _, tc := range testCases {
		encoded := ZigZagEncodeInt(tc)
		decoded := ZigZagDecodeInt(encoded)
		if decoded != tc {
			t.Errorf("ZigZagDecodeInt(ZigZagEncodeInt(%d)) = %d, expected %d", tc, decoded, tc)
		}
	}
}

// TestZigZagEncodeInt64 tests the ZigZagEncodeInt64 function.
func TestZigZagEncodeInt64(t *testing.T) {
	testCases := []struct {
		input    int64
		expected int64
	}{
		{0, 0},                         // 0 encodes to 0
		{-1, 1},                        // -1 encodes to 1
		{1, 2},                         // 1 encodes to 2
		{-2, 3},                        // -2 encodes to 3
		{2, 4},                         // 2 encodes to 4
		{math.MaxInt64, -2},            // MaxInt64 encodes to -2
		{math.MinInt64, math.MaxInt64}, // MinInt64 encodes to MaxInt64
	}

	for _, tc := range testCases {
		result := ZigZagEncodeInt64(tc.input)
		if result != tc.expected {
			t.Errorf("ZigZagEncodeInt64(%d) = %d, expected %d", tc.input, result, tc.expected)
		}
	}
}

// TestZigZagDecodeInt64 tests the ZigZagDecodeInt64 function.
func TestZigZagDecodeInt64(t *testing.T) {
	testCases := []struct {
		input    int64
		expected int64
	}{
		{0, 0},  // 0 decodes to 0
		{1, -1}, // 1 decodes to -1
		{2, 1},  // 2 decodes to 1
		{3, -2}, // 3 decodes to -2
		{4, 2},  // 4 decodes to 2
		{5, -3}, // 5 decodes to -3
	}

	for _, tc := range testCases {
		result := ZigZagDecodeInt64(tc.input)
		if result != tc.expected {
			t.Errorf("ZigZagDecodeInt64(%d) = %d, expected %d", tc.input, result, tc.expected)
		}
	}
}

// TestZigZagInt64RoundTrip tests that ZigZagDecodeInt64 reverses ZigZagEncodeInt64.
func TestZigZagInt64RoundTrip(t *testing.T) {
	testCases := []int64{
		0,
		1,
		-1,
		2,
		-2,
		100,
		-100,
		1000000,
		-1000000,
		math.MaxInt64,
		math.MinInt64,
		math.MaxInt64 - 1,
		math.MinInt64 + 1,
	}

	for _, tc := range testCases {
		encoded := ZigZagEncodeInt64(tc)
		decoded := ZigZagDecodeInt64(encoded)
		if decoded != tc {
			t.Errorf("ZigZagDecodeInt64(ZigZagEncodeInt64(%d)) = %d, expected %d", tc, decoded, tc)
		}
	}
}

// TestBitUtilEdgeCases tests edge cases and boundary conditions.
func TestBitUtilEdgeCases(t *testing.T) {
	// Test IsZeroOrPowerOfTwo with various edge cases
	edgeCases := []struct {
		val      int
		expected bool
	}{
		{0, true},    // 0 is considered a power of two
		{1, true},    // 1 is 2^0
		{-1, false},  // -1 is all bits set
		{-2, false},  // -2 is ...1110
		{-8, false},  // negative numbers are not powers of two
	}

	for _, tc := range edgeCases {
		result := IsZeroOrPowerOfTwo(tc.val)
		if result != tc.expected {
			t.Errorf("IsZeroOrPowerOfTwo(%d) = %v, expected %v", tc.val, result, tc.expected)
		}
	}
}

// BenchmarkIsZeroOrPowerOfTwo benchmarks the IsZeroOrPowerOfTwo function.
func BenchmarkIsZeroOrPowerOfTwo(b *testing.B) {
	for i := 0; i < b.N; i++ {
		IsZeroOrPowerOfTwo(i)
	}
}

// BenchmarkNextHighestPowerOfTwo benchmarks the NextHighestPowerOfTwo function.
func BenchmarkNextHighestPowerOfTwo(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NextHighestPowerOfTwo(i)
	}
}

// BenchmarkInterleave benchmarks the Interleave function.
func BenchmarkInterleave(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Interleave(i, i+1)
	}
}

// BenchmarkDeinterleave benchmarks the Deinterleave function.
func BenchmarkDeinterleave(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Deinterleave(int64(i))
	}
}

// BenchmarkZigZagEncodeInt benchmarks the ZigZagEncodeInt function.
func BenchmarkZigZagEncodeInt(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ZigZagEncodeInt(i)
	}
}

// BenchmarkZigZagDecodeInt benchmarks the ZigZagDecodeInt function.
func BenchmarkZigZagDecodeInt(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ZigZagDecodeInt(i)
	}
}

// BenchmarkFlipFlop benchmarks the FlipFlop function.
func BenchmarkFlipFlop(b *testing.B) {
	for i := 0; i < b.N; i++ {
		FlipFlop(int64(i))
	}
}
