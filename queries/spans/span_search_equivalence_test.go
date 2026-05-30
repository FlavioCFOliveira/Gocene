// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/spans/TestSpanSearchEquivalence.java

package spans

import "testing"

// TestSpanSearchEquivalence_All is skipped because it requires full index
// and search equivalence framework not yet complete in Gocene.
func TestSpanSearchEquivalence_All(t *testing.T) {
	t.Fatal("requires full span search equivalence infrastructure; deferred to backlog")
}
