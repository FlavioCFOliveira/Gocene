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

// doubleRangeSizeBytes mirrors org.apache.lucene.document.DoubleRange.BYTES,
// the byte-width of a single packed double dimension value (8 bytes /
// Double.BYTES). Kept as a package-private constant so the encoding helper
// and the slow-range-query constructor agree on the layout without pulling
// DoubleRange details into search/.
const doubleRangeSizeBytes = 8

// DoubleRangeSlowRangeQuery is the Go port of Apache Lucene 10.4.0
// org.apache.lucene.document.DoubleRangeSlowRangeQuery
// (lucene/core/src/java/org/apache/lucene/document/DoubleRangeSlowRangeQuery.java).
//
// The query matches documents whose DoubleRange doc-values intersect the
// supplied [min, max] query rectangle on every dimension. The match is
// scored as a constant score (boost) — there is no per-doc scoring signal.
//
// # Divergence from Lucene
//
//  1. Package: the Java type is package-private in org.apache.lucene.document.
//     In Gocene queries live in search/ to avoid the search<->document
//     import cycle. search/ imports document/ for the encoder
//     ([document.EncodeDoubleRangeLucene]) and the QueryType enum.
//
//  2. Exposure: the Java class is package-private. In Gocene the type is
//     unexported (doubleRangeSlowRangeQuery) but the factory
//     [NewDoubleRangeSlowRangeQuery] is exported so external callers
//     (typically the future DoubleRange.newSlowIntersectsQuery factory) can
//     construct it. This matches the pattern established by GOC-3207
//     (FloatRangeSlowRangeQuery), GOC-3211 (IntRangeSlowRangeQuery) and
//     GOC-3214 (LongRangeSlowRangeQuery).
//
//  3. Inheritance: the Java type extends BinaryRangeFieldRangeQuery. Go uses
//     composition: doubleRangeSlowRangeQuery embeds *binaryRangeFieldRangeQuery
//     for the shared field/numDims/queryPackedValue plumbing and overrides
//     Equals/HashCode/Visit/String with the double-aware variants.
//
//  4. Element type: Java's `double` is IEEE 754 64-bit. The Go port uses
//     float64 to preserve the exact bit-pattern, matching the encoder
//     [document.EncodeDoubleRangeLucene] which already takes []float64.
//     Lucene rejects NaN inside DoubleRange.verifyAndEncode; the Gocene
//     encoder mirrors that rejection.
type doubleRangeSlowRangeQuery struct {
	*binaryRangeFieldRangeQuery

	field string
	min   []float64
	max   []float64
}

// NewDoubleRangeSlowRangeQuery constructs a DoubleRangeSlowRangeQuery for the
// given field. The two arrays must have the same length (one entry per
// dimension), each min[d] <= max[d], and neither may contain NaN (Lucene
// rejects the inverse with IllegalArgumentException inside
// DoubleRange.verifyAndEncode, mirrored here by
// [document.EncodeDoubleRangeLucene]).
//
// queryType must be [document.RangeFieldQueryTypeIntersects]; the binary
// base rejects every other variant, matching the Java reference.
func NewDoubleRangeSlowRangeQuery(field string, min, max []float64, queryType document.RangeFieldQueryType) (Query, error) {
	if len(min) != len(max) {
		return nil, fmt.Errorf("min length %d != max length %d", len(min), len(max))
	}
	if len(min) == 0 {
		return nil, fmt.Errorf("min/max must contain at least one dimension")
	}
	packed, err := encodeDoubleRanges(min, max)
	if err != nil {
		return nil, err
	}
	base, err := newBinaryRangeFieldRangeQuery(field, packed, doubleRangeSizeBytes, len(min), queryType)
	if err != nil {
		return nil, err
	}
	// Defensive copies so the caller cannot mutate the query payload via the
	// slices it passed in; the Java reference does not need this because
	// arrays are by-reference but the DoubleRange writers always allocate
	// fresh arrays before reaching this constructor.
	dupMin := append([]float64(nil), min...)
	dupMax := append([]float64(nil), max...)
	return &doubleRangeSlowRangeQuery{
		binaryRangeFieldRangeQuery: base,
		field:                      field,
		min:                        dupMin,
		max:                        dupMax,
	}, nil
}

// Field returns the field name. Shadows the base method for documentation
// clarity and to surface the field on the concrete type's API.
func (q *doubleRangeSlowRangeQuery) Field() string { return q.field }

// Min returns a defensive copy of the per-dimension query lower bounds.
func (q *doubleRangeSlowRangeQuery) Min() []float64 {
	out := make([]float64, len(q.min))
	copy(out, q.min)
	return out
}

// Max returns a defensive copy of the per-dimension query upper bounds.
func (q *doubleRangeSlowRangeQuery) Max() []float64 {
	out := make([]float64, len(q.max))
	copy(out, q.max)
	return out
}

// Equals mirrors the Java reference: two DoubleRangeSlowRangeQuery are equal
// iff they share field, min, and max arrays. Note this is stricter than the
// base binary equality (which only compares field + packed payload): two
// queries that differ in -0.0 / +0.0 handling could share the same packed
// payload but differ in min/max bit-pattern; the Java reference compares the
// original arrays via java.util.Arrays.equals (which uses
// Double.doubleToLongBits), so we do too.
func (q *doubleRangeSlowRangeQuery) Equals(other Query) bool {
	o, ok := other.(*doubleRangeSlowRangeQuery)
	if !ok {
		return false
	}
	if q == o {
		return true
	}
	if q.field != o.field {
		return false
	}
	return float64SliceEquals(q.min, o.min) && float64SliceEquals(q.max, o.max)
}

// HashCode mirrors Java's Objects/Arrays-based hash: a per-type constant
// rolled through (31*h + field-hash + Arrays.hashCode(min) + Arrays.hashCode(max)).
// java.util.Arrays.hashCode(double[]) seeds at 1 and folds each double via
//
//	long bits = Double.doubleToLongBits(v); h = 31*h + (int)(bits ^ (bits >>> 32))
//
// We reproduce that integer arithmetic exactly so the hash matches the Java
// reference for the same input arrays.
func (q *doubleRangeSlowRangeQuery) HashCode() int {
	h := classHashDoubleRangeSlowRangeQuery
	h = 31*h + stringHash(q.field)
	h = 31*h + float64SliceHash(q.min)
	h = 31*h + float64SliceHash(q.max)
	return h
}

// Visit mirrors the Java reference: the visitor is asked for the field; on
// accept the query reports itself as a leaf. Shadows the base implementation
// so the leaf reported to the visitor is the concrete double query, not the
// embedded binary base.
func (q *doubleRangeSlowRangeQuery) Visit(visitor QueryVisitor) {
	if visitor.AcceptField(q.field) {
		visitor.VisitLeaf(q)
	}
}

// String formats the query as Lucene does: optional "field:" prefix when
// rendered out of context, followed by "[ [min0, min1, ...] TO [max0, max1, ...] ]".
// Mirrors java.util.Arrays.toString for double[].
func (q *doubleRangeSlowRangeQuery) String(field string) string {
	var b strings.Builder
	if q.field != field {
		b.WriteString(q.field)
		b.WriteByte(':')
	}
	b.WriteByte('[')
	b.WriteString(formatFloat64Slice(q.min))
	b.WriteString(" TO ")
	b.WriteString(formatFloat64Slice(q.max))
	b.WriteByte(']')
	return b.String()
}

// Rewrite mirrors the Java reference, which simply forwards to
// super.rewrite(IndexSearcher) — i.e. returns the query unchanged.
func (q *doubleRangeSlowRangeQuery) Rewrite(_ IndexReader) (Query, error) { return q, nil }

// Clone returns the query unchanged. The encoded payload and double arrays
// are owned by the query and never mutated through its API.
func (q *doubleRangeSlowRangeQuery) Clone() Query { return q }

// CreateWeight delegates to the binary base so the doc-values plumbing is
// reused verbatim. The double wrapper contributes only equality/visit and
// the public min/max accessors.
func (q *doubleRangeSlowRangeQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	w, err := q.binaryRangeFieldRangeQuery.CreateWeight(searcher, needsScores, boost)
	if err != nil {
		return nil, err
	}
	// Re-point the BaseWeight at the concrete double query so GetQuery
	// returns the DoubleRangeSlowRangeQuery instead of the embedded base.
	if brw, ok := w.(*binaryRangeFieldRangeWeight); ok {
		brw.BaseWeight = NewBaseWeight(q)
	}
	return w, nil
}

// encodeDoubleRanges packs an N-dimensional [min, max] payload via the existing
// Lucene-compatible encoder so the byte stream is identical to the Java
// reference (DoubleRange.verifyAndEncode + DoubleToSortableLong + LongToSortableBytes).
func encodeDoubleRanges(min, max []float64) ([]byte, error) {
	return document.EncodeDoubleRangeLucene(min, max)
}

// float64SliceEquals mirrors java.util.Arrays.equals(double[], double[]) which
// uses Double.doubleToLongBits for the comparison (so NaN==NaN, -0.0 != +0.0).
// We do the same via math.Float64bits.
func float64SliceEquals(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if math.Float64bits(a[i]) != math.Float64bits(b[i]) {
			return false
		}
	}
	return true
}

// float64SliceHash mirrors java.util.Arrays.hashCode(double[]).
// The Java reference seeds at 1 and folds each double value via
//
//	long bits = Double.doubleToLongBits(v);
//	h = 31*h + (int)(bits ^ (bits >>> 32))
//
// We reproduce that integer arithmetic exactly so the hash matches the Java
// reference for the same input arrays.
func float64SliceHash(a []float64) int {
	h := int32(1)
	for _, v := range a {
		bits := math.Float64bits(v)
		// (int)(bits ^ (bits >>> 32)) in Java: XOR low and high 32 bits then
		// narrow to int32. Float64bits already yields a uint64 with logical
		// shift semantics, so we XOR-shift-truncate to int32 directly.
		mix := int32(bits ^ (bits >> 32))
		h = 31*h + mix
	}
	return int(h)
}

// formatFloat64Slice formats a double slice as java.util.Arrays.toString
// does: "[v0, v1, v2]" with the default Double.toString rendering. The Go
// 'g' verb matches Java's Double.toString for finite values; the special
// cases (NaN / Infinity) are reproduced explicitly even though
// DoubleRange.verifyAndEncode rejects NaN — toString is also invoked from
// debug paths after construction, so the rendering remains robust.
func formatFloat64Slice(a []float64) string {
	if len(a) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, v := range a {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(formatFloat64(v))
	}
	b.WriteByte(']')
	return b.String()
}

// formatFloat64 renders a float64 the way java.lang.Double.toString does for
// the common finite-value range. Special values match Java's literal names.
func formatFloat64(v float64) string {
	switch {
	case math.IsNaN(v):
		return "NaN"
	case math.IsInf(v, 1):
		return "Infinity"
	case math.IsInf(v, -1):
		return "-Infinity"
	default:
		// Shortest round-trip representation. Go's %g matches Java's
		// Double.toString for the common finite-value range; callers compare
		// strings only in tests, not as a wire format.
		return fmt.Sprintf("%g", v)
	}
}

// classHashDoubleRangeSlowRangeQuery seeds the double query hash. Distinct
// from classHashBinaryRangeFieldRangeQuery, classHashFloatRangeSlowRangeQuery,
// classHashIntRangeSlowRangeQuery and classHashLongRangeSlowRangeQuery so a
// double query and a binary-base, float, int or long query with the same
// packed payload do not collide.
const classHashDoubleRangeSlowRangeQuery = 0x6472_7372 // "drsr"

// Ensure doubleRangeSlowRangeQuery implements Query.
var _ Query = (*doubleRangeSlowRangeQuery)(nil)
