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
	"github.com/FlavioCFOliveira/Gocene/util"
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

// TestUpdateableDocIdSetIterator_IntoBitSet mirrors
// TestUpdateableDocIdSetIterator.testIntoBitSet.
func TestUpdateableDocIdSetIterator_IntoBitSet(t *testing.T) {
	// Scenario 1: range [10,18), start at doc=10, intoBitSet(15, bits, 5).
	iterator := comparators.NewUpdateableDocIdSetIterator()
	iterator.Update(rangeIter(10, 18))

	if doc, err := iterator.NextDoc(); err != nil || doc != 10 {
		t.Fatalf("NextDoc() = (%d, %v), want (10, nil)", doc, err)
	}

	bits, err := util.NewFixedBitSet(100)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	iterator.IntoBitSet(15, bits, 5)
	// Expected: bits 5..9 set (docs 10..14 minus offset 5).
	for i := 5; i < 10; i++ {
		if !bits.Get(i) {
			t.Errorf("expected bit %d to be set (doc %d - offset 5)", i, i+5)
		}
	}
	if bits.Get(10) {
		t.Error("bit 10 should not be set (doc 15 >= upTo 15)")
	}
	if doc := iterator.DocID(); doc != 15 {
		t.Errorf("DocID after intoBitSet: want 15, got %d", doc)
	}

	// Scenario 2: update with range [12,25), intoBitSet(20, bits, 8).
	iterator.Update(rangeIter(12, 25))
	bits.ClearAll()
	iterator.IntoBitSet(20, bits, 8)
	// Expected: bits 7..11 set (docs 15..19 minus offset 8).
	for i := 7; i < 12; i++ {
		if !bits.Get(i) {
			t.Errorf("expected bit %d to be set (doc %d - offset 8)", i, i+8)
		}
	}
	if bits.Get(12) {
		t.Error("bit 12 should not be set")
	}
	if doc := iterator.DocID(); doc != 20 {
		t.Errorf("DocID after intoBitSet: want 20, got %d", doc)
	}

	// Scenario 3: pre-positioned inner (at 23), intoBitSet(30, bits, 10).
	in := advancedRange(15, 25, 23, t)
	iterator.Update(in)
	bits.ClearAll()
	iterator.IntoBitSet(30, bits, 10)
	// Expected: bits 13..14 set (docs 23..24 minus offset 10).
	for i := 13; i < 15; i++ {
		if !bits.Get(i) {
			t.Errorf("expected bit %d to be set (doc %d - offset 10)", i, i+10)
		}
	}
	if bits.Get(15) {
		t.Error("bit 15 should not be set (doc 25 past maxDoc)")
	}
	if doc := iterator.DocID(); doc != search.NO_MORE_DOCS {
		t.Errorf("DocID after intoBitSet: want NO_MORE_DOCS, got %d", doc)
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

	// New range [8,25) synced to doc 10 → runEnd = 25.
	iterator.Update(rangeIter(8, 25))
	if got := iterator.DocIDRunEnd(); got != 25 {
		t.Errorf("DocIDRunEnd() after update[8,25) = %d, want 25", got)
	}
	if iterator.DocID() != 10 {
		t.Errorf("DocID() = %d, want 10 (no side effect)", iterator.DocID())
	}

	// Pre-positioned at 12 → inner is ahead of doc(10) → DocIDRunEnd falls back.
	in := advancedRange(5, 25, 12, t)
	iterator.Update(in)
	// inner.DocID() = 12 > doc 10 → inner is NOT at current doc → fallback = doc+1 = 11.
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
