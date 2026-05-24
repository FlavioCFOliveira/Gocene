// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of
// org.apache.lucene.sandbox.search.TestSortedDvMultiRangeQuery.
//
// Deviations from Java:
//   - All test methods that require IndexSearcher / RandomIndexWriter
//     (testDuelWithStandardDisjunction, testEquals, testNumericEquals,
//     testToString, testNumericToString, testOverrideToString,
//     testOverrideNumericsToString, testMissingField, testEdgeCases,
//     testNumericCases) are deferred to backlog #2693 since the Gocene search
//     pipeline, DocValuesMultiRangeQuery.SortedSetStabbingBuilder, and
//     SortedNumericDocValuesMultiRangeQuery execution are not yet available.
//   - The present tests exercise the structural API of the query types.
package search

import (
	"bytes"
	"testing"
)

// TestSortedDvMultiRangeQuery_SortedSetConstructor verifies that
// NewSortedSetDocValuesMultiRangeQuery stores the field and term ranges.
func TestSortedDvMultiRangeQuery_SortedSetConstructor(t *testing.T) {
	lo1 := []byte{0x50, 0x00, 0x00, 0x03}
	hi1 := []byte{0x50, 0x00, 0x00, 0x05}
	lo2 := []byte{0x50, 0x00, 0x00, 0x07}
	hi2 := []byte{0x50, 0x00, 0x00, 0x09}
	ranges := [][2][]byte{{lo1, hi1}, {lo2, hi2}}

	q := NewSortedSetDocValuesMultiRangeQuery("foo", ranges)

	if q.Field != "foo" {
		t.Errorf("Field = %q; want %q", q.Field, "foo")
	}
	if len(q.Terms) != 2 {
		t.Fatalf("Terms len = %d; want 2", len(q.Terms))
	}
	if !bytes.Equal(q.Terms[0][0], lo1) || !bytes.Equal(q.Terms[0][1], hi1) {
		t.Errorf("Terms[0] = %v / %v; want %v / %v", q.Terms[0][0], q.Terms[0][1], lo1, hi1)
	}
	if !bytes.Equal(q.Terms[1][0], lo2) || !bytes.Equal(q.Terms[1][1], hi2) {
		t.Errorf("Terms[1] = %v / %v; want %v / %v", q.Terms[1][0], q.Terms[1][1], lo2, hi2)
	}
}

// TestSortedDvMultiRangeQuery_SortedNumericConstructor verifies that
// NewSortedNumericDocValuesMultiRangeQuery stores the field and int64 ranges.
func TestSortedDvMultiRangeQuery_SortedNumericConstructor(t *testing.T) {
	ranges := [][2]int64{{3, 5}, {7, 9}}
	q := NewSortedNumericDocValuesMultiRangeQuery("foo", ranges)

	if q.Ranges == nil || len(q.Ranges) != 2 {
		t.Fatalf("Ranges len = %d; want 2", len(q.Ranges))
	}
	if q.Ranges[0][0] != 3 || q.Ranges[0][1] != 5 {
		t.Errorf("Ranges[0] = %v; want [3 5]", q.Ranges[0])
	}
	if q.Ranges[1][0] != 7 || q.Ranges[1][1] != 9 {
		t.Errorf("Ranges[1] = %v; want [7 9]", q.Ranges[1])
	}
}

// TestSortedDvMultiRangeQuery_SortedSetIsolation verifies slice isolation
// for SortedSetDocValuesMultiRangeQuery.
func TestSortedDvMultiRangeQuery_SortedSetIsolation(t *testing.T) {
	lo := []byte{1, 2, 3, 4}
	hi := []byte{5, 6, 7, 8}
	ranges := [][2][]byte{{lo, hi}}
	q := NewSortedSetDocValuesMultiRangeQuery("f", ranges)

	ranges[0][0][0] = 0xFF
	ranges[0][1][0] = 0xFF

	if q.Terms[0][0][0] != 1 {
		t.Errorf("SortedSet not isolated: Terms[0][0][0] = %d; want 1", q.Terms[0][0][0])
	}
	if q.Terms[0][1][0] != 5 {
		t.Errorf("SortedSet not isolated: Terms[0][1][0] = %d; want 5", q.Terms[0][1][0])
	}
}

// TestSortedDvMultiRangeQuery_SortedNumericIsolation verifies slice isolation
// for SortedNumericDocValuesMultiRangeQuery.
func TestSortedDvMultiRangeQuery_SortedNumericIsolation(t *testing.T) {
	ranges := [][2]int64{{10, 20}}
	q := NewSortedNumericDocValuesMultiRangeQuery("f", ranges)
	ranges[0][0] = 999
	ranges[0][1] = 999

	if q.Ranges[0][0] != 10 {
		t.Errorf("SortedNumeric not isolated: Ranges[0][0] = %d; want 10", q.Ranges[0][0])
	}
	if q.Ranges[0][1] != 20 {
		t.Errorf("SortedNumeric not isolated: Ranges[0][1] = %d; want 20", q.Ranges[0][1])
	}
}
