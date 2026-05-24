// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of
// org.apache.lucene.sandbox.search.TestInetAddrSsDvMultiRangeQuery.
//
// Deviations from Java:
//   - testBasics and testRandom require IndexSearcher / RandomIndexWriter and
//     DocValuesMultiRangeQuery.SortedSetStabbingBuilder; deferred to backlog
//     #2693 until the Gocene search pipeline is available.
//   - The present tests cover the structural API of
//     SortedSetDocValuesMultiRangeQuery and the concatenateByteArrays helper
//     (ported verbatim from the Java test as a pure Go function).
package search

import (
	"bytes"
	"testing"
)

// concatenateByteArrays mirrors the static helper from the Java test.
func concatenateByteArrays(a, b []byte) []byte {
	result := make([]byte, len(a)+len(b))
	copy(result, a)
	copy(result[len(a):], b)
	return result
}

// TestInetAddrSsDvMultiRangeQuery_ConcatenateByteArrays verifies the
// concatenateByteArrays helper (ported from the Java test utility).
func TestInetAddrSsDvMultiRangeQuery_ConcatenateByteArrays(t *testing.T) {
	tests := []struct {
		a, b []byte
		want []byte
	}{
		{[]byte{1, 2}, []byte{3, 4}, []byte{1, 2, 3, 4}},
		{[]byte{}, []byte{5, 6}, []byte{5, 6}},
		{[]byte{7, 8}, []byte{}, []byte{7, 8}},
		{[]byte{}, []byte{}, []byte{}},
	}
	for _, tc := range tests {
		got := concatenateByteArrays(tc.a, tc.b)
		if !bytes.Equal(got, tc.want) {
			t.Errorf("concatenateByteArrays(%v, %v) = %v; want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

// TestInetAddrSsDvMultiRangeQuery_ConstructorAccumulates verifies that
// NewSortedSetDocValuesMultiRangeQuery copies the supplied term ranges and
// assigns the correct field name.
func TestInetAddrSsDvMultiRangeQuery_ConstructorAccumulates(t *testing.T) {
	lo1 := []byte{1, 2, 3, 4}
	hi1 := []byte{1, 2, 3, 5}
	lo2 := []byte{127, 0, 0, 1}
	hi2 := []byte{127, 0, 0, 2}
	ranges := [][2][]byte{{lo1, hi1}, {lo2, hi2}}

	q := NewSortedSetDocValuesMultiRangeQuery("field", ranges)
	if q.Field != "field" {
		t.Errorf("Field = %q; want %q", q.Field, "field")
	}
	if len(q.Terms) != 2 {
		t.Fatalf("Terms len = %d; want 2", len(q.Terms))
	}
	if !bytes.Equal(q.Terms[0][0], lo1) {
		t.Errorf("Terms[0][0] = %v; want %v", q.Terms[0][0], lo1)
	}
	if !bytes.Equal(q.Terms[1][1], hi2) {
		t.Errorf("Terms[1][1] = %v; want %v", q.Terms[1][1], hi2)
	}
}

// TestInetAddrSsDvMultiRangeQuery_IsolatesCallerSlice verifies that mutations
// to the original ranges slice after construction do not affect the query.
func TestInetAddrSsDvMultiRangeQuery_IsolatesCallerSlice(t *testing.T) {
	lo := []byte{10, 20, 30, 40}
	hi := []byte{50, 60, 70, 80}
	ranges := [][2][]byte{{lo, hi}}

	q := NewSortedSetDocValuesMultiRangeQuery("f", ranges)

	// Mutate after construction.
	ranges[0][0][0] = 99
	ranges[0][1][0] = 99

	if q.Terms[0][0][0] != 10 {
		t.Errorf("query not isolated: Terms[0][0][0] = %d; want 10", q.Terms[0][0][0])
	}
	if q.Terms[0][1][0] != 50 {
		t.Errorf("query not isolated: Terms[0][1][0] = %d; want 50", q.Terms[0][1][0])
	}
}
