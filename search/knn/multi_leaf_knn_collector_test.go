// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package knn

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util/hnsw"
)

// TestGlobalScoreCoordination is the Go peer of
// org.apache.lucene.search.knn.TestMultiLeafKnnCollector.testGlobalScoreCoordination,
// which validates the fix for GH#13462.
//
// Two MultiLeafKnnCollectors share a 7-slot BlockingFloatHeap. The
// first fills its sub-collector with scores 100..106; we assert that
// the global heap and the first collector's minCompetitiveSimilarity
// both pin to 100 (the smallest of the seven). The second is then fed
// 10, 11, 12, 13, 200, 14, 300 — an explicitly out-of-order sequence
// designed to expose the bug where the global queue would
// short-circuit on a partial sort. After this run the global heap
// must contain {102, 103, 104, 105, 106, 200, 300} and the second
// collector's minCompetitiveSimilarity must be 102.
func TestGlobalScoreCoordination(t *testing.T) {
	const k = 7
	globalHeap := hnsw.NewBlockingFloatHeap(k)
	collector1 := NewMultiLeafKnnCollector(k, globalHeap, hnsw.NewTopKnnCollector(k, math.MaxInt32, nil))
	collector2 := NewMultiLeafKnnCollector(k, globalHeap, hnsw.NewTopKnnCollector(k, math.MaxInt32, nil))

	// Fill collector1 with the seven scores 100..106.
	for i := 0; i < k; i++ {
		collector1.Collect(0, 100+float32(i))
	}

	if got, want := globalHeap.Peek(), float32(100); got != want {
		t.Fatalf("after collector1 fill, globalHeap.Peek() = %v, want %v", got, want)
	}
	if got, want := collector1.MinCompetitiveSimilarity(), float32(100); got != want {
		t.Fatalf("after collector1 fill, collector1.MinCompetitiveSimilarity() = %v, want %v", got, want)
	}

	// Feed collector2 in the unsorted order from the Java test.
	collector2.Collect(0, 10)
	collector2.Collect(0, 11)
	collector2.Collect(0, 12)
	collector2.Collect(0, 13)
	collector2.Collect(0, 200)
	collector2.Collect(0, 14)
	collector2.Collect(0, 300)

	if got, want := globalHeap.Peek(), float32(102); got != want {
		t.Fatalf("after collector2 run, globalHeap.Peek() = %v, want %v", got, want)
	}
	if got, want := collector2.MinCompetitiveSimilarity(), float32(102); got != want {
		t.Fatalf("after collector2 run, collector2.MinCompetitiveSimilarity() = %v, want %v", got, want)
	}
}

// TestMinCompetitiveSimilarityPreFill ensures that before the
// sub-collector has retained k entries, minCompetitiveSimilarity is
// -Inf. Mirrors the Java guard
// "if (kResultsCollected == false) return Float.NEGATIVE_INFINITY".
func TestMinCompetitiveSimilarityPreFill(t *testing.T) {
	const k = 4
	globalHeap := hnsw.NewBlockingFloatHeap(k)
	c := NewMultiLeafKnnCollector(k, globalHeap, hnsw.NewTopKnnCollector(k, math.MaxInt32, nil))
	if got := c.MinCompetitiveSimilarity(); !math.IsInf(float64(got), -1) {
		t.Fatalf("pre-fill MinCompetitiveSimilarity = %v, want -Inf", got)
	}
	c.Collect(0, 1)
	c.Collect(1, 2)
	c.Collect(2, 3)
	if got := c.MinCompetitiveSimilarity(); !math.IsInf(float64(got), -1) {
		t.Fatalf("under-k MinCompetitiveSimilarity = %v, want -Inf", got)
	}
}

// TestMultiLeafConstructorGuards covers the panicking paths of
// NewMultiLeafKnnCollectorWithConfig.
func TestMultiLeafConstructorGuards(t *testing.T) {
	heap := hnsw.NewBlockingFloatHeap(2)
	sub := hnsw.NewTopKnnCollector(2, math.MaxInt32, nil)

	cases := []struct {
		name       string
		greediness float32
		interval   int
		wantPanic  string
	}{
		{"greediness<0", -0.1, 1, "knn: greediness must be in [0,1]"},
		{"greediness>1", 1.1, 1, "knn: greediness must be in [0,1]"},
		{"interval==0", 0.5, 0, "knn: interval must be positive"},
		{"interval<0", 0.5, -1, "knn: interval must be positive"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("expected panic, got nil")
				}
				if msg, ok := r.(string); !ok || msg != tc.wantPanic {
					t.Fatalf("panic = %v, want %q", r, tc.wantPanic)
				}
			}()
			NewMultiLeafKnnCollectorWithConfig(2, tc.greediness, tc.interval, heap, sub)
		})
	}

	t.Run("nil sub", func(t *testing.T) {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatalf("expected panic, got nil")
			}
		}()
		NewMultiLeafKnnCollector(2, heap, nil)
	})
}

// TestMultiLeafDelegatesUnoverriddenMethods exercises the embedded
// AbstractCollector pass-through: K(), VisitLimit(), VisitedCount(),
// EarlyTerminated(), TopDocs(), GetSearchStrategy() must all reach
// the wrapped sub-collector.
func TestMultiLeafDelegatesUnoverriddenMethods(t *testing.T) {
	const k = 3
	heap := hnsw.NewBlockingFloatHeap(k)
	sub := hnsw.NewTopKnnCollector(k, 5, nil)
	c := NewMultiLeafKnnCollector(k, heap, sub)

	if got, want := c.K(), k; got != want {
		t.Errorf("K() = %d, want %d", got, want)
	}
	if got, want := c.VisitLimit(), int64(5); got != want {
		t.Errorf("VisitLimit() = %d, want %d", got, want)
	}
	if got, want := c.VisitedCount(), int64(0); got != want {
		t.Errorf("VisitedCount() = %d, want %d", got, want)
	}
	if c.EarlyTerminated() {
		t.Errorf("EarlyTerminated() = true, want false on empty collector")
	}
	if c.GetSearchStrategy() != nil {
		t.Errorf("GetSearchStrategy() = non-nil, want nil")
	}
	c.IncVisitedCount(5)
	if !c.EarlyTerminated() {
		t.Errorf("EarlyTerminated() = false after IncVisitedCount(5) into limit=5")
	}
	td := c.TopDocs()
	if td == nil {
		t.Fatalf("TopDocs() returned nil")
	}
}
