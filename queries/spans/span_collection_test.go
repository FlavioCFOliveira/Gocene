// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/spans/TestSpanCollection.java

package spans

import "testing"

// TestSpanCollection_All is skipped because it requires full span collection
// infrastructure (SpanCollector) not yet complete in Gocene.
func TestSpanCollection_All(t *testing.T) {
	t.Fatal("requires full SpanCollector infrastructure; deferred to backlog")
}
