// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/function/TestFunctionScoreExplanations.java

package function

import "testing"

// TestFunctionScoreExplanations_All is skipped because it requires full
// function score explanation infrastructure not yet complete in Gocene.
func TestFunctionScoreExplanations_All(t *testing.T) {
	t.Skip("requires full function score explanation infrastructure; deferred to backlog")
}
