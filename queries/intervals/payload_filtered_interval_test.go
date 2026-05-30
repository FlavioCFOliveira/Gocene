// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/intervals/TestPayloadFilteredInterval.java

package intervals

import "testing"

// TestPayloadFilteredInterval_All is skipped because it requires MockTokenizer,
// SimplePayloadFilter, RandomIndexWriter, and full index integration not yet
// available in Gocene.
func TestPayloadFilteredInterval_All(t *testing.T) {
	t.Fatal("requires MockTokenizer + RandomIndexWriter + full index integration; deferred to backlog")
}
