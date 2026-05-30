// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/function/TestFieldScoreQuery.java

package function

import "testing"

// TestFieldScoreQuery_All is skipped because it requires full function test
// setup (FunctionTestSetup with indexed numeric field data) not yet in Gocene.
func TestFieldScoreQuery_All(t *testing.T) {
	t.Fatal("requires FunctionTestSetup + indexed numeric fields; deferred to backlog")
}
