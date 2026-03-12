// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"
)

func TestNewBKDTree(t *testing.T) {
	tests := []struct {
		numDims     int
		bytesPerDim int
		wantErr     bool
	}{
		{1, 4, false},  // Single dimension (int32)
		{2, 8, false},  // Two dimensions (lat/lon as int64)
		{1, 1, false},  // Minimum bytes
		{1, 8, false},  // Maximum bytes
		{0, 4, true},   // Invalid: 0 dimensions
		{1, 0, true},   // Invalid: 0 bytes
		{1, 9, true},   // Invalid: too many bytes
	}

	for _, test := range tests {
		tree, err := NewBKDTree(test.numDims, test.bytesPerDim)
		if test.wantErr {
			if err == nil {
				t.Errorf("NewBKDTree(%d, %d) expected error, got nil", test.numDims, test.bytesPerDim)
			}
			continue
		}
		if err != nil {
			t.Errorf("NewBKDTree(%d, %d) unexpected error: %v", test.numDims, test.bytesPerDim, err)
			continue
		}
		if tree.NumDims() != test.numDims {
			t.Errorf("Expected NumDims() = %d, got %d", test.numDims, tree.NumDims())
		}
		if tree.BytesPerDim() != test.bytesPerDim {
			t.Errorf("Expected BytesPerDim() = %d, got %d", test.bytesPerDim, tree.BytesPerDim())
		}
	}
}

func TestBKDTree_PackUnpack(t *testing.T) {
	tree, err := NewBKDTree(2, 4) // 2 dimensions, 4 bytes each
	if err != nil {
		t.Fatalf("NewBKDTree failed: %v", err)
	}

	original := []int64{100, 200}
	packed, err := tree.Pack(original)
	if err != nil {
		t.Fatalf("Pack failed: %v", err)
	}
	if len(packed) != 8 { // 2 dims * 4 bytes
		t.Errorf("Expected 8 bytes, got %d", len(packed))
	}

	unpacked, err := tree.Unpack(packed)
	if err != nil {
		t.Fatalf("Unpack failed: %v", err)
	}
	if len(unpacked) != 2 {
		t.Errorf("Expected 2 values, got %d", len(unpacked))
	}
	if unpacked[0] != 100 || unpacked[1] != 200 {
		t.Errorf("Expected [100, 200], got %v", unpacked)
	}
}

func TestBKDTree_PackInvalidLength(t *testing.T) {
	tree, _ := NewBKDTree(2, 4)

	_, err := tree.Pack([]int64{100}) // Wrong length
	if err == nil {
		t.Error("Expected error for wrong number of values")
	}
}

func TestEncodeDecodeDimension(t *testing.T) {
	tests := []struct {
		value int64
		bytes int
	}{
		{0, 4},
		{1, 4},
		{100, 4},
		{-1, 4},
		{-100, 4},
		{0x7FFFFFFF, 4}, // Max int32
		{-0x80000000, 4}, // Min int32
		{0, 8},
		{0x7FFFFFFFFFFFFFFF, 8}, // Max int64
		{-0x8000000000000000, 8}, // Min int64
	}

	for _, test := range tests {
		buf := make([]byte, test.bytes)
		encodeDimension(test.value, buf, test.bytes)
		decoded := decodeDimension(buf, test.bytes)
		if decoded != test.value {
			t.Errorf("encode/decode %d with %d bytes: got %d", test.value, test.bytes, decoded)
		}
	}
}

func TestPointValues(t *testing.T) {
	tree, _ := NewBKDTree(1, 4)
	pv := NewPointValues(tree)

	// Add some points
	err := pv.Add(0, []int64{100})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	err = pv.Add(1, []int64{200})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	err = pv.Add(2, []int64{50})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	if pv.Size() != 3 {
		t.Errorf("Expected size 3, got %d", pv.Size())
	}

	// Test range intersection
	minPacked, _ := tree.Pack([]int64{75})
	maxPacked, _ := tree.Pack([]int64{150})

	result := pv.Intersect(minPacked, maxPacked)
	if len(result) != 1 || result[0] != 0 {
		t.Errorf("Expected [0] for range [75, 150], got %v", result)
	}

	// Test wider range
	minPacked, _ = tree.Pack([]int64{0})
	maxPacked, _ = tree.Pack([]int64{250})

	result = pv.Intersect(minPacked, maxPacked)
	if len(result) != 3 {
		t.Errorf("Expected 3 results for range [0, 250], got %v", result)
	}
}

func TestPointValues_MultiDimension(t *testing.T) {
	tree, _ := NewBKDTree(2, 4) // 2D points (like lat/lon)
	pv := NewPointValues(tree)

	// Add points
	pv.Add(0, []int64{10, 20})
	pv.Add(1, []int64{30, 40})
	pv.Add(2, []int64{50, 60})

	// Query range that includes only first point
	minPacked, _ := tree.Pack([]int64{0, 0})
	maxPacked, _ := tree.Pack([]int64{20, 30})

	result := pv.Intersect(minPacked, maxPacked)
	if len(result) != 1 || result[0] != 0 {
		t.Errorf("Expected [0] for 2D range query, got %v", result)
	}
}

func TestPointRangeQuery(t *testing.T) {
	q := NewPointRangeQuery("price", []int64{10}, []int64{100})

	tests := []struct {
		values []int64
		want   bool
	}{
		{[]int64{50}, true},   // Inside range
		{[]int64{10}, true},   // At lower bound
		{[]int64{100}, true},  // At upper bound
		{[]int64{5}, false},   // Below range
		{[]int64{200}, false}, // Above range
	}

	for _, test := range tests {
		got := q.Matches(test.values)
		if got != test.want {
			t.Errorf("Matches(%v) = %v, want %v", test.values, got, test.want)
		}
	}
}

func TestPointRangeQuery_Exclusive(t *testing.T) {
	q := NewPointRangeQuery("price", []int64{10}, []int64{100})
	q.SetInclusive(false, false) // Exclusive bounds

	tests := []struct {
		values []int64
		want   bool
	}{
		{[]int64{50}, true},   // Inside range
		{[]int64{10}, false},  // At lower bound (exclusive)
		{[]int64{100}, false}, // At upper bound (exclusive)
	}

	for _, test := range tests {
		got := q.Matches(test.values)
		if got != test.want {
			t.Errorf("Matches(%v) = %v, want %v", test.values, got, test.want)
		}
	}
}

func TestPointRangeQuery_InvalidLength(t *testing.T) {
	q := NewPointRangeQuery("price", []int64{10, 20}, []int64{100, 200})

	// Query expects 2 dimensions
	if q.Matches([]int64{50}) {
		t.Error("Expected false for wrong number of dimensions")
	}
}

func TestPointValues_Clear(t *testing.T) {
	tree, _ := NewBKDTree(1, 4)
	pv := NewPointValues(tree)

	pv.Add(0, []int64{100})
	pv.Add(1, []int64{200})

	if pv.Size() != 2 {
		t.Errorf("Expected size 2, got %d", pv.Size())
	}

	pv.Clear()

	if pv.Size() != 0 {
		t.Errorf("Expected size 0 after clear, got %d", pv.Size())
	}
}
