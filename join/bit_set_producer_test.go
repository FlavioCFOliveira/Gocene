// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

func TestNewFixedBitSet(t *testing.T) {
	bs := NewFixedBitSet(100)
	if bs == nil {
		t.Fatal("Expected FixedBitSet to be created")
	}

	if bs.Size() != 100 {
		t.Errorf("Expected size 100, got %d", bs.Size())
	}
}

func TestFixedBitSetSetAndGet(t *testing.T) {
	bs := NewFixedBitSet(100)

	// Set some bits
	bs.Set(0)
	bs.Set(50)
	bs.Set(99)

	// Check they are set
	if !bs.Get(0) {
		t.Error("Expected bit 0 to be set")
	}
	if !bs.Get(50) {
		t.Error("Expected bit 50 to be set")
	}
	if !bs.Get(99) {
		t.Error("Expected bit 99 to be set")
	}

	// Check unset bits
	if bs.Get(1) {
		t.Error("Expected bit 1 to be unset")
	}
	if bs.Get(49) {
		t.Error("Expected bit 49 to be unset")
	}

	// Out of bounds
	if bs.Get(-1) {
		t.Error("Expected bit -1 to be unset (out of bounds)")
	}
	if bs.Get(100) {
		t.Error("Expected bit 100 to be unset (out of bounds)")
	}
}

func TestFixedBitSetClear(t *testing.T) {
	bs := NewFixedBitSet(100)

	bs.Set(50)
	if !bs.Get(50) {
		t.Error("Expected bit 50 to be set")
	}

	bs.Clear(50)
	if bs.Get(50) {
		t.Error("Expected bit 50 to be cleared")
	}
}

func TestFixedBitSetCardinality(t *testing.T) {
	bs := NewFixedBitSet(100)

	if bs.Cardinality() != 0 {
		t.Errorf("Expected cardinality 0, got %d", bs.Cardinality())
	}

	bs.Set(0)
	bs.Set(50)
	bs.Set(99)

	if bs.Cardinality() != 3 {
		t.Errorf("Expected cardinality 3, got %d", bs.Cardinality())
	}
}

func TestFixedBitSetNextSetBit(t *testing.T) {
	bs := NewFixedBitSet(100)

	// No bits set
	if bs.NextSetBit(0) != -1 {
		t.Error("Expected -1 when no bits set")
	}

	// Set some bits
	bs.Set(10)
	bs.Set(20)
	bs.Set(30)

	// Find first set bit
	if bs.NextSetBit(0) != 10 {
		t.Errorf("Expected next set bit from 0 to be 10, got %d", bs.NextSetBit(0))
	}

	// Find next set bit after 10
	if bs.NextSetBit(11) != 20 {
		t.Errorf("Expected next set bit from 11 to be 20, got %d", bs.NextSetBit(11))
	}

	// Find next set bit after 20
	if bs.NextSetBit(21) != 30 {
		t.Errorf("Expected next set bit from 21 to be 30, got %d", bs.NextSetBit(21))
	}

	// No more set bits
	if bs.NextSetBit(31) != -1 {
		t.Error("Expected -1 when no more set bits")
	}
}

func TestFixedBitSetLargeSize(t *testing.T) {
	// Test with size larger than 64
	bs := NewFixedBitSet(200)

	bs.Set(100)
	bs.Set(150)

	if !bs.Get(100) {
		t.Error("Expected bit 100 to be set")
	}
	if !bs.Get(150) {
		t.Error("Expected bit 150 to be set")
	}
	if bs.Cardinality() != 2 {
		t.Errorf("Expected cardinality 2, got %d", bs.Cardinality())
	}
}

func TestPopcount(t *testing.T) {
	tests := []struct {
		value    uint64
		expected int
	}{
		{0, 0},
		{1, 1},
		{2, 1},
		{3, 2},
		{0xFF, 8},
		{0xFFFF, 16},
		{0xFFFFFFFFFFFFFFFF, 64},
	}

	for _, test := range tests {
		result := popcount(test.value)
		if result != test.expected {
			t.Errorf("popcount(%d) = %d, expected %d", test.value, result, test.expected)
		}
	}
}

func TestTrailingZeros(t *testing.T) {
	tests := []struct {
		value    uint64
		expected int
	}{
		{0, 64},
		{1, 0},
		{2, 1},
		{4, 2},
		{8, 3},
		{0x10, 4},
		{0x100, 8},
		{0x1000, 12},
	}

	for _, test := range tests {
		result := trailingZeros(test.value)
		if result != test.expected {
			t.Errorf("trailingZeros(%d) = %d, expected %d", test.value, result, test.expected)
		}
	}
}

func TestNewQueryBitSetProducer(t *testing.T) {
	query := &mockQuery{}
	producer := NewQueryBitSetProducer(query)

	if producer == nil {
		t.Fatal("Expected QueryBitSetProducer to be created")
	}

	if producer.query != query {
		t.Error("Expected producer to store the query")
	}
}

func TestQueryBitSetProducerGetBitSet(t *testing.T) {
	query := &mockQuery{}
	producer := NewQueryBitSetProducer(query)

	// Create a mock context
	ctx := &index.LeafReaderContext{}

	// GetBitSet should return a bit set (even if empty due to mock)
	bs, err := producer.GetBitSet(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if bs == nil {
		t.Error("Expected bit set to be returned")
	}
}

func TestNewFixedBitSetCachingWrapper(t *testing.T) {
	query := &mockQuery{}
	producer := NewQueryBitSetProducer(query)
	wrapper := NewFixedBitSetCachingWrapper(producer)

	if wrapper == nil {
		t.Fatal("Expected FixedBitSetCachingWrapper to be created")
	}

	if wrapper.producer != producer {
		t.Error("Expected wrapper to store the producer")
	}

	if wrapper.cache == nil {
		t.Error("Expected cache to be initialized")
	}
}

func TestFixedBitSetCachingWrapperGetBitSet(t *testing.T) {
	query := &mockQuery{}
	producer := NewQueryBitSetProducer(query)
	wrapper := NewFixedBitSetCachingWrapper(producer)

	// Create a mock context
	ctx := &index.LeafReaderContext{}

	// First call should cache the result
	bs1, err := wrapper.GetBitSet(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if bs1 == nil {
		t.Fatal("Expected bit set to be returned")
	}

	// Second call should return cached result
	bs2, err := wrapper.GetBitSet(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Should be the same instance
	if bs1 != bs2 {
		t.Error("Expected cached bit set to be returned on second call")
	}
}

func TestFixedBitSetCachingWrapperClear(t *testing.T) {
	query := &mockQuery{}
	producer := NewQueryBitSetProducer(query)
	wrapper := NewFixedBitSetCachingWrapper(producer)

	// Create a mock context and get a bit set to cache it
	ctx := &index.LeafReaderContext{}
	_, err := wrapper.GetBitSet(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Clear the cache
	wrapper.Clear()

	// Cache should be empty
	if len(wrapper.cache) != 0 {
		t.Error("Expected cache to be empty after clear")
	}
}

func TestBitSetProducerInterface(t *testing.T) {
	// Test that QueryBitSetProducer implements BitSetProducer
	query := &mockQuery{}
	var producer BitSetProducer = NewQueryBitSetProducer(query)

	if producer == nil {
		t.Error("Expected QueryBitSetProducer to implement BitSetProducer")
	}

	// Test that FixedBitSetCachingWrapper implements BitSetProducer
	wrapper := NewFixedBitSetCachingWrapper(producer)
	var _ BitSetProducer = wrapper
}

// TestQueryBitSetProducerCache verifies that QueryBitSetProducer caches
// computed bitsets keyed by the leaf reader's core cache key.
func TestQueryBitSetProducerCache(t *testing.T) {
	// Use a query whose weight returns nil scorer (empty result).
	mq := &mockQuery{}
	producer := NewQueryBitSetProducer(mq)

	segmentInfo := index.NewSegmentInfo("seg", 2, nil)
	leaf := index.NewLeafReader(segmentInfo)
	leaf.SetMaxDoc(2)

	ctx := index.NewLeafReaderContext(leaf, nil, 0, 0)

	bs1, err := producer.GetBitSet(ctx)
	if err != nil {
		t.Fatalf("GetBitSet error: %v", err)
	}
	if bs1 == nil {
		t.Fatal("expected non-nil bitset")
	}
	if bs1.Cardinality() != 0 {
		t.Fatal("expected empty bitset because scorer is nil")
	}

	// Second call with the same reader context must return the cached instance.
	bs2, err := producer.GetBitSet(ctx)
	if err != nil {
		t.Fatalf("GetBitSet second call error: %v", err)
	}
	if bs1 != bs2 {
		t.Fatal("expected cached bitset to be returned on second call")
	}

	// A new leaf reader (different cache key) should trigger a recompute.
	leaf2 := index.NewLeafReader(segmentInfo)
	leaf2.SetMaxDoc(2)
	ctx2 := index.NewLeafReaderContext(leaf2, nil, 0, 0)
	bs3, err := producer.GetBitSet(ctx2)
	if err != nil {
		t.Fatalf("GetBitSet new reader error: %v", err)
	}
	if bs3 == bs1 {
		t.Fatal("expected new bitset for a different reader")
	}
}
