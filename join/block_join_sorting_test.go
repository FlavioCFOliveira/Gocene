// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package join contains tests porting
// org.apache.lucene.search.join.TestBlockJoinSorting.
//
// The sole test method testNestedSorting requires IndexWriter, DirectoryReader,
// and IndexSearcher with block-join sort fields and comparators. This depends
// on SegmentReader coreReaders wiring not yet available in Gocene.
// The test is stubbed with t.Skip; structural descriptors are verified directly.
package join

import "testing"

// TestBlockJoinSorting_NestedSorting corresponds to
// TestBlockJoinSorting.testNestedSorting.
// Skipped: requires DirectoryReader + IndexSearcher with nested sort fields.
func TestBlockJoinSorting_NestedSorting(t *testing.T) {
	t.Skip("requires DirectoryReader + IndexSearcher: SegmentReader coreReaders not yet wired")
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
