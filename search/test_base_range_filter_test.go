// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestBaseRangeFilter.java
//
// testPad is pure string-padding logic — no IndexWriter/IndexSearcher is
// involved (the previous stub deferring to "IndexWriter+IndexSearcher
// integration" was incorrect). It verifies that BaseTestRangeFilter.pad
// produces fixed-width, lexicographically-monotonic strings so that signed int
// ranges can be encoded as sortable terms.

package search_test

import (
	"strconv"
	"testing"
)

// intLength mirrors BaseTestRangeFilter.intLength: the decimal width of
// Integer.MAX_VALUE (len("2147483647") == 10).
const intLength = 10

// padRangeFilter ports BaseTestRangeFilter.pad: a simple padding function that
// maps any int32 to a fixed-width, sign-prefixed decimal string whose natural
// (lexicographic) order matches the signed integer order.
//
// For negative n, the magnitude is folded into the unsigned range via
// MAX_INT + n + 1 (matching Java's int arithmetic) and prefixed with '-' so
// that more-negative values produce lexicographically smaller strings.
func padRangeFilter(n int32) string {
	var b []byte
	p := "0"
	v := int64(n)
	if n < 0 {
		p = "-"
		// Java: n = Integer.MAX_VALUE + n + 1 (computed in 32-bit int space).
		v = int64(int32(int64(2147483647) + int64(n) + 1))
	}
	b = append(b, p...)
	s := strconv.FormatInt(v, 10)
	for i := len(s); i <= intLength; i++ {
		b = append(b, '0')
	}
	b = append(b, s...)
	return string(b)
}

// TestBaseRangeFilter_Pad ports TestBaseRangeFilter.testPad.
func TestBaseRangeFilter_Pad(t *testing.T) {
	tests := []int32{-9999999, -99560, -100, -3, -1, 0, 3, 9, 10, 1000, 999999999}
	for i := 0; i < len(tests)-1; i++ {
		a := tests[i]
		b := tests[i+1]
		aa := padRangeFilter(a)
		bb := padRangeFilter(b)
		if len(aa) != len(bb) {
			t.Fatalf("length of %d:%s vs %d:%s differ: %d vs %d", a, aa, b, bb, len(aa), len(bb))
		}
		if !(aa < bb) {
			t.Fatalf("compare less than failed: %d:%s vs %d:%s", a, aa, b, bb)
		}
	}
}
