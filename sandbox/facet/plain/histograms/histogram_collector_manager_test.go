// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of
// org.apache.lucene.sandbox.facet.plain.histograms.TestHistogramCollectorManager
// (algorithmic subset that does not require a live IndexSearcher).
package histograms

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/internal/hppc"
)

// TestHistogramCollectorManager_DefaultMaxBuckets verifies the default
// bucket limit is 1024.
func TestHistogramCollectorManager_DefaultMaxBuckets(t *testing.T) {
	m, err := NewHistogramCollectorManager("f", 4)
	if err != nil {
		t.Fatalf("NewHistogramCollectorManager: %v", err)
	}
	if m.MaxBuckets() != 1024 {
		t.Errorf("default MaxBuckets = %d; want 1024", m.MaxBuckets())
	}
}

// TestHistogramCollectorManager_InvalidBucketWidth verifies that a
// bucketWidth < 2 is rejected.
func TestHistogramCollectorManager_InvalidBucketWidth(t *testing.T) {
	if _, err := NewHistogramCollectorManager("f", 1); err == nil {
		t.Error("expected error for bucketWidth=1")
	}
	if _, err := NewHistogramCollectorManager("f", 0); err == nil {
		t.Error("expected error for bucketWidth=0")
	}
	if _, err := NewHistogramCollectorManager("f", -1); err == nil {
		t.Error("expected error for bucketWidth=-1")
	}
}

// TestHistogramCollectorManager_InvalidMaxBuckets verifies that maxBuckets<1
// is rejected.
func TestHistogramCollectorManager_InvalidMaxBuckets(t *testing.T) {
	if _, err := NewHistogramCollectorManagerWithMax("f", 4, 0); err == nil {
		t.Error("expected error for maxBuckets=0")
	}
}

// TestHistogramCollectorManager_Reduce verifies that Reduce correctly
// merges counts from multiple collectors.
func TestHistogramCollectorManager_Reduce(t *testing.T) {
	m, err := NewHistogramCollectorManager("f", 4)
	if err != nil {
		t.Fatalf("NewHistogramCollectorManager: %v", err)
	}

	c0 := m.NewCollector()
	c1 := m.NewCollector()

	// Manually populate counts to simulate segment collection.
	c0.GetCounts()[0] = 2 // bucket 0: docs with values [0,4)
	c0.GetCounts()[1] = 1 // bucket 1: docs with values [4,8)
	c1.GetCounts()[1] = 3 // bucket 1 from another segment
	c1.GetCounts()[2] = 1 // bucket 2

	reduced, err := m.Reduce([]*HistogramCollector{c0, c1})
	if err != nil {
		t.Fatalf("Reduce: %v", err)
	}

	want := hppc.LongIntHashMap{0: 2, 1: 4, 2: 1}
	if len(reduced) != len(want) {
		t.Fatalf("reduced len = %d; want %d", len(reduced), len(want))
	}
	for k, v := range want {
		if got := reduced[k]; got != v {
			t.Errorf("bucket %d: got %d; want %d", k, got, v)
		}
	}
}

// TestHistogramCollectorManager_ReduceMaxBucketsExceeded verifies that
// Reduce panics when the bucket count exceeds the limit.
func TestHistogramCollectorManager_ReduceMaxBucketsExceeded(t *testing.T) {
	m, err := NewHistogramCollectorManagerWithMax("f", 4, 1)
	if err != nil {
		t.Fatalf("NewHistogramCollectorManagerWithMax: %v", err)
	}

	c0 := m.NewCollector()
	c1 := m.NewCollector()
	c0.GetCounts()[0] = 1
	c1.GetCounts()[1] = 1

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when maxBuckets exceeded")
		}
	}()
	_, _ = m.Reduce([]*HistogramCollector{c0, c1})
}

// TestCheckMaxBuckets verifies the panic behaviour.
func TestCheckMaxBuckets(t *testing.T) {
	// Should not panic.
	CheckMaxBuckets(5, 10)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when size > maxBuckets")
		}
	}()
	CheckMaxBuckets(11, 10)
}

// TestFloorDiv verifies the floorDiv helper for positive and negative inputs.
func TestFloorDiv(t *testing.T) {
	cases := []struct {
		a, b, want int64
	}{
		{0, 4, 0},
		{3, 4, 0},
		{4, 4, 1},
		{7, 4, 1},
		{8, 4, 2},
		{-1, 4, -1},
		{-4, 4, -1},
		{-5, 4, -2},
	}
	for _, tc := range cases {
		if got := floorDiv(tc.a, tc.b); got != tc.want {
			t.Errorf("floorDiv(%d, %d) = %d; want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

// TestHistogramCollector_NaiveMultiValued exercises the naive multi-valued
// leaf collector directly with a stub SortedNumericDocValuesEx.
func TestHistogramCollector_NaiveMultiValued(t *testing.T) {
	// Documents: doc 0 has values [3, 8]; doc 1 has values [4, 6, 8].
	//
	// With bucketWidth=4:
	//   doc 0: floor(3/4)=0, floor(8/4)=2  → buckets 0 and 2 (1 doc each)
	//   doc 1: floor(4/4)=1, floor(6/4)=1, floor(8/4)=2
	//          → bucket 1 (1 doc) and 2 (1 doc, same bucket skipped)
	//          Actually: unique buckets for doc 1 are 1 and 2 → 2 buckets.
	//   Expected: {0:1, 1:1, 2:2}
	docs := []struct{ values []int64 }{{[]int64{3, 8}}, {[]int64{4, 6, 8}}}
	idx := -1

	// Build stub.
	sv := &stubSortedNumericDV{docs: docs}
	counts := make(hppc.LongIntHashMap)
	lc := &histogramNaiveLeafCollector{
		values:      sv,
		bucketWidth: 4,
		maxBuckets:  1024,
		counts:      counts,
	}
	for doc := range docs {
		if err := lc.Collect(doc); err != nil {
			t.Fatalf("Collect(%d): %v", doc, err)
		}
	}
	_ = idx

	want := hppc.LongIntHashMap{0: 1, 1: 1, 2: 2}
	if len(counts) != len(want) {
		t.Fatalf("counts len = %d; want %d", len(counts), len(want))
	}
	for k, v := range want {
		if got := counts[k]; got != v {
			t.Errorf("bucket %d: got %d; want %d", k, got, v)
		}
	}
}

// TestHistogramCollector_NaiveSingleValued exercises the naive single-valued
// leaf collector.
func TestHistogramCollector_NaiveSingleValued(t *testing.T) {
	// doc 0 → value 3 → bucket 0; doc 1 → value 4 → bucket 1; doc 2 → value 6 → bucket 1.
	sv := &stubNumericDV{values: []int64{3, 4, 6}}
	counts := make(hppc.LongIntHashMap)
	lc := &histogramNaiveSingleValuedLeafCollector{
		values:      sv,
		bucketWidth: 4,
		maxBuckets:  1024,
		counts:      counts,
	}
	for doc := 0; doc < 3; doc++ {
		if err := lc.Collect(doc); err != nil {
			t.Fatalf("Collect(%d): %v", doc, err)
		}
	}
	want := hppc.LongIntHashMap{0: 1, 1: 2}
	for k, v := range want {
		if got := counts[k]; got != v {
			t.Errorf("bucket %d: got %d; want %d", k, got, v)
		}
	}
}

// ---- stubs ----

// stubSortedNumericDV is a stub SortedNumericDocValuesEx for testing.
type stubSortedNumericDV struct {
	docs  []struct{ values []int64 }
	cur   int
	pos   int
	valid bool
}

type stubSortedNumericValues = stubSortedNumericDV

func (s *stubSortedNumericDV) AdvanceExact(doc int) (bool, error) {
	if doc < len(s.docs) {
		s.cur = doc
		s.pos = 0
		s.valid = true
		return true, nil
	}
	s.valid = false
	return false, nil
}

func (s *stubSortedNumericDV) DocValueCount() int {
	return len(s.docs[s.cur].values)
}

func (s *stubSortedNumericDV) NextValue() (int64, error) {
	v := s.docs[s.cur].values[s.pos]
	s.pos++
	return v, nil
}

// stubNumericDV is a stub NumericDocValuesEx for testing.
type stubNumericDV struct {
	values []int64
	cur    int
	valid  bool
}

func (s *stubNumericDV) AdvanceExact(doc int) (bool, error) {
	if doc < len(s.values) {
		s.cur = doc
		s.valid = true
		return true, nil
	}
	s.valid = false
	return false, nil
}

func (s *stubNumericDV) LongValue() (int64, error) {
	return s.values[s.cur], nil
}

func (s *stubNumericDV) DocIDRunEnd() (int, error) {
	// Stub: run ends immediately.
	return s.cur + 1, nil
}

func (s *stubNumericDV) LongValues(n int, docBuffer []int, dst []int64, missing int64) error {
	for i := 0; i < n; i++ {
		doc := docBuffer[i]
		if doc < len(s.values) {
			dst[i] = s.values[doc]
		} else {
			dst[i] = missing
		}
	}
	return nil
}
