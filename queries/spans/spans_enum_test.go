// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/spans/TestSpansEnum.java

package spans

import "testing"

// TestSpansEnum_All is skipped because it requires full span index integration.
func TestSpansEnum_All(t *testing.T) {
	t.Skip("requires full span index integration; deferred to backlog")
}
