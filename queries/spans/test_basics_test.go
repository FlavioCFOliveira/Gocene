// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/spans/TestBasics.java

package spans

import "testing"

// TestBasics_All is skipped because it requires a full indexed corpus,
// SpanNearQuery, SpanOrQuery, SpanNotQuery, SpanFirstQuery and related
// infrastructure not yet complete in Gocene.
func TestBasics_All(t *testing.T) {
	t.Skip("requires full span query index integration; deferred to backlog")
}
