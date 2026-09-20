// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// stubKnnStrategy is a minimal KnnSearchStrategy for testing.
type stubKnnStrategy struct{ name string }

func (s *stubKnnStrategy) StrategyName() string { return s.name }

var _ search.KnnSearchStrategy = (*stubKnnStrategy)(nil)

// stubBitSet is a minimal util.BitSet stub that marks every other bit as a parent.
type stubBitSet struct {
	size int
}

func (s *stubBitSet) Get(i int) bool              { return i%2 == 0 }
func (s *stubBitSet) Length() int                 { return s.size }
func (s *stubBitSet) Set(i int)                   {}
func (s *stubBitSet) Clear(i int)                 {}
func (s *stubBitSet) ClearAll()                   {}
func (s *stubBitSet) ClearRange(_, _ int)         {}
func (s *stubBitSet) GetAndSet(i int) bool        { return i%2 == 0 }
func (s *stubBitSet) Cardinality() int            { return s.size / 2 }
func (s *stubBitSet) ApproximateCardinality() int { return s.size / 2 }
func (s *stubBitSet) NextSetBitBounded(from int) int {
	for i := from; i < s.size; i++ {
		if i%2 == 0 {
			return i
		}
	}
	return util.NO_MORE_DOCS
}
func (s *stubBitSet) NextSetBitInRange(from, to int) int { return s.NextSetBitBounded(from) }
func (s *stubBitSet) PrevSetBit(from int) int {
	for i := from; i >= 0; i-- {
		if i%2 == 0 {
			return i
		}
	}
	return -1
}
func (s *stubBitSet) OrIterator(_ util.DocIdSetIterator) error { return nil }
func (s *stubBitSet) RamBytesUsed() int64                      { return 0 }

var _ util.BitSet = (*stubBitSet)(nil)

func newStubBitSet(size int) *stubBitSet { return &stubBitSet{size: size} }

func TestDiversifyingNearestChildrenKnnCollector_Construction(t *testing.T) {
	bs := newStubBitSet(20)
	c, err := NewDiversifyingNearestChildrenKnnCollector(5, 100, bs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.K() != 5 {
		t.Errorf("K() = %d, want 5", c.K())
	}
	if c.NumCollected() != 0 {
		t.Errorf("NumCollected() = %d, want 0", c.NumCollected())
	}
}

func TestDiversifyingNearestChildrenKnnCollector_InvalidK(t *testing.T) {
	bs := newStubBitSet(10)
	_, err := NewDiversifyingNearestChildrenKnnCollector(0, 100, bs)
	if err == nil {
		t.Fatal("expected error for k=0")
	}
}

func TestDiversifyingNearestChildrenKnnCollector_CollectSingleParent(t *testing.T) {
	// parentBitSet: even indices are parents.
	// child=1 → parent=2, child=3 → parent=4
	bs := newStubBitSet(10)
	c, _ := NewDiversifyingNearestChildrenKnnCollector(5, 100, bs)

	accepted, err := c.Collect(1, 0.9)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if !accepted {
		t.Error("Collect(1, 0.9): expected true (first entry)")
	}
	if c.NumCollected() != 1 {
		t.Errorf("NumCollected() = %d, want 1", c.NumCollected())
	}
}

func TestDiversifyingNearestChildrenKnnCollector_DiversifyPerParent(t *testing.T) {
	// Two children of the same parent (parent=2): second collect should update if score > first.
	bs := newStubBitSet(10)
	c, _ := NewDiversifyingNearestChildrenKnnCollector(5, 100, bs)

	_, _ = c.Collect(1, 0.5)         // child=1, parent=2
	accepted, _ := c.Collect(1, 0.8) // same parent=2, higher score
	if !accepted {
		t.Error("expected update accepted for higher score on same parent")
	}
	// Still only one entry in heap (one per parent).
	if c.NumCollected() != 1 {
		t.Errorf("NumCollected() = %d, want 1 (one per parent)", c.NumCollected())
	}
}

func TestDiversifyingNearestChildrenKnnCollector_MinCompetitiveSimilarity(t *testing.T) {
	bs := newStubBitSet(20)
	c, _ := NewDiversifyingNearestChildrenKnnCollector(2, 100, bs)

	// Before filling, should return -Inf.
	if c.MinCompetitiveSimilarity() != float32(math.Inf(-1)) {
		t.Errorf("expected -Inf before heap is full")
	}

	// children for two different parents.
	_, _ = c.Collect(1, 0.7) // parent=2
	_, _ = c.Collect(3, 0.5) // parent=4

	// Now full (k=2). MinCompetitiveSimilarity should be the min score in heap.
	mcs := c.MinCompetitiveSimilarity()
	if mcs <= 0 {
		t.Errorf("MinCompetitiveSimilarity() = %v, want > 0", mcs)
	}
}

func TestDiversifyingNearestChildrenKnnCollector_TopDocs(t *testing.T) {
	bs := newStubBitSet(20)
	c, _ := NewDiversifyingNearestChildrenKnnCollector(3, 100, bs)

	_, _ = c.Collect(1, 0.9) // parent=2
	_, _ = c.Collect(3, 0.7) // parent=4
	_, _ = c.Collect(5, 0.5) // parent=6

	docs := c.TopDocs()
	if len(docs) != 3 {
		t.Fatalf("TopDocs() len = %d, want 3", len(docs))
	}
	// First result should have highest score.
	if docs[0].Score < docs[1].Score || docs[1].Score < docs[2].Score {
		t.Errorf("TopDocs not in descending score order: %v", docs)
	}
}

func TestDiversifyingNearestChildrenKnnCollector_String(t *testing.T) {
	bs := newStubBitSet(10)
	c, _ := NewDiversifyingNearestChildrenKnnCollector(3, 100, bs)
	s := c.String()
	if s == "" {
		t.Error("String() returned empty string")
	}
}

// TestDiversifyingNearestChildrenKnnCollector_StrategyStoredAndRetrieved verifies
// that NewDiversifyingNearestChildrenKnnCollectorWithStrategy stores the strategy
// and GetStrategy returns the same instance.
func TestDiversifyingNearestChildrenKnnCollector_StrategyStoredAndRetrieved(t *testing.T) {
	bs := newStubBitSet(20)
	strategy := &stubKnnStrategy{name: "hnsw"}

	c, err := NewDiversifyingNearestChildrenKnnCollectorWithStrategy(5, 100, strategy, bs)
	if err != nil {
		t.Fatalf("NewDiversifyingNearestChildrenKnnCollectorWithStrategy: %v", err)
	}
	got := c.GetStrategy()
	if got == nil {
		t.Fatal("GetStrategy() returned nil, want non-nil")
	}
	if got.StrategyName() != "hnsw" {
		t.Errorf("GetStrategy().StrategyName() = %q, want %q", got.StrategyName(), "hnsw")
	}
}

// TestDiversifyingNearestChildrenKnnCollector_NoStrategyNil verifies that a
// collector created without a strategy returns nil from GetStrategy.
func TestDiversifyingNearestChildrenKnnCollector_NoStrategyNil(t *testing.T) {
	bs := newStubBitSet(10)
	c, _ := NewDiversifyingNearestChildrenKnnCollector(3, 50, bs)
	if got := c.GetStrategy(); got != nil {
		t.Errorf("GetStrategy() = %v, want nil", got)
	}
}

// TestDiversifyingNearestChildrenKnnCollector_StrategyDoesNotAffectCollection
// verifies that storing a strategy does not change collection results
// (the strategy is deferred to HNSW traversal wiring).
func TestDiversifyingNearestChildrenKnnCollector_StrategyDoesNotAffectCollection(t *testing.T) {
	bs := newStubBitSet(20)
	strategy := &stubKnnStrategy{name: "seeded"}

	withStrategy, _ := NewDiversifyingNearestChildrenKnnCollectorWithStrategy(3, 100, strategy, bs)
	without, _ := NewDiversifyingNearestChildrenKnnCollector(3, 100, bs)

	// Same collect sequence.
	for _, c := range []*DiversifyingNearestChildrenKnnCollector{withStrategy, without} {
		_, _ = c.Collect(1, 0.9)
		_, _ = c.Collect(3, 0.7)
	}

	d1 := withStrategy.TopDocs()
	d2 := without.TopDocs()
	if len(d1) != len(d2) {
		t.Errorf("TopDocs length mismatch: with=%d without=%d", len(d1), len(d2))
	}
}
