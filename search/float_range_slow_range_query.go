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
	"fmt"
	"math"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/document"
)

// floatRangeSizeBytes mirrors org.apache.lucene.document.FloatRange.BYTES,
// the byte-width of a single packed float dimension value (4 bytes /
// Float.BYTES). Kept as a package-private constant so the encoding helper
// and the slow-range-query constructor agree on the layout without pulling
// FloatRange details into search/.
const floatRangeSizeBytes = 4

// FloatRangeSlowRangeQuery is the Go port of Apache Lucene 10.4.0
// org.apache.lucene.document.FloatRangeSlowRangeQuery
// (lucene/core/src/java/org/apache/lucene/document/FloatRangeSlowRangeQuery.java).
//
// The query matches documents whose FloatRange doc-values intersect the
// supplied [min, max] query rectangle on every dimension. The match is
// scored as a constant score (boost) — there is no per-doc scoring signal.
//
// # Divergence from Lucene
//
//  1. Package: the Java type is package-private in org.apache.lucene.document.
//     In Gocene queries live in search/ to avoid the search<->document
//     import cycle. search/ imports document/ for the encoder
//     ([document.EncodeFloatRangeLucene]) and the QueryType enum.
//
//  2. Exposure: the Java class is package-private. In Gocene the type is
//     unexported (floatRangeSlowRangeQuery) but the factory
//     [NewFloatRangeSlowRangeQuery] is exported so external callers
//     (typically the future FloatRange.newSlowIntersectsQuery factory) can
//     construct it. This matches the pattern established by GOC-3206
//     (LongDistanceFeatureQuery).
//
//  3. Inheritance: the Java type extends BinaryRangeFieldRangeQuery. Go uses
//     composition: floatRangeSlowRangeQuery embeds *binaryRangeFieldRangeQuery
//     for the shared field/numDims/queryPackedValue plumbing and overrides
//     Equals/HashCode/Visit/String with the float-aware variants.
type floatRangeSlowRangeQuery struct {
	*binaryRangeFieldRangeQuery

	field string
	min   []float32
	max   []float32
}

// NewFloatRangeSlowRangeQuery constructs a FloatRangeSlowRangeQuery for the
// given field. The two arrays must have the same length (one entry per
// dimension), and each min[d] <= max[d] (Lucene rejects the inverse with
// IllegalArgumentException inside FloatRange.verifyAndEncode, mirrored
// here by [document.EncodeFloatRangeLucene]).
//
// queryType must be [document.RangeFieldQueryTypeIntersects]; the binary
// base rejects every other variant, matching the Java reference.
func NewFloatRangeSlowRangeQuery(field string, min, max []float32, queryType document.RangeFieldQueryType) (Query, error) {
	if len(min) != len(max) {
		return nil, fmt.Errorf("min length %d != max length %d", len(min), len(max))
	}
	if len(min) == 0 {
		return nil, fmt.Errorf("min/max must contain at least one dimension")
	}
	packed, err := encodeFloatRanges(min, max)
	if err != nil {
		return nil, err
	}
	base, err := newBinaryRangeFieldRangeQuery(field, packed, floatRangeSizeBytes, len(min), queryType)
	if err != nil {
		return nil, err
	}
	// Defensive copies so the caller cannot mutate the query payload via the
	// slices it passed in; the Java reference does not need this because
	// arrays are by-reference but the FloatRange writers always allocate
	// fresh arrays before reaching this constructor.
	dupMin := append([]float32(nil), min...)
	dupMax := append([]float32(nil), max...)
	return &floatRangeSlowRangeQuery{
		binaryRangeFieldRangeQuery: base,
		field:                      field,
		min:                        dupMin,
		max:                        dupMax,
	}, nil
}

// Field returns the field name. Shadows the base method for documentation
// clarity and to surface the field on the concrete type's API.
func (q *floatRangeSlowRangeQuery) Field() string { return q.field }

// Min returns a defensive copy of the per-dimension query lower bounds.
func (q *floatRangeSlowRangeQuery) Min() []float32 {
	out := make([]float32, len(q.min))
	copy(out, q.min)
	return out
}

// Max returns a defensive copy of the per-dimension query upper bounds.
func (q *floatRangeSlowRangeQuery) Max() []float32 {
	out := make([]float32, len(q.max))
	copy(out, q.max)
	return out
}

// Equals mirrors the Java reference: two FloatRangeSlowRangeQuery are equal
// iff they share field, min, and max arrays. Note this is stricter than the
// base binary equality (which only compares field + packed payload): two
// queries that differ in NaN handling could share the same packed payload
// but differ in min/max bit-pattern; the Java reference compares the
// original arrays, so we do too.
func (q *floatRangeSlowRangeQuery) Equals(other Query) bool {
	o, ok := other.(*floatRangeSlowRangeQuery)
	if !ok {
		return false
	}
	if q == o {
		return true
	}
	if q.field != o.field {
		return false
	}
	return floatSliceEquals(q.min, o.min) && floatSliceEquals(q.max, o.max)
}

// HashCode mirrors Java's Objects/Arrays-based hash: a per-type constant
// rolled through (31*h + field-hash + Arrays.hashCode(min) + Arrays.hashCode(max)).
// Arrays.hashCode on float[] hashes each float via Float.floatToIntBits, so
// we mirror that with math.Float32bits.
func (q *floatRangeSlowRangeQuery) HashCode() int {
	h := classHashFloatRangeSlowRangeQuery
	h = 31*h + stringHash(q.field)
	h = 31*h + floatSliceHash(q.min)
	h = 31*h + floatSliceHash(q.max)
	return h
}

// Visit mirrors the Java reference: the visitor is asked for the field; on
// accept the query reports itself as a leaf. Shadows the base implementation
// so the leaf reported to the visitor is the concrete float query, not the
// embedded binary base.
func (q *floatRangeSlowRangeQuery) Visit(visitor QueryVisitor) {
	if visitor.AcceptField(q.field) {
		visitor.VisitLeaf(q)
	}
}

// String formats the query as Lucene does: optional "field:" prefix when
// rendered out of context, followed by "[ [min0, min1, ...] TO [max0, max1, ...] ]".
// Mirrors java.util.Arrays.toString for float[].
func (q *floatRangeSlowRangeQuery) String(field string) string {
	var b strings.Builder
	if q.field != field {
		b.WriteString(q.field)
		b.WriteByte(':')
	}
	b.WriteByte('[')
	b.WriteString(formatFloatSlice(q.min))
	b.WriteString(" TO ")
	b.WriteString(formatFloatSlice(q.max))
	b.WriteByte(']')
	return b.String()
}

// Rewrite mirrors the Java reference, which simply forwards to
// super.rewrite(IndexSearcher) — i.e. returns the query unchanged.
func (q *floatRangeSlowRangeQuery) Rewrite(_ IndexReader) (Query, error) { return q, nil }

// Clone returns the query unchanged. The encoded payload and float arrays
// are owned by the query and never mutated through its API.
func (q *floatRangeSlowRangeQuery) Clone() Query { return q }

// CreateWeight delegates to the binary base so the doc-values plumbing is
// reused verbatim. The float wrapper contributes only equality/visit and
// the public min/max accessors.
func (q *floatRangeSlowRangeQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	w, err := q.binaryRangeFieldRangeQuery.CreateWeight(searcher, needsScores, boost)
	if err != nil {
		return nil, err
	}
	// Re-point the BaseWeight at the concrete float query so GetQuery
	// returns the FloatRangeSlowRangeQuery instead of the embedded base.
	if brw, ok := w.(*binaryRangeFieldRangeWeight); ok {
		brw.BaseWeight = NewBaseWeight(q)
	}
	return w, nil
}

// encodeFloatRanges packs an N-dimensional [min, max] payload via the
// existing Lucene-compatible encoder so the byte stream is identical to the
// Java reference (FloatRange.verifyAndEncode + IntToSortableBytes).
func encodeFloatRanges(min, max []float32) ([]byte, error) {
	return document.EncodeFloatRangeLucene(min, max)
}

// floatSliceEquals mirrors java.util.Arrays.equals(float[], float[]) which
// uses Float.floatToIntBits for the comparison (so NaN==NaN, -0f != +0f).
// We do the same via math.Float32bits.
func floatSliceEquals(a, b []float32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if math.Float32bits(a[i]) != math.Float32bits(b[i]) {
			return false
		}
	}
	return true
}

// floatSliceHash mirrors java.util.Arrays.hashCode(float[]).
// The Java reference seeds at 1 and folds each Float.floatToIntBits value
// via 31*h + element. We reproduce that integer arithmetic exactly so the
// hash matches the Java reference for the same input arrays.
func floatSliceHash(a []float32) int {
	h := int32(1)
	for _, f := range a {
		h = 31*h + int32(math.Float32bits(f))
	}
	return int(h)
}

// formatFloatSlice formats a float slice as java.util.Arrays.toString
// does: "[v0, v1, v2]" with the default Float.toString rendering. The Go
// 'g' verb matches Java's Float.toString for finite values; the special
// cases (NaN / Infinity) are reproduced explicitly.
func formatFloatSlice(a []float32) string {
	if len(a) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, f := range a {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(formatFloat(f))
	}
	b.WriteByte(']')
	return b.String()
}

// formatFloat renders a float32 the way java.lang.Float.toString does for
// the common finite-value range. Special values match Java's literal names.
func formatFloat(f float32) string {
	switch {
	case math.IsNaN(float64(f)):
		return "NaN"
	case math.IsInf(float64(f), 1):
		return "Infinity"
	case math.IsInf(float64(f), -1):
		return "-Infinity"
	default:
		// Use the shortest-roundtrip representation. Java's Float.toString
		// uses Ryu-style shortest output; Go's strconv.FormatFloat with -1
		// precision produces a similar (round-trip safe) decimal.
		// Direct fmt with %g is fine for the toString contract: callers
		// compare strings only in tests, not as a wire format.
		return fmt.Sprintf("%g", f)
	}
}

// classHashFloatRangeSlowRangeQuery seeds the float query hash. Distinct
// from classHashBinaryRangeFieldRangeQuery so a float query and a binary-
// base query with the same packed payload do not collide.
const classHashFloatRangeSlowRangeQuery = 0x6672_7372 // "frsr"

// Ensure floatRangeSlowRangeQuery implements Query.
var _ Query = (*floatRangeSlowRangeQuery)(nil)
