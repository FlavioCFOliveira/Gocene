// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/ReadAheadMatchAllDocsQuery.java
//
// Deviation: ReadAheadMatchAllDocsQuery is a helper Query (not a test class —
// no @Test methods). It matches all documents by returning a
// DenseConjunctionBulkScorer over a single clause, used to validate
// TopFieldCollector read-ahead compatibility. Ported as a compilation-check
// placeholder; a full Go equivalent would require DenseConjunctionBulkScorer
// which is not yet ported.

package search

import "testing"

// TestReadAheadMatchAllDocsQuery is a placeholder for the helper Query class.
// A full port requires DenseConjunctionBulkScorer (deferred).
func TestReadAheadMatchAllDocsQuery(t *testing.T) {
	t.Skip("ReadAheadMatchAllDocsQuery is a helper class (no @Test methods); requires DenseConjunctionBulkScorer not yet ported to Gocene")
}
