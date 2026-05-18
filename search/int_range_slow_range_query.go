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

// intRangeSizeBytes mirrors org.apache.lucene.document.IntRange.BYTES,
// the byte-width of a single packed int dimension value (4 bytes /
// Integer.BYTES). Kept as a package-private constant so the encoding
// helper and the slow-range-query constructor agree on the layout without
// pulling IntRange details into search/.
const intRangeSizeBytes = 4

// IntRangeSlowRangeQuery is the Go port of Apache Lucene 10.4.0
// org.apache.lucene.document.IntRangeSlowRangeQuery
// (lucene/core/src/java/org/apache/lucene/document/IntRangeSlowRangeQuery.java).
//
// The query matches documents whose IntRange doc-values intersect the
// supplied [min, max] query rectangle on every dimension. The match is
// scored as a constant score (boost) — there is no per-doc scoring signal.
//
// # Divergence from Lucene
//
//  1. Package: the Java type is package-private in org.apache.lucene.document.
//     In Gocene queries live in search/ to avoid the search<->document
//     import cycle. search/ imports document/ for the encoder
//     ([document.EncodeIntRangeLucene]) and the QueryType enum.
//
//  2. Exposure: the Java class is package-private. In Gocene the type is
//     unexported (intRangeSlowRangeQuery) but the factory
//     [NewIntRangeSlowRangeQuery] is exported so external callers
//     (typically the future IntRange.newSlowIntersectsQuery factory) can
//     construct it. This matches the pattern established by GOC-3207
//     (FloatRangeSlowRangeQuery).
//
//  3. Inheritance: the Java type extends BinaryRangeFieldRangeQuery. Go uses
//     composition: intRangeSlowRangeQuery embeds *binaryRangeFieldRangeQuery
//     for the shared field/numDims/queryPackedValue plumbing and overrides
//     Equals/HashCode/Visit/String with the int-aware variants.
//
//  4. Element type: Java's `int` is 32-bit signed. The Go port uses int32 to
//     preserve the exact value range and bit-pattern, matching the encoder
//     [document.EncodeIntRangeLucene] which already takes []int32.
type intRangeSlowRangeQuery struct {
	*binaryRangeFieldRangeQuery

	field string
	min   []int32
	max   []int32
}

// NewIntRangeSlowRangeQuery constructs an IntRangeSlowRangeQuery for the
// given field. The two arrays must have the same length (one entry per
// dimension), and each min[d] <= max[d] (Lucene rejects the inverse with
// IllegalArgumentException inside IntRange.verifyAndEncode, mirrored
// here by [document.EncodeIntRangeLucene]).
//
// queryType must be [document.RangeFieldQueryTypeIntersects]; the binary
// base rejects every other variant, matching the Java reference.
func NewIntRangeSlowRangeQuery(field string, min, max []int32, queryType document.RangeFieldQueryType) (Query, error) {
	if len(min) != len(max) {
		return nil, fmt.Errorf("min length %d != max length %d", len(min), len(max))
	}
	if len(min) == 0 {
		return nil, fmt.Errorf("min/max must contain at least one dimension")
	}
	packed, err := encodeIntRanges(min, max)
	if err != nil {
		return nil, err
	}
	base, err := newBinaryRangeFieldRangeQuery(field, packed, intRangeSizeBytes, len(min), queryType)
	if err != nil {
		return nil, err
	}
	// Defensive copies so the caller cannot mutate the query payload via the
	// slices it passed in; the Java reference does not need this because
	// arrays are by-reference but the IntRange writers always allocate
	// fresh arrays before reaching this constructor.
	dupMin := append([]int32(nil), min...)
	dupMax := append([]int32(nil), max...)
	return &intRangeSlowRangeQuery{
		binaryRangeFieldRangeQuery: base,
		field:                      field,
		min:                        dupMin,
		max:                        dupMax,
	}, nil
}

// Field returns the field name. Shadows the base method for documentation
// clarity and to surface the field on the concrete type's API.
func (q *intRangeSlowRangeQuery) Field() string { return q.field }

// Min returns a defensive copy of the per-dimension query lower bounds.
func (q *intRangeSlowRangeQuery) Min() []int32 {
	out := make([]int32, len(q.min))
	copy(out, q.min)
	return out
}

// Max returns a defensive copy of the per-dimension query upper bounds.
func (q *intRangeSlowRangeQuery) Max() []int32 {
	out := make([]int32, len(q.max))
	copy(out, q.max)
	return out
}

// Equals mirrors the Java reference: two IntRangeSlowRangeQuery are equal
// iff they share field, min, and max arrays.
func (q *intRangeSlowRangeQuery) Equals(other Query) bool {
	o, ok := other.(*intRangeSlowRangeQuery)
	if !ok {
		return false
	}
	if q == o {
		return true
	}
	if q.field != o.field {
		return false
	}
	return int32SliceEquals(q.min, o.min) && int32SliceEquals(q.max, o.max)
}

// HashCode mirrors Java's Objects/Arrays-based hash: a per-type constant
// rolled through (31*h + field-hash + Arrays.hashCode(min) + Arrays.hashCode(max)).
// java.util.Arrays.hashCode(int[]) seeds at 1 and folds each element via
// 31*h + element, which we reproduce exactly in int32 arithmetic.
func (q *intRangeSlowRangeQuery) HashCode() int {
	h := classHashIntRangeSlowRangeQuery
	h = 31*h + stringHash(q.field)
	h = 31*h + int32SliceHash(q.min)
	h = 31*h + int32SliceHash(q.max)
	return h
}

// Visit mirrors the Java reference: the visitor is asked for the field; on
// accept the query reports itself as a leaf. Shadows the base implementation
// so the leaf reported to the visitor is the concrete int query, not the
// embedded binary base.
func (q *intRangeSlowRangeQuery) Visit(visitor QueryVisitor) {
	if visitor.AcceptField(q.field) {
		visitor.VisitLeaf(q)
	}
}

// String formats the query as Lucene does: optional "field:" prefix when
// rendered out of context, followed by "[ [min0, min1, ...] TO [max0, max1, ...] ]".
// Mirrors java.util.Arrays.toString for int[].
func (q *intRangeSlowRangeQuery) String(field string) string {
	var b strings.Builder
	if q.field != field {
		b.WriteString(q.field)
		b.WriteByte(':')
	}
	b.WriteByte('[')
	b.WriteString(formatInt32Slice(q.min))
	b.WriteString(" TO ")
	b.WriteString(formatInt32Slice(q.max))
	b.WriteByte(']')
	return b.String()
}

// Rewrite mirrors the Java reference, which simply forwards to
// super.rewrite(IndexSearcher) — i.e. returns the query unchanged.
func (q *intRangeSlowRangeQuery) Rewrite(_ IndexReader) (Query, error) { return q, nil }

// Clone returns the query unchanged. The encoded payload and int arrays
// are owned by the query and never mutated through its API.
func (q *intRangeSlowRangeQuery) Clone() Query { return q }

// CreateWeight delegates to the binary base so the doc-values plumbing is
// reused verbatim. The int wrapper contributes only equality/visit and
// the public min/max accessors.
func (q *intRangeSlowRangeQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	w, err := q.binaryRangeFieldRangeQuery.CreateWeight(searcher, needsScores, boost)
	if err != nil {
		return nil, err
	}
	// Re-point the BaseWeight at the concrete int query so GetQuery
	// returns the IntRangeSlowRangeQuery instead of the embedded base.
	if brw, ok := w.(*binaryRangeFieldRangeWeight); ok {
		brw.BaseWeight = NewBaseWeight(q)
	}
	return w, nil
}

// encodeIntRanges packs an N-dimensional [min, max] payload via the existing
// Lucene-compatible encoder so the byte stream is identical to the Java
// reference (IntRange.verifyAndEncode + IntToSortableBytes).
func encodeIntRanges(min, max []int32) ([]byte, error) {
	return document.EncodeIntRangeLucene(min, max)
}

// int32SliceEquals mirrors java.util.Arrays.equals(int[], int[]).
func int32SliceEquals(a, b []int32) bool {
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

// int32SliceHash mirrors java.util.Arrays.hashCode(int[]).
// The Java reference seeds at 1 and folds each int value via 31*h + element.
// We reproduce that integer arithmetic exactly so the hash matches the Java
// reference for the same input arrays.
func int32SliceHash(a []int32) int {
	h := int32(1)
	for _, v := range a {
		h = 31*h + v
	}
	return int(h)
}

// formatInt32Slice formats an int slice as java.util.Arrays.toString does:
// "[v0, v1, v2]" with the default Integer.toString rendering.
func formatInt32Slice(a []int32) string {
	if len(a) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, v := range a {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.FormatInt(int64(v), 10))
	}
	b.WriteByte(']')
	return b.String()
}

// classHashIntRangeSlowRangeQuery seeds the int query hash. Distinct from
// classHashBinaryRangeFieldRangeQuery and classHashFloatRangeSlowRangeQuery
// so an int query and a binary-base or float query with the same packed
// payload do not collide.
const classHashIntRangeSlowRangeQuery = 0x6972_7372 // "irsr"

// Ensure intRangeSlowRangeQuery implements Query.
var _ Query = (*intRangeSlowRangeQuery)(nil)
