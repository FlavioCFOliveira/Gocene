// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/spi"

	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/bkd"
)

// PointRangeQuery is a query that matches documents containing points
// within a specified range. This is used for numeric range queries
// on fields indexed using point values (e.g., IntPoint, LongPoint).
//
// This is the Go port of Lucene's org.apache.lucene.search.PointRangeQuery.
type PointRangeQuery struct {
	*BaseQuery
	field       string
	lowerValue  []byte
	upperValue  []byte
	numDims     int
	bytesPerDim int
}

// NewPointRangeQuery creates a new PointRangeQuery.
//
// Parameters:
//   - field: The field name
//   - lowerValue: The lower bound (inclusive), or nil for unbounded
//   - upperValue: The upper bound (inclusive), or nil for unbounded
//
// Returns an error if the values are invalid.
func NewPointRangeQuery(field string, lowerValue, upperValue []byte) (*PointRangeQuery, error) {
	if len(lowerValue) != len(upperValue) {
		return nil, fmt.Errorf("lower and upper values must have same length")
	}

	return &PointRangeQuery{
		BaseQuery:   &BaseQuery{},
		field:       field,
		lowerValue:  lowerValue,
		upperValue:  upperValue,
		numDims:     1,
		bytesPerDim: len(lowerValue),
	}, nil
}

// NewPointRangeQueryMultiDim creates a new PointRangeQuery for multi-dimensional points.
//
// Parameters:
//   - field: The field name
//   - lowerValue: The lower bound (inclusive) for each dimension
//   - upperValue: The upper bound (inclusive) for each dimension
//   - numDims: The number of dimensions
//
// Returns an error if the values are invalid.
func NewPointRangeQueryMultiDim(field string, lowerValue, upperValue []byte, numDims int) (*PointRangeQuery, error) {
	if len(lowerValue) != len(upperValue) {
		return nil, fmt.Errorf("lower and upper values must have same length")
	}
	if len(lowerValue)%numDims != 0 {
		return nil, fmt.Errorf("value length must be divisible by numDims")
	}

	return &PointRangeQuery{
		BaseQuery:   &BaseQuery{},
		field:       field,
		lowerValue:  lowerValue,
		upperValue:  upperValue,
		numDims:     numDims,
		bytesPerDim: len(lowerValue) / numDims,
	}, nil
}

// Field returns the field name.
func (q *PointRangeQuery) Field() string {
	return q.field
}

// LowerValue returns the lower bound value.
func (q *PointRangeQuery) LowerValue() []byte {
	return q.lowerValue
}

// UpperValue returns the upper bound value.
func (q *PointRangeQuery) UpperValue() []byte {
	return q.upperValue
}

// NumDims returns the number of dimensions.
func (q *PointRangeQuery) NumDims() int {
	return q.numDims
}

// BytesPerDim returns the number of bytes per dimension.
func (q *PointRangeQuery) BytesPerDim() int {
	return q.bytesPerDim
}

// Equals checks if this query equals another.
func (q *PointRangeQuery) Equals(other spi.Query) bool {
	if o, ok := other.(*PointRangeQuery); ok {
		if q.field != o.field || q.numDims != o.numDims || q.bytesPerDim != o.bytesPerDim {
			return false
		}
		if len(q.lowerValue) != len(o.lowerValue) || len(q.upperValue) != len(o.upperValue) {
			return false
		}
		for i := range q.lowerValue {
			if q.lowerValue[i] != o.lowerValue[i] {
				return false
			}
		}
		for i := range q.upperValue {
			if q.upperValue[i] != o.upperValue[i] {
				return false
			}
		}
		return true
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *PointRangeQuery) HashCode() int {
	hash := 0
	for _, c := range q.field {
		hash = hash*31 + int(c)
	}
	for _, b := range q.lowerValue {
		hash = hash*31 + int(b)
	}
	for _, b := range q.upperValue {
		hash = hash*31 + int(b)
	}
	hash = hash*31 + q.numDims
	hash = hash*31 + q.bytesPerDim
	return hash
}

// Rewrite rewrites the query to a simpler form.
func (q *PointRangeQuery) Rewrite(searcher *IndexSearcher) (Query, error) {
	// For now, return itself
	// A full implementation would potentially rewrite to MatchAllDocsQuery
	// if the range covers all possible values
	return q, nil
}

// CreateWeight creates a Weight for this query.
func (q *PointRangeQuery) CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	return NewPointRangeWeight(q, searcher, scoreMode.NeedsScores()), nil
}

// String returns a string representation of this query.
func (q *PointRangeQuery) String() string {
	return fmt.Sprintf("%s:[%v TO %v]", q.field, q.lowerValue, q.upperValue)
}

// Ensure PointRangeQuery implements Query
var _ Query = (*PointRangeQuery)(nil)

// PointRangeWeight is the Weight implementation for PointRangeQuery.
type PointRangeWeight struct {
	*BaseWeight
	query       *PointRangeQuery
	searcher    *IndexSearcher
	needsScores bool
}

// NewPointRangeWeight creates a new PointRangeWeight.
func NewPointRangeWeight(query *PointRangeQuery, searcher *IndexSearcher, needsScores bool) *PointRangeWeight {
	return &PointRangeWeight{
		BaseWeight:  NewBaseWeight(query),
		query:       query,
		searcher:    searcher,
		needsScores: needsScores,
	}
}

// ScorerSupplier creates a lazy ScorerSupplier for BKD-tree intersection.
//
// Port of PointRangeQuery.createWeight().scorerSupplier (Lucene 10.4.0).
// When the leaf reader exposes a codecs-level PointValues (via the
// pointValuesProvider capability interface), real BKD intersection is
// performed.  Otherwise the weight falls back to "no matches".
func (w *PointRangeWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	reader := context.LeafReader()
	if reader == nil {
		return nil, nil
	}

	pv, ok := getPointRangePointValues(reader, w.query.field)
	if !ok || pv == nil || pv.GetDocCount() == 0 {
		return nil, nil
	}

	q := w.query
	cmp := bkd.GetUnsignedComparator(q.bytesPerDim)

	fieldMin, err := pv.GetMinPackedValue()
	if err != nil {
		return nil, err
	}
	fieldMax, err := pv.GetMaxPackedValue()
	if err != nil {
		return nil, err
	}

	// Prune: if the query range is entirely outside the field's packed range,
	// no documents can match.
	for i := 0; i < q.numDims; i++ {
		off := i * q.bytesPerDim
		if cmp(q.lowerValue, off, fieldMax, off) > 0 ||
			cmp(q.upperValue, off, fieldMin, off) < 0 {
			return nil, nil
		}
	}

	maxDoc := reader.MaxDoc()

	// Fast path: all docs match.
	if pv.GetDocCount() == maxDoc {
		allMatch := true
		for i := 0; i < q.numDims; i++ {
			off := i * q.bytesPerDim
			if cmp(q.lowerValue, off, fieldMin, off) > 0 ||
				cmp(q.upperValue, off, fieldMax, off) < 0 {
				allMatch = false
				break
			}
		}
		if allMatch {
			disi := newRangeDocIdSetIterator(maxDoc)
			return NewDefaultScorerSupplier(NewConstantScoreScorer(float32(1.0), COMPLETE, disi)), nil
		}
	}

	return &pointRangeScorerSupplier{
		pv:         pv,
		query:      q,
		comparator: cmp,
		maxDoc:     maxDoc,
		estCost:    -1,
	}, nil
}

// Scorer delegates to ScorerSupplier.Get.
func (w *PointRangeWeight) Scorer(context *index.LeafReaderContext) (Scorer, error) {
	supplier, err := w.ScorerSupplier(context)
	if err != nil || supplier == nil {
		return nil, err
	}
	return supplier.Get(0)
}

// pointRangeScorerSupplier is the lazy supplier for PointRangeWeight.
type pointRangeScorerSupplier struct {
	BaseScorerSupplier
	pv         pointRangePointValues
	query      *PointRangeQuery
	comparator bkd.ByteArrayComparator
	maxDoc     int
	estCost    int64
}

func (s *pointRangeScorerSupplier) Get(_ int64) (Scorer, error) {
	builder := util.NewDocIdSetBuilder(s.maxDoc)
	v := &pointRangeIntersectVisitor{
		query:      s.query,
		comparator: s.comparator,
		builder:    builder,
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
	return NewConstantScoreScorer(float32(1.0), COMPLETE, newUtilDocIdSetIteratorAdapter(iter)), nil
}

func (s *pointRangeScorerSupplier) Cost() int64 {
	if s.estCost < 0 {
		// Java: cost = values.estimateDocCount(visitor)
		// (PointRangeQuery.java:330). estimateDocCount wraps an IOException in
		// an UncheckedIOException, which crosses cost()'s throws-free
		// signature; Go renders that unchecked throw as a panic.
		estCost, err := s.pv.EstimateDocCount(&pointRangeIntersectVisitor{
			query:      s.query,
			comparator: s.comparator,
		})
		if err != nil {
			panic(err)
		}
		s.estCost = estCost
		if s.estCost < 0 {
			s.estCost = 0
		}
	}
	return s.estCost
}

// pointRangeIntersectVisitor drives BKD intersection for PointRangeQuery.
type pointRangeIntersectVisitor struct {
	query      *PointRangeQuery
	comparator bkd.ByteArrayComparator
	builder    *util.DocIdSetBuilder
	adder      util.BulkAdder
}

func (v *pointRangeIntersectVisitor) Grow(count int) {
	if v.builder != nil {
		v.adder = v.builder.Grow(count)
	}
}

func (v *pointRangeIntersectVisitor) Visit(docID int) error {
	if v.adder != nil {
		v.adder.Add(docID)
	}
	return nil
}

func (v *pointRangeIntersectVisitor) VisitByPackedValue(docID int, packedValue []byte) error {
	if v.matchesPoint(packedValue) {
		if v.adder != nil {
			v.adder.Add(docID)
		}
	}
	return nil
}

// matchesPoint returns true when packedValue is within [lowerValue, upperValue]
// across all dimensions.
func (v *pointRangeIntersectVisitor) matchesPoint(packed []byte) bool {
	q := v.query
	for dim := 0; dim < q.numDims; dim++ {
		off := dim * q.bytesPerDim
		if v.comparator(packed, off, q.lowerValue, off) < 0 ||
			v.comparator(packed, off, q.upperValue, off) > 0 {
			return false
		}
	}
	return true
}

// Compare returns the BKD pruning relation for a cell.
// Returns 0=outside, 1=inside, 2=crosses (matching geo.Relation order).
func (v *pointRangeIntersectVisitor) Compare(minPV, maxPV []byte) geo.Relation {
	q := v.query
	inside := true
	for dim := 0; dim < q.numDims; dim++ {
		off := dim * q.bytesPerDim
		// outside: lower > cellMax OR upper < cellMin
		if v.comparator(q.lowerValue, off, maxPV, off) > 0 ||
			v.comparator(q.upperValue, off, minPV, off) < 0 {
			return 0 // CELL_OUTSIDE_QUERY
		}
		// partially outside: lower <= cellMin AND upper >= cellMax?
		if v.comparator(q.lowerValue, off, minPV, off) > 0 ||
			v.comparator(q.upperValue, off, maxPV, off) < 0 {
			inside = false
		}
	}
	if inside {
		return 1 // CELL_INSIDE_QUERY
	}
	return 2 // CELL_CROSSES_QUERY
}

// pointRangePointValues is the narrow interface required by PointRangeWeight.
// Declared locally to avoid importing codecs (cycle through codecs/lucene90).
// GetMinPackedValue / GetMaxPackedValue match index.PointValues signatures
// (error-returning) so a concrete type can implement both without conflicts.
// pointRangePointValues is an alias of index.PointValues. Before the two
// PointValues renderings were merged it was a narrow structural interface
// carrying the visitor-driven surface (Intersect / EstimatePointCount) that
// index.PointValues did not declare; org.apache.lucene.index.PointValues
// declares intersect and estimatePointCount as public final members, so the
// whole surface is now on the one interface and the narrow duplicate has no
// Lucene counterpart.
type pointRangePointValues = index.PointValues

// pointRangeIntersectVisitorI is the visitor shape for pointRangePointValues.
// It is an alias of index.IntersectVisitor (rmp #4769) so the
// on-disk BKD-backed PointValues returned by LeafReader.GetPointValues — whose
// Intersect method takes index.IntersectVisitor — satisfies
// pointRangePointValues. Without the alias the method-parameter type identity
// would differ and the type assertion in getPointRangePointValues would fail
// for the real codec reader (only in-package stubs would match).
type pointRangeIntersectVisitorI = index.IntersectVisitor

// getPointRangePointValues type-asserts the leaf reader to expose BKD point values.
func getPointRangePointValues(reader index.LeafReaderInterface, field string) (pointRangePointValues, bool) {
	type pvProvider interface {
		GetPointValues(field string) (index.PointValues, error)
	}
	pvp, ok := reader.(pvProvider)
	if !ok {
		return nil, false
	}
	raw, err := pvp.GetPointValues(field)
	if err != nil || raw == nil {
		return nil, false
	}
	pv, ok := raw.(pointRangePointValues)
	return pv, ok
}

// Explain returns an explanation of the score for the given document.
//
// PointRangeQuery is a constant-score query, so this ports
// org.apache.lucene.search.ConstantScoreWeight.explain: pull a Scorer and
// advance to doc; a hit yields a match valued at the constant score with the
// query string as the description, and a miss yields
// "<query> doesn't match id <doc>". The value is taken from the live Scorer so
// it equals the scored value.
func (w *PointRangeWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	matched, score, err := scorerMatch(w, context, doc)
	if err != nil {
		return nil, err
	}
	if matched {
		desc := w.query.String()
		if score != 1.0 {
			desc = fmt.Sprintf("%s^%v", desc, score)
		}
		return MatchExplanation(score, desc), nil
	}
	return NoMatchExplanation(fmt.Sprintf("%s doesn't match id %d", w.query, doc)), nil
}

// BulkScorer creates a bulk scorer for efficient bulk scoring.
func (w *PointRangeWeight) BulkScorer(context *index.LeafReaderContext) (BulkScorer, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return NewDefaultBulkScorer(scorer), nil
}

// IsCacheable returns true if this weight can be cached for the given leaf.
func (w *PointRangeWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return true
}

// Count returns the count of matching documents in sub-linear time.
func (w *PointRangeWeight) Count(context *index.LeafReaderContext) (int, error) {
	return -1, nil
}

// Matches returns the matches for a specific document.
func (w *PointRangeWeight) Matches(context *index.LeafReaderContext, doc int) (Matches, error) {
	return nil, nil
}

// Ensure PointRangeWeight implements Weight
var _ Weight = (*PointRangeWeight)(nil)

// BulkScorer mirrors the concrete body of ScorerSupplier.bulkScorer() in Apache
// Lucene 10.5.0: new DefaultBulkScorer(get(Long.MAX_VALUE)).
func (p *pointRangeScorerSupplier) BulkScorer() (BulkScorer, error) {
	return DefaultScorerSupplierBulkScorer(p)
}

// SetTopLevelScoringClause mirrors ScorerSupplier.setTopLevelScoringClause(),
// whose body in Apache Lucene 10.5.0 is empty.
func (p *pointRangeScorerSupplier) SetTopLevelScoringClause() error {
	return nil
}

// Visit mirrors PointRangeQuery.visit(QueryVisitor) of Apache Lucene 10.5.0
// (PointRangeQuery.java).
func (q *PointRangeQuery) Visit(visitor QueryVisitor) {
	if visitor.AcceptField(q.field) {
		visitor.VisitLeaf(q)
	}
}

// VisitByDocIDSetIterator renders the default body of
// PointValues.IntersectVisitor.visit(DocIdSetIterator), which v does not
// override.
func (v *pointRangeIntersectVisitor) VisitByDocIDSetIterator(iterator spi.DocIdSetIterator) error {
	return spi.DefaultVisitByDocIDSetIterator(v, iterator)
}

// VisitByIntsRef renders the default body of
// PointValues.IntersectVisitor.visit(IntsRef), which v does not override.
func (v *pointRangeIntersectVisitor) VisitByIntsRef(ref *util.IntsRef) error {
	return spi.DefaultVisitByIntsRef(v, ref)
}

// VisitByDocIDSetIteratorAndPackedValue renders the default body of
// PointValues.IntersectVisitor.visit(DocIdSetIterator, byte[]), which v
// does not override.
func (v *pointRangeIntersectVisitor) VisitByDocIDSetIteratorAndPackedValue(iterator spi.DocIdSetIterator, packedValue []byte) error {
	return spi.DefaultVisitByDocIDSetIteratorAndPackedValue(v, iterator, packedValue)
}
