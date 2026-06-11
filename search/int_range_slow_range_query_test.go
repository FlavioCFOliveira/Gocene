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
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
)

// NOTE: simplified vs Java peer because Lucene 10.4.0 does not ship a
// dedicated TestIntRangeSlowRangeQuery (the reference confirms TR's "none
// located" note). The tests below cover the same observable contract:
// constructor validation, equals/hashCode reflexivity and distinguishability,
// Visit field-acceptance, String formatting, encoded byte payload identity
// vs the Lucene encoder, and the in-memory match path on the
// TwoPhaseIterator (no RandomIndexWriter / CheckHits dependency).

// TestIntRangeSlowRangeQuery_FactoryValidation exercises the constructor
// guards: dimension-length mismatches, empty arrays, inverted ranges
// (rejected inside EncodeIntRangeLucene), and non-INTERSECTS query types
// (rejected by the binary base).
func TestIntRangeSlowRangeQuery_FactoryValidation(t *testing.T) {
	cases := []struct {
		name     string
		field    string
		min, max []int32
		qType    document.RangeFieldQueryType
		wantErr  bool
	}{
		{
			name:    "empty field is rejected by base",
			field:   "",
			min:     []int32{0},
			max:     []int32{1},
			qType:   document.RangeFieldQueryTypeIntersects,
			wantErr: true,
		},
		{
			name:    "different min/max lengths",
			field:   "f",
			min:     []int32{0},
			max:     []int32{1, 2},
			qType:   document.RangeFieldQueryTypeIntersects,
			wantErr: true,
		},
		{
			name:    "empty min/max",
			field:   "f",
			min:     []int32{},
			max:     []int32{},
			qType:   document.RangeFieldQueryTypeIntersects,
			wantErr: true,
		},
		{
			name:    "inverted range (min > max)",
			field:   "f",
			min:     []int32{5},
			max:     []int32{1},
			qType:   document.RangeFieldQueryTypeIntersects,
			wantErr: true,
		},
		{
			name:    "WITHIN rejected — INTERSECTS only",
			field:   "f",
			min:     []int32{0},
			max:     []int32{1},
			qType:   document.RangeFieldQueryTypeWithin,
			wantErr: true,
		},
		{
			name:    "CONTAINS rejected — INTERSECTS only",
			field:   "f",
			min:     []int32{0},
			max:     []int32{1},
			qType:   document.RangeFieldQueryTypeContains,
			wantErr: true,
		},
		{
			name:    "valid 1D",
			field:   "f",
			min:     []int32{0},
			max:     []int32{1},
			qType:   document.RangeFieldQueryTypeIntersects,
			wantErr: false,
		},
		{
			name:    "valid 3D",
			field:   "geo",
			min:     []int32{-1, -2, -3},
			max:     []int32{1, 2, 3},
			qType:   document.RangeFieldQueryTypeIntersects,
			wantErr: false,
		},
		{
			name:    "valid extremes (MinInt32..MaxInt32)",
			field:   "f",
			min:     []int32{-2147483648},
			max:     []int32{2147483647},
			qType:   document.RangeFieldQueryTypeIntersects,
			wantErr: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewIntRangeSlowRangeQuery(tc.field, tc.min, tc.max, tc.qType)
			gotErr := err != nil
			if gotErr != tc.wantErr {
				t.Fatalf("NewIntRangeSlowRangeQuery err = %v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

// TestIntRangeSlowRangeQuery_EqualsAndHashCode mirrors the standard Lucene
// contract: identical inputs collide and equal; differences in field, min,
// or max disturb equality and produce different hash codes.
func TestIntRangeSlowRangeQuery_EqualsAndHashCode(t *testing.T) {
	mk := func(t *testing.T, field string, min, max []int32) Query {
		t.Helper()
		q, err := NewIntRangeSlowRangeQuery(field, min, max, document.RangeFieldQueryTypeIntersects)
		if err != nil {
			t.Fatalf("constructor: %v", err)
		}
		return q
	}

	q1 := mk(t, "f", []int32{0, 1}, []int32{2, 3})
	q2 := mk(t, "f", []int32{0, 1}, []int32{2, 3})
	if !q1.Equals(q2) || !q2.Equals(q1) {
		t.Fatalf("expected q1 == q2 for identical inputs")
	}
	if q1.HashCode() != q2.HashCode() {
		t.Fatalf("expected hash(q1) == hash(q2): %d vs %d", q1.HashCode(), q2.HashCode())
	}

	q3 := mk(t, "g", []int32{0, 1}, []int32{2, 3})
	if q1.Equals(q3) {
		t.Fatalf("expected q1 != q3 (different field)")
	}
	if q1.HashCode() == q3.HashCode() {
		t.Fatalf("expected hash(q1) != hash(q3) for different field")
	}

	q4 := mk(t, "f", []int32{5, 1}, []int32{6, 3})
	if q1.Equals(q4) {
		t.Fatalf("expected q1 != q4 (different min)")
	}
	q5 := mk(t, "f", []int32{0, 1}, []int32{2, 4})
	if q1.Equals(q5) {
		t.Fatalf("expected q1 != q5 (different max)")
	}

	// Cross-type: a binary base query with the same packed payload is not
	// equal to an int query — they are distinct concrete types.
	concrete := q1.(*intRangeSlowRangeQuery)
	if concrete.Equals(concrete.binaryRangeFieldRangeQuery) {
		t.Fatalf("int query should not Equals its embedded binary base")
	}

	// Cross-type: an int query should not Equals a float query with the
	// "same" numeric inputs — distinct concrete types and distinct class
	// hashes are the load-bearing guarantee here.
	floatQ, err := NewFloatRangeSlowRangeQuery("f", []float32{0, 1}, []float32{2, 3}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("float constructor: %v", err)
	}
	if q1.Equals(floatQ) {
		t.Fatalf("int query should not Equals a float query with equivalent inputs")
	}
	if q1.HashCode() == floatQ.HashCode() {
		t.Fatalf("expected distinct class hashes for int vs float slow-range queries")
	}
}

// TestIntRangeSlowRangeQuery_String covers the two branches of toString:
// when the caller-provided field matches the query field, the prefix is
// omitted; otherwise the prefix is "<field>:".
func TestIntRangeSlowRangeQuery_String(t *testing.T) {
	q, err := NewIntRangeSlowRangeQuery("f", []int32{0, 1}, []int32{2, 3}, document.RangeFieldQueryTypeIntersects)
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
	// java.util.Arrays.toString(new int[]{0,1}) -> "[0, 1]"; the full
	// rendering for the inputs above is "f:[[0, 1] TO [2, 3]]".
	if want := "f:[[0, 1] TO [2, 3]]"; withPrefix != want {
		t.Fatalf("String(out-of-context) = %q, want %q", withPrefix, want)
	}

	withoutPrefix := stringer.String("f")
	if strings.HasPrefix(withoutPrefix, "f:") {
		t.Fatalf("expected no field prefix when in-context, got %q", withoutPrefix)
	}
	if want := "[[0, 1] TO [2, 3]]"; withoutPrefix != want {
		t.Fatalf("String(in-context) = %q, want %q", withoutPrefix, want)
	}
}

// TestIntRangeSlowRangeQuery_Visit verifies the visitor contract:
// VisitLeaf fires for the accepted field, the leaf reported is the query
// itself, and the visitor is silent when AcceptField returns false.
func TestIntRangeSlowRangeQuery_Visit(t *testing.T) {
	q, err := NewIntRangeSlowRangeQuery("f", []int32{0}, []int32{1}, document.RangeFieldQueryTypeIntersects)
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
		t.Fatalf("expected VisitLeaf to receive the int query, got %T", visited.leaf)
	}

	rejecting := &recordingFloatRangeVisitor{rejectField: "f"}
	q.(interface{ Visit(QueryVisitor) }).Visit(rejecting)
	if rejecting.leaf != nil {
		t.Fatalf("expected VisitLeaf not to fire when AcceptField returns false")
	}
}

// TestIntRangeSlowRangeQuery_Rewrite documents the no-op rewrite contract:
// the query rewrites to itself (matching super.rewrite in Java).
func TestIntRangeSlowRangeQuery_Rewrite(t *testing.T) {
	q, err := NewIntRangeSlowRangeQuery("f", []int32{0}, []int32{1}, document.RangeFieldQueryTypeIntersects)
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

// TestIntRangeSlowRangeQuery_Clone documents the no-op clone contract.
func TestIntRangeSlowRangeQuery_Clone(t *testing.T) {
	q, err := NewIntRangeSlowRangeQuery("f", []int32{0}, []int32{1}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	if q.Clone() != q {
		t.Fatalf("expected Clone() to return the same instance")
	}
}

// TestIntRangeSlowRangeQuery_MinMaxDefensiveCopy verifies that the
// constructor and accessors return defensive copies, so mutating the
// caller's slice after construction does not corrupt the query.
func TestIntRangeSlowRangeQuery_MinMaxDefensiveCopy(t *testing.T) {
	min := []int32{0, 1}
	max := []int32{2, 3}
	q, err := NewIntRangeSlowRangeQuery("f", min, max, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}

	// Mutate the caller's slices after construction.
	min[0] = 999
	max[1] = -999

	concrete := q.(*intRangeSlowRangeQuery)
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

// TestIntRangeSlowRangeQuery_EncodedPayload_Identity ensures the packed
// query payload is byte-identical to the Lucene encoder for the same
// inputs. The byte stream identity is the AC for the codec/index/store
// dimension — the encoded query goes on the wire for distributed search.
func TestIntRangeSlowRangeQuery_EncodedPayload_Identity(t *testing.T) {
	min := []int32{-1, 0, 5}
	max := []int32{2, 3, 10}

	q, err := NewIntRangeSlowRangeQuery("f", min, max, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}

	concrete := q.(*intRangeSlowRangeQuery)
	got := concrete.QueryPackedValue()
	want, err := document.EncodeIntRangeLucene(min, max)
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

// TestIntRangeSlowRangeQuery_Match_INTERSECTS_1D walks the per-dim
// INTERSECTS predicate for a 1D query and a set of candidate ranges, on
// both sides of the boundary, asserting the exact matches/misses.
func TestIntRangeSlowRangeQuery_Match_INTERSECTS_1D(t *testing.T) {
	q, err := NewIntRangeSlowRangeQuery("f", []int32{1}, []int32{3}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	base := q.(*intRangeSlowRangeQuery).binaryRangeFieldRangeQuery

	cases := []struct {
		name string
		min  int32
		max  int32
		want bool
	}{
		{"fully inside", 2, 2, true},
		{"equal to query", 1, 3, true},
		{"overlap on the right edge", 3, 5, true},
		{"overlap on the left edge", -1, 1, true},
		{"strictly contains query", 0, 5, true},
		{"touching right boundary", 3, 4, true},
		{"touching left boundary", 0, 1, true},
		{"entirely to the right", 4, 5, false},
		{"entirely to the left", -5, 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			packed, err := document.EncodeIntRangeLucene([]int32{tc.min}, []int32{tc.max})
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

// TestIntRangeSlowRangeQuery_Match_INTERSECTS_2D verifies the
// multi-dimensional short-circuit: any dim that fails to intersect causes
// the whole predicate to fail.
func TestIntRangeSlowRangeQuery_Match_INTERSECTS_2D(t *testing.T) {
	q, err := NewIntRangeSlowRangeQuery(
		"f",
		[]int32{1, 10},
		[]int32{3, 30},
		document.RangeFieldQueryTypeIntersects,
	)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	base := q.(*intRangeSlowRangeQuery).binaryRangeFieldRangeQuery

	mustEncode := func(min, max []int32) []byte {
		t.Helper()
		packed, err := document.EncodeIntRangeLucene(min, max)
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		return packed
	}

	if !base.Match(mustEncode([]int32{2, 15}, []int32{2, 20})) {
		t.Fatalf("expected match for candidate fully inside both dims")
	}
	if base.Match(mustEncode([]int32{4, 15}, []int32{5, 20})) {
		t.Fatalf("expected miss when dim 0 falls outside (dim 1 inside)")
	}
	if base.Match(mustEncode([]int32{2, 100}, []int32{2, 200})) {
		t.Fatalf("expected miss when dim 1 falls outside (dim 0 inside)")
	}
}

// TestIntRangeSlowRangeQuery_ScorerSupplier_MissingField verifies the
// fast path: when the leaf reader has no doc-values for the field, the
// supplier must be nil (matching the Java null-Scorer contract).
func TestIntRangeSlowRangeQuery_ScorerSupplier_MissingField(t *testing.T) {
	q, err := NewIntRangeSlowRangeQuery("f", []int32{0}, []int32{1}, document.RangeFieldQueryTypeIntersects)
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

// TestIntRangeSlowRangeQuery_Scorer_TwoPhase_Iteration drives the scorer
// through an in-memory BinaryDocValues iterator carrying a small set of
// encoded ranges, asserting the matching doc IDs and the constant score
// (boost).
func TestIntRangeSlowRangeQuery_Scorer_TwoPhase_Iteration(t *testing.T) {
	q, err := NewIntRangeSlowRangeQuery("f", []int32{1}, []int32{3}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	const boost = float32(2.5)
	w, err := q.CreateWeight(nil, false, boost)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	brw := w.(*binaryRangeFieldRangeWeight)

	docs := []docRangeInt{
		{docID: 0, min: -5, max: 0}, // miss (entirely left)
		{docID: 1, min: 0, max: 1},  // match (overlap left edge)
		{docID: 2, min: 2, max: 2},  // match (inside)
		{docID: 3, min: 4, max: 5},  // miss (entirely right)
		{docID: 4, min: 3, max: 4},  // match (overlap right edge)
	}
	stub := newStubBinaryDocValuesInt(t, docs)
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

// TestIntRangeSlowRangeQuery_Scorer_DocValuesError surfaces an error
// raised by the underlying BinaryDocValues iterator through the
// TwoPhaseIterator match function.
func TestIntRangeSlowRangeQuery_Scorer_DocValuesError(t *testing.T) {
	q, err := NewIntRangeSlowRangeQuery("f", []int32{1}, []int32{3}, document.RangeFieldQueryTypeIntersects)
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

// --- int-flavoured test scaffolding ----------------------------------------

// docRangeInt is a per-document packed-range fixture for int32 ranges.
type docRangeInt struct {
	docID  int
	min    int32
	max    int32
	packed []byte
}

// newStubBinaryDocValuesInt encodes each docRangeInt via the Lucene encoder
// so the scorer exercises the exact production decode path. Reuses
// stubBinaryDocValues from the float test file by mapping into docRange
// with the packed bytes pre-computed (the iterator only consults .packed,
// the float min/max fields are unused once .packed is set).
func newStubBinaryDocValuesInt(t *testing.T, docs []docRangeInt) *stubBinaryDocValues {
	t.Helper()
	out := make([]docRange, len(docs))
	for i, d := range docs {
		packed, err := document.EncodeIntRangeLucene([]int32{d.min}, []int32{d.max})
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		out[i] = docRange{docID: d.docID, packed: packed}
	}
	return &stubBinaryDocValues{docs: out, idx: -1}
}