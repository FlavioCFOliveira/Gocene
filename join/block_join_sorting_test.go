// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package join contains tests porting
// org.apache.lucene.search.join.TestBlockJoinSorting.
//
// The sole test method testNestedSorting needs block-join sorting
// (ToParentBlockJoinSortField driving a parent sort over child DocValues) wired
// end-to-end, which Gocene does not yet have; it remains deferred with a
// re-pointed skip. The structural sort-field descriptor is verified directly.
package join

import "testing"

// TestBlockJoinSorting_NestedSorting corresponds to
// TestBlockJoinSorting.testNestedSorting. It sorts parent docs by an aggregate
// of child SortedDocValues via ToParentBlockJoinSortField + searcher.search(q,
// n, sort). This is blocked on the missing field-sorted-over-DocValues search
// subsystem: Gocene has no searcher.search(query,n,sort), TopFieldCollector
// sorts by score (never using FieldComparators or the sort fields' DocValues),
// and there are no SortedDocValues/Numeric field comparators wired into the
// search loop.
func TestBlockJoinSorting_NestedSorting(t *testing.T) {
	t.Skip("requires end-to-end field-sorted search over DocValues (rmp #4778) + ToParentBlockJoinSortField/BlockJoinSelector.wrap wiring (rmp #4779)")
}

// TestBlockJoinSorting_SortFieldDescriptor verifies that
// ToParentBlockJoinSortField can be constructed and its accessor works,
// mirroring the structural intent of the sorting test.
func TestBlockJoinSorting_SortFieldDescriptor(t *testing.T) {
	sf := NewToParentBlockJoinSortField("childField", SortInt, false, BlockJoinMin)
	if sf == nil {
		t.Fatal("expected non-nil ToParentBlockJoinSortField")
	}
	if sf.Field != "childField" {
		t.Errorf("Field = %q, want %q", sf.Field, "childField")
	}
	if sf.Selector != BlockJoinMin {
		t.Errorf("Selector = %v, want BlockJoinMin", sf.Selector)
	}
	if sf.Reverse {
		t.Error("Reverse should be false")
	}
	if !sf.IsAscending() {
		t.Error("IsAscending() should be true")
	}
}
