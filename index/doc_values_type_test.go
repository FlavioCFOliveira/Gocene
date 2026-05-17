// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "testing"

// TestDocValuesType_OrdinalsMatchLucene104 locks in the on-disk ordinal
// contract with org.apache.lucene.index.DocValuesType (Lucene 10.4.0).
func TestDocValuesType_OrdinalsMatchLucene104(t *testing.T) {
	tests := []struct {
		ord  int
		name string
		dvt  DocValuesType
	}{
		{0, "NONE", DocValuesTypeNone},
		{1, "NUMERIC", DocValuesTypeNumeric},
		{2, "BINARY", DocValuesTypeBinary},
		{3, "SORTED", DocValuesTypeSorted},
		{4, "SORTED_NUMERIC", DocValuesTypeSortedNumeric},
		{5, "SORTED_SET", DocValuesTypeSortedSet},
	}
	for _, tc := range tests {
		if int(tc.dvt) != tc.ord {
			t.Errorf("%s: ordinal=%d want %d", tc.name, int(tc.dvt), tc.ord)
		}
		if tc.dvt.String() != tc.name {
			t.Errorf("ordinal %d: String=%q want %q", tc.ord, tc.dvt.String(), tc.name)
		}
	}
}

func TestDocValuesType_Predicates(t *testing.T) {
	if DocValuesTypeNone.HasDocValues() {
		t.Errorf("NONE.HasDocValues() = true, want false")
	}
	if !DocValuesTypeNumeric.HasDocValues() {
		t.Errorf("NUMERIC.HasDocValues() = false, want true")
	}
	if DocValuesTypeBinary.IsSorted() {
		t.Errorf("BINARY.IsSorted() = true, want false")
	}
	if !DocValuesTypeSorted.IsSorted() {
		t.Errorf("SORTED.IsSorted() = false, want true")
	}
	if !DocValuesTypeSortedNumeric.IsSorted() {
		t.Errorf("SORTED_NUMERIC.IsSorted() = false, want true")
	}
	if !DocValuesTypeSortedSet.IsSorted() {
		t.Errorf("SORTED_SET.IsSorted() = false, want true")
	}
	if DocValuesTypeSorted.IsMultiValued() {
		t.Errorf("SORTED.IsMultiValued() = true, want false")
	}
	if !DocValuesTypeSortedSet.IsMultiValued() {
		t.Errorf("SORTED_SET.IsMultiValued() = false, want true")
	}
	if !DocValuesTypeSortedNumeric.IsMultiValued() {
		t.Errorf("SORTED_NUMERIC.IsMultiValued() = false, want true")
	}
}

func TestDocValuesSkipIndexType_OrdinalsMatchLucene104(t *testing.T) {
	if int(DocValuesSkipIndexTypeNone) != 0 {
		t.Errorf("NONE ordinal = %d, want 0", int(DocValuesSkipIndexTypeNone))
	}
	if int(DocValuesSkipIndexTypeRange) != 1 {
		t.Errorf("RANGE ordinal = %d, want 1", int(DocValuesSkipIndexTypeRange))
	}
	if DocValuesSkipIndexTypeNone.String() != "NONE" {
		t.Errorf("NONE.String() = %q, want NONE", DocValuesSkipIndexTypeNone.String())
	}
	if DocValuesSkipIndexTypeRange.String() != "RANGE" {
		t.Errorf("RANGE.String() = %q, want RANGE", DocValuesSkipIndexTypeRange.String())
	}
}

func TestDocValuesSkipIndexType_IsCompatibleWith(t *testing.T) {
	// NONE accepts all
	for _, dvt := range []DocValuesType{
		DocValuesTypeNone, DocValuesTypeNumeric, DocValuesTypeBinary,
		DocValuesTypeSorted, DocValuesTypeSortedNumeric, DocValuesTypeSortedSet,
	} {
		if !DocValuesSkipIndexTypeNone.IsCompatibleWith(dvt) {
			t.Errorf("NONE.IsCompatibleWith(%s) = false, want true", dvt)
		}
	}
	// RANGE accepts NUMERIC/SORTED_NUMERIC/SORTED/SORTED_SET only
	type tc struct {
		dvt  DocValuesType
		want bool
	}
	for _, c := range []tc{
		{DocValuesTypeNone, false},
		{DocValuesTypeNumeric, true},
		{DocValuesTypeBinary, false},
		{DocValuesTypeSorted, true},
		{DocValuesTypeSortedNumeric, true},
		{DocValuesTypeSortedSet, true},
	} {
		if got := DocValuesSkipIndexTypeRange.IsCompatibleWith(c.dvt); got != c.want {
			t.Errorf("RANGE.IsCompatibleWith(%s) = %v, want %v", c.dvt, got, c.want)
		}
	}
}
