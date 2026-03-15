// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"testing"
)

// stringHelperNewBytesRef is a helper to create a BytesRef from a string (similar to Lucene's newBytesRef)
func stringHelperNewBytesRef(s string) *BytesRef {
	return NewBytesRef([]byte(s))
}

func TestBytesDifference(t *testing.T) {
	// Test case: "foobar" vs "foozo" - common prefix is "foo" (3 bytes)
	left := stringHelperNewBytesRef("foobar")
	right := stringHelperNewBytesRef("foozo")
	result, err := BytesDifference(left, right)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result != 3 {
		t.Errorf("Expected bytesDifference=3 for 'foobar' vs 'foozo', got %d", result)
	}

	// Test case: "foo" vs "for" - common prefix is "fo" (2 bytes)
	result, err = BytesDifference(stringHelperNewBytesRef("foo"), stringHelperNewBytesRef("for"))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result != 2 {
		t.Errorf("Expected bytesDifference=2 for 'foo' vs 'for', got %d", result)
	}

	// Test case: "foo1234" vs "for1234" - common prefix is "fo" (2 bytes)
	result, err = BytesDifference(stringHelperNewBytesRef("foo1234"), stringHelperNewBytesRef("for1234"))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result != 2 {
		t.Errorf("Expected bytesDifference=2 for 'foo1234' vs 'for1234', got %d", result)
	}

	// Test case: "foo" vs "fz" - common prefix is "f" (1 byte)
	result, err = BytesDifference(stringHelperNewBytesRef("foo"), stringHelperNewBytesRef("fz"))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result != 1 {
		t.Errorf("Expected bytesDifference=1 for 'foo' vs 'fz', got %d", result)
	}

	// Test case: "foo" vs "g" - no common prefix (0 bytes)
	result, err = BytesDifference(stringHelperNewBytesRef("foo"), stringHelperNewBytesRef("g"))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result != 0 {
		t.Errorf("Expected bytesDifference=0 for 'foo' vs 'g', got %d", result)
	}

	// Test case: "foo" vs "food" - "foo" is prefix of "food" (3 bytes)
	result, err = BytesDifference(stringHelperNewBytesRef("foo"), stringHelperNewBytesRef("food"))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result != 3 {
		t.Errorf("Expected bytesDifference=3 for 'foo' vs 'food', got %d", result)
	}

	// Test case: equal terms should throw IllegalArgumentException (error in Go)
	_, err = BytesDifference(stringHelperNewBytesRef("ab"), stringHelperNewBytesRef("ab"))
	if err == nil {
		t.Error("Expected error for equal terms (out of order detection)")
	}
}

func TestStartsWith(t *testing.T) {
	// Test basic startsWith
	ref := stringHelperNewBytesRef("foobar")
	prefix := stringHelperNewBytesRef("foo")
	if !StartsWith(ref, prefix) {
		t.Error("Expected 'foobar' to start with 'foo'")
	}

	// Test startsWith whole string
	ref = stringHelperNewBytesRef("foobar")
	prefix = stringHelperNewBytesRef("foobar")
	if !StartsWith(ref, prefix) {
		t.Error("Expected 'foobar' to start with 'foobar' (whole string)")
	}

	// Test negative case
	ref = stringHelperNewBytesRef("foobar")
	prefix = stringHelperNewBytesRef("bar")
	if StartsWith(ref, prefix) {
		t.Error("Expected 'foobar' NOT to start with 'bar'")
	}

	// Test prefix longer than ref
	ref = stringHelperNewBytesRef("foo")
	prefix = stringHelperNewBytesRef("foobar")
	if StartsWith(ref, prefix) {
		t.Error("Expected short ref NOT to start with longer prefix")
	}

	// Test with nil
	if StartsWith(nil, stringHelperNewBytesRef("foo")) {
		t.Error("Expected nil ref to not start with anything")
	}
	if StartsWith(stringHelperNewBytesRef("foo"), nil) {
		t.Error("Expected ref to not start with nil prefix")
	}
}

func TestEndsWith(t *testing.T) {
	// Test basic endsWith
	ref := stringHelperNewBytesRef("foobar")
	suffix := stringHelperNewBytesRef("bar")
	if !EndsWith(ref, suffix) {
		t.Error("Expected 'foobar' to end with 'bar'")
	}

	// Test endsWith whole string
	ref = stringHelperNewBytesRef("foobar")
	suffix = stringHelperNewBytesRef("foobar")
	if !EndsWith(ref, suffix) {
		t.Error("Expected 'foobar' to end with 'foobar' (whole string)")
	}

	// Test negative case
	ref = stringHelperNewBytesRef("foobar")
	suffix = stringHelperNewBytesRef("foo")
	if EndsWith(ref, suffix) {
		t.Error("Expected 'foobar' NOT to end with 'foo'")
	}

	// Test suffix longer than ref
	ref = stringHelperNewBytesRef("bar")
	suffix = stringHelperNewBytesRef("foobar")
	if EndsWith(ref, suffix) {
		t.Error("Expected short ref NOT to end with longer suffix")
	}

	// Test with nil
	if EndsWith(nil, stringHelperNewBytesRef("bar")) {
		t.Error("Expected nil ref to not end with anything")
	}
	if EndsWith(stringHelperNewBytesRef("foobar"), nil) {
		t.Error("Expected ref to not end with nil suffix")
	}
}

func TestMurmurHash3(t *testing.T) {
	// Hashes computed using murmur3_32 from https://code.google.com/p/pyfasthash
	// These values match the expected values from Lucene's TestStringHelper

	// Test: "foo" with seed 0 should give 0xf6a5c420 (signed: -156908512)
	hash := MurmurHash3_x86_32_BytesRef(stringHelperNewBytesRef("foo"), 0)
	expected := -156908512 // int32(0xf6a5c420)
	if hash != expected {
		t.Errorf("Expected hash %d (0xf6a5c420) for 'foo' with seed 0, got %d (0x%x)", expected, hash, uint32(hash))
	}

	// Test: "foo" with seed 16 should give 0xcd018ef6 (signed: -855535882)
	hash = MurmurHash3_x86_32_BytesRef(stringHelperNewBytesRef("foo"), 16)
	expected = -855535882 // int32(0xcd018ef6)
	if hash != expected {
		t.Errorf("Expected hash %d (0xcd018ef6) for 'foo' with seed 16, got %d (0x%x)", expected, hash, uint32(hash))
	}

	// Test: long string with seed 0 should give 0x111e7435 (signed: 287208501)
	longStr := "You want weapons? We're in a library! Books! The best weapons in the world!"
	hash = MurmurHash3_x86_32_BytesRef(stringHelperNewBytesRef(longStr), 0)
	expected = 287208501 // int32(0x111e7435)
	if hash != expected {
		t.Errorf("Expected hash %d (0x111e7435) for long string with seed 0, got %d (0x%x)", expected, hash, uint32(hash))
	}

	// Test: long string with seed 3476 should give 0x2c628cd0 (signed: 744656080)
	hash = MurmurHash3_x86_32_BytesRef(stringHelperNewBytesRef(longStr), 3476)
	expected = 744656080 // int32(0x2c628cd0)
	if hash != expected {
		t.Errorf("Expected hash %d (0x2c628cd0) for long string with seed 3476, got %d (0x%x)", expected, hash, uint32(hash))
	}
}

func TestMurmurHash3Direct(t *testing.T) {
	// Test the direct byte array version
	data := []byte("foo")

	// Test with offset 0
	hash := MurmurHash3_x86_32(data, 0, len(data), 0)
	expected := -156908512 // int32(0xf6a5c420)
	if hash != expected {
		t.Errorf("Direct hash mismatch: expected %d (0xf6a5c420), got %d (0x%x)", expected, hash, uint32(hash))
	}

	// Test with offset (simulate BytesRef with offset)
	data = []byte("xxfooyyy")
	hash = MurmurHash3_x86_32(data, 2, 3, 0) // hash "foo" starting at offset 2
	if hash != expected {
		t.Errorf("Hash with offset mismatch: expected %d (0xf6a5c420), got %d (0x%x)", expected, hash, uint32(hash))
	}

	// Test empty data
	hash = MurmurHash3_x86_32(nil, 0, 0, 0)
	// Empty hash with seed 0 should be 0
	_ = hash
}

func TestSortKeyLength(t *testing.T) {
	// Test case: "foo" vs "for" - bytesDifference=2, so sortKeyLength=3
	result, err := SortKeyLength(stringHelperNewBytesRef("foo"), stringHelperNewBytesRef("for"))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result != 3 {
		t.Errorf("Expected sortKeyLength=3 for 'foo' vs 'for', got %d", result)
	}

	// Test case: "foo1234" vs "for1234" - bytesDifference=2, so sortKeyLength=3
	result, err = SortKeyLength(stringHelperNewBytesRef("foo1234"), stringHelperNewBytesRef("for1234"))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result != 3 {
		t.Errorf("Expected sortKeyLength=3 for 'foo1234' vs 'for1234', got %d", result)
	}

	// Test case: "foo" vs "fz" - bytesDifference=1, so sortKeyLength=2
	result, err = SortKeyLength(stringHelperNewBytesRef("foo"), stringHelperNewBytesRef("fz"))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result != 2 {
		t.Errorf("Expected sortKeyLength=2 for 'foo' vs 'fz', got %d", result)
	}

	// Test case: "foo" vs "g" - bytesDifference=0, so sortKeyLength=1
	result, err = SortKeyLength(stringHelperNewBytesRef("foo"), stringHelperNewBytesRef("g"))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result != 1 {
		t.Errorf("Expected sortKeyLength=1 for 'foo' vs 'g', got %d", result)
	}

	// Test case: "foo" vs "food" - bytesDifference=3, so sortKeyLength=4
	result, err = SortKeyLength(stringHelperNewBytesRef("foo"), stringHelperNewBytesRef("food"))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result != 4 {
		t.Errorf("Expected sortKeyLength=4 for 'foo' vs 'food', got %d", result)
	}

	// Test case: equal terms should throw IllegalArgumentException (error in Go)
	_, err = SortKeyLength(stringHelperNewBytesRef("ab"), stringHelperNewBytesRef("ab"))
	if err == nil {
		t.Error("Expected error for equal terms (out of order detection)")
	}
}

func TestBytesDifferenceWithOffset(t *testing.T) {
	// Test BytesDifference with BytesRef that have offsets
	br1 := &BytesRef{
		Bytes:  []byte("xxfoobaryy"),
		Offset: 2,
		Length: 6,
	} // represents "foobar"

	br2 := &BytesRef{
		Bytes:  []byte("afoozobbb"),
		Offset: 1,
		Length: 5,
	} // represents "foozo"

	result, err := BytesDifference(br1, br2)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result != 3 {
		t.Errorf("Expected bytesDifference=3 with offsets, got %d", result)
	}
}

func TestStartsWithWithOffset(t *testing.T) {
	// Test StartsWith with BytesRef that have offsets
	ref := &BytesRef{
		Bytes:  []byte("xxfoobaryy"),
		Offset: 2,
		Length: 6,
	} // represents "foobar"

	prefix := &BytesRef{
		Bytes:  []byte("afoo"),
		Offset: 1,
		Length: 3,
	} // represents "foo"

	if !StartsWith(ref, prefix) {
		t.Error("Expected StartsWith to work with offsets")
	}
}

func TestEndsWithWithOffset(t *testing.T) {
	// Test EndsWith with BytesRef that have offsets
	ref := &BytesRef{
		Bytes:  []byte("xxfoobaryy"),
		Offset: 2,
		Length: 6,
	} // represents "foobar"

	suffix := &BytesRef{
		Bytes:  []byte("abar"),
		Offset: 1,
		Length: 3,
	} // represents "bar"

	if !EndsWith(ref, suffix) {
		t.Error("Expected EndsWith to work with offsets")
	}
}

func TestBytesDifferenceEdgeCases(t *testing.T) {
	// Test with nil
	result, err := BytesDifference(nil, stringHelperNewBytesRef("foo"))
	if err != nil {
		t.Errorf("Unexpected error with nil: %v", err)
	}
	if result != 0 {
		t.Errorf("Expected 0 for nil priorTerm, got %d", result)
	}

	result, err = BytesDifference(stringHelperNewBytesRef("foo"), nil)
	if err != nil {
		t.Errorf("Unexpected error with nil: %v", err)
	}
	if result != 0 {
		t.Errorf("Expected 0 for nil currentTerm, got %d", result)
	}

	// Test with empty strings
	result, err = BytesDifference(stringHelperNewBytesRef(""), stringHelperNewBytesRef("a"))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result != 0 {
		t.Errorf("Expected 0 for empty vs 'a', got %d", result)
	}

	// Test empty vs empty (should error - they're equal)
	_, err = BytesDifference(stringHelperNewBytesRef(""), stringHelperNewBytesRef(""))
	if err == nil {
		t.Error("Expected error for empty vs empty (equal terms)")
	}
}

func TestSortKeyLengthEdgeCases(t *testing.T) {
	// Test with nil
	result, err := SortKeyLength(nil, stringHelperNewBytesRef("foo"))
	if err != nil {
		t.Errorf("Unexpected error with nil: %v", err)
	}
	if result != 1 {
		t.Errorf("Expected 1 for nil priorTerm, got %d", result)
	}

	// Test empty strings
	result, err = SortKeyLength(stringHelperNewBytesRef(""), stringHelperNewBytesRef("a"))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result != 1 {
		t.Errorf("Expected 1 for empty vs 'a', got %d", result)
	}
}

func TestMurmurHash3EdgeCases(t *testing.T) {
	// Test with nil BytesRef
	hash := MurmurHash3_x86_32_BytesRef(nil, 0)
	// Should not panic and return some value
	_ = hash

	// Test with empty BytesRef
	emptyRef := NewBytesRefEmpty()
	hash = MurmurHash3_x86_32_BytesRef(emptyRef, 0)
	_ = hash

	// Test various lengths to exercise tail handling
	testCases := []struct {
		input  string
		seed   int
		desc   string
	}{
		{"a", 0, "1 byte"},
		{"ab", 0, "2 bytes"},
		{"abc", 0, "3 bytes"},
		{"abcd", 0, "4 bytes"},
		{"abcde", 0, "5 bytes"},
		{"abcdef", 0, "6 bytes"},
		{"abcdefg", 0, "7 bytes"},
		{"abcdefgh", 0, "8 bytes"},
		{"abcdefghi", 0, "9 bytes"},
		{"abcdefghij", 0, "10 bytes"},
		{"abcdefghijk", 0, "11 bytes"},
		{"abcdefghijkl", 0, "12 bytes"},
		{"abcdefghijklm", 0, "13 bytes"},
		{"abcdefghijklmn", 0, "14 bytes"},
		{"abcdefghijklmno", 0, "15 bytes"},
		{"abcdefghijklmnop", 0, "16 bytes"},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			hash := MurmurHash3_x86_32_BytesRef(stringHelperNewBytesRef(tc.input), tc.seed)
			// Just verify it doesn't panic and produces consistent results
			hash2 := MurmurHash3_x86_32_BytesRef(stringHelperNewBytesRef(tc.input), tc.seed)
			if hash != hash2 {
				t.Errorf("Hash not consistent for %s", tc.desc)
			}
		})
	}
}

// Benchmark tests
func BenchmarkBytesDifference(b *testing.B) {
	left := stringHelperNewBytesRef("foobar")
	right := stringHelperNewBytesRef("foozo")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BytesDifference(left, right)
	}
}

func BenchmarkStartsWith(b *testing.B) {
	ref := stringHelperNewBytesRef("foobar")
	prefix := stringHelperNewBytesRef("foo")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		StartsWith(ref, prefix)
	}
}

func BenchmarkEndsWith(b *testing.B) {
	ref := stringHelperNewBytesRef("foobar")
	suffix := stringHelperNewBytesRef("bar")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EndsWith(ref, suffix)
	}
}

func BenchmarkMurmurHash3(b *testing.B) {
	data := stringHelperNewBytesRef("test data for hashing")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MurmurHash3_x86_32_BytesRef(data, 0)
	}
}
