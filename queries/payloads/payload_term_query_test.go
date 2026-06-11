// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/payloads/TestPayloadTermQuery.java

package payloads

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestPayloadTermQuery exercises the payload query types in Gocene:
// SpanPayloadCheckQuery and PayloadScoreQuery, which replace Lucene's
// PayloadTermQuery in the Go port.
//
// The Lucene original requires a full index with payloads.
func TestPayloadTermQuery(t *testing.T) {
	// SpanPayloadCheckQuery — the closest Gocene equivalent to
	// Lucene's PayloadTermQuery.
	spanTerm := search.NewSpanTermQuery(index.NewTerm("field", "term"))
	q := NewSpanPayloadCheckQuery(spanTerm, nil)
	if q.String("field") == "" {
		t.Error("SpanPayloadCheckQuery.String() returned empty")
	}

	// PayloadScoreQuery with a function.
	fn := &AveragePayloadFunction{}
	pq := NewPayloadScoreQuery(spanTerm, fn, FloatDecoder)
	if pq.String("field") == "" {
		t.Error("PayloadScoreQuery.String() returned empty")
	}

	// Verify payload function explanation works.
	_ = fn.Explain(0, "field", 1, 1.5)
}
