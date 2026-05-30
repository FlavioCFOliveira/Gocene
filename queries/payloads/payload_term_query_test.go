// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/payloads/TestPayloadTermQuery.java

package payloads

import "testing"

// TestPayloadTermQuery_All is skipped because it requires full payload
// term query and scoring infrastructure not yet complete in Gocene.
func TestPayloadTermQuery_All(t *testing.T) {
	t.Fatal("requires full PayloadTermQuery infrastructure; deferred to backlog")
}
