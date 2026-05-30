// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/payloads/TestPayloadSpanPositions.java

package payloads

import "testing"

// TestPayloadSpanPositions_All is skipped because it requires full payload + span
// position tracking infrastructure not yet complete in Gocene.
func TestPayloadSpanPositions_All(t *testing.T) {
	t.Fatal("requires full payload + span position infrastructure; deferred to backlog")
}
