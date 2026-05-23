// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/function/TestDocValuesFieldSources.java

package function

import "testing"

// TestDocValuesFieldSources_All is skipped because it requires RandomIndexWriter
// and doc-values field infrastructure not yet complete in Gocene.
func TestDocValuesFieldSources_All(t *testing.T) {
	t.Skip("requires RandomIndexWriter + doc-values infrastructure; deferred to backlog")
}
