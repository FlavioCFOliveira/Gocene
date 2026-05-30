// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/intervals/TestComplexMatches.java

package intervals

import "testing"

// TestComplexMatches_All is skipped because it requires MatchesTestBase,
// IntervalQuery execution against an indexed corpus, and wildcard interval
// sources that depend on PrefixQuery.toAutomaton which is not yet ported.
func TestComplexMatches_All(t *testing.T) {
	t.Fatal("requires MatchesTestBase + full interval query execution; deferred to backlog")
}
