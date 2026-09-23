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
	"github.com/FlavioCFOliveira/Gocene/spi"
	"math"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/document"
)

// floatRangeSizeBytes mirrors org.apache.lucene.document.FloatRange.BYTES,
// the byte-width of a single packed float dimension value (4 bytes /
// Float.BYTES).
const floatRangeSizeBytes = 4

// floatRangeSlowRangeQuery is the Go port of Apache Lucene 10.5.0
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
//     ([document.Encode], the Go rendering of FloatRange.verifyAndEncode)
//     and the QueryType enum.
//
//  2. Exposure: the Java class is package-private. In Gocene the type is
//     unexported (floatRangeSlowRangeQuery) but the factory
//     [NewFloatRangeSlowRangeQuery] is exported so external callers
//     (typically FloatRange.newSlowIntersectsQuery) can construct it.
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
// dimension), each min[d] <= max[d], and neither may contain NaN (Lucene
// rejects both inside FloatRange.verifyAndEncode, mirrored here by
// [document.Encode]).
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
// iff they share field, min, and max arrays.
func (q *floatRangeSlowRangeQuery) Equals(other spi.Query) bool {
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
	return float32SliceEquals(q.min, o.min) && float32SliceEquals(q.max, o.max)
}

// HashCode mirrors Java's Objects/Arrays-based hash: a per-type constant
// rolled through (31*h + field-hash + Arrays.hashCode(min) + Arrays.hashCode(max)).
func (q *floatRangeSlowRangeQuery) HashCode() int {
	h := classHashFloatRangeSlowRangeQuery
	h = 31*h + stringHash(q.field)
	h = 31*h + float32SliceHash(q.min)
	h = 31*h + float32SliceHash(q.max)
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
	b.WriteString(formatFloat32Slice(q.min))
	b.WriteString(" TO ")
	b.WriteString(formatFloat32Slice(q.max))
	b.WriteByte(']')
	return b.String()
}

// Rewrite mirrors the Java reference, which simply forwards to
// super.rewrite(IndexSearcher) — i.e. returns the query unchanged.
func (q *floatRangeSlowRangeQuery) Rewrite(_ *IndexSearcher) (Query, error) { return q, nil }

// CreateWeight delegates to the binary base so the doc-values plumbing is
// reused verbatim. The float wrapper contributes only equality/visit and
// the public min/max accessors.
func (q *floatRangeSlowRangeQuery) CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	w, err := q.binaryRangeFieldRangeQuery.CreateWeight(searcher, scoreMode, boost)
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

// encodeFloatRanges packs an N-dimensional [min, max] payload via the existing
// Lucene-compatible encoder so the byte stream is identical to the Java
// reference (FloatRange.verifyAndEncode + FloatToSortableInt +
// IntToSortableBytes).
func encodeFloatRanges(min, max []float32) ([]byte, error) {
	return document.Encode(min, max)
}

// float32SliceEquals mirrors java.util.Arrays.equals(float[], float[]), which
// compares Float.floatToIntBits so that NaN equals NaN and -0.0 differs from 0.0.
func float32SliceEquals(a, b []float32) bool {
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

// float32SliceHash mirrors java.util.Arrays.hashCode(float[]).
// The Java reference seeds at 1 and folds each element via
// 31*h + Float.floatToIntBits(element).
func float32SliceHash(a []float32) int {
	h := int32(1)
	for _, v := range a {
		h = 31*h + int32(math.Float32bits(v))
	}
	return int(h)
}

// formatFloat32Slice formats a float slice as java.util.Arrays.toString does:
// "[v0, v1, v2]" with the default Float.toString rendering.
func formatFloat32Slice(a []float32) string {
	if len(a) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, v := range a {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(formatFloat32(v))
	}
	b.WriteByte(']')
	return b.String()
}

// formatFloat32 renders a float32 the way java.lang.Float.toString does for
// the common finite-value range. Special values match Java's literal names.
func formatFloat32(v float32) string {
	return formatJavaFloatingPoint(float64(v), 32)
}

// classHashFloatRangeSlowRangeQuery seeds the float query hash. Distinct from
// classHashBinaryRangeFieldRangeQuery, classHashIntRangeSlowRangeQuery,
// classHashLongRangeSlowRangeQuery and classHashDoubleRangeSlowRangeQuery so
// a float query and a binary-base, int, long or double query with the same
// packed payload do not collide.
const classHashFloatRangeSlowRangeQuery = 0x6672_7372 // "frsr"

// Ensure floatRangeSlowRangeQuery implements Query.
var _ Query = (*floatRangeSlowRangeQuery)(nil)
