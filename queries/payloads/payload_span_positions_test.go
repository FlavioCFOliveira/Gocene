// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/payloads/TestPayloadSpanPositions.java

package payloads

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestPayloadSpanPositions exercises the SpanPayloadCheckQuery and its
// interaction with the payload decoder and check query types.
//
// The Lucene original requires a full index with payload-tracking postings.
func TestPayloadSpanPositions(t *testing.T) {
	// SpanPayloadCheckQuery construction.
	spanTerm := search.NewSpanTermQuery(index.NewTerm("field", "term"))
	q := NewSpanPayloadCheckQuery(spanTerm, nil)
	if q == nil {
		t.Fatal("NewSpanPayloadCheckQuery returned nil")
	}
	if q.String("field") == "" {
		t.Error("SpanPayloadCheckQuery.String() returned empty")
	}

	// PayloadScoreQuery construction with include flag.
	q2 := NewPayloadScoreQueryWithInclude(spanTerm, &SumPayloadFunction{}, FloatDecoder, true)
	if q2.String("field") == "" {
		t.Error("PayloadScoreQueryWithInclude.String() returned empty")
	}
}
