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
	"bytes"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util/bkd"
)

// binaryRangeFieldRangeQuery is the Go port of Apache Lucene 10.4.0
// org.apache.lucene.document.BinaryRangeFieldRangeQuery
// (lucene/core/src/java/org/apache/lucene/document/BinaryRangeFieldRangeQuery.java).
//
// The Java reference is an abstract, package-private class shared by the
// FloatRangeSlowRangeQuery / DoubleRangeSlowRangeQuery / IntRangeSlowRangeQuery /
// LongRangeSlowRangeQuery family. It evaluates whether each per-document
// packed range intersects the query packed range, walking binary doc-values
// behind a TwoPhaseIterator and emitting a constant score per matching doc.
//
// # Divergence from Lucene
//
//  1. Package: the Java type lives in the document package (package-private).
//     In Gocene queries live in search/ to avoid a search<->document import
//     cycle (queries reference search.Weight/search.Scorer, which would
//     otherwise pull search into document). search may import document
//     (one-way), which is exactly the dependency this query needs to read
//     [document.BinaryRangeDocValues].
//
//  2. Inheritance: Go has no class inheritance. The concrete query types
//     (e.g. floatRangeSlowRangeQuery) embed *binaryRangeFieldRangeQuery and
//     delegate the shared behaviour (createWeight, equals/hashCode/visit
//     against the packed payload, rewrite). Equality/visit overrides on the
//     concrete type are layered on top.
//
//  3. INTERSECTS-only: matching Java, the constructor rejects every query
//     type other than [document.RangeFieldQueryTypeIntersects].
//
// The receiver fields mirror the Java field set 1:1; the
// [bkd.GetUnsignedComparator] dependency provides the unsigned-byte
// comparator that Lucene gets from ArrayUtil.getUnsignedComparator.
type binaryRangeFieldRangeQuery struct {
	BaseQuery

	field                string
	queryPackedValue     []byte
	numBytesPerDimension int
	numDims              int
	queryType            document.RangeFieldQueryType
	comparator           bkd.ByteArrayComparator
}

// newBinaryRangeFieldRangeQuery constructs the shared base query. Only
// [document.RangeFieldQueryTypeIntersects] is accepted, matching the Java
// reference's UnsupportedOperationException.
func newBinaryRangeFieldRangeQuery(
	field string,
	queryPackedValue []byte,
	numBytesPerDimension int,
	numDims int,
	queryType document.RangeFieldQueryType,
) (*binaryRangeFieldRangeQuery, error) {
	if field == "" {
		return nil, fmt.Errorf("field name cannot be empty")
	}
	if numBytesPerDimension <= 0 {
		return nil, fmt.Errorf("numBytesPerDimension must be positive; got %d", numBytesPerDimension)
	}
	if numDims <= 0 {
		return nil, fmt.Errorf("numDims must be positive; got %d", numDims)
	}
	if got, want := len(queryPackedValue), 2*numBytesPerDimension*numDims; got != want {
		return nil, fmt.Errorf(
			"queryPackedValue length %d does not match 2 * numBytesPerDimension (%d) * numDims (%d) = %d",
			got, numBytesPerDimension, numDims, want,
		)
	}
	if queryType != document.RangeFieldQueryTypeIntersects {
		return nil, fmt.Errorf(
			"INTERSECTS is the only query type supported for this field type right now; got %s",
			queryType,
		)
	}
	return &binaryRangeFieldRangeQuery{
		field:                field,
		queryPackedValue:     queryPackedValue,
		numBytesPerDimension: numBytesPerDimension,
		numDims:              numDims,
		queryType:            queryType,
		comparator:           bkd.GetUnsignedComparator(numBytesPerDimension),
	}, nil
}

// Field returns the target field name.
func (q *binaryRangeFieldRangeQuery) Field() string { return q.field }

// QueryPackedValue returns the packed query payload (read-only view).
func (q *binaryRangeFieldRangeQuery) QueryPackedValue() []byte { return q.queryPackedValue }

// NumBytesPerDimension returns the byte width of a single dimension value.
func (q *binaryRangeFieldRangeQuery) NumBytesPerDimension() int { return q.numBytesPerDimension }

// NumDims returns the number of dimensions per range.
func (q *binaryRangeFieldRangeQuery) NumDims() int { return q.numDims }

// QueryType returns the spatial relation type of this query.
func (q *binaryRangeFieldRangeQuery) QueryType() document.RangeFieldQueryType { return q.queryType }

// Equals reports whether two binaryRangeFieldRangeQuery share field and
// packed payload. Mirrors the Java reference, which compares only those two
// fields (numDims / numBytesPerDimension are implied by the payload length).
func (q *binaryRangeFieldRangeQuery) Equals(other Query) bool {
	o, ok := other.(*binaryRangeFieldRangeQuery)
	if !ok {
		return false
	}
	if q == o {
		return true
	}
	return q.field == o.field && bytes.Equal(q.queryPackedValue, o.queryPackedValue)
}

// HashCode mirrors the Java hashCode: classHash + field hash + packed-value
// hash, weaving in the field/payload identity used for equality. The
// classHash() Lucene helper hashes the runtime type; in Gocene we substitute
// a stable constant so two instances of the same concrete type collide,
// matching the spirit of the Java identity.
func (q *binaryRangeFieldRangeQuery) HashCode() int {
	h := classHashBinaryRangeFieldRangeQuery
	h = 31*h + stringHash(q.field)
	h = 31*h + bytesHash(q.queryPackedValue)
	return h
}

// Visit dispatches to QueryVisitor only when the visitor accepts the target
// field. Mirrors Lucene's two-step accept/visitLeaf protocol.
func (q *binaryRangeFieldRangeQuery) Visit(visitor QueryVisitor) {
	if visitor.AcceptField(q.field) {
		visitor.VisitLeaf(q)
	}
}

// Rewrite returns the query unchanged. The Java reference invokes
// super.rewrite(IndexSearcher) which is a no-op for leaf queries.
func (q *binaryRangeFieldRangeQuery) Rewrite(_ IndexReader) (Query, error) { return q, nil }

// String prints the field, dimensions and packed payload size. The shared
// base type intentionally yields a generic representation; concrete leaves
// (e.g. floatRangeSlowRangeQuery) provide a richer toString that surfaces
// the original min/max arrays.
func (q *binaryRangeFieldRangeQuery) String(field string) string {
	if field == q.field {
		return fmt.Sprintf("BinaryRangeFieldRangeQuery(numDims=%d,bytesPerDim=%d)", q.numDims, q.numBytesPerDimension)
	}
	return fmt.Sprintf("%s:BinaryRangeFieldRangeQuery(numDims=%d,bytesPerDim=%d)", q.field, q.numDims, q.numBytesPerDimension)
}

// CreateWeight builds a ConstantScoreWeight equivalent over the binary
// doc-values for the target field. The supplier returns nil when the field
// is unknown to the leaf, matching Lucene's null-Scorer fast path.
func (q *binaryRangeFieldRangeQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	w := &binaryRangeFieldRangeWeight{
		query: q,
		boost: boost,
	}
	w.BaseWeight = NewBaseWeight(q)
	return w, nil
}

// Clone returns the query unchanged. The packed payload is treated as
// immutable: the struct is shared by value semantics, and the byte slice is
// not exposed as mutable through the API surface.
func (q *binaryRangeFieldRangeQuery) Clone() Query { return q }

// Matches evaluates the INTERSECTS predicate across every dimension for the
// supplied packed candidate. Mirrors Lucene's
// RangeFieldQuery.QueryType.INTERSECTS.matches(queryPackedValue, packedValue,
// numDims, bytesPerDim, comparator), which iterates dimensions and short-
// circuits on the first non-matching dim.
//
// Exported (Match, not matches) so the floatRangeSlowRangeQuery scorer can
// reuse the comparator-driven byte arithmetic without duplicating it.
func (q *binaryRangeFieldRangeQuery) Match(packedValue []byte) bool {
	return intersectsMatches(q.queryPackedValue, packedValue, q.numDims, q.numBytesPerDimension, q.comparator)
}

// intersectsMatches is the per-document INTERSECTS predicate, mirroring
// Lucene 10.4.0's RangeFieldQuery.QueryType.INTERSECTS.matches over a
// packed range payload laid out as [min0, min1, ..., max0, max1, ...].
func intersectsMatches(queryPackedValue, packedValue []byte, numDims, bytesPerDim int, comparator bkd.ByteArrayComparator) bool {
	for dim := 0; dim < numDims; dim++ {
		if !intersectsMatchesDim(queryPackedValue, packedValue, numDims, bytesPerDim, dim, comparator) {
			return false
		}
	}
	return true
}

// intersectsMatchesDim is the single-dimension INTERSECTS check. The packed
// payload contains numDims values for the min followed by numDims values
// for the max, each value being bytesPerDim wide. A query [qMin, qMax]
// intersects a document range [dMin, dMax] on this dim iff
// qMax >= dMin and qMin <= dMax (using the unsigned comparator).
func intersectsMatchesDim(queryPackedValue, packedValue []byte, numDims, bytesPerDim, dim int, comparator bkd.ByteArrayComparator) bool {
	minOffset := dim * bytesPerDim
	maxOffset := minOffset + bytesPerDim*numDims
	return comparator(queryPackedValue, maxOffset, packedValue, minOffset) >= 0 &&
		comparator(queryPackedValue, minOffset, packedValue, maxOffset) <= 0
}

// binaryRangeFieldRangeWeight is the per-Weight half of the binary range
// query. It owns the boost (used in lieu of the Lucene
// ConstantScoreWeight.score()) and the back-pointer to the parent query
// (used to read the packed comparison payload).
type binaryRangeFieldRangeWeight struct {
	*BaseWeight
	query *binaryRangeFieldRangeQuery
	boost float32
}

// ScorerSupplier returns a ScorerSupplier whose Scorer walks the binary
// doc-values for the field and emits the boost as the constant score.
// Returns (nil, nil) when the field is unknown to the leaf — matching the
// Java fast path that returns a null Scorer.
func (w *binaryRangeFieldRangeWeight) ScorerSupplier(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
	reader := ctx.LeafReader()
	if reader == nil {
		return nil, nil
	}
	provider, ok := reader.(binaryDocValuesProvider)
	if !ok {
		// The leaf does not expose binary doc-values (e.g. MultiReader,
		// CodecReader). Matching Lucene's null-Scorer fast path.
		return nil, nil
	}
	dv, err := provider.GetBinaryDocValues(w.query.field)
	if err != nil {
		return nil, err
	}
	if dv == nil {
		return nil, nil
	}
	scorer := newBinaryRangeFieldRangeScorer(w, dv)
	return NewScorerSupplierAdapter(scorer), nil
}

// binaryDocValuesProvider is the narrow capability surface this query
// requires from a leaf reader. The canonical implementations are
// [index.LeafReader], [index.FilterLeafReader] and [index.ParallelLeafReader].
// Defined locally to avoid widening [index.LeafReaderInterface], which
// would break readers that legitimately do not expose binary doc-values
// (e.g. composite readers used as leaves in some test paths).
type binaryDocValuesProvider interface {
	GetBinaryDocValues(field string) (index.BinaryDocValues, error)
}

// Scorer is the convenience entry point that mirrors the Java
// Weight.scorer() shape. It delegates to ScorerSupplier exactly as the
// Lucene Weight does.
func (w *binaryRangeFieldRangeWeight) Scorer(ctx *index.LeafReaderContext) (Scorer, error) {
	supplier, err := w.ScorerSupplier(ctx)
	if err != nil {
		return nil, err
	}
	if supplier == nil {
		return nil, nil
	}
	return supplier.Get(0)
}

// IsCacheable mirrors DocValues.isCacheable from the Java reference. In
// Gocene the DocValues cache wiring is not yet ported; until it lands, the
// safest answer is true (cacheable) for the doc-values-only Weight, which
// matches the Java code path for fields without updates.
func (w *binaryRangeFieldRangeWeight) IsCacheable(_ *index.LeafReaderContext) bool { return true }

// binaryRangeFieldRangeScorer wraps the BinaryDocValues iterator behind a
// two-phase iterator: the approximation simply forwards the doc-values
// iterator, and the match phase decodes the packed value and runs the
// INTERSECTS predicate. Constant score equals the supplied boost.
type binaryRangeFieldRangeScorer struct {
	*BaseScorer

	weight  *binaryRangeFieldRangeWeight
	dv      index.BinaryDocValues
	twoPh   *TwoPhaseIterator
	iter    DocIdSetIterator
	matched bool
}

func newBinaryRangeFieldRangeScorer(weight *binaryRangeFieldRangeWeight, dv index.BinaryDocValues) *binaryRangeFieldRangeScorer {
	s := &binaryRangeFieldRangeScorer{
		BaseScorer: NewBaseScorer(weight),
		weight:     weight,
		dv:         dv,
	}
	approx := &binaryRangeDocValuesIterator{dv: dv}
	s.twoPh = NewTwoPhaseIterator(approx, func() (bool, error) {
		value, err := dv.Get(approx.DocID())
		if err != nil {
			return false, err
		}
		if value == nil {
			return false, nil
		}
		return weight.query.Match(value), nil
	})
	s.iter = s.twoPh.AsDocIdSetIterator()
	return s
}

// DocID forwards to the underlying iterator.
func (s *binaryRangeFieldRangeScorer) DocID() int { return s.iter.DocID() }

// NextDoc advances the scorer to the next matching document.
func (s *binaryRangeFieldRangeScorer) NextDoc() (int, error) { return s.iter.NextDoc() }

// Advance moves the scorer to the first matching document at or beyond target.
func (s *binaryRangeFieldRangeScorer) Advance(target int) (int, error) {
	return s.iter.Advance(target)
}

// Cost returns the underlying iterator's cost estimate.
func (s *binaryRangeFieldRangeScorer) Cost() int64 { return s.iter.Cost() }

// DocIDRunEnd returns the end of the current run.
func (s *binaryRangeFieldRangeScorer) DocIDRunEnd() int { return s.iter.DocIDRunEnd() }

// Score returns the constant boost score.
func (s *binaryRangeFieldRangeScorer) Score() float32 { return s.weight.boost }

// GetMaxScore returns the constant boost score (no per-doc variability).
func (s *binaryRangeFieldRangeScorer) GetMaxScore(_ int) float32 { return s.weight.boost }

// AsTwoPhase exposes the underlying TwoPhaseIterator so a BulkScorer can
// optimise around it. Mirrors Scorer.asTwoPhaseIterator in Lucene.
func (s *binaryRangeFieldRangeScorer) AsTwoPhase() *TwoPhaseIterator { return s.twoPh }

// Ensure the scorer satisfies the Scorer interface.
var _ Scorer = (*binaryRangeFieldRangeScorer)(nil)

// binaryRangeDocValuesIterator is a tiny DocIdSetIterator that delegates to
// a [index.BinaryDocValues]. It exists because the doc-values type only
// exposes the Lucene iterator API (DocID/NextDoc/Advance) but does not
// satisfy DocIdSetIterator directly (Cost / DocIDRunEnd are absent).
type binaryRangeDocValuesIterator struct {
	dv index.BinaryDocValues
}

// DocID forwards to the underlying doc-values iterator.
func (it *binaryRangeDocValuesIterator) DocID() int { return it.dv.DocID() }

// NextDoc forwards to the underlying doc-values iterator.
func (it *binaryRangeDocValuesIterator) NextDoc() (int, error) { return it.dv.NextDoc() }

// Advance forwards to the underlying doc-values iterator.
func (it *binaryRangeDocValuesIterator) Advance(target int) (int, error) {
	return it.dv.Advance(target)
}

// Cost returns an unknown-cost sentinel. Doc-values iterators in Gocene do
// not yet surface a cost estimate; downstream code already uses 0 as the
// sentinel for "unknown but non-empty".
func (it *binaryRangeDocValuesIterator) Cost() int64 { return 0 }

// DocIDRunEnd returns the current doc + 1, mirroring the default
// AbstractDocIdSetIterator behaviour for sparse iterators.
func (it *binaryRangeDocValuesIterator) DocIDRunEnd() int { return it.dv.DocID() + 1 }

var _ DocIdSetIterator = (*binaryRangeDocValuesIterator)(nil)

// classHashBinaryRangeFieldRangeQuery seeds the binary base hash. The Java
// reference uses Query.classHash(), which returns a runtime-class-derived
// constant; Gocene uses a per-type literal so the embedded base never
// collides with the concrete leaves (e.g. floatRangeSlowRangeQuery).
const classHashBinaryRangeFieldRangeQuery = 0x6272_6672 // "brfr"

// stringHash returns the FNV-1a 32-bit hash of s as an int. Kept local to
// avoid pulling hash/fnv into a critical-path package; the encoded query
// payload is the dominant hash contributor.
func stringHash(s string) int {
	const (
		offset = 2166136261
		prime  = 16777619
	)
	var h uint32 = offset
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= prime
	}
	return int(int32(h))
}

// bytesHash returns the FNV-1a 32-bit hash of b as an int.
func bytesHash(b []byte) int {
	const (
		offset = 2166136261
		prime  = 16777619
	)
	var h uint32 = offset
	for _, c := range b {
		h ^= uint32(c)
		h *= prime
	}
	return int(int32(h))
}

// Ensure binaryRangeFieldRangeQuery implements Query.
var _ Query = (*binaryRangeFieldRangeQuery)(nil)
