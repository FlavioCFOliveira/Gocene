// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"
)

func TestNewIndexSorter(t *testing.T) {
	sort := NewSort()
	sorter := NewIndexSorter(sort)

	if sorter == nil {
		t.Fatal("expected IndexSorter to be created")
	}

	if sorter.GetSort() != sort {
		t.Error("GetSort should return the sort")
	}
}

func TestNewIndexSorter_NilSort(t *testing.T) {
	sorter := NewIndexSorter(nil)

	if sorter == nil {
		t.Fatal("expected IndexSorter to be created even with nil sort")
	}

	if sorter.GetSort() != nil {
		t.Error("GetSort should return nil")
	}
}

func TestIndexSorter_NeedsSorting(t *testing.T) {
	// Test with nil sort
	sorter := NewIndexSorter(nil)
	if sorter.NeedsSorting(nil) {
		t.Error("NeedsSorting should return false for nil sort")
	}

	// Test with empty sort
	emptySort := NewSort()
	sorter = NewIndexSorter(emptySort)
	if sorter.NeedsSorting(nil) {
		t.Error("NeedsSorting should return false for empty sort")
	}

	// Test with sort having fields
	sortField := NewSortField("test", SortTypeString)
	sortWithFields := NewSort(sortField)
	sorter = NewIndexSorter(sortWithFields)
	if !sorter.NeedsSorting(nil) {
		t.Error("NeedsSorting should return true for sort with fields")
	}
}

func TestIndexSorter_SortType(t *testing.T) {
	// Test with nil sort
	sorter := NewIndexSorter(nil)
	if sorter.SortType() != "none" {
		t.Errorf("expected SortType 'none', got '%s'", sorter.SortType())
	}

	// Test with sort
	sort := NewSort(NewSortField("test", SortTypeString))
	sorter = NewIndexSorter(sort)
	if sorter.SortType() != "custom" {
		t.Errorf("expected SortType 'custom', got '%s'", sorter.SortType())
	}
}

func TestIndexSorter_SetSort(t *testing.T) {
	sorter := NewIndexSorter(nil)

	sort := NewSort(NewSortField("test", SortTypeString))
	sorter.SetSort(sort)

	if sorter.GetSort() != sort {
		t.Error("SetSort should set the sort")
	}
}

func TestIndexSorter_SortSegment(t *testing.T) {
	// Create a minimal LeafReader
	segmentInfo := &SegmentInfo{
		name:     "test_segment",
		docCount: 5,
	}
	reader := NewLeafReader(segmentInfo)

	// Test with nil sort
	sorter := NewIndexSorter(nil)
	mapping, err := sorter.SortSegment(reader)
	if err != nil {
		t.Fatalf("SortSegment returned error: %v", err)
	}

	if len(mapping) != 5 {
		t.Errorf("expected mapping length 5, got %d", len(mapping))
	}

	// Should be identity mapping
	for i := 0; i < 5; i++ {
		if mapping[i] != i {
			t.Errorf("expected identity mapping at %d, got %d", i, mapping[i])
		}
	}
}
