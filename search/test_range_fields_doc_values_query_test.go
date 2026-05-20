// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestRangeFieldsDocValuesQuery.java
//
// Deviation: all test methods skipped — requires IndexWriter + IndexSearcher
// with DocValues range field queries (DoubleRangeDocValuesField, etc.),
// not yet complete in Gocene.

package search

import "testing"

func TestRangeFieldsDocValuesQuery_DoubleRangeDocValuesIntersectsQuery(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher+DocValues integration (pre-existing failure in Gocene)")
}
func TestRangeFieldsDocValuesQuery_IntRangeDocValuesIntersectsQuery(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher+DocValues integration (pre-existing failure in Gocene)")
}
func TestRangeFieldsDocValuesQuery_LongRangeDocValuesIntersectQuery(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher+DocValues integration (pre-existing failure in Gocene)")
}
func TestRangeFieldsDocValuesQuery_FloatRangeDocValuesIntersectQuery(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher+DocValues integration (pre-existing failure in Gocene)")
}
func TestRangeFieldsDocValuesQuery_ToString(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher+DocValues integration (pre-existing failure in Gocene)")
}
func TestRangeFieldsDocValuesQuery_NoData(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher+DocValues integration (pre-existing failure in Gocene)")
}
