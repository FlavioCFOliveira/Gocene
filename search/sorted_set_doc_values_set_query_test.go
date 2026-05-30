// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "testing"

// TestSortedSetDocValuesSetQuery is a Sprint 55 stub port of Lucene's
// TestSortedSetDocValuesSetQuery (lucene/core/src/test/org/apache/lucene/
// document/TestSortedSetDocValuesSetQuery.java).
//
// Gocene does not yet provide a SortedSetDocValuesSetQuery type: GOC-3220
// ported only SortedNumericDocValuesSetQuery (see
// sorted_numeric_doc_values_set_query.go). Until the SORTED_SET variant is
// ported, this test is intentionally skipped so the suite stays green while
// reserving the canonical file name and entry point.
func TestSortedSetDocValuesSetQuery(t *testing.T) {
	t.Fatal("SortedSetDocValuesSetQuery not yet ported in Gocene; see GOC-3220 follow-up")
}
