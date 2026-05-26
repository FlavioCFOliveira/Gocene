// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/bkd"
)

// RangeFieldQuery matches documents whose indexed range field satisfies
// a spatial relation (INTERSECTS, WITHIN, CONTAINS, CROSSES) against the
// query range.
//
// Port of org.apache.lucene.document.RangeFieldQuery (Lucene 10.4.0).
//
// # Deviation from Lucene
//
// Lucene's RangeFieldQuery lives in the document package and is package-private.
// In Gocene it lives in search/ to avoid an import cycle (search → document is
// fine; the inverse is not).  The struct carries queryMin + queryMax separately
// (matching how the current Gocene RangeField factory callers call it) and
// computes the packed ranges payload lazily inside CreateWeight.
type RangeFieldQuery struct {
	field       string
	queryMin    []byte // min packed value (numDims * bytesPerDim bytes)
	queryMax    []byte // max packed value (numDims * bytesPerDim bytes)
	numDims     int    // number of range dimensions
	bytesPerDim int    // byte width of a single dimension value
	queryType   RangeFieldQueryType
}

// RangeFieldQueryType defines the spatial-relation semantics of a RangeFieldQuery.
// Mirrors org.apache.lucene.document.RangeFieldQuery.QueryType.
type RangeFieldQueryType int

const (
	// RangeFieldQueryTypeIntersects matches documents whose range overlaps
	// the query range.
	RangeFieldQueryTypeIntersects RangeFieldQueryType = iota
	// RangeFieldQueryTypeContains matches documents whose range fully contains
	// the query range.
	RangeFieldQueryTypeContains
	// RangeFieldQueryTypeWithin matches documents whose range is fully contained
	// by the query range.
	RangeFieldQueryTypeWithin
	// RangeFieldQueryTypeCrosses matches documents whose range partially overlaps
	// the query range (intersects but does not contain or equal the query).
	RangeFieldQueryTypeCrosses
)

// NewRangeFieldQuery creates a RangeFieldQuery.
//
// queryMin and queryMax must each be numDims*bytesPerDim bytes long.  The
// legacy two-argument form (no numDims/bytesPerDim) sets both to 0,
// disabling BKD-tree intersection; callers that need real intersection should
// use NewRangeFieldQueryFull.
func NewRangeFieldQuery(field string, queryMin, queryMax []byte, queryType RangeFieldQueryType) *RangeFieldQuery {
	return &RangeFieldQuery{
		field:     field,
		queryMin:  queryMin,
		queryMax:  queryMax,
		queryType: queryType,
	}
}

// NewRangeFieldQueryFull creates a RangeFieldQuery with full dimension metadata.
//
// queryMin and queryMax are each numDims*bytesPerDim bytes wide (one encoded
// value per dimension).  bytesPerDim must be in [1, 16].  numDims must be in
// [1, 4], matching the Lucene constraint.
func NewRangeFieldQueryFull(
	field string,
	queryMin, queryMax []byte,
	numDims, bytesPerDim int,
	queryType RangeFieldQueryType,
) (*RangeFieldQuery, error) {
	if field == "" {
		return nil, fmt.Errorf("field name cannot be empty")
	}
	if numDims < 1 || numDims > 4 {
		return nil, fmt.Errorf("numDims must be in [1,4]; got %d", numDims)
	}
	if bytesPerDim < 1 {
		return nil, fmt.Errorf("bytesPerDim must be >= 1; got %d", bytesPerDim)
	}
	want := numDims * bytesPerDim
	if len(queryMin) != want || len(queryMax) != want {
		return nil, fmt.Errorf(
			"queryMin and queryMax must be numDims*bytesPerDim=%d bytes; got %d and %d",
			want, len(queryMin), len(queryMax),
		)
	}
	minCopy := make([]byte, want)
	copy(minCopy, queryMin)
	maxCopy := make([]byte, want)
	copy(maxCopy, queryMax)
	return &RangeFieldQuery{
		field:       field,
		queryMin:    minCopy,
		queryMax:    maxCopy,
		numDims:     numDims,
		bytesPerDim: bytesPerDim,
		queryType:   queryType,
	}, nil
}

// Field returns the target field name.
func (q *RangeFieldQuery) Field() string { return q.field }

// QueryMin returns the query minimum packed value.
func (q *RangeFieldQuery) QueryMin() []byte { return q.queryMin }

// QueryMax returns the query maximum packed value.
func (q *RangeFieldQuery) QueryMax() []byte { return q.queryMax }

// NumDims returns the number of range dimensions (0 if unset).
func (q *RangeFieldQuery) NumDims() int { return q.numDims }

// BytesPerDim returns the byte width of one dimension value (0 if unset).
func (q *RangeFieldQuery) BytesPerDim() int { return q.bytesPerDim }

// QueryType returns the spatial-relation type.
func (q *RangeFieldQuery) QueryType() RangeFieldQueryType { return q.queryType }

// Rewrite returns q unchanged.
func (q *RangeFieldQuery) Rewrite(_ IndexReader) (Query, error) { return q, nil }

// CreateWeight builds a ConstantScoreWeight that uses BKD-tree intersection.
//
// If numDims or bytesPerDim are 0 (legacy constructor), the weight falls
// back to matching no documents rather than panicking.
//
// Port of RangeFieldQuery.createWeight (Lucene 10.4.0).
func (q *RangeFieldQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	if q.numDims == 0 || q.bytesPerDim == 0 {
		// No dimension metadata → return empty weight.
		return NewConstantScoreWeight(q, boost, func(_ *index.LeafReaderContext) (ScorerSupplier, error) {
			return nil, nil
		}, nil), nil
	}

	// Build a packed ranges payload matching Lucene's layout:
	// [min0..minN-1, max0..maxN-1] where each slice is bytesPerDim wide.
	ranges := make([]byte, 2*q.numDims*q.bytesPerDim)
	copy(ranges[:q.numDims*q.bytesPerDim], q.queryMin)
	copy(ranges[q.numDims*q.bytesPerDim:], q.queryMax)

	comparator := bkd.GetUnsignedComparator(q.bytesPerDim)
	numDims := q.numDims
	bytesPerDim := q.bytesPerDim
	queryType := q.queryType

	supplier := func(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
		reader := ctx.LeafReader()
		if reader == nil {
			return nil, nil
		}
		pv, ok := getRangeFieldPointValues(reader, q.field)
		if !ok || pv == nil {
			return nil, nil
		}

		maxDoc := reader.MaxDoc()

		// Fast path: all docs match → wrap a dense iterator.
		allDocsMatch := false
		if pv.GetDocCount() == maxDoc {
			minPV, err1 := pv.GetMinPackedValue()
			maxPV, err2 := pv.GetMaxPackedValue()
			if err1 == nil && err2 == nil {
				rel := rfqCompare(queryType, ranges, minPV, maxPV,
					numDims, bytesPerDim, comparator)
				if rel == rfqRelCellInside {
					allDocsMatch = true
				}
			}
		}

		if allDocsMatch {
			disi := newRangeDocIdSetIterator(maxDoc)
			return NewScorerSupplierAdapter(NewConstantScoreScorer(boost, COMPLETE, disi)), nil
		}

		// Full intersection via BKD tree.
		return &rangeFieldScorerSupplier{
			pv:          pv,
			ranges:      ranges,
			numDims:     numDims,
			bytesPerDim: bytesPerDim,
			queryType:   queryType,
			comparator:  comparator,
			boost:       boost,
			maxDoc:      maxDoc,
			estCost:     -1,
		}, nil
	}

	return NewConstantScoreWeight(q, boost, supplier, nil), nil
}

// rangeFieldScorerSupplier is the lazy ScorerSupplier produced per leaf.
// It defers BKD intersection until Get is called, matching Lucene's lazy
// DocIdSetBuilder pattern.
type rangeFieldScorerSupplier struct {
	BaseScorerSupplier
	pv          rangeFieldPointValues
	ranges      []byte
	numDims     int
	bytesPerDim int
	queryType   RangeFieldQueryType
	comparator  bkd.ByteArrayComparator
	boost       float32
	maxDoc      int
	estCost     int64
}

func (s *rangeFieldScorerSupplier) Get(_ int64) (Scorer, error) {
	builder := util.NewDocIdSetBuilder(s.maxDoc)
	v := &rangeFieldIntersectVisitor{
		ranges:      s.ranges,
		numDims:     s.numDims,
		bytesPerDim: s.bytesPerDim,
		queryType:   s.queryType,
		comparator:  s.comparator,
		builder:     builder,
	}
	if err := s.pv.Intersect(v); err != nil {
		return nil, err
	}
	docSet, err := builder.Build()
	if err != nil {
		return nil, err
	}
	iter := docSet.Iterator()
	if iter == nil {
		return nil, nil
	}
	return NewConstantScoreScorer(s.boost, COMPLETE, newUtilDocIdSetIteratorAdapter(iter)), nil
}

func (s *rangeFieldScorerSupplier) Cost() int64 {
	if s.estCost < 0 {
		// estimate: use point count from pv
		s.estCost = s.pv.EstimatePointCount(&rangeFieldIntersectVisitor{
			ranges:      s.ranges,
			numDims:     s.numDims,
			bytesPerDim: s.bytesPerDim,
			queryType:   s.queryType,
			comparator:  s.comparator,
		})
		if s.estCost < 0 {
			s.estCost = 0
		}
	}
	return s.estCost
}

// rangeFieldIntersectVisitor implements the codecs.IntersectVisitor shim for
// range field intersection.  It does NOT import codecs (would create a cycle
// through codecs/lucene90); instead rangeFieldPointValues uses a locally
// defined interface whose method signatures match exactly.
type rangeFieldIntersectVisitor struct {
	ranges      []byte
	numDims     int
	bytesPerDim int
	queryType   RangeFieldQueryType
	comparator  bkd.ByteArrayComparator
	builder     *util.DocIdSetBuilder
	adder       util.BulkAdder
}

// Grow satisfies the intersectVisitorShim.Grow signature.
func (v *rangeFieldIntersectVisitor) Grow(count int) {
	if v.builder != nil {
		v.adder = v.builder.Grow(count)
	}
}

// Visit adds docID unconditionally (called when the cell is fully inside).
func (v *rangeFieldIntersectVisitor) Visit(docID int) error {
	if v.adder != nil {
		v.adder.Add(docID)
	}
	return nil
}

// VisitByPackedValue adds docID only when the range matches the query.
func (v *rangeFieldIntersectVisitor) VisitByPackedValue(docID int, packedValue []byte) error {
	if rfqMatches(v.queryType, v.ranges, packedValue, v.numDims, v.bytesPerDim, v.comparator) {
		if v.adder != nil {
			v.adder.Add(docID)
		}
	}
	return nil
}

// Compare returns the relation for BKD pruning.  Return values match
// codecs.Relation: 0=outside, 1=inside, 2=crosses.
func (v *rangeFieldIntersectVisitor) Compare(min, max []byte) int {
	return int(rfqCompare(v.queryType, v.ranges, min, max, v.numDims, v.bytesPerDim, v.comparator))
}

// rangeFieldPointValues is the narrow interface this package requires from
// a BKD-tree point-values reader.  GetMinPackedValue / GetMaxPackedValue match
// the index.PointValues signatures (error-returning) so that a concrete type
// can satisfy both index.PointValues (metadata) and this extended interface
// (intersection) without signature conflicts.
type rangeFieldPointValues interface {
	Intersect(visitor intersectVisitorRFQ) error
	EstimatePointCount(visitor intersectVisitorRFQ) int64
	GetMinPackedValue() ([]byte, error)
	GetMaxPackedValue() ([]byte, error)
	GetNumDimensions() int
	GetBytesPerDimension() int
	GetDocCount() int
}

// intersectVisitorRFQ is the visitor shape expected by rangeFieldPointValues.
// Its method set matches codecs.IntersectVisitor, but the Compare return type
// is int (not codecs.Relation) to avoid importing codecs.
type intersectVisitorRFQ interface {
	Visit(docID int) error
	VisitByPackedValue(docID int, packedValue []byte) error
	Compare(minPackedValue, maxPackedValue []byte) int
	Grow(count int)
}

// getRangeFieldPointValues type-asserts reader to rangeFieldPointValues for
// the given field.  Returns (nil, false) if the reader does not expose this
// surface for the field.
func getRangeFieldPointValues(reader index.LeafReaderInterface, field string) (rangeFieldPointValues, bool) {
	type pointValuesProvider interface {
		GetPointValues(field string) (index.PointValues, error)
	}
	pvp, ok := reader.(pointValuesProvider)
	if !ok {
		return nil, false
	}
	raw, err := pvp.GetPointValues(field)
	if err != nil || raw == nil {
		return nil, false
	}
	// index.PointValues (in doc_values_interfaces.go) has only metadata methods;
	// codecs.PointValues additionally has Intersect / EstimatePointCount.
	// We cast to our local rangeFieldPointValues interface so search/ stays
	// independent of codecs/.
	pv, ok := raw.(rangeFieldPointValues)
	return pv, ok
}

// ── range relation constants ─────────────────────────────────────────────────

const (
	rfqRelCellOutside = 0
	rfqRelCellInside  = 1
	rfqRelCellCrosses = 2
)

// rfqCompare computes the BKD-pruning relation for a cell [minPV, maxPV]
// against the query ranges payload.  Mirrors QueryType.compare (per-dim loop)
// in Lucene 10.4.0.
func rfqCompare(qType RangeFieldQueryType, ranges, minPV, maxPV []byte, numDims, bytesPerDim int, cmp bkd.ByteArrayComparator) int {
	if qType == RangeFieldQueryTypeCrosses {
		// CROSSES = INTERSECTS AND NOT WITHIN
		iRel := rfqCompare(RangeFieldQueryTypeIntersects, ranges, minPV, maxPV, numDims, bytesPerDim, cmp)
		if iRel == rfqRelCellOutside {
			return rfqRelCellOutside
		}
		wRel := rfqCompare(RangeFieldQueryTypeWithin, ranges, minPV, maxPV, numDims, bytesPerDim, cmp)
		if wRel == rfqRelCellInside {
			return rfqRelCellOutside
		}
		if iRel == rfqRelCellInside && wRel == rfqRelCellOutside {
			return rfqRelCellInside
		}
		return rfqRelCellCrosses
	}

	inside := true
	for dim := 0; dim < numDims; dim++ {
		rel := rfqCompareDim(qType, ranges, minPV, maxPV, numDims, bytesPerDim, dim, cmp)
		if rel == rfqRelCellOutside {
			return rfqRelCellOutside
		}
		if rel != rfqRelCellInside {
			inside = false
		}
	}
	if inside {
		return rfqRelCellInside
	}
	return rfqRelCellCrosses
}

// rfqCompareDim computes the single-dimension BKD relation.
// Mirrors the abstract QueryType.compare(dim) in Lucene.
//
// The ranges payload layout: [min0..minN-1, max0..maxN-1], each entry
// bytesPerDim wide.  So for dimension dim:
//
//	minOffset = dim * bytesPerDim   (into ranges)
//	maxOffset = (dim + numDims) * bytesPerDim
func rfqCompareDim(qType RangeFieldQueryType, ranges, minPV, maxPV []byte, numDims, bytesPerDim, dim int, cmp bkd.ByteArrayComparator) int {
	minOffset := dim * bytesPerDim
	maxOffset := (dim + numDims) * bytesPerDim

	switch qType {
	case RangeFieldQueryTypeIntersects:
		// cell is outside if qMax < cellMin OR qMin > cellMax
		if cmp(ranges, maxOffset, minPV, minOffset) < 0 ||
			cmp(ranges, minOffset, maxPV, maxOffset) > 0 {
			return rfqRelCellOutside
		}
		// cell is inside if qMax >= cellMax AND qMin <= cellMin
		if cmp(ranges, maxOffset, maxPV, minOffset) >= 0 &&
			cmp(ranges, minOffset, minPV, maxOffset) <= 0 {
			return rfqRelCellInside
		}
		return rfqRelCellCrosses

	case RangeFieldQueryTypeWithin:
		// all ranges must be at least one point outside: qMax < cellMax OR qMin > cellMin
		if cmp(ranges, maxOffset, minPV, maxOffset) < 0 ||
			cmp(ranges, minOffset, maxPV, minOffset) > 0 {
			return rfqRelCellOutside
		}
		// all ranges are within: qMax >= cellMax AND qMin <= cellMin
		if cmp(ranges, maxOffset, maxPV, maxOffset) >= 0 &&
			cmp(ranges, minOffset, minPV, minOffset) <= 0 {
			return rfqRelCellInside
		}
		return rfqRelCellCrosses

	case RangeFieldQueryTypeContains:
		// all ranges are either < qMax or > qMin
		if cmp(ranges, maxOffset, maxPV, maxOffset) > 0 ||
			cmp(ranges, minOffset, minPV, minOffset) < 0 {
			return rfqRelCellOutside
		}
		// all ranges contain: qMax <= cellMax AND qMin >= cellMin
		if cmp(ranges, maxOffset, minPV, maxOffset) <= 0 &&
			cmp(ranges, minOffset, maxPV, minOffset) >= 0 {
			return rfqRelCellInside
		}
		return rfqRelCellCrosses

	default:
		return rfqRelCellCrosses
	}
}

// rfqMatches returns true when a single document's packed range satisfies the
// query type against the query ranges payload.  Called from
// rangeFieldIntersectVisitor.VisitByPackedValue.
//
// Mirrors QueryType.matches(queryPackedValue, packedValue, numDims, bytesPerDim, comparator).
func rfqMatches(qType RangeFieldQueryType, ranges, packedValue []byte, numDims, bytesPerDim int, cmp bkd.ByteArrayComparator) bool {
	if qType == RangeFieldQueryTypeCrosses {
		return rfqMatches(RangeFieldQueryTypeIntersects, ranges, packedValue, numDims, bytesPerDim, cmp) &&
			!rfqMatches(RangeFieldQueryTypeWithin, ranges, packedValue, numDims, bytesPerDim, cmp)
	}
	for dim := 0; dim < numDims; dim++ {
		if !rfqMatchesDim(qType, ranges, packedValue, numDims, bytesPerDim, dim, cmp) {
			return false
		}
	}
	return true
}

// rfqMatchesDim checks one dimension for the point-level match.
func rfqMatchesDim(qType RangeFieldQueryType, ranges, packedValue []byte, numDims, bytesPerDim, dim int, cmp bkd.ByteArrayComparator) bool {
	minOffset := dim * bytesPerDim
	maxOffset := (dim + numDims) * bytesPerDim

	switch qType {
	case RangeFieldQueryTypeIntersects:
		// qMax >= docMin AND qMin <= docMax
		return cmp(ranges, maxOffset, packedValue, minOffset) >= 0 &&
			cmp(ranges, minOffset, packedValue, maxOffset) <= 0

	case RangeFieldQueryTypeWithin:
		// qMin <= docMin AND qMax >= docMax
		return cmp(ranges, minOffset, packedValue, minOffset) <= 0 &&
			cmp(ranges, maxOffset, packedValue, maxOffset) >= 0

	case RangeFieldQueryTypeContains:
		// qMin >= docMin AND qMax <= docMax
		return cmp(ranges, minOffset, packedValue, minOffset) >= 0 &&
			cmp(ranges, maxOffset, packedValue, maxOffset) <= 0

	default:
		return false
	}
}

// newRangeDocIdSetIterator returns a dense DocIdSetIterator covering [0, maxDoc).
func newRangeDocIdSetIterator(maxDoc int) DocIdSetIterator {
	return NewRangeDocIdSetIterator(0, maxDoc)
}

// Clone returns a copy of the query.
func (q *RangeFieldQuery) Clone() Query {
	minCopy := make([]byte, len(q.queryMin))
	copy(minCopy, q.queryMin)
	maxCopy := make([]byte, len(q.queryMax))
	copy(maxCopy, q.queryMax)
	return &RangeFieldQuery{
		field:       q.field,
		queryMin:    minCopy,
		queryMax:    maxCopy,
		numDims:     q.numDims,
		bytesPerDim: q.bytesPerDim,
		queryType:   q.queryType,
	}
}

// Equals reports structural equality.
func (q *RangeFieldQuery) Equals(other Query) bool {
	o, ok := other.(*RangeFieldQuery)
	if !ok {
		return false
	}
	if q.field != o.field || q.queryType != o.queryType ||
		q.numDims != o.numDims || q.bytesPerDim != o.bytesPerDim {
		return false
	}
	if len(q.queryMin) != len(o.queryMin) || len(q.queryMax) != len(o.queryMax) {
		return false
	}
	for i := range q.queryMin {
		if q.queryMin[i] != o.queryMin[i] {
			return false
		}
	}
	for i := range q.queryMax {
		if q.queryMax[i] != o.queryMax[i] {
			return false
		}
	}
	return true
}

// HashCode returns a hash code.
func (q *RangeFieldQuery) HashCode() int {
	h := 17
	h = 31*h + len(q.field)
	for i := 0; i < len(q.field); i++ {
		h = 31*h + int(q.field[i])
	}
	for _, b := range q.queryMin {
		h = 31*h + int(b)
	}
	for _, b := range q.queryMax {
		h = 31*h + int(b)
	}
	h = 31*h + int(q.queryType)
	h = 31*h + q.numDims
	h = 31*h + q.bytesPerDim
	return h
}

// String returns a human-readable representation.
func (q *RangeFieldQuery) String(field string) string {
	if field != "" && field == q.field {
		return fmt.Sprintf("RangeFieldQuery(type=%v)", q.queryType)
	}
	return fmt.Sprintf("RangeFieldQuery(field=%s, type=%v)", q.field, q.queryType)
}

// Ensure RangeFieldQuery implements Query.
var _ Query = (*RangeFieldQuery)(nil)
