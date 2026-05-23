// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/spans/TestQueryRescorerWithSpans.java

package spans

import "testing"

// TestQueryRescorerWithSpans_All is skipped because it requires full index,
// rescorer, and span infrastructure not yet complete in Gocene.
func TestQueryRescorerWithSpans_All(t *testing.T) {
	t.Skip("requires full index + rescorer + span infrastructure; deferred to backlog")
}
