// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "testing"

// TestMergeTrigger_OrdinalsMatchLucene104 locks in the wire-format ordering
// of MergeTrigger constants per Apache Lucene 10.4.0.
func TestMergeTrigger_OrdinalsMatchLucene104(t *testing.T) {
	cases := []struct {
		ord  int
		name string
		mt   MergeTrigger
	}{
		{0, "SEGMENT_FLUSH", SEGMENT_FLUSH},
		{1, "FULL_FLUSH", FULL_FLUSH},
		{2, "EXPLICIT", EXPLICIT},
		{3, "MERGE_FINISHED", MERGE_FINISHED},
		{4, "CLOSING", CLOSING},
		{5, "COMMIT", COMMIT},
		{6, "GET_READER", GET_READER},
		{7, "ADD_INDEXES", ADD_INDEXES},
	}
	for _, c := range cases {
		if int(c.mt) != c.ord {
			t.Errorf("%s ordinal=%d want %d", c.name, int(c.mt), c.ord)
		}
		if c.mt.String() != c.name {
			t.Errorf("%d.String()=%q want %q", c.ord, c.mt.String(), c.name)
		}
	}
}
