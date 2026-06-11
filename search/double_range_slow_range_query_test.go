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
)

// NOTE: simplified vs Java peer because Lucene 10.4.0 does not ship a
// dedicated TestDoubleRangeSlowRangeQuery. The tests below cover the same
// observable contract: constructor validation, equals/hashCode reflexivity
// and distinguishability, Visit field-acceptance, String formatting,
// encoded byte payload identity vs the Lucene encoder, and the in-memory
// match path on the TwoPhaseIterator (no RandomIndexWriter / CheckHits
// dependency).

// TestDoubleRangeSlowRangeQuery_FactoryValidation exercises the constructor
// guards: dimension-length mismatches, empty arrays, inverted ranges
// (rejected inside EncodeDoubleRangeLucene), NaN values (rejected by
// DoubleRange.verifyAndEncode), and non-INTERSECTS query types (rejected
// by the binary base). Includes the -MaxFloat64..MaxFloat64 extremes,
// ensuring the encoder accepts the full IEEE 754 finite double range.
func TestDoubleRangeSlowRangeQuery_FactoryValidation(t *testing.T) {
	nan := math.NaN()
	cases := []struct {
		name     string
		field    string
		min, max []float64
		qType    document.RangeFieldQueryType
		wantErr  bool
	}{
		{
			name:    "empty field is rejected by base",
			field:   "",
			min:     []float64{0},
			max:     []float64{1},
			qType:   document.RangeFieldQueryTypeIntersects,
			wantErr: true,
		},
		{
			name:    "different min/max lengths",
			field:   "f",
			min:     []float64{0},
			max:     []float64{1, 2},
			qType:   document.RangeFieldQueryTypeIntersects,
			wantErr: true,
		},
		{
			name:    "empty min/max",
			field:   "f",
			min:     []float64{},
			max:     []float64{},
			qType:   document.RangeFieldQueryTypeIntersects,
			wantErr: true,
		},
		{
			name:    "inverted range (min > max)",
			field:   "f",
			min:     []float64{2.5},
			max:     []float64{1.5},
			qType:   document.RangeFieldQueryTypeIntersects,
			wantErr: true,
		},
		{
			name:    "NaN in min rejected",
			field:   "f",
			min:     []float64{nan},
			max:     []float64{1},
			qType:   document.RangeFieldQueryTypeIntersects,
			wantErr: true,
		},
		{
			name:    "NaN in max rejected",
			field:   "f",
			min:     []float64{0},
			max:     []float64{nan},
			qType:   document.RangeFieldQueryTypeIntersects,
			wantErr: true,
		},
		{
			name:    "WITHIN rejected — INTERSECTS only",
			field:   "f",
			min:     []float64{0},
			max:     []float64{1},
			qType:   document.RangeFieldQueryTypeWithin,
			wantErr: true,
		},
		{
			name:    "CONTAINS rejected — INTERSECTS only",
			field:   "f",
			min:     []float64{0},
			max:     []float64{1},
			qType:   document.RangeFieldQueryTypeContains,
			wantErr: true,
		},
		{
			name:    "valid 1D",
			field:   "f",
			min:     []float64{0},
			max:     []float64{1},
			qType:   document.RangeFieldQueryTypeIntersects,
			wantErr: false,
		},
		{
			name:    "valid 3D",
			field:   "geo",
			min:     []float64{-1, -2, -3},
			max:     []float64{1, 2, 3},
			qType:   document.RangeFieldQueryTypeIntersects,
			wantErr: false,
		},
		{
			name:    "valid extremes (-MaxFloat64..MaxFloat64)",
			field:   "f",
			min:     []float64{-math.MaxFloat64},
			max:     []float64{math.MaxFloat64},
			qType:   document.RangeFieldQueryTypeIntersects,
			wantErr: false,
		},
		{
			name:    "valid -Inf..+Inf",
			field:   "f",
			min:     []float64{math.Inf(-1)},
			max:     []float64{math.Inf(1)},
			qType:   document.RangeFieldQueryTypeIntersects,
			wantErr: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewDoubleRangeSlowRangeQuery(tc.field, tc.min, tc.max, tc.qType)
			gotErr := err != nil
			if gotErr != tc.wantErr {
				t.Fatalf("NewDoubleRangeSlowRangeQuery err = %v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

// TestDoubleRangeSlowRangeQuery_EqualsAndHashCode mirrors the standard Lucene
// contract: identical inputs collide and equal; differences in field, min,
// or max disturb equality and produce different hash codes; cross-type
// queries with numerically-equivalent inputs are distinct via the class
// hash seeds.
func TestDoubleRangeSlowRangeQuery_EqualsAndHashCode(t *testing.T) {
	mk := func(t *testing.T, field string, min, max []float64) Query {
		t.Helper()
		q, err := NewDoubleRangeSlowRangeQuery(field, min, max, document.RangeFieldQueryTypeIntersects)
		if err != nil {
			t.Fatalf("constructor: %v", err)
		}
		return q
	}

	q1 := mk(t, "f", []float64{0, 1}, []float64{2, 3})
	q2 := mk(t, "f", []float64{0, 1}, []float64{2, 3})
	if !q1.Equals(q2) || !q2.Equals(q1) {
		t.Fatalf("expected q1 == q2 for identical inputs")
	}
	if q1.HashCode() != q2.HashCode() {
		t.Fatalf("expected hash(q1) == hash(q2): %d vs %d", q1.HashCode(), q2.HashCode())
	}

	q3 := mk(t, "g", []float64{0, 1}, []float64{2, 3})
	if q1.Equals(q3) {
		t.Fatalf("expected q1 != q3 (different field)")
	}
	if q1.HashCode() == q3.HashCode() {
		t.Fatalf("expected hash(q1) != hash(q3) for different field")
	}

	q4 := mk(t, "f", []float64{0.5, 1}, []float64{2, 3})
	if q1.Equals(q4) {
		t.Fatalf("expected q1 != q4 (different min)")
	}
	q5 := mk(t, "f", []float64{0, 1}, []float64{2, 4})
	if q1.Equals(q5) {
		t.Fatalf("expected q1 != q5 (different max)")
	}

	// Cross-type: a binary base query with the same packed payload is not
	// equal to a double query — they are distinct concrete types.
	concrete := q1.(*doubleRangeSlowRangeQuery)
	if concrete.Equals(concrete.binaryRangeFieldRangeQuery) {
		t.Fatalf("double query should not Equals its embedded binary base")
	}

	// Cross-type: a double query should not Equals a long query, even with
	// numerically-identical inputs — distinct concrete types and distinct
	// class hashes are the load-bearing guarantee here.
	longQ, err := NewLongRangeSlowRangeQuery("f", []int64{0, 1}, []int64{2, 3}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("long constructor: %v", err)
	}
	if q1.Equals(longQ) {
		t.Fatalf("double query should not Equals a long query with equivalent inputs")
	}
	if q1.HashCode() == longQ.HashCode() {
		t.Fatalf("expected distinct class hashes for double vs long slow-range queries")
	}

	// Cross-type: a double query should not Equals a float query either.
	floatQ, err := NewFloatRangeSlowRangeQuery("f", []float32{0, 1}, []float32{2, 3}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("float constructor: %v", err)
	}
	if q1.Equals(floatQ) {
		t.Fatalf("double query should not Equals a float query with equivalent inputs")
	}
	if q1.HashCode() == floatQ.HashCode() {
		t.Fatalf("expected distinct class hashes for double vs float slow-range queries")
	}

	// Cross-type: a double query should not Equals an int query either.
	intQ, err := NewIntRangeSlowRangeQuery("f", []int32{0, 1}, []int32{2, 3}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("int constructor: %v", err)
	}
	if q1.Equals(intQ) {
		t.Fatalf("double query should not Equals an int query with equivalent inputs")
	}
	if q1.HashCode() == intQ.HashCode() {
		t.Fatalf("expected distinct class hashes for double vs int slow-range queries")
	}
}

// TestDoubleRangeSlowRangeQuery_Equals_NegativeZero ensures -0.0 / +0.0
// arrays compare unequal under bit-pattern semantics (matching
// java.util.Arrays.equals(double[],double[]) which uses
// Double.doubleToLongBits, where -0.0 and +0.0 have distinct bits).
func TestDoubleRangeSlowRangeQuery_Equals_NegativeZero(t *testing.T) {
	negZero := math.Copysign(0, -1)
	q1, err := NewDoubleRangeSlowRangeQuery("f", []float64{negZero}, []float64{1}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("constructor with -0.0: %v", err)
	}
	q2, err := NewDoubleRangeSlowRangeQuery("f", []float64{0.0}, []float64{1}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("constructor with +0.0: %v", err)
	}
	if q1.Equals(q2) {
		t.Fatalf("-0.0 and +0.0 arrays should bit-compare unequal (Arrays.equals semantics)")
	}
}

// TestDoubleRangeSlowRangeQuery_String covers the two branches of toString:
// when the caller-provided field matches the query field, the prefix is
// omitted; otherwise the prefix is "<field>:".
func TestDoubleRangeSlowRangeQuery_String(t *testing.T) {
	q, err := NewDoubleRangeSlowRangeQuery("f", []float64{0.5, 1.5}, []float64{2.5, 3.5}, document.RangeFieldQueryTypeIntersects)
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
	if !strings.Contains(withPrefix, " TO ") || !strings.HasSuffix(withPrefix, "]") {
		t.Fatalf("expected ' TO ' separator and trailing ']' in %q", withPrefix)
	}

	withoutPrefix := stringer.String("f")
	if strings.HasPrefix(withoutPrefix, "f:") {
		t.Fatalf("expected no field prefix when in-context, got %q", withoutPrefix)
	}
	if !strings.HasPrefix(withoutPrefix, "[") || !strings.HasSuffix(withoutPrefix, "]") {
		t.Fatalf("expected bracketed payload in %q", withoutPrefix)
	}
}

// TestDoubleRangeSlowRangeQuery_Visit verifies the visitor contract:
// VisitLeaf fires for the accepted field, the leaf reported is the query
// itself, and the visitor is silent when AcceptField returns false.
func TestDoubleRangeSlowRangeQuery_Visit(t *testing.T) {
	q, err := NewDoubleRangeSlowRangeQuery("f", []float64{0}, []float64{1}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}

	// Reuse the recording visitor from the float test file — same package,
	// same observable contract.
	visited := &recordingFloatRangeVisitor{}
	q.(interface{ Visit(QueryVisitor) }).Visit(visited)
	if !visited.acceptedField {
		t.Fatalf("expected AcceptField(\"f\") to be invoked")
	}
	if visited.leaf != q {
		t.Fatalf("expected VisitLeaf to receive the double query, got %T", visited.leaf)
	}

	rejecting := &recordingFloatRangeVisitor{rejectField: "f"}
	q.(interface{ Visit(QueryVisitor) }).Visit(rejecting)
	if rejecting.leaf != nil {
		t.Fatalf("expected VisitLeaf not to fire when AcceptField returns false")
	}
}

// TestDoubleRangeSlowRangeQuery_Rewrite documents the no-op rewrite contract:
// the query rewrites to itself (matching super.rewrite in Java).
func TestDoubleRangeSlowRangeQuery_Rewrite(t *testing.T) {
	q, err := NewDoubleRangeSlowRangeQuery("f", []float64{0}, []float64{1}, document.RangeFieldQueryTypeIntersects)
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

// TestDoubleRangeSlowRangeQuery_Clone documents the no-op clone contract.
func TestDoubleRangeSlowRangeQuery_Clone(t *testing.T) {
	q, err := NewDoubleRangeSlowRangeQuery("f", []float64{0}, []float64{1}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	if q.Clone() != q {
		t.Fatalf("expected Clone() to return the same instance")
	}
}

// TestDoubleRangeSlowRangeQuery_MinMaxDefensiveCopy verifies that the
// constructor and accessors return defensive copies, so mutating the
// caller's slice after construction does not corrupt the query.
func TestDoubleRangeSlowRangeQuery_MinMaxDefensiveCopy(t *testing.T) {
	min := []float64{0, 1}
	max := []float64{2, 3}
	q, err := NewDoubleRangeSlowRangeQuery("f", min, max, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}

	// Mutate the caller's slices after construction.
	min[0] = 999
	max[1] = -999

	concrete := q.(*doubleRangeSlowRangeQuery)
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

// TestDoubleRangeSlowRangeQuery_EncodedPayload_Identity ensures the packed
// query payload is byte-identical to the Lucene encoder for the same
// inputs. The byte stream identity is the AC for the codec/index/store
// dimension — the encoded query goes on the wire for distributed search.
func TestDoubleRangeSlowRangeQuery_EncodedPayload_Identity(t *testing.T) {
	min := []float64{-1.5, 0, 1.5, -math.MaxFloat64}
	max := []float64{2.5, 3.5, 4.5, math.MaxFloat64}

	q, err := NewDoubleRangeSlowRangeQuery("f", min, max, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}

	concrete := q.(*doubleRangeSlowRangeQuery)
	got := concrete.QueryPackedValue()
	want, err := document.EncodeDoubleRangeLucene(min, max)
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

// TestDoubleRangeSlowRangeQuery_Match_INTERSECTS_1D walks the per-dim
// INTERSECTS predicate for a 1D query and a set of candidate ranges, on
// both sides of the boundary, asserting the exact matches/misses.
func TestDoubleRangeSlowRangeQuery_Match_INTERSECTS_1D(t *testing.T) {
	q, err := NewDoubleRangeSlowRangeQuery("f", []float64{1.0}, []float64{3.0}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	base := q.(*doubleRangeSlowRangeQuery).binaryRangeFieldRangeQuery

	cases := []struct {
		name string
		min  float64
		max  float64
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
			packed, err := document.EncodeDoubleRangeLucene([]float64{tc.min}, []float64{tc.max})
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

// TestDoubleRangeSlowRangeQuery_Match_INTERSECTS_2D verifies the
// multi-dimensional short-circuit: any dim that fails to intersect causes
// the whole predicate to fail.
func TestDoubleRangeSlowRangeQuery_Match_INTERSECTS_2D(t *testing.T) {
	q, err := NewDoubleRangeSlowRangeQuery(
		"f",
		[]float64{1.0, 10.0},
		[]float64{3.0, 30.0},
		document.RangeFieldQueryTypeIntersects,
	)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	base := q.(*doubleRangeSlowRangeQuery).binaryRangeFieldRangeQuery

	mustEncode := func(min, max []float64) []byte {
		t.Helper()
		packed, err := document.EncodeDoubleRangeLucene(min, max)
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		return packed
	}

	if !base.Match(mustEncode([]float64{2, 15}, []float64{2.5, 20})) {
		t.Fatalf("expected match for candidate fully inside both dims")
	}
	if base.Match(mustEncode([]float64{4, 15}, []float64{5, 20})) {
		t.Fatalf("expected miss when dim 0 falls outside (dim 1 inside)")
	}
	if base.Match(mustEncode([]float64{2, 100}, []float64{2.5, 200})) {
		t.Fatalf("expected miss when dim 1 falls outside (dim 0 inside)")
	}
}

// TestDoubleRangeSlowRangeQuery_ScorerSupplier_MissingField verifies the
// fast path: when the leaf reader has no doc-values for the field, the
// supplier must be nil (matching the Java null-Scorer contract).
func TestDoubleRangeSlowRangeQuery_ScorerSupplier_MissingField(t *testing.T) {
	q, err := NewDoubleRangeSlowRangeQuery("f", []float64{0}, []float64{1}, document.RangeFieldQueryTypeIntersects)
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

// TestDoubleRangeSlowRangeQuery_Scorer_TwoPhase_Iteration drives the scorer
// through an in-memory BinaryDocValues iterator carrying a small set of
// encoded ranges, asserting the matching doc IDs and the constant score
// (boost).
func TestDoubleRangeSlowRangeQuery_Scorer_TwoPhase_Iteration(t *testing.T) {
	q, err := NewDoubleRangeSlowRangeQuery("f", []float64{1.0}, []float64{3.0}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	const boost = float32(2.5)
	w, err := q.CreateWeight(nil, false, boost)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	brw := w.(*binaryRangeFieldRangeWeight)

	docs := []docRangeDouble{
		{docID: 0, min: 0.0, max: 0.5}, // miss (entirely left)
		{docID: 1, min: 0.5, max: 1.5}, // match (overlap left edge)
		{docID: 2, min: 2.0, max: 2.5}, // match (inside)
		{docID: 3, min: 4.0, max: 5.0}, // miss (entirely right)
		{docID: 4, min: 2.5, max: 4.0}, // match (overlap right edge)
	}
	stub := newStubBinaryDocValuesDouble(t, docs)
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

// TestDoubleRangeSlowRangeQuery_Scorer_DocValuesError surfaces an error
// raised by the underlying BinaryDocValues iterator through the
// TwoPhaseIterator match function.
func TestDoubleRangeSlowRangeQuery_Scorer_DocValuesError(t *testing.T) {
	q, err := NewDoubleRangeSlowRangeQuery("f", []float64{1.0}, []float64{3.0}, document.RangeFieldQueryTypeIntersects)
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

	if _, err := scorer.NextDoc(); err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("expected wantErr to surface from NextDoc, got %v", err)
	}

// --- double-flavoured test scaffolding -------------------------------------

// docRangeDouble is a per-document packed-range fixture for float64 ranges.
type docRangeDouble struct {
	docID int
	min   float64
	max   float64
}

// newStubBinaryDocValuesDouble encodes each docRangeDouble via the Lucene
// encoder so the scorer exercises the exact production decode path. Reuses
// stubBinaryDocValues from the float test file by mapping into docRange
// with the packed bytes pre-computed (the iterator only consults .packed,
// the float32 min/max fields are unused once .packed is set).
func newStubBinaryDocValuesDouble(t *testing.T, docs []docRangeDouble) *stubBinaryDocValues {
	t.Helper()
	out := make([]docRange, len(docs))
	for i, d := range docs {
		packed, err := document.EncodeDoubleRangeLucene([]float64{d.min}, []float64{d.max})
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		out[i] = docRange{docID: d.docID, packed: packed}
	}
	return &stubBinaryDocValues{docs: out, idx: -1}
}