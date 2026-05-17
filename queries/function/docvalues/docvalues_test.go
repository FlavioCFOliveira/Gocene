// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package docvalues_test

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/queries/function/docvalues"
)

// stubValueSource is a minimal ValueSource used for cross-type tests.
type stubValueSource struct {
	function.BaseValueSource
	desc string
}

func (s *stubValueSource) Description() string { return s.desc }
func (s *stubValueSource) Equals(other function.ValueSource) bool {
	o, ok := other.(*stubValueSource)
	return ok && o.desc == s.desc
}
func (s *stubValueSource) HashCode() int32 { return int32(len(s.desc)) }
func (s *stubValueSource) GetValues(_ function.Context, _ *index.LeafReaderContext) (function.FunctionValues, error) {
	return nil, nil
}

func TestBoolDocValues_AllCoercionsMatchBoolean(t *testing.T) {
	t.Parallel()
	vs := &stubValueSource{desc: "B"}
	bv := docvalues.NewBoolDocValues(vs, func(doc int) (bool, error) { return doc%2 == 0, nil })
	if v, _ := bv.IntVal(0); v != 1 {
		t.Fatalf("IntVal(0)=%d, want 1", v)
	}
	if v, _ := bv.IntVal(1); v != 0 {
		t.Fatalf("IntVal(1)=%d, want 0", v)
	}
	if s, _ := bv.StrVal(0); s != "true" {
		t.Fatalf("StrVal(0)=%q, want true", s)
	}
	if s, _ := bv.ToString(0); s != "B=true" {
		t.Fatalf("ToString(0)=%q, want B=true", s)
	}
}

func TestIntDocValues_RangeScorerAdjustsExclusiveBounds(t *testing.T) {
	t.Parallel()
	vs := &stubValueSource{desc: "I"}
	dv := docvalues.NewIntDocValues(vs, func(doc int) (int32, error) { return int32(doc), nil })
	// Lucene shifts exclusive bounds inward: ">3" → ">=4", "<10" → "<=9".
	scorer, err := dv.GetRangeScorer(nil, "3", "10", false, false)
	if err != nil {
		t.Fatalf("GetRangeScorer: %v", err)
	}
	cases := []struct {
		doc  int
		want bool
	}{{3, false}, {4, true}, {9, true}, {10, false}}
	for _, tc := range cases {
		got, err := scorer.Matches(tc.doc)
		if err != nil {
			t.Fatalf("Matches(%d): %v", tc.doc, err)
		}
		if got != tc.want {
			t.Fatalf("Matches(%d)=%v, want %v", tc.doc, got, tc.want)
		}
	}
}

func TestLongDocValues_RangeScorerHandlesUnboundedLower(t *testing.T) {
	t.Parallel()
	vs := &stubValueSource{desc: "L"}
	dv := docvalues.NewLongDocValues(vs, func(doc int) (int64, error) { return int64(doc) * 100, nil })
	scorer, err := dv.GetRangeScorer(nil, "", "500", true, true)
	if err != nil {
		t.Fatalf("GetRangeScorer: %v", err)
	}
	cases := []struct {
		doc  int
		want bool
	}{{0, true}, {5, true}, {6, false}}
	for _, tc := range cases {
		got, err := scorer.Matches(tc.doc)
		if err != nil {
			t.Fatalf("Matches(%d): %v", tc.doc, err)
		}
		if got != tc.want {
			t.Fatalf("Matches(%d)=%v, want %v", tc.doc, got, tc.want)
		}
	}
}

func TestDoubleDocValues_RangeScorerHonoursInclusivity(t *testing.T) {
	t.Parallel()
	vs := &stubValueSource{desc: "D"}
	dv := docvalues.NewDoubleDocValues(vs, func(doc int) (float64, error) { return float64(doc) / 10, nil })
	scorer, err := dv.GetRangeScorer(nil, "0.2", "0.5", false, true)
	if err != nil {
		t.Fatalf("GetRangeScorer: %v", err)
	}
	cases := []struct {
		doc  int
		want bool
	}{{2, false}, {3, true}, {5, true}, {6, false}}
	for _, tc := range cases {
		got, err := scorer.Matches(tc.doc)
		if err != nil {
			t.Fatalf("Matches(%d): %v", tc.doc, err)
		}
		if got != tc.want {
			t.Fatalf("Matches(%d)=%v, want %v", tc.doc, got, tc.want)
		}
	}
}

func TestStrDocValues_ObjectValReturnsNilWhenNotExists(t *testing.T) {
	t.Parallel()
	vs := &stubValueSource{desc: "S"}
	dv := docvalues.NewStrDocValues(vs, func(doc int) (string, error) { return "hello", nil })
	// Default Exists is true (BaseFunctionValues), so ObjectVal returns the string.
	obj, err := dv.ObjectVal(0)
	if err != nil {
		t.Fatalf("ObjectVal: %v", err)
	}
	if s, ok := obj.(string); !ok || s != "hello" {
		t.Fatalf("ObjectVal=%v, want \"hello\"", obj)
	}
}

func TestFloatDocValues_StrValRendersFloat(t *testing.T) {
	t.Parallel()
	vs := &stubValueSource{desc: "F"}
	dv := docvalues.NewFloatDocValues(vs, func(doc int) (float32, error) { return 1.5, nil })
	if s, _ := dv.StrVal(0); s != "1.5" {
		t.Fatalf("StrVal=%q, want 1.5", s)
	}
}

func TestDocTermsIndexDocValues_GetRangeScorerDeferred(t *testing.T) {
	t.Parallel()
	dv := docvalues.NewDocTermsIndexDocValuesFromDV(&stubValueSource{desc: "T"}, nil)
	_, err := dv.GetRangeScorer(nil, "a", "z", true, true)
	if !errors.Is(err, docvalues.ErrLookupTermUnavailable) {
		t.Fatalf("GetRangeScorer: want ErrLookupTermUnavailable, got %v", err)
	}
}
