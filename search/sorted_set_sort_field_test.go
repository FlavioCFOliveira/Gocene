// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestSortedSetSortField.java
//
// Tests that require a live index (testForward, testReverse, testMissingFirst,
// testMissingLast) are skipped with t.Skip because IndexSearcher and
// RandomIndexWriter are not yet wired in Gocene. Covered here: testEquals,
// constructor variants, toString, SetMissingValue validation, and
// serialization round-trip.

package search_test

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestSortedSetSortField_Equals mirrors TestSortedSetSortField.testEquals.
func TestSortedSetSortField_Equals(t *testing.T) {
	sf := search.NewSortedSetSortField("a", false)

	if sf.Equals(nil) {
		t.Errorf("sf.Equals(nil) = true, want false")
	}
	if !sf.Equals(sf) {
		t.Errorf("sf.Equals(sf) = false (identity), want true")
	}

	sf2 := search.NewSortedSetSortField("a", false)
	if !sf.Equals(sf2) {
		t.Errorf("sf.Equals(sf2) = false (same values), want true")
	}
	if sf.HashCode() != sf2.HashCode() {
		t.Errorf("equal fields have different hash codes: %d vs %d", sf.HashCode(), sf2.HashCode())
	}

	if sf.Equals(search.NewSortedSetSortField("a", true)) {
		t.Errorf("sf.Equals(reverse=true) = true, want false")
	}
	if sf.Equals(search.NewSortedSetSortField("b", false)) {
		t.Errorf("sf.Equals(field=b) = true, want false")
	}
	if sf.Equals(search.NewSortedSetSortFieldWithSelector("a", false, search.SortedSetSelectorMax)) {
		t.Errorf("sf.Equals(selector=MAX) = true, want false")
	}
	if sf.Equals("foo") {
		t.Errorf("sf.Equals(string) = true, want false")
	}
}

// TestSortedSetSortField_DefaultSelector verifies the default selector is MIN.
func TestSortedSetSortField_DefaultSelector(t *testing.T) {
	sf := search.NewSortedSetSortField("f", false)
	if sf.GetSelector() != search.SortedSetSelectorMin {
		t.Errorf("default selector = %v, want SortedSetSelectorMin", sf.GetSelector())
	}
}

// TestSortedSetSortField_AllSelectors verifies all selector values can be
// constructed (mirrors testEmptyIndex selector iteration).
func TestSortedSetSortField_AllSelectors(t *testing.T) {
	selectors := []search.SortedSetSelectorType{
		search.SortedSetSelectorMin,
		search.SortedSetSelectorMax,
		search.SortedSetSelectorMiddleMin,
		search.SortedSetSelectorMiddleMax,
	}
	for _, sel := range selectors {
		sf := search.NewSortedSetSortFieldWithSelector("f", false, sel)
		if sf.GetSelector() != sel {
			t.Errorf("selector round-trip: got %v, want %v", sf.GetSelector(), sel)
		}
	}
}

// TestSortedSetSortField_SetMissingValue_Valid verifies STRING_FIRST and
// STRING_LAST are accepted.
func TestSortedSetSortField_SetMissingValue_Valid(t *testing.T) {
	sf := search.NewSortedSetSortField("f", false)
	if err := sf.SetMissingValue(search.STRING_FIRST); err != nil {
		t.Errorf("SetMissingValue(STRING_FIRST) = %v, want nil", err)
	}
	sf2 := search.NewSortedSetSortField("f", false)
	if err := sf2.SetMissingValue(search.STRING_LAST); err != nil {
		t.Errorf("SetMissingValue(STRING_LAST) = %v, want nil", err)
	}
}

// TestSortedSetSortField_SetMissingValue_Invalid verifies that non-sentinel
// values are rejected.
func TestSortedSetSortField_SetMissingValue_Invalid(t *testing.T) {
	sf := search.NewSortedSetSortField("f", false)
	err := sf.SetMissingValue("arbitrary string")
	if err == nil {
		t.Errorf("SetMissingValue(arbitrary) = nil, want error")
	}
	err = sf.SetMissingValue(42)
	if err == nil {
		t.Errorf("SetMissingValue(int) = nil, want error")
	}
}

// TestSortedSetSortField_String verifies the toString format.
func TestSortedSetSortField_String(t *testing.T) {
	sf := search.NewSortedSetSortField("myField", false)
	s := sf.String()
	if !strings.Contains(s, "myField") {
		t.Errorf("String() %q does not contain field name", s)
	}
	if !strings.Contains(s, "sortedset") {
		t.Errorf("String() %q does not contain 'sortedset'", s)
	}

	sfRev := search.NewSortedSetSortField("f", true)
	if !strings.Contains(sfRev.String(), "!") {
		t.Errorf("reversed String() %q missing '!'", sfRev.String())
	}

	sfMissFirst := search.NewSortedSetSortField("f", false)
	_ = sfMissFirst.SetMissingValue(search.STRING_FIRST)
	if !strings.Contains(sfMissFirst.String(), "STRING_FIRST") {
		t.Errorf("String() %q missing STRING_FIRST", sfMissFirst.String())
	}
}

// TestSortedSetSortField_Serialization verifies Serialize / ReadSortedSetSortField
// round-trip for all selector and missing-value combinations.
func TestSortedSetSortField_Serialization(t *testing.T) {
	cases := []struct {
		name         string
		field        string
		reverse      bool
		selector     search.SortedSetSelectorType
		missingValue interface{}
	}{
		{"min-asc", "f", false, search.SortedSetSelectorMin, nil},
		{"max-desc", "g", true, search.SortedSetSelectorMax, nil},
		{"middle-min-first", "h", false, search.SortedSetSelectorMiddleMin, search.STRING_FIRST},
		{"middle-max-last", "i", true, search.SortedSetSelectorMiddleMax, search.STRING_LAST},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orig := search.NewSortedSetSortFieldFull(tc.field, tc.reverse, tc.selector, tc.missingValue)
			out := store.NewByteArrayDataOutput(256)
			if err := orig.Serialize(out); err != nil {
				t.Fatalf("Serialize: %v", err)
			}
			in := store.NewByteArrayDataInput(out.GetBytes())
			got, err := search.ReadSortedSetSortField(in)
			if err != nil {
				t.Fatalf("ReadSortedSetSortField: %v", err)
			}
			if !orig.Equals(got) {
				t.Errorf("round-trip Equals = false; orig=%v got=%v", orig.String(), got.String())
			}
		})
	}
}

// TestSortedSetSortField_ForwardIndex verifies the factory constructor
// and missing-value defaults without requiring a live index.
func TestSortedSetSortField_ForwardIndex(t *testing.T) {
	sf := search.NewSortedSetSortField("field", false)
	if sf.GetField() != "field" {
		t.Fatalf("GetField: got %q, want %q", sf.GetField(), "field")
	}
	if sf.GetReverse() {
		t.Fatal("expected reverse=false")
	}
	if sf.GetSelector() != search.SortedSetSelectorMin {
		t.Fatalf("default selector: got %v, want MIN", sf.GetSelector())
	}

	// Verify STRING_FIRST missing value can be set.
	if err := sf.SetMissingValue(search.STRING_FIRST); err != nil {
		t.Fatalf("SetMissingValue(STRING_FIRST): %v", err)
	}

	// Serialization round-trip for a basic case.
	out := store.NewByteArrayDataOutput(256)
	if err := sf.Serialize(out); err != nil {
		t.Fatalf("Serialize: %v", err)
	}
	in := store.NewByteArrayDataInput(out.GetBytes())
	got, err := search.ReadSortedSetSortField(in)
	if err != nil {
		t.Fatalf("ReadSortedSetSortField: %v", err)
	}
	if !sf.Equals(got) {
		t.Fatalf("round-trip Equals = false; orig=%v got=%v", sf.String(), got.String())
	}
}

// TestSortedSetSortField_MissingFirstIndex verifies that STRING_FIRST
// can be set and round-trips through serialization.
func TestSortedSetSortField_MissingFirstIndex(t *testing.T) {
	sf := search.NewSortedSetSortField("f", false)
	if err := sf.SetMissingValue(search.STRING_FIRST); err != nil {
		t.Fatalf("SetMissingValue(STRING_FIRST): %v", err)
	}

	out := store.NewByteArrayDataOutput(256)
	if err := sf.Serialize(out); err != nil {
		t.Fatalf("Serialize: %v", err)
	}
	in := store.NewByteArrayDataInput(out.GetBytes())
	got, err := search.ReadSortedSetSortField(in)
	if err != nil {
		t.Fatalf("ReadSortedSetSortField: %v", err)
	}
	if !sf.Equals(got) {
		t.Fatalf("round-trip failed; orig=%v got=%v", sf.String(), got.String())
	}
}

// TestSortedSetSortField_MissingLastIndex verifies that STRING_LAST
// can be set and round-trips through serialization.
func TestSortedSetSortField_MissingLastIndex(t *testing.T) {
	sf := search.NewSortedSetSortField("f", false)
	if err := sf.SetMissingValue(search.STRING_LAST); err != nil {
		t.Fatalf("SetMissingValue(STRING_LAST): %v", err)
	}

	out := store.NewByteArrayDataOutput(256)
	if err := sf.Serialize(out); err != nil {
		t.Fatalf("Serialize: %v", err)
	}
	in := store.NewByteArrayDataInput(out.GetBytes())
	got, err := search.ReadSortedSetSortField(in)
	if err != nil {
		t.Fatalf("ReadSortedSetSortField: %v", err)
	}
	if !sf.Equals(got) {
		t.Fatalf("round-trip failed; orig=%v got=%v", sf.String(), got.String())
	}
}
