// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/payloads/TestPayloadSpans.java

package payloads

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestPayloadSpans exercises payloaded span query types available in Gocene:
// SpanPayloadCheckQuery and PayloadScoreQuery.
//
// The Lucene original requires a full index with payloads.
func TestPayloadSpans(t *testing.T) {
	// SpanPayloadCheckQuery with non-nil payloads.
	spanTerm := search.NewSpanTermQuery(index.NewTerm("field", "term"))
	payloads := []*util.BytesRef{
		util.NewBytesRef([]byte("p1")),
		util.NewBytesRef([]byte("p2")),
	}
	q := NewSpanPayloadCheckQuery(spanTerm, payloads)
	if q.String("field") == "" {
		t.Error("SpanPayloadCheckQuery.String() returned empty")
	}

	// PayloadScoreQuery with different function types.
	for _, fn := range []PayloadFunction{
		&AveragePayloadFunction{},
		&SumPayloadFunction{},
		&MinPayloadFunction{},
		&MaxPayloadFunction{},
	} {
		pq := NewPayloadScoreQuery(spanTerm, fn, FloatDecoder)
		if pq.String("field") == "" {
			t.Errorf("PayloadScoreQuery with %T returned empty String()", fn)
		}
	}
}
