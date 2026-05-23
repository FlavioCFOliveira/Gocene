// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/spans/TestSpanQueryVisitor.java

package spans

import "testing"

// TestSpanQueryVisitor_ExtractTermsEquivalent verifies that query.Visit collects
// the expected terms from a compound query tree including span sub-queries.
//
// Deviation: skipped because SpanNearQuery and SpanTermQuery are stubs in Gocene
// (the full span sub-query tree is not yet implemented).
func TestSpanQueryVisitor_ExtractTermsEquivalent(t *testing.T) {
	t.Skip("SpanNearQuery and SpanTermQuery are stubs; full span query visitor not yet implemented")
}
