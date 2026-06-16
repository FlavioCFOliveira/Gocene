// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package search

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// NOTE: simplified vs Java peer because Lucene 10.4.0 does not ship a
// dedicated TestFloatRangeSlowRangeQuery (the reference confirms TR's
// "none located" note). The tests below cover the same observable
// contract: constructor validation, equals/hashCode reflexivity and
// distinguishability, Visit field-acceptance, String formatting, encoded
// byte payload identity vs the Lucene encoder, and the in-memory
// match path on the TwoPhaseIterator (no RandomIndexWriter / CheckHits
// dependency).

// TestFloatRangeSlowRangeQuery_FactoryValidation exercises the constructor
// guards: dimension-length mismatches, empty arrays, inverted ranges
// (rejected inside EncodeFloatRangeLucene), and non-INTERSECTS query types
// (rejected by the binary base).
func TestFloatRangeSlowRangeQuery_FactoryValidation(t *testing.T) {
	cases := []struct {
		name     string
		field    string
		min, max []float32
		qType    document.RangeFieldQueryType
		wantErr  bool
	}{
		{
			name:    "empty field is allowed by FloatRange but rejected by base",
			field:   "",
			min:     []float32{0},
			max:     []float32{1},
			qType:   document.RangeFieldQueryTypeIntersects,
			wantErr: true,
		},
		{
			name:    "different min/max lengths",
			field:   "f",
			min:     []float32{0},
			max:     []float32{1, 2},
			qType:   document.RangeFieldQueryTypeIntersects,
			wantErr: true,
		},
		{
			name:    "empty min/max",
			field:   "f",
			min:     []float32{},
			max:     []float32{},
			qType:   document.RangeFieldQueryTypeIntersects,
			wantErr: true,
		},
		{
			name:    "inverted range (min > max)",
			field:   "f",
			min:     []float32{2.5},
			max:     []float32{1.5},
			qType:   document.RangeFieldQueryTypeIntersects,
			wantErr: true,
		},
		{
			name:    "WITHIN rejected — INTERSECTS only",
			field:   "f",
			min:     []float32{0},
			max:     []float32{1},
			qType:   document.RangeFieldQueryTypeWithin,
			wantErr: true,
		},
		{
			name:    "CONTAINS rejected — INTERSECTS only",
			field:   "f",
			min:     []float32{0},
			max:     []float32{1},
			qType:   document.RangeFieldQueryTypeContains,
			wantErr: true,
		},
		{
			name:    "valid 1D",
			field:   "f",
			min:     []float32{0},
			max:     []float32{1},
			qType:   document.RangeFieldQueryTypeIntersects,
			wantErr: false,
		},
		{
			name:    "valid 3D",
			field:   "geo",
			min:     []float32{-1, -2, -3},
			max:     []float32{1, 2, 3},
			qType:   document.RangeFieldQueryTypeIntersects,
			wantErr: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewFloatRangeSlowRangeQuery(tc.field, tc.min, tc.max, tc.qType)
			gotErr := err != nil
			if gotErr != tc.wantErr {
				t.Fatalf("NewFloatRangeSlowRangeQuery err = %v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

// TestFloatRangeSlowRangeQuery_EqualsAndHashCode mirrors the standard
// Lucene contract: identical inputs collide and equal; differences in
// field, min, or max disturb equality and produce different hash codes
// for the inputs tested.
func TestFloatRangeSlowRangeQuery_EqualsAndHashCode(t *testing.T) {
	mk := func(t *testing.T, field string, min, max []float32) Query {
		t.Helper()
		q, err := NewFloatRangeSlowRangeQuery(field, min, max, document.RangeFieldQueryTypeIntersects)
		if err != nil {
			t.Fatalf("constructor: %v", err)
		}
		return q
	}

	q1 := mk(t, "f", []float32{0, 1}, []float32{2, 3})
	q2 := mk(t, "f", []float32{0, 1}, []float32{2, 3})
	if !q1.Equals(q2) || !q2.Equals(q1) {
		t.Fatalf("expected q1 == q2 for identical inputs")
	}
	if q1.HashCode() != q2.HashCode() {
		t.Fatalf("expected hash(q1) == hash(q2): %d vs %d", q1.HashCode(), q2.HashCode())
	}

	q3 := mk(t, "g", []float32{0, 1}, []float32{2, 3})
	if q1.Equals(q3) {
		t.Fatalf("expected q1 != q3 (different field)")
	}
	if q1.HashCode() == q3.HashCode() {
		t.Fatalf("expected hash(q1) != hash(q3) for different field")
	}

	q4 := mk(t, "f", []float32{0.5, 1}, []float32{2, 3})
	if q1.Equals(q4) {
		t.Fatalf("expected q1 != q4 (different min)")
	}
	q5 := mk(t, "f", []float32{0, 1}, []float32{2, 4})
	if q1.Equals(q5) {
		t.Fatalf("expected q1 != q5 (different max)")
	}

	// Cross-type: a binary base query with the same packed payload is not
	// equal to a float query — they are distinct concrete types.
	concrete := q1.(*floatRangeSlowRangeQuery)
	if concrete.Equals(concrete.binaryRangeFieldRangeQuery) {
		t.Fatalf("float query should not Equals its embedded binary base")
	}
}

// TestFloatRangeSlowRangeQuery_Equals_NaN ensures NaN-bearing arrays
// compare equal to themselves under bit-pattern semantics (matching
// java.util.Arrays.equals).
func TestFloatRangeSlowRangeQuery_Equals_NaN(t *testing.T) {
	nan := float32(math.NaN())
	q1, err := NewFloatRangeSlowRangeQuery("f", []float32{nan}, []float32{nan}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("constructor with NaN: %v", err)
	}
	q2, err := NewFloatRangeSlowRangeQuery("f", []float32{nan}, []float32{nan}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("constructor with NaN (2): %v", err)
	}
	if !q1.Equals(q2) {
		t.Fatalf("NaN-bearing arrays should bit-compare equal (Arrays.equals semantics)")
	}
}

// TestFloatRangeSlowRangeQuery_String covers the two branches of toString:
// when the caller-provided field matches the query field, the prefix is
// omitted; otherwise the prefix is "<field>:".
func TestFloatRangeSlowRangeQuery_String(t *testing.T) {
	q, err := NewFloatRangeSlowRangeQuery("f", []float32{0.5, 1.5}, []float32{2.5, 3.5}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}

	stringer, ok := q.(interface{ String(string) string })
	if !ok {
		t.Fatalf("query should expose String(string) string")
	}

	withPrefix := stringer.String("other")
	if !strings.HasPrefix(withPrefix, "f:[") {
		t.Fatalf("expected prefix 'f:[' when out-of-context, got %q", withPrefix)
	}
	if !strings.Contains(withPrefix, "TO") || !strings.HasSuffix(withPrefix, "]") {
		t.Fatalf("expected 'TO' separator and trailing ']' in %q", withPrefix)
	}

	withoutPrefix := stringer.String("f")
	if strings.HasPrefix(withoutPrefix, "f:") {
		t.Fatalf("expected no field prefix when in-context, got %q", withoutPrefix)
	}
	if !strings.HasPrefix(withoutPrefix, "[") || !strings.HasSuffix(withoutPrefix, "]") {
		t.Fatalf("expected bracketed payload in %q", withoutPrefix)
	}
}

// TestFloatRangeSlowRangeQuery_Visit verifies the visitor contract:
// VisitLeaf fires for the accepted field, the leaf reported is the query
// itself, and the visitor is silent when AcceptField returns false.
func TestFloatRangeSlowRangeQuery_Visit(t *testing.T) {
	q, err := NewFloatRangeSlowRangeQuery("f", []float32{0}, []float32{1}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}

	visited := &recordingFloatRangeVisitor{}
	q.(interface{ Visit(QueryVisitor) }).Visit(visited)
	if !visited.acceptedField {
		t.Fatalf("expected AcceptField(\"f\") to be invoked")
	}
	if visited.leaf != q {
		t.Fatalf("expected VisitLeaf to receive the float query, got %T", visited.leaf)
	}

	rejecting := &recordingFloatRangeVisitor{rejectField: "f"}
	q.(interface{ Visit(QueryVisitor) }).Visit(rejecting)
	if rejecting.leaf != nil {
		t.Fatalf("expected VisitLeaf not to fire when AcceptField returns false")
	}
}

type recordingFloatRangeVisitor struct {
	EmptyQueryVisitorBase
	acceptedField bool
	rejectField   string
	leaf          Query
}

func (v *recordingFloatRangeVisitor) AcceptField(field string) bool {
	if field == v.rejectField {
		return false
	}
	v.acceptedField = true
	return true
}

func (v *recordingFloatRangeVisitor) VisitLeaf(q Query) { v.leaf = q }

// TestFloatRangeSlowRangeQuery_Rewrite documents the no-op rewrite contract:
// the query rewrites to itself (matching super.rewrite in Java).
func TestFloatRangeSlowRangeQuery_Rewrite(t *testing.T) {
	q, err := NewFloatRangeSlowRangeQuery("f", []float32{0}, []float32{1}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	rw, err := q.Rewrite(nil)
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	if rw != q {
		t.Fatalf("expected Rewrite to return the same instance")
	}
}

// TestFloatRangeSlowRangeQuery_Clone documents the no-op clone contract.
func TestFloatRangeSlowRangeQuery_Clone(t *testing.T) {
	q, err := NewFloatRangeSlowRangeQuery("f", []float32{0}, []float32{1}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	if q.Clone() != q {
		t.Fatalf("expected Clone() to return the same instance")
	}
}

// TestFloatRangeSlowRangeQuery_MinMaxDefensiveCopy verifies that the
// constructor and accessors return defensive copies, so mutating the
// caller's slice after construction does not corrupt the query.
func TestFloatRangeSlowRangeQuery_MinMaxDefensiveCopy(t *testing.T) {
	min := []float32{0, 1}
	max := []float32{2, 3}
	q, err := NewFloatRangeSlowRangeQuery("f", min, max, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}

	// Mutate the caller's slices after construction.
	min[0] = 999
	max[1] = -999

	concrete := q.(*floatRangeSlowRangeQuery)
	if concrete.min[0] != 0 || concrete.max[1] != 3 {
		t.Fatalf("constructor must copy min/max defensively; got min=%v max=%v", concrete.min, concrete.max)
	}

	gotMin := concrete.Min()
	gotMax := concrete.Max()
	gotMin[0] = 42
	gotMax[1] = 42
	if concrete.min[0] != 0 || concrete.max[1] != 3 {
		t.Fatalf("Min/Max accessors must return copies; internal state was mutated")
	}
}

// TestFloatRangeSlowRangeQuery_EncodedPayload_Identity ensures the packed
// query payload is byte-identical to the Lucene encoder for the same
// inputs. The byte stream identity is the AC for the codec/index/store
// dimension — the encoded query goes on the wire for distributed search.
func TestFloatRangeSlowRangeQuery_EncodedPayload_Identity(t *testing.T) {
	min := []float32{-1.5, 0, 1.5}
	max := []float32{2.5, 3.5, 4.5}

	q, err := NewFloatRangeSlowRangeQuery("f", min, max, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}

	concrete := q.(*floatRangeSlowRangeQuery)
	got := concrete.QueryPackedValue()
	want, err := document.EncodeFloatRangeLucene(min, max)
	if err != nil {
		t.Fatalf("reference encoder: %v", err)
	}

	if len(got) != len(want) {
		t.Fatalf("packed length %d != reference length %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("packed[%d] = %#x, want %#x (full got=%x want=%x)", i, got[i], want[i], got, want)
		}
	}
}

// TestFloatRangeSlowRangeQuery_Match_INTERSECTS_1D walks the per-dim
// INTERSECTS predicate for a 1D query and a set of candidate ranges, on
// both sides of the boundary, asserting the exact matches/misses.
func TestFloatRangeSlowRangeQuery_Match_INTERSECTS_1D(t *testing.T) {
	q, err := NewFloatRangeSlowRangeQuery("f", []float32{1.0}, []float32{3.0}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	base := q.(*floatRangeSlowRangeQuery).binaryRangeFieldRangeQuery

	cases := []struct {
		name string
		min  float32
		max  float32
		want bool
	}{
		{"fully inside", 1.5, 2.5, true},
		{"equal to query", 1.0, 3.0, true},
		{"overlap on the right edge", 2.5, 3.5, true},
		{"overlap on the left edge", 0.5, 1.5, true},
		{"strictly contains query", 0.0, 5.0, true},
		{"touching right boundary", 3.0, 4.0, true},
		{"touching left boundary", 0.0, 1.0, true},
		{"entirely to the right", 4.0, 5.0, false},
		{"entirely to the left", -1.0, 0.5, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			packed, err := document.EncodeFloatRangeLucene([]float32{tc.min}, []float32{tc.max})
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			got := base.Match(packed)
			if got != tc.want {
				t.Fatalf("Match(%v..%v) = %v, want %v", tc.min, tc.max, got, tc.want)
			}
		})
	}
}

// TestFloatRangeSlowRangeQuery_Match_INTERSECTS_2D verifies the
// multi-dimensional short-circuit: any dim that fails to intersect causes
// the whole predicate to fail.
func TestFloatRangeSlowRangeQuery_Match_INTERSECTS_2D(t *testing.T) {
	q, err := NewFloatRangeSlowRangeQuery(
		"f",
		[]float32{1.0, 10.0},
		[]float32{3.0, 30.0},
		document.RangeFieldQueryTypeIntersects,
	)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	base := q.(*floatRangeSlowRangeQuery).binaryRangeFieldRangeQuery

	mustEncode := func(min, max []float32) []byte {
		t.Helper()
		packed, err := document.EncodeFloatRangeLucene(min, max)
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		return packed
	}

	if !base.Match(mustEncode([]float32{2, 15}, []float32{2.5, 20})) {
		t.Fatalf("expected match for candidate fully inside both dims")
	}
	if base.Match(mustEncode([]float32{4, 15}, []float32{5, 20})) {
		t.Fatalf("expected miss when dim 0 falls outside (dim 1 inside)")
	}
	if base.Match(mustEncode([]float32{2, 100}, []float32{2.5, 200})) {
		t.Fatalf("expected miss when dim 1 falls outside (dim 0 inside)")
	}
}

// TestFloatRangeSlowRangeQuery_ScorerSupplier_MissingField verifies the
// fast path: when the leaf reader has no doc-values for the field, the
// supplier must be nil (matching the Java null-Scorer contract).
func TestFloatRangeSlowRangeQuery_ScorerSupplier_MissingField(t *testing.T) {
	q, err := NewFloatRangeSlowRangeQuery("f", []float32{0}, []float32{1}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	w, err := q.CreateWeight(nil, false, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	brw := w.(*binaryRangeFieldRangeWeight)

	missing := &stubLeafReader{}
	supplier, err := brw.scorerSupplierForTest(missing)
	if err != nil {
		t.Fatalf("scorerSupplierForTest: %v", err)
	}
	if supplier != nil {
		t.Fatalf("expected nil ScorerSupplier when field is missing, got %T", supplier)
	}
}

// TestFloatRangeSlowRangeQuery_Scorer_TwoPhase_Iteration drives the
// scorer through an in-memory BinaryDocValues iterator carrying a small
// set of encoded ranges, asserting the matching doc IDs and the constant
// score (boost).
func TestFloatRangeSlowRangeQuery_Scorer_TwoPhase_Iteration(t *testing.T) {
	q, err := NewFloatRangeSlowRangeQuery("f", []float32{1.0}, []float32{3.0}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	const boost = 2.5
	w, err := q.CreateWeight(nil, false, boost)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	brw := w.(*binaryRangeFieldRangeWeight)

	docs := []docRange{
		{docID: 0, min: 0.0, max: 0.5}, // miss (entirely left)
		{docID: 1, min: 0.5, max: 1.5}, // match (overlap left edge)
		{docID: 2, min: 2.0, max: 2.5}, // match (inside)
		{docID: 3, min: 4.0, max: 5.0}, // miss (entirely right)
		{docID: 4, min: 2.5, max: 4.0}, // match (overlap right edge)
	}
	stub := newStubBinaryDocValues(t, docs)
	leaf := &stubLeafReader{dv: stub, field: "f"}

	supplier, err := brw.scorerSupplierForTest(leaf)
	if err != nil {
		t.Fatalf("scorerSupplierForTest: %v", err)
	}
	if supplier == nil {
		t.Fatalf("expected non-nil ScorerSupplier")
	}
	scorer, err := supplier.Get(0)
	if err != nil {
		t.Fatalf("supplier.Get: %v", err)
	}

	got := make([]int, 0, len(docs))
	for {
		doc, err := scorer.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if doc == NO_MORE_DOCS {
			break
		}
		if s := scorer.Score(); s != boost {
			t.Fatalf("Score() = %v, want %v (boost)", s, boost)
		}
		got = append(got, doc)
	}
	want := []int{1, 2, 4}
	if len(got) != len(want) {
		t.Fatalf("matched docs = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("matched docs = %v, want %v", got, want)
		}
	}
}

// TestFloatRangeSlowRangeQuery_Scorer_DocValuesError surfaces an error
// raised by the underlying BinaryDocValues iterator through the
// TwoPhaseIterator match function.
func TestFloatRangeSlowRangeQuery_Scorer_DocValuesError(t *testing.T) {
	q, err := NewFloatRangeSlowRangeQuery("f", []float32{1.0}, []float32{3.0}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	w, err := q.CreateWeight(nil, false, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	brw := w.(*binaryRangeFieldRangeWeight)

	wantErr := errors.New("boom")
	dv := &errorBinaryDocValues{err: wantErr}
	leaf := &stubLeafReader{dv: dv, field: "f"}

	supplier, err := brw.scorerSupplierForTest(leaf)
	if err != nil {
		t.Fatalf("scorerSupplierForTest: %v", err)
	}
	scorer, err := supplier.Get(0)
	if err != nil {
		t.Fatalf("supplier.Get: %v", err)
	}

	// The error surfaces on the match-phase call inside NextDoc.
	if _, err := scorer.NextDoc(); err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("expected wantErr to surface from NextDoc, got %v", err)
	}
}

// --- test scaffolding --------------------------------------------------------

// scorerSupplierForTest is the test-only seam that lets the unit tests
// drive a binaryRangeFieldRangeWeight with a pure in-memory leaf reader
// (no LeafReaderContext / SegmentReader plumbing needed). It mirrors the
// production ScorerSupplier path exactly, just without the LeafReaderContext
// wrapping.
func (w *binaryRangeFieldRangeWeight) scorerSupplierForTest(reader binaryDocValuesProvider) (ScorerSupplier, error) {
	if reader == nil {
		return nil, nil
	}
	dv, err := reader.GetBinaryDocValues(w.query.field)
	if err != nil {
		return nil, err
	}
	if dv == nil {
		return nil, nil
	}
	scorer := newBinaryRangeFieldRangeScorer(w, dv)
	return NewScorerSupplierAdapter(scorer), nil
}

// docRange is a per-document packed-range fixture.
type docRange struct {
	docID  int
	min    float32
	max    float32
	packed []byte
}

// stubLeafReader implements [binaryDocValuesProvider] with a fixed
// BinaryDocValues and field name. Returns nil for any other field — that
// drives the missing-field fast path in the production ScorerSupplier.
type stubLeafReader struct {
	dv    index.BinaryDocValues
	field string
}

// GetBinaryDocValues mirrors the production signature.
func (r *stubLeafReader) GetBinaryDocValues(field string) (index.BinaryDocValues, error) {
	if r.dv == nil || field != r.field {
		return nil, nil
	}
	return r.dv, nil
}

// newStubBinaryDocValues encodes each docRange via the Lucene encoder so
// the scorer exercises the exact production decode path.
func newStubBinaryDocValues(t *testing.T, docs []docRange) *stubBinaryDocValues {
	t.Helper()
	out := make([]docRange, len(docs))
	for i, d := range docs {
		packed, err := document.EncodeFloatRangeLucene([]float32{d.min}, []float32{d.max})
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		d.packed = packed
		out[i] = d
	}
	return &stubBinaryDocValues{docs: out, idx: -1}
}

// stubBinaryDocValues is an in-memory BinaryDocValues over a sorted set of
// docRange fixtures. Doc-ID order follows the input slice.
type stubBinaryDocValues struct {
	docs []docRange
	idx  int
}

// Cost returns the stub's known document count.
func (s *stubBinaryDocValues) Cost() int64 { return int64(len(s.docs)) }

// Advance moves the iterator to the first doc at or after target.
func (s *stubBinaryDocValues) Advance(target int) (int, error) {
	for i := s.idx + 1; i < len(s.docs); i++ {
		if s.docs[i].docID >= target {
			s.idx = i
			return s.docs[i].docID, nil
		}
	}
	s.idx = len(s.docs)
	return NO_MORE_DOCS, nil
}

// NextDoc advances to the next doc.
func (s *stubBinaryDocValues) NextDoc() (int, error) {
	s.idx++
	if s.idx >= len(s.docs) {
		s.idx = len(s.docs)
		return NO_MORE_DOCS, nil
	}
	return s.docs[s.idx].docID, nil
}

// DocID returns the current doc, or -1 / NO_MORE_DOCS at the edges.
func (s *stubBinaryDocValues) DocID() int {
	if s.idx < 0 {
		return -1
	}
	if s.idx >= len(s.docs) {
		return NO_MORE_DOCS
	}
	return s.docs[s.idx].docID
}

func (s *stubBinaryDocValues) AdvanceExact(target int) (bool, error) {
	got, err := s.Advance(target)
	if err != nil {
		return false, err
	}
	return got == target, nil
}

func (s *stubBinaryDocValues) BinaryValue() ([]byte, error) {
	if s.idx < 0 || s.idx >= len(s.docs) {
		return nil, nil
	}
	return s.docs[s.idx].packed, nil
}

// errorBinaryDocValues yields one doc, then fails on Get. Used to verify
// the error path through the match function in TwoPhaseIterator.
type errorBinaryDocValues struct {
	err  error
	doc  int
	done bool
}

func (e *errorBinaryDocValues) Advance(int) (int, error) { return e.NextDoc() }
func (e *errorBinaryDocValues) AdvanceExact(int) (bool, error) {
	return false, e.err
}
func (e *errorBinaryDocValues) BinaryValue() ([]byte, error) { return nil, e.err }
func (e *errorBinaryDocValues) Cost() int64                  { return 1 }
func (e *errorBinaryDocValues) NextDoc() (int, error) {
	if e.done {
		return NO_MORE_DOCS, nil
	}
	e.done = true
	e.doc = 0
	return 0, nil
}
func (e *errorBinaryDocValues) DocID() int { return e.doc }