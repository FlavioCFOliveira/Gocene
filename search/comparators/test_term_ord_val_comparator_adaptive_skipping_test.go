// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/comparators/TestTermOrdValComparatorAdaptiveSkipping.java
//
// The original Java test requires IndexWriter + IndexSearcher integration with
// TermOrdValComparator and sort skip-index support, which is not yet available
// in Gocene. The tests below validate the comparator types that exist in this
// package and the concrete UpdateableDocIdSetIterator implementation.

package comparators

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestComparatorConstructors verifies that all comparator constructors in the
// package return non-nil values and produce unique instances.
func TestComparatorConstructors(t *testing.T) {
	tests := []struct {
		name string
		ctor func() any
	}{
		{"DocComparator", func() any { return NewDocComparator() }},
		{"DoubleComparator", func() any { return NewDoubleComparator() }},
		{"FloatComparator", func() any { return NewFloatComparator() }},
		{"IntComparator", func() any { return NewIntComparator() }},
		{"LongComparator", func() any { return NewLongComparator() }},
		{"NumericComparator", func() any { return NewNumericComparator() }},
		{"TermOrdValComparator", func() any { return NewTermOrdValComparator() }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := tt.ctor()
			if a == nil {
				t.Errorf("New%v() returned nil", tt.name)
			}
			// Verify each call returns a fresh instance.
			b := tt.ctor()
			if a == b {
				t.Errorf("New%v() returned same instance on consecutive calls", tt.name)
			}
		})
	}
}

// TestUpdateableDocIdSetIterator_EdgeCases exercises additional edge cases
// for UpdateableDocIdSetIterator beyond the basic iteration tests.
func TestUpdateableDocIdSetIterator_EdgeCases(t *testing.T) {
	// When doc is NO_MORE_DOCS, NextDoc stays exhausted.
	it := NewUpdateableDocIdSetIterator()
	it.Update(search.NewEmptyDocIdSetIterator())
	if doc, _ := it.NextDoc(); doc != search.NO_MORE_DOCS {
		t.Errorf("NextDoc on empty: want NO_MORE_DOCS, got %d", doc)
	}
	if doc := it.DocID(); doc != search.NO_MORE_DOCS {
		t.Errorf("DocID: want NO_MORE_DOCS, got %d", doc)
	}
}

// TestUpdateableDocIdSetIterator_CostExists verifies the Cost method works
// by default on a fresh empty iterator.
func TestUpdateableDocIdSetIterator_CostExists(t *testing.T) {
	it := NewUpdateableDocIdSetIterator()
	if cost := it.Cost(); cost != 0 {
		t.Errorf("Cost on empty: want 0, got %d", cost)
	}
}

// TestUpdateableDocIdSetIterator_AdvanceBeyondMax verifies that advancing
// past the end of a range returns NO_MORE_DOCS.
func TestUpdateableDocIdSetIterator_AdvanceBeyondMax(t *testing.T) {
	it := NewUpdateableDocIdSetIterator()
	it.Update(search.NewRangeDocIdSetIterator(5, 10))

	doc, err := it.Advance(100)
	if err != nil {
		t.Fatalf("Advance(100): %v", err)
	}
	if doc != search.NO_MORE_DOCS {
		t.Errorf("Advance(100): want NO_MORE_DOCS, got %d", doc)
	}
}

// TestUpdateableDocIdSetIterator_UpdateAfterExhaustion verifies that Update
// re-arms an exhausted iterator.
func TestUpdateableDocIdSetIterator_UpdateAfterExhaustion(t *testing.T) {
	it := NewUpdateableDocIdSetIterator()
	it.Update(search.NewRangeDocIdSetIterator(0, 3))

	// Exhaust the iterator.
	for doc, _ := it.NextDoc(); doc != search.NO_MORE_DOCS; doc, _ = it.NextDoc() {
	}
	if doc := it.DocID(); doc != search.NO_MORE_DOCS {
		t.Fatalf("expected exhausted, got doc=%d", doc)
	}

	// Update should re-arm it.
	it.Update(search.NewRangeDocIdSetIterator(10, 12))
	doc, err := it.NextDoc()
	if err != nil || doc != 10 {
		t.Fatalf("after re-arm: NextDoc = (%d, %v), want (10, nil)", doc, err)
	}
}
