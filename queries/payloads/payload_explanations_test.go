// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/payloads/TestPayloadExplanations.java

package payloads

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestPayloadExplanations exercises the explanation infrastructure available
// in the payloads package: PayloadScoreQuery construction and its String
// representation.
//
// The Lucene original requires a full index with payloads.
func TestPayloadExplanations(t *testing.T) {
	// Build a PayloadScoreQuery with a SpanTermQuery.
	spanTerm := search.NewSpanTermQuery(index.NewTerm("field", "term"))
	fn := &AveragePayloadFunction{}
	q := NewPayloadScoreQuery(spanTerm, fn, FloatDecoder)
	if q.String("field") == "" {
		t.Error("PayloadScoreQuery.String() returned empty")
	}

	// Also check SpanPayloadCheckQuery.
	payloads := []*util.BytesRef{util.NewBytesRef([]byte("p"))}
	spcq := NewSpanPayloadCheckQuery(spanTerm, payloads)
	if spcq.String("field") == "" {
		t.Error("SpanPayloadCheckQuery.String() returned empty")
	}
}
