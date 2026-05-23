// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/function/TestFunctionQuerySort.java

package function

import "testing"

// TestFunctionQuerySort_All is skipped because it requires full function query
// sort infrastructure not yet complete in Gocene.
func TestFunctionQuerySort_All(t *testing.T) {
	t.Skip("requires full function query sort infrastructure; deferred to backlog")
}
