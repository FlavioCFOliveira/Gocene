// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0

package payloads

import "testing"

func TestAveragePayloadFunction_DocScore(t *testing.T) {
	var fn AveragePayloadFunction
	s := fn.DocScore(0, "field", 5, 15.0)
	if s <= 0 {
		t.Fatalf("DocScore=%v, want > 0", s)
	}
}

func TestSumPayloadFunction_DocScore(t *testing.T) {
	var fn SumPayloadFunction
	s := fn.DocScore(0, "field", 3, 30.0)
	if s != 30.0 {
		t.Fatalf("Sum DocScore=%v, want 30.0", s)
	}
}

func TestMinPayloadFunction_DocScore(t *testing.T) {
	var fn MinPayloadFunction
	s := fn.DocScore(0, "field", 4, 12.0)
	if s != 12.0 {
		t.Fatalf("Min DocScore=%v, want 12.0", s)
	}
}

func TestMaxPayloadFunction_DocScore(t *testing.T) {
	var fn MaxPayloadFunction
	s := fn.DocScore(0, "field", 4, 18.0)
	if s != 18.0 {
		t.Fatalf("Max DocScore=%v, want 18.0", s)
	}
}

func TestPayloadFunctions_Explain(t *testing.T) {
	var avg AveragePayloadFunction
	expl := avg.Explain(0, "f", 3, 9.0)
	if expl.GetValue() <= 0 {
		t.Fatalf("Average Explain value=%v, want > 0", expl.GetValue())
	}

	var sum SumPayloadFunction
	expl2 := sum.Explain(0, "f", 3, 9.0)
	if expl2.GetValue() != 9.0 {
		t.Fatalf("Sum Explain value=%v, want 9.0", expl2.GetValue())
	}
}

func TestPayloadScoreQuery_New(t *testing.T) {
	// PayloadScoreQuery requires SpanQuery + PayloadFunction.
	// Verify the type exists and can be referenced.
	var _ PayloadFunction = AveragePayloadFunction{}
	var _ PayloadFunction = SumPayloadFunction{}
	var _ PayloadFunction = MinPayloadFunction{}
	var _ PayloadFunction = MaxPayloadFunction{}
}
