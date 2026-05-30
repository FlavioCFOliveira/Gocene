// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/spans/TestSpanSimilarity.java

package spans

import "testing"

// TestSpanSimilarity_All is skipped because it requires full span similarity
// scoring infrastructure not yet complete in Gocene.
func TestSpanSimilarity_All(t *testing.T) {
	t.Fatal("requires full span similarity scoring; deferred to backlog")
}
