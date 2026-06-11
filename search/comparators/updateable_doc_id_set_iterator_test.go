// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/comparators/TestUpdateableDocIdSetIterator.java

package comparators_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/search/comparators"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// rangeIter is a thin wrapper around RangeDocIdSetIterator positioned via
// Advance so tests can pre-position it before handing it to Update.
func rangeIter(min, max int) search.DocIdSetIterator {
	return search.NewRangeDocIdSetIterator(min, max)
}

func advancedRange(min, max, target int, t *testing.T) search.DocIdSetIterator {
	t.Helper()
	it := rangeIter(min, max)
	if _, err := it.Advance(target); err != nil {
		t.Fatalf("Advance(%d) on range[%d,%d): %v", target, min, max, err)
	}
	return it
}

// ─── tests ───────────────────────────────────────────────────────────────────

// TestUpdateableDocIdSetIterator_NextDoc mirrors
// TestUpdateableDocIdSetIterator.testNextDoc.
func TestUpdateableDocIdSetIterator_NextDoc(t *testing.T) {
	iterator := comparators.NewUpdateableDocIdSetIterator()

	iterator.Update(rangeIter(10, 20))
	doc, err := iterator.NextDoc()
	if err != nil || doc != 10 {
		t.Fatalf("NextDoc() = (%d, %v), want (10, nil)", doc, err)
	}

	iterator.Update(rangeIter(10, 12))
	doc, err = iterator.NextDoc()
	if err != nil || doc != 11 {
		t.Fatalf("NextDoc() = (%d, %v), want (11, nil)", doc, err)
	}

	// Pre-positioned inner iterator at 14.
	in := advancedRange(10, 15, 14, t)
	iterator.Update(in)
	doc, err = iterator.NextDoc()
	if err != nil || doc != 14 {
		t.Fatalf("NextDoc() = (%d, %v), want (14, nil)", doc, err)
	}

	doc, err = iterator.NextDoc()
	if err != nil || doc != search.NO_MORE_DOCS {
		t.Fatalf("NextDoc() = (%d, %v), want (NO_MORE_DOCS, nil)", doc, err)
	}
}

// TestUpdateableDocIdSetIterator_Advance mirrors
// TestUpdateableDocIdSetIterator.testAdvance.
func TestUpdateableDocIdSetIterator_Advance(t *testing.T) {
	iterator := comparators.NewUpdateableDocIdSetIterator()

	iterator.Update(rangeIter(10, 20))
	doc, err := iterator.Advance(12)
	if err != nil || doc != 12 {
		t.Fatalf("Advance(12) = (%d, %v), want (12, nil)", doc, err)
	}

	iterator.Update(rangeIter(10, 15))
	doc, err = iterator.Advance(13)
	if err != nil || doc != 13 {
		t.Fatalf("Advance(13) = (%d, %v), want (13, nil)", doc, err)
	}

	// Pre-positioned inner iterator at 15.
	in := advancedRange(10, 20, 15, t)
	iterator.Update(in)
	doc, err = iterator.Advance(15)
	if err != nil || doc != 15 {
		t.Fatalf("Advance(15) = (%d, %v), want (15, nil)", doc, err)
	}

	doc, err = iterator.Advance(20)
	if err != nil || doc != search.NO_MORE_DOCS {
		t.Fatalf("Advance(20) = (%d, %v), want (NO_MORE_DOCS, nil)", doc, err)
	}
}

// TestUpdateableDocIdSetIterator_AvailableIteration replaces the Java
// intoBitSet test. intoBitSet is not on Gocene's DocIdSetIterator interface.
// Instead we verify Cost delegation, multiple sequential Update calls, and
// iteration through multiple dynamic ranges.
func TestUpdateableDocIdSetIterator_AvailableIteration(t *testing.T) {
	iterator := comparators.NewUpdateableDocIdSetIterator()

	// Cost delegates to the inner iterator.
	iterator.Update(rangeIter(100, 200))
	if got := iterator.Cost(); got != 100 {
		t.Errorf("Cost() = %d, want 100", got)
	}

	// Update to a smaller range dynamically.
	iterator.Update(rangeIter(150, 155))
	if got := iterator.Cost(); got != 5 {
		t.Errorf("Cost() after narrow update = %d, want 5", got)
	}

	// Iterate through the narrow range.
	var docs []int
	for {
		d, err := iterator.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if d == search.NO_MORE_DOCS {
			break
		}
		docs = append(docs, d)
	}
	if len(docs) != 5 {
		t.Errorf("visited docs = %v, want 5 docs [150..155)", docs)
	}
	if docs[0] != 150 || docs[4] != 154 {
		t.Errorf("range bounds: got %v, want [150,151,152,153,154]", docs)
	}

	// Update back to the original wide range, start fresh.
	iterator.Update(rangeIter(100, 200))
	doc, err := iterator.NextDoc()
	if err != nil || doc != 100 {
		t.Fatalf("NextDoc() = (%d, %v), want (100, nil)", doc, err)
	}
}

// TestUpdateableDocIdSetIterator_DocIDRunEnd mirrors
// TestUpdateableDocIdSetIterator.testDocIDRunEnd.
func TestUpdateableDocIdSetIterator_DocIDRunEnd(t *testing.T) {
	iterator := comparators.NewUpdateableDocIdSetIterator()
	iterator.Update(rangeIter(10, 18))
	doc, err := iterator.NextDoc()
	if err != nil || doc != 10 {
		t.Fatalf("NextDoc() = (%d, %v), want (10, nil)", doc, err)
	}
	// RangeDocIdSetIterator[10,18).DocIDRunEnd at doc 10 = 18.
	if got := iterator.DocIDRunEnd(); got != 18 {
		t.Errorf("DocIDRunEnd() = %d, want 18", got)
	}
	if iterator.DocID() != 10 {
		t.Errorf("DocID() = %d, want 10 (no side effect)", iterator.DocID())
	}

	// New range [8,25) synced to doc 10 -> runEnd = 25.
	iterator.Update(rangeIter(8, 25))
	if got := iterator.DocIDRunEnd(); got != 25 {
		t.Errorf("DocIDRunEnd() after update[8,25) = %d, want 25", got)
	}
	if iterator.DocID() != 10 {
		t.Errorf("DocID() = %d, want 10 (no side effect)", iterator.DocID())
	}

	// Pre-positioned at 12 -> inner is ahead of doc(10) -> DocIDRunEnd falls back.
	in := advancedRange(5, 25, 12, t)
	iterator.Update(in)
	// inner.DocID() = 12 > doc 10 -> inner is NOT at current doc -> fallback = doc+1 = 11.
	if got := iterator.DocIDRunEnd(); got != 11 {
		t.Errorf("DocIDRunEnd() with in at 12, doc=10 = %d, want 11", got)
	}
	if iterator.DocID() != 10 {
		t.Errorf("DocID() = %d, want 10 (no side effect)", iterator.DocID())
	}
	// NextDoc should advance to 12 (where in already is).
	doc, err = iterator.NextDoc()
	if err != nil || doc != 12 {
		t.Fatalf("NextDoc() = (%d, %v), want (12, nil)", doc, err)
	}
}

// TestUpdateableDocIdSetIterator_NilPanics verifies Update rejects nil.
func TestUpdateableDocIdSetIterator_NilPanics(t *testing.T) {
	iterator := comparators.NewUpdateableDocIdSetIterator()
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on nil Update, got none")
		}
	}()
	iterator.Update(nil)
}

// TestUpdateableDocIdSetIterator_ImplementsDocIdSetIterator checks interface satisfaction.
func TestUpdateableDocIdSetIterator_ImplementsDocIdSetIterator(t *testing.T) {
	var _ search.DocIdSetIterator = comparators.NewUpdateableDocIdSetIterator()
}

// TestUpdateableDocIdSetIterator_CostDelegation verifies Cost returns the
// inner iterator's cost across multiple updates.
func TestUpdateableDocIdSetIterator_CostDelegation(t *testing.T) {
	iterator := comparators.NewUpdateableDocIdSetIterator()

	iterator.Update(rangeIter(0, 50))
	if got := iterator.Cost(); got != 50 {
		t.Errorf("Cost = %d, want 50", got)
	}

	iterator.Update(rangeIter(100, 101)) // single doc
	if got := iterator.Cost(); got != 1 {
		t.Errorf("Cost = %d, want 1", got)
	}

	iterator.Update(search.NewRangeDocIdSetIterator(0, 0)) // empty
	if got := iterator.Cost(); got != 0 {
		t.Errorf("Cost = %d, want 0 (empty range)", got)
	}
}

// TestUpdateableDocIdSetIterator_MultipleUpdatesSequential verifies that
// updating repeatedly and iterating produces correct results each time.
func TestUpdateableDocIdSetIterator_MultipleUpdatesSequential(t *testing.T) {
	iterator := comparators.NewUpdateableDocIdSetIterator()

	cases := []struct {
		from, to int
		want     []int
	}{
		{5, 8, []int{5, 6, 7}},
		{1, 3, []int{1, 2}},
		{100, 105, []int{100, 101, 102, 103, 104}},
	}
	for _, tc := range cases {
		iterator.Update(rangeIter(tc.from, tc.to))
		var got []int
		for {
			d, err := iterator.NextDoc()
			if err != nil {
				t.Fatalf("NextDoc on [%d,%d): %v", tc.from, tc.to, err)
			}
			if d == search.NO_MORE_DOCS {
				break
			}
			got = append(got, d)
		}
		if len(got) != len(tc.want) {
			t.Errorf("[%d,%d): got %v, want %v", tc.from, tc.to, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("[%d,%d): got %v, want %v", tc.from, tc.to, got, tc.want)
				break
			}
		}
	}
}

// TestUpdateableDocIdSetIterator_EmptyRange verifies that an empty range
// iterator causes NextDoc to return NO_MORE_DOCS immediately.
func TestUpdateableDocIdSetIterator_EmptyRange(t *testing.T) {
	iterator := comparators.NewUpdateableDocIdSetIterator()

	// Empty range: [10,10) is empty.
	iterator.Update(search.NewRangeDocIdSetIterator(10, 10))
	doc, err := iterator.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc on empty range: %v", err)
	}
	if doc != search.NO_MORE_DOCS {
		t.Errorf("DocID on empty range = %d, want NO_MORE_DOCS", doc)
	}

	// Update to non-empty range and verify it works.
	iterator.Update(rangeIter(0, 3))
	doc, err = iterator.NextDoc()
	if err != nil || doc != 0 {
		t.Fatalf("NextDoc after fresh range = (%d, %v), want (0, nil)", doc, err)
	}
}
