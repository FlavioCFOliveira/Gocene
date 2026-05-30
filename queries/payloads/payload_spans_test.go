// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/payloads/TestPayloadSpans.java

package payloads

import "testing"

// TestPayloadSpans_All is skipped because it requires full payload + span
// index integration not yet complete in Gocene.
func TestPayloadSpans_All(t *testing.T) {
	t.Fatal("requires full payload + span index integration; deferred to backlog")
}
