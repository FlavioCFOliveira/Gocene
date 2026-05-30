// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/function/TestSortedSetFieldSource.java

package function

import "testing"

// TestSortedSetFieldSource_All is skipped because it requires full
// function test setup (FunctionTestSetup with indexed sorted-set field data)
// not yet complete in Gocene.
func TestSortedSetFieldSource_All(t *testing.T) {
	t.Fatal("requires FunctionTestSetup + indexed sorted-set fields; deferred to backlog")
}
