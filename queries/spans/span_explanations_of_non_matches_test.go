// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/spans/TestSpanExplanationsOfNonMatches.java

package spans

import "testing"

// TestSpanExplanationsOfNonMatches_All is skipped because it requires full span
// explanation infrastructure not yet complete in Gocene.
func TestSpanExplanationsOfNonMatches_All(t *testing.T) {
	t.Skip("requires full span explanation infrastructure; deferred to backlog")
}
