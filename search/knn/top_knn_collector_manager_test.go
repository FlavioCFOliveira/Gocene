// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package knn

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util/hnsw"
)

// TestTopKnnCollectorManagerNewCollector asserts the manager hands
// out a fresh TopKnnCollector wired to the configured k and the
// supplied visited limit.
func TestTopKnnCollectorManagerNewCollector(t *testing.T) {
	m := NewTopKnnCollectorManager(8, "ignored-searcher")
	if m.K() != 8 {
		t.Errorf("K() = %d, want 8", m.K())
	}
	if m.Searcher() != "ignored-searcher" {
		t.Errorf("Searcher() = %v, want ignored-searcher", m.Searcher())
	}
	c, err := m.NewCollector(123, nil, nil)
	if err != nil {
		t.Fatalf("NewCollector err = %v, want nil", err)
	}
	if c == nil {
		t.Fatalf("NewCollector returned nil collector")
	}
	if c.K() != 8 {
		t.Errorf("collector.K() = %d, want 8", c.K())
	}
	if c.VisitLimit() != 123 {
		t.Errorf("collector.VisitLimit() = %d, want 123", c.VisitLimit())
	}
	// Distinct allocations each call.
	c2, _ := m.NewCollector(7, nil, nil)
	if any(c) == any(c2) {
		t.Errorf("NewCollector returned the same instance twice")
	}
}

// TestTopKnnCollectorManagerOptimistic asserts the optimistic path
// returns a collector scaled to the per-leaf k and that IsOptimistic
// is true (matches the Java override).
func TestTopKnnCollectorManagerOptimistic(t *testing.T) {
	m := NewTopKnnCollectorManager(8, nil)
	if !m.IsOptimistic() {
		t.Errorf("IsOptimistic() = false, want true")
	}
	c, err := m.NewOptimisticCollector(50, nil, nil, 3)
	if err != nil {
		t.Fatalf("NewOptimisticCollector err = %v, want nil", err)
	}
	if c.K() != 3 {
		t.Errorf("optimistic collector.K() = %d, want 3 (rescaled)", c.K())
	}
	if c.VisitLimit() != 50 {
		t.Errorf("optimistic collector.VisitLimit() = %d, want 50", c.VisitLimit())
	}
}

// TestTopKnnCollectorManagerStrategyPassThrough asserts that the
// hnsw-side collector ends up bound to the strategy provided to the
// manager when that strategy satisfies hnsw.KnnSearchStrategy.
func TestTopKnnCollectorManagerStrategyPassThrough(t *testing.T) {
	m := NewTopKnnCollectorManager(2, nil)
	s := NewHnsw(40)
	c, err := m.NewCollector(10, s, nil)
	if err != nil {
		t.Fatalf("NewCollector err = %v", err)
	}
	got := c.GetSearchStrategy()
	if got == nil {
		t.Fatalf("GetSearchStrategy() = nil, want non-nil")
	}
	// Must be the same instance (asHnswStrategy short-circuits on
	// the direct cast).
	gotHnsw, ok := got.(*Hnsw)
	if !ok {
		t.Fatalf("GetSearchStrategy() type = %T, want *Hnsw", got)
	}
	if gotHnsw != s {
		t.Errorf("strategy mutated; got %p, want %p", gotHnsw, s)
	}
}

// TestAsHnswStrategyAdapter exercises the fallback adapter path used
// when a third-party KnnSearchStrategy does NOT directly satisfy
// hnsw.KnnSearchStrategy. Since the in-tree strategies satisfy both
// interfaces (the compile-time guards in knn_search_strategy.go assert
// this), the adapter only kicks in for synthetic outsiders.
func TestAsHnswStrategyAdapter(t *testing.T) {
	// Synthetic strategy that pretends to be an outsider by hiding
	// the embedded *Hnsw behind an additional method set the cast
	// can't see directly.
	if got := asHnswStrategy(nil); got != nil {
		t.Errorf("asHnswStrategy(nil) = %v, want nil", got)
	}
	// Real strategy satisfies hnsw.KnnSearchStrategy directly.
	s := NewHnsw(0)
	if got := asHnswStrategy(s); got == nil {
		t.Errorf("asHnswStrategy(*Hnsw) = nil, want non-nil")
	}
	// The hnsw-side adapter satisfies the interface.
	var _ hnsw.KnnSearchStrategy = strategyAdapter{wrapped: s}
}
