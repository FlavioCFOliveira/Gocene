// Copyright 2026 Gocene. All rights reserved.

package payloads

import "testing"

func TestAveragePayloadFunction_DocScore(t *testing.T) {
	fn := NewAveragePayloadFunction()
	if fn == nil {
		t.Fatal("NewAveragePayloadFunction returned nil")
	}
	s := fn.DocScore(0, "field", 5, 15.0)
	if s <= 0 {
		t.Fatalf("DocScore=%v, want > 0", s)
	}
}

func TestSumPayloadFunction_DocScore(t *testing.T) {
	fn := NewSumPayloadFunction()
	s := fn.DocScore(0, "field", 3, 30.0)
	if s != 30.0 {
		t.Fatalf("Sum DocScore=%v, want 30.0", s)
	}
}

func TestMinPayloadFunction(t *testing.T) {
	fn := NewMinPayloadFunction()
	s := fn.DocScore(0, "field", 4, 12.0)
	if s != 12.0 {
		t.Fatalf("Min DocScore=%v, want 12.0", s)
	}
}

func TestMaxPayloadFunction(t *testing.T) {
	fn := NewMaxPayloadFunction()
	s := fn.DocScore(0, "field", 4, 18.0)
	if s != 18.0 {
		t.Fatalf("Max DocScore=%v, want 18.0", s)
	}
}
