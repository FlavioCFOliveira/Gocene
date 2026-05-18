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
	"strconv"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/document"
)

// longRangeSizeBytes mirrors org.apache.lucene.document.LongRange.BYTES,
// the byte-width of a single packed long dimension value (8 bytes /
// Long.BYTES). Kept as a package-private constant so the encoding helper
// and the slow-range-query constructor agree on the layout without
// pulling LongRange details into search/.
const longRangeSizeBytes = 8

// LongRangeSlowRangeQuery is the Go port of Apache Lucene 10.4.0
// org.apache.lucene.document.LongRangeSlowRangeQuery
// (lucene/core/src/java/org/apache/lucene/document/LongRangeSlowRangeQuery.java).
//
// The query matches documents whose LongRange doc-values intersect the
// supplied [min, max] query rectangle on every dimension. The match is
// scored as a constant score (boost) — there is no per-doc scoring signal.
//
// # Divergence from Lucene
//
//  1. Package: the Java type is package-private in org.apache.lucene.document.
//     In Gocene queries live in search/ to avoid the search<->document
//     import cycle. search/ imports document/ for the encoder
//     ([document.EncodeLongRangeLucene]) and the QueryType enum.
//
//  2. Exposure: the Java class is package-private. In Gocene the type is
//     unexported (longRangeSlowRangeQuery) but the factory
//     [NewLongRangeSlowRangeQuery] is exported so external callers
//     (typically the future LongRange.newSlowIntersectsQuery factory) can
//     construct it. This matches the pattern established by GOC-3207
//     (FloatRangeSlowRangeQuery) and GOC-3211 (IntRangeSlowRangeQuery).
//
//  3. Inheritance: the Java type extends BinaryRangeFieldRangeQuery. Go uses
//     composition: longRangeSlowRangeQuery embeds *binaryRangeFieldRangeQuery
//     for the shared field/numDims/queryPackedValue plumbing and overrides
//     Equals/HashCode/Visit/String with the long-aware variants.
//
//  4. Element type: Java's `long` is 64-bit signed. The Go port uses int64
//     to preserve the exact value range and bit-pattern, matching the
//     encoder [document.EncodeLongRangeLucene] which already takes []int64.
type longRangeSlowRangeQuery struct {
	*binaryRangeFieldRangeQuery

	field string
	min   []int64
	max   []int64
}

// NewLongRangeSlowRangeQuery constructs a LongRangeSlowRangeQuery for the
// given field. The two arrays must have the same length (one entry per
// dimension), and each min[d] <= max[d] (Lucene rejects the inverse with
// IllegalArgumentException inside LongRange.verifyAndEncode, mirrored
// here by [document.EncodeLongRangeLucene]).
//
// queryType must be [document.RangeFieldQueryTypeIntersects]; the binary
// base rejects every other variant, matching the Java reference.
func NewLongRangeSlowRangeQuery(field string, min, max []int64, queryType document.RangeFieldQueryType) (Query, error) {
	if len(min) != len(max) {
		return nil, fmt.Errorf("min length %d != max length %d", len(min), len(max))
	}
	if len(min) == 0 {
		return nil, fmt.Errorf("min/max must contain at least one dimension")
	}
	packed, err := encodeLongRanges(min, max)
	if err != nil {
		return nil, err
	}
	base, err := newBinaryRangeFieldRangeQuery(field, packed, longRangeSizeBytes, len(min), queryType)
	if err != nil {
		return nil, err
	}
	// Defensive copies so the caller cannot mutate the query payload via the
	// slices it passed in; the Java reference does not need this because
	// arrays are by-reference but the LongRange writers always allocate
	// fresh arrays before reaching this constructor.
	dupMin := append([]int64(nil), min...)
	dupMax := append([]int64(nil), max...)
	return &longRangeSlowRangeQuery{
		binaryRangeFieldRangeQuery: base,
		field:                      field,
		min:                        dupMin,
		max:                        dupMax,
	}, nil
}

// Field returns the field name. Shadows the base method for documentation
// clarity and to surface the field on the concrete type's API.
func (q *longRangeSlowRangeQuery) Field() string { return q.field }

// Min returns a defensive copy of the per-dimension query lower bounds.
func (q *longRangeSlowRangeQuery) Min() []int64 {
	out := make([]int64, len(q.min))
	copy(out, q.min)
	return out
}

// Max returns a defensive copy of the per-dimension query upper bounds.
func (q *longRangeSlowRangeQuery) Max() []int64 {
	out := make([]int64, len(q.max))
	copy(out, q.max)
	return out
}

// Equals mirrors the Java reference: two LongRangeSlowRangeQuery are equal
// iff they share field, min, and max arrays.
func (q *longRangeSlowRangeQuery) Equals(other Query) bool {
	o, ok := other.(*longRangeSlowRangeQuery)
	if !ok {
		return false
	}
	if q == o {
		return true
	}
	if q.field != o.field {
		return false
	}
	return int64SliceEquals(q.min, o.min) && int64SliceEquals(q.max, o.max)
}

// HashCode mirrors Java's Objects/Arrays-based hash: a per-type constant
// rolled through (31*h + field-hash + Arrays.hashCode(min) + Arrays.hashCode(max)).
// java.util.Arrays.hashCode(long[]) seeds at 1 and folds each element via
// 31*h + (int)(v ^ (v >>> 32)), which we reproduce exactly in int32
// arithmetic so the hash matches Java for the same input arrays.
func (q *longRangeSlowRangeQuery) HashCode() int {
	h := classHashLongRangeSlowRangeQuery
	h = 31*h + stringHash(q.field)
	h = 31*h + int64SliceHash(q.min)
	h = 31*h + int64SliceHash(q.max)
	return h
}

// Visit mirrors the Java reference: the visitor is asked for the field; on
// accept the query reports itself as a leaf. Shadows the base implementation
// so the leaf reported to the visitor is the concrete long query, not the
// embedded binary base.
func (q *longRangeSlowRangeQuery) Visit(visitor QueryVisitor) {
	if visitor.AcceptField(q.field) {
		visitor.VisitLeaf(q)
	}
}

// String formats the query as Lucene does: optional "field:" prefix when
// rendered out of context, followed by "[ [min0, min1, ...] TO [max0, max1, ...] ]".
// Mirrors java.util.Arrays.toString for long[].
func (q *longRangeSlowRangeQuery) String(field string) string {
	var b strings.Builder
	if q.field != field {
		b.WriteString(q.field)
		b.WriteByte(':')
	}
	b.WriteByte('[')
	b.WriteString(formatInt64Slice(q.min))
	b.WriteString(" TO ")
	b.WriteString(formatInt64Slice(q.max))
	b.WriteByte(']')
	return b.String()
}

// Rewrite mirrors the Java reference, which simply forwards to
// super.rewrite(IndexSearcher) — i.e. returns the query unchanged.
func (q *longRangeSlowRangeQuery) Rewrite(_ IndexReader) (Query, error) { return q, nil }

// Clone returns the query unchanged. The encoded payload and long arrays
// are owned by the query and never mutated through its API.
func (q *longRangeSlowRangeQuery) Clone() Query { return q }

// CreateWeight delegates to the binary base so the doc-values plumbing is
// reused verbatim. The long wrapper contributes only equality/visit and
// the public min/max accessors.
func (q *longRangeSlowRangeQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	w, err := q.binaryRangeFieldRangeQuery.CreateWeight(searcher, needsScores, boost)
	if err != nil {
		return nil, err
	}
	// Re-point the BaseWeight at the concrete long query so GetQuery
	// returns the LongRangeSlowRangeQuery instead of the embedded base.
	if brw, ok := w.(*binaryRangeFieldRangeWeight); ok {
		brw.BaseWeight = NewBaseWeight(q)
	}
	return w, nil
}

// encodeLongRanges packs an N-dimensional [min, max] payload via the existing
// Lucene-compatible encoder so the byte stream is identical to the Java
// reference (LongRange.verifyAndEncode + LongToSortableBytes).
func encodeLongRanges(min, max []int64) ([]byte, error) {
	return document.EncodeLongRangeLucene(min, max)
}

// int64SliceEquals mirrors java.util.Arrays.equals(long[], long[]).
func int64SliceEquals(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// int64SliceHash mirrors java.util.Arrays.hashCode(long[]).
// The Java reference seeds at 1 and folds each long value via
//
//	h = 31*h + (int)(v ^ (v >>> 32))
//
// We reproduce that integer arithmetic exactly so the hash matches the Java
// reference for the same input arrays.
func int64SliceHash(a []int64) int {
	h := int32(1)
	for _, v := range a {
		// (int)(v ^ (v >>> 32)) in Java: XOR the low and high 32 bits, then
		// narrow to int32. In Go we cast through uint64 to guarantee a
		// logical shift, XOR, then truncate to int32.
		uv := uint64(v)
		mix := int32(uv ^ (uv >> 32))
		h = 31*h + mix
	}
	return int(h)
}

// formatInt64Slice formats a long slice as java.util.Arrays.toString does:
// "[v0, v1, v2]" with the default Long.toString rendering.
func formatInt64Slice(a []int64) string {
	if len(a) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, v := range a {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.FormatInt(v, 10))
	}
	b.WriteByte(']')
	return b.String()
}

// classHashLongRangeSlowRangeQuery seeds the long query hash. Distinct
// from classHashBinaryRangeFieldRangeQuery, classHashFloatRangeSlowRangeQuery
// and classHashIntRangeSlowRangeQuery so a long query and a binary-base,
// float, or int query with the same packed payload do not collide.
const classHashLongRangeSlowRangeQuery = 0x6c72_7372 // "lrsr"

// Ensure longRangeSlowRangeQuery implements Query.
var _ Query = (*longRangeSlowRangeQuery)(nil)
