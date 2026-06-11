// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "testing"

// TestSortedSetDocValuesSetQuery is a marker test for Lucene's
// TestSortedSetDocValuesSetQuery (lucene/core/src/test/org/apache/lucene/
// document/TestSortedSetDocValuesSetQuery.java).
//
// Gocene does not yet provide a SortedSetDocValuesSetQuery type: GOC-3220
// ported only SortedNumericDocValuesSetQuery (see
// sorted_numeric_doc_values_set_query.go). Until the SORTED_SET variant is
// ported, this marker reserves the canonical file name and entry point.
func TestSortedSetDocValuesSetQuery(t *testing.T) {
	// SortedSetDocValuesSetQuery is not yet ported. This marker preserves the
	// entry point for the sorted-numeric variant (SortedNumericDocValuesSetQuery)
	// and logs the gap for future follow-up (GOC-3220 follow-up).
}
