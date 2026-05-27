// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestTopGroups_AreSortsCompatible_FieldDifference rejects a merge when the
// group-sort fields differ between two TopGroups.
func TestTopGroups_AreSortsCompatible_FieldDifference(t *testing.T) {
	a := NewTopGroups(
		*search.NewSort(search.NewSortField("price", search.SortFieldTypeInt)),
		*search.NewSortByScore(),
		0, 10,
	)
	b := NewTopGroups(
		*search.NewSort(search.NewSortField("date", search.SortFieldTypeLong)),
		*search.NewSortByScore(),
		0, 10,
	)
	if a.areSortsCompatible(b) {
		t.Error("expected incompatible sorts when field names differ")
	}
}

// TestTopGroups_AreSortsCompatible_ReverseDifference rejects a merge when
// only the reverse flag differs.
func TestTopGroups_AreSortsCompatible_ReverseDifference(t *testing.T) {
	asc := search.NewSortField("name", search.SortFieldTypeString)
	desc := search.NewSortField("name", search.SortFieldTypeString)
	desc.Reverse = true
	a := NewTopGroups(*search.NewSort(asc), *search.NewSortByScore(), 0, 10)
	b := NewTopGroups(*search.NewSort(desc), *search.NewSortByScore(), 0, 10)
	if a.areSortsCompatible(b) {
		t.Error("expected incompatible sorts when reverse flag differs")
	}
}

// TestTopGroups_AreSortsCompatible_Match accepts two TopGroups built with
// identical sort definitions.
func TestTopGroups_AreSortsCompatible_Match(t *testing.T) {
	groupSort := *search.NewSort(search.NewSortField("name", search.SortFieldTypeString))
	docSort := *search.NewSortByScore()
	a := NewTopGroups(groupSort, docSort, 0, 10)
	b := NewTopGroups(groupSort, docSort, 0, 10)
	if !a.areSortsCompatible(b) {
		t.Error("expected matching sorts to be compatible")
	}
}

// TestTopGroups_AreSortsCompatible_DocSortMismatch rejects a merge when the
// group sort matches but the doc sort differs.
func TestTopGroups_AreSortsCompatible_DocSortMismatch(t *testing.T) {
	groupSort := *search.NewSort(search.NewSortField("name", search.SortFieldTypeString))
	a := NewTopGroups(groupSort, *search.NewSortByScore(), 0, 10)
	b := NewTopGroups(groupSort, *search.NewSort(search.NewSortField("ts", search.SortFieldTypeLong)), 0, 10)
	if a.areSortsCompatible(b) {
		t.Error("expected incompatible doc sorts to fail compatibility check")
	}
}

// TestTopGroups_Merge_IncompatibleSorts confirms that an explicit Merge
// fails when areSortsCompatible returns false.
func TestTopGroups_Merge_IncompatibleSorts(t *testing.T) {
	a := NewTopGroups(*search.NewSort(search.NewSortField("a", search.SortFieldTypeString)), *search.NewSortByScore(), 0, 10)
	b := NewTopGroups(*search.NewSort(search.NewSortField("b", search.SortFieldTypeString)), *search.NewSortByScore(), 0, 10)
	if err := a.Merge(b); err == nil {
		t.Error("expected merge error for incompatible sorts")
	}
}
