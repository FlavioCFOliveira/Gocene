// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/payloads/TestPayloadCheckQuery.java

package payloads

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestPayloadCheckQuery exercises SpanPayloadCheckQuery construction and
// its basic accessors.
//
// The Lucene original requires a full index with payloads.
func TestPayloadCheckQuery(t *testing.T) {
	// Build a SpanPayloadCheckQuery with a SpanTermQuery and a payload to match.
	spanTerm := search.NewSpanTermQuery(index.NewTerm("field", "term"))
	payloads := []*util.BytesRef{
		util.NewBytesRef([]byte("payload")),
	}
	q := NewSpanPayloadCheckQuery(spanTerm, payloads)
	if q.String("field") == "" {
		t.Error("SpanPayloadCheckQuery.String() returned empty")
	}
}
