// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/function/docvalues/TestBoolValOfNumericDVs.java

package docvalues

import "testing"

// TestBoolValOfNumericDVs_All is skipped because it requires full
// numeric DocValues infrastructure (indexed numeric DV fields + reader)
// not yet complete in Gocene.
func TestBoolValOfNumericDVs_All(t *testing.T) {
	t.Fatal("requires indexed numeric DocValues + FunctionValues infrastructure; deferred to backlog")
}
