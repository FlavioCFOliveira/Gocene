// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package sortedset

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/facets"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// fakeSortedSetDocValues returns canned ordinals indexed by docID.
type fakeSortedSetDocValues struct {
	byDoc map[int][]int
}

func (f *fakeSortedSetDocValues) GetOrdinals(docID int) []int {
	return f.byDoc[docID]
}
func (f *fakeSortedSetDocValues) GetLabel(_ int) string { return "" }
func (f *fakeSortedSetDocValues) GetValueCount() int    { return len(f.byDoc) }

func TestSortedSetDocValuesAccumulator_DocValuesResolverInvoked(t *testing.T) {
	acc, err := NewSortedSetDocValuesAccumulator(facets.NewFacetsConfig(), "color")
	if err != nil {
		t.Fatalf("new accumulator: %v", err)
	}

	var capturedField string
	acc.SetDocValuesResolver(func(_ index.IndexReaderInterface, field string) (SortedSetDocValues, error) {
		capturedField = field
		return &fakeSortedSetDocValues{byDoc: map[int][]int{0: {1}, 1: {1, 2}}}, nil
	})

	got, err := acc.getSortedSetDocValues(nil)
	if err != nil {
		t.Fatalf("getSortedSetDocValues: %v", err)
	}
	if got == nil {
		t.Fatal("expected SortedSetDocValues, got nil")
	}
	if capturedField != "color" {
		t.Errorf("captured field = %q, want %q", capturedField, "color")
	}
	if ords := got.GetOrdinals(1); len(ords) != 2 {
		t.Errorf("ordinals for doc 1 = %v, want 2 entries", ords)
	}
}

func TestSortedSetDocValuesAccumulator_DocValuesResolverError(t *testing.T) {
	acc, _ := NewSortedSetDocValuesAccumulator(facets.NewFacetsConfig(), "color")
	want := errors.New("resolver boom")
	acc.SetDocValuesResolver(func(_ index.IndexReaderInterface, _ string) (SortedSetDocValues, error) {
		return nil, want
	})

	_, err := acc.getSortedSetDocValues(nil)
	if err == nil || !errors.Is(err, want) {
		t.Fatalf("expected wrapped resolver error, got %v", err)
	}
}

func TestSortedSetDocValuesAccumulator_NoResolverReturnsNil(t *testing.T) {
	acc, _ := NewSortedSetDocValuesAccumulator(facets.NewFacetsConfig(), "color")
	got, err := acc.getSortedSetDocValues(nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != nil {
		t.Errorf("expected nil SortedSetDocValues without resolver, got %#v", got)
	}
}
