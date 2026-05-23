// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/spans/TestSpanMatches.java

package spans

import "testing"

// TestSpanMatches_All is skipped because it requires a fully functional index
// with span support (SpanNearQuery, SpanTermQuery) that are not yet complete in Gocene.
func TestSpanMatches_All(t *testing.T) {
	t.Skip("requires full span index integration; SpanNearQuery/SpanTermQuery are stubs")
}
