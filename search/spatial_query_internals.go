// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SpatialVisitor is the visitor a concrete shape query supplies to
// SpatialQuery. It mirrors the abstract nested class
// org.apache.lucene.document.SpatialQuery.SpatialVisitor (Lucene
// 10.4.0): a bundle of three callbacks (relate, intersects, within,
// contains) that the parent uses to walk the BKD tree and decide
// per-doc inclusion.
//
// # Why an interface instead of a struct
//
// The Java reference is an abstract class with four abstract
// methods. Gocene models it as an interface so concrete subclasses
// (LatLonShapeQuery, XYShapeQuery, etc.) can implement it with
// their own backing data without the awkwardness of struct
// embedding. The two helper methods (GetInnerFunction,
// GetLeafPredicate) are provided as package-private helpers
// (innerFunctionFor, leafPredicateFor) so every implementation
// gets the same dispatch shape without re-implementing it.
//
// The visitor returned by a query's spatialVisitorFactory must be
// safe for concurrent use across leaves: SpatialQuery captures it
// once in CreateWeight and reuses it for every per-leaf
// ScorerSupplier invocation. Implementations are expected to be
// stateless.
type SpatialVisitor interface {
	// Relate returns the relation between the supplied cell
	// [minPackedValue, maxPackedValue] and the query geometry.
	// Mirrors abstract SpatialVisitor.relate.
	Relate(minPackedValue, maxPackedValue []byte) spatialRelation

	// Intersects returns the per-doc predicate that decides whether
	// a packed value contributes to an INTERSECTS / DISJOINT query.
	// Mirrors abstract SpatialVisitor.intersects.
	Intersects() func(packedValue []byte) bool

	// Within returns the per-doc predicate that decides whether a
	// packed value contributes to a WITHIN query.
	// Mirrors abstract SpatialVisitor.within.
	Within() func(packedValue []byte) bool

	// Contains returns the per-doc function that classifies a
	// packed value against the query geometry for the CONTAINS
	// branch. Mirrors abstract SpatialVisitor.contains.
	Contains() func(packedValue []byte) geo.WithinRelation

	// GetInnerFunction returns the (min, max) → relation function
	// the SpatialQuery pipeline uses to drive Compare hooks. The
	// returned closure transposes the Relate result for DISJOINT
	// queries and forwards as-is for the other relations.
	// Mirrors private SpatialVisitor.getInnerFunction.
	GetInnerFunction(queryRelation document.QueryRelation) func(min, max []byte) spatialRelation

	// GetLeafPredicate returns the per-doc predicate the pipeline
	// uses on packed values. The predicate routes INTERSECTS to
	// Intersects, WITHIN to Within, DISJOINT to !Intersects, and
	// CONTAINS to a wrapper that interprets the Contains result.
	// Mirrors private SpatialVisitor.getLeafPredicate.
	GetLeafPredicate(queryRelation document.QueryRelation) func(packedValue []byte) bool
}

// BaseSpatialVisitor is a partial SpatialVisitor implementation
// concrete subclasses can embed. It supplies the two helper
// methods (GetInnerFunction / GetLeafPredicate) so subclasses only
// have to implement the four abstract Java hooks (Relate,
// Intersects, Within, Contains).
//
// The base captures its own SpatialVisitor reference at
// construction time so the helper closures route Relate /
// Intersects / Within / Contains through the embedding type's
// overrides — mirroring Java's late-binding dispatch.
type BaseSpatialVisitor struct {
	self SpatialVisitor
}

// NewBaseSpatialVisitor builds a BaseSpatialVisitor that routes
// helper-method dispatches through self. Concrete embedders should
// call NewBaseSpatialVisitor(impl) inside their own constructors
// and store the result in an embedded *BaseSpatialVisitor field.
func NewBaseSpatialVisitor(self SpatialVisitor) *BaseSpatialVisitor {
	return &BaseSpatialVisitor{self: self}
}

// GetInnerFunction routes Relate calls through self so subclass
// overrides win, and transposes the result for DISJOINT queries.
func (b *BaseSpatialVisitor) GetInnerFunction(queryRelation document.QueryRelation) func(min, max []byte) spatialRelation {
	if queryRelation == document.QueryRelationDisjoint {
		return func(min, max []byte) spatialRelation {
			return transposeSpatialRelation(b.self.Relate(min, max))
		}
	}
	return b.self.Relate
}

// GetLeafPredicate returns the per-doc predicate for the four
// supported relations. CONTAINS routes through Contains and
// classifies the result as match iff CANDIDATE; DISJOINT routes
// through Intersects and negates; the other two return the
// matching subclass predicate directly.
func (b *BaseSpatialVisitor) GetLeafPredicate(queryRelation document.QueryRelation) func(packedValue []byte) bool {
	switch queryRelation {
	case document.QueryRelationIntersects:
		return b.self.Intersects()
	case document.QueryRelationWithin:
		return b.self.Within()
	case document.QueryRelationDisjoint:
		intersects := b.self.Intersects()
		return func(packed []byte) bool { return !intersects(packed) }
	case document.QueryRelationContains:
		contains := b.self.Contains()
		return func(packed []byte) bool {
			return contains(packed) == geo.WithinCandidate
		}
	default:
		// Mirrors the Java IllegalArgumentException; surfaced as
		// a panic because the value comes from an enum the
		// SpatialQuery constructor validates.
		panic(fmt.Sprintf("search: unsupported query relation %v", queryRelation))
	}
}

// spatialRelation classifies a BKD cell against the query region.
// It mirrors org.apache.lucene.index.PointValues.Relation; it lives
// in the search package as a private alias so SpatialQuery does not
// have to import codecs/ (which would re-introduce the
// document → search cycle via codecs → document → search).
//
// The three values are deliberately stable across
// codecs.Relation, geo.Relation and this one — adapters between the
// three are pure switches with no semantic difference.
type spatialRelation int

const (
	spatialCellInsideQuery spatialRelation = iota
	spatialCellOutsideQuery
	spatialCellCrossesQuery
)

// String returns a human-readable label, useful for tests and
// diagnostics.
func (r spatialRelation) String() string {
	switch r {
	case spatialCellInsideQuery:
		return "CELL_INSIDE_QUERY"
	case spatialCellOutsideQuery:
		return "CELL_OUTSIDE_QUERY"
	case spatialCellCrossesQuery:
		return "CELL_CROSSES_QUERY"
	default:
		return fmt.Sprintf("spatialRelation(%d)", int(r))
	}
}

// spatialIntersectVisitor is the visitor SpatialQuery hands to a
// spatialPointSource. It mirrors the subset of
// org.apache.lucene.index.PointValues.IntersectVisitor that
// SpatialQuery's seven internal visitor factories actually call.
//
// The five hooks (Visit, VisitWithPackedValue, VisitIterator,
// VisitIteratorWithPackedValue, Compare, Grow) match every visit
// overload the Java reference uses, including the two iterator
// bulks (visit(DocIdSetIterator) and visit(DocIdSetIterator,
// byte[])) and the IntsRef bulk. IntsRef is folded into the
// iterator hook here because every Gocene caller that would emit
// an IntsRef can equally well wrap it in a util.DocIdSetIterator;
// the per-doc fallback in iterator-less sources stays functionally
// identical.
//
// This visitor surface is wider than codecs.IntersectVisitor (which
// only carries Visit + VisitByPackedValue) and intentionally
// matches the xyPointVisitor surface from
// xy_point_in_geometry_query.go so a future unification of point
// visitors can swap both call sites in one go.
type spatialIntersectVisitor interface {
	Visit(docID int) error
	VisitWithPackedValue(docID int, packedValue []byte) error
	VisitIterator(iter util.DocIdSetIterator) error
	VisitIteratorWithPackedValue(iter util.DocIdSetIterator, packedValue []byte) error
	Grow(count int)
	Compare(minPackedValue, maxPackedValue []byte) spatialRelation
}

// spatialPointSource is the visitor-driven point source SpatialQuery
// walks. It mirrors the subset of
// org.apache.lucene.index.PointValues used by SpatialQuery
// (getMin/getMax/getDocCount/size/estimateDocCount/intersect).
//
// The interface is package-private because the canonical
// PointValues port has not landed yet; the noop default matches
// the same pattern xyPointInGeometryQuery uses.
type spatialPointSource interface {
	// GetMinPackedValue returns the segment-wide minimum packed
	// value for the indexed field.
	GetMinPackedValue() []byte

	// GetMaxPackedValue returns the segment-wide maximum packed
	// value for the indexed field.
	GetMaxPackedValue() []byte

	// GetDocCount returns the number of documents in the segment
	// that have at least one indexed value for the field.
	GetDocCount() int

	// SizeAsInt returns the total number of indexed values, capped
	// at math.MaxInt to keep the comparison in
	// SpatialQuery.getScorerSupplier in plain int arithmetic.
	// Mirrors PointValues.size() on indices where the value fits
	// in an int.
	SizeAsInt() int

	// Intersect walks the BKD tree and dispatches each match (and
	// each visited cell) through the visitor's hooks. Returns
	// errCollectionTerminated only when callers actively use that
	// sentinel for early termination.
	Intersect(visitor spatialIntersectVisitor) error

	// EstimateDocCount returns a coarse upper bound on the number
	// of documents the visitor would match. Used by
	// relationScorerSupplier.Cost to size pre-allocations.
	EstimateDocCount(visitor spatialIntersectVisitor) (int64, error)
}

// spatialLeafLookup resolves the per-leaf point source for a
// (ctx, field) pair. Returning a nil source signals "field absent
// from this leaf", matching the Java reference's null check on
// getPointValues / fieldInfos.fieldInfo.
//
// The lookup also returns the FieldInfo and maxDoc, both of which
// SpatialQuery.getScorerSupplier needs to drive its branching.
type spatialLeafLookup func(ctx *index.LeafReaderContext, field string) (
	source spatialPointSource,
	fieldInfo *index.FieldInfo,
	maxDoc int,
	err error,
)

// noopSpatialLeafLookup returns (nil, nil, 0, nil) for every call,
// which SpatialQuery.CreateWeight interprets as "no source for this
// leaf" (the Weight's per-leaf supplier returns nil ScorerSupplier).
// It is retained as an explicit opt-out (via WithSpatialQueryLeafLookup)
// and as the fallback the production lookup degrades to when a leaf does
// not expose a BKD-backed PointValues; it is no longer the default.
//
// Mirrors noopXYPointSource in xy_point_in_geometry_query.go.
func noopSpatialLeafLookup(_ *index.LeafReaderContext, _ string) (
	spatialPointSource, *index.FieldInfo, int, error,
) {
	return nil, nil, 0, nil
}

// defaultSpatialLeafLookup resolves the visitor-driven point source from
// the production LeafReader path: it pulls the field's BKD-backed
// index.PointValues via LeafReader.GetPointValues and, when that reader
// exposes the rich visitor-driven Intersect surface (the codec's
// *pointValues does), adapts it to the spatialPointSource contract the
// SpatialQuery pipeline walks. Returns (nil, nil, 0, nil) — the
// null-Scorer fast path — when the field is unknown to the leaf or the
// reader is metadata-only.
//
// This is the production default for SpatialQuery (and therefore for the
// LatLonPoint / XYPoint polygon & geometry queries built on it). It is
// the SpatialQuery analogue of defaultLatLonPointDistanceLeafLookup and
// defaultXYPointInGeometryLeafLookup.
func defaultSpatialLeafLookup(ctx *index.LeafReaderContext, field string) (
	spatialPointSource, *index.FieldInfo, int, error,
) {
	if ctx == nil {
		return nil, nil, 0, nil
	}
	leaf := ctx.LeafReader()
	if leaf == nil {
		return nil, nil, 0, nil
	}
	type pointReader interface {
		GetPointValues(field string) (index.PointValues, error)
	}
	type fieldInfosProvider interface {
		GetFieldInfos() *index.FieldInfos
	}
	pr, ok := leaf.(pointReader)
	if !ok {
		return nil, nil, 0, nil
	}
	raw, err := pr.GetPointValues(field)
	if err != nil {
		return nil, nil, 0, err
	}
	if raw == nil {
		return nil, nil, 0, nil
	}
	var fi *index.FieldInfo
	if fip, ok := leaf.(fieldInfosProvider); ok {
		if infos := fip.GetFieldInfos(); infos != nil {
			fi = infos.GetByName(field)
		}
	}
	if fi == nil {
		return nil, nil, 0, nil
	}
	source := newSpatialPointSourceFromIndexPointValues(raw)
	if source == nil {
		return nil, nil, 0, nil
	}
	return source, fi, leaf.MaxDoc(), nil
}

// spatialPointTreeIntersect is the rich, visitor-driven read surface a
// BKD-backed PointValues exposes beyond the metadata-only
// index.PointValues. The on-disk reader returned by
// LeafReader.GetPointValues (the codec's *pointValues) satisfies it
// structurally; the parameter type is the index-package alias so the
// type assertion succeeds for the real codec reader.
type spatialPointTreeIntersect interface {
	Intersect(visitor index.PointTreeIntersectVisitor) error
	EstimatePointCount(visitor index.PointTreeIntersectVisitor) int64
	GetMinPackedValue() ([]byte, error)
	GetMaxPackedValue() ([]byte, error)
	GetDocCount() int
	GetValueCount() int64
}

// newSpatialPointSourceFromIndexPointValues adapts a BKD-backed
// index.PointValues to the spatialPointSource contract used by the
// SpatialQuery pipeline. Returns nil when the concrete PointValues does
// not expose the rich visitor-driven surface (e.g. an in-test
// metadata-only stub), so callers fall through to the null-Scorer fast
// path.
func newSpatialPointSourceFromIndexPointValues(pv index.PointValues) spatialPointSource {
	rich, ok := pv.(spatialPointTreeIntersect)
	if !ok {
		return nil
	}
	return &bkdSpatialPointSource{pv: rich}
}

// bkdSpatialPointSource drives a BKD-backed PointValues, translating
// between the SpatialQuery pipeline's spatialIntersectVisitor and the
// index.PointTreeIntersectVisitor the BKD reader expects.
type bkdSpatialPointSource struct {
	pv spatialPointTreeIntersect
}

func (s *bkdSpatialPointSource) GetMinPackedValue() []byte {
	v, err := s.pv.GetMinPackedValue()
	if err != nil {
		return nil
	}
	return v
}

func (s *bkdSpatialPointSource) GetMaxPackedValue() []byte {
	v, err := s.pv.GetMaxPackedValue()
	if err != nil {
		return nil
	}
	return v
}

func (s *bkdSpatialPointSource) GetDocCount() int { return s.pv.GetDocCount() }

// SizeAsInt returns the total number of indexed values, capped at
// math.MaxInt, mirroring PointValues.size() folded into int arithmetic.
func (s *bkdSpatialPointSource) SizeAsInt() int {
	n := s.pv.GetValueCount()
	if n > int64(maxIntForSpatialSize) {
		return maxIntForSpatialSize
	}
	if n < 0 {
		return 0
	}
	return int(n)
}

func (s *bkdSpatialPointSource) Intersect(visitor spatialIntersectVisitor) error {
	return s.pv.Intersect(&spatialVisitorBridge{v: visitor})
}

func (s *bkdSpatialPointSource) EstimateDocCount(visitor spatialIntersectVisitor) (int64, error) {
	return s.pv.EstimatePointCount(&spatialVisitorBridge{v: visitor}), nil
}

// maxIntForSpatialSize is math.MaxInt expressed without importing math
// at this site; it caps PointValues.size() into int range.
const maxIntForSpatialSize = int(^uint(0) >> 1)

// spatialVisitorBridge adapts a spatialIntersectVisitor to the
// index.PointTreeIntersectVisitor surface the BKD reader invokes. The
// reader only drives Visit / VisitByPackedValue / Compare / Grow (the
// bulk-iterator methods on spatialIntersectVisitor are not part of the
// BKD reader's intersect path).
//
// Compare must translate between the two enum orderings: the BKD reader
// uses the codecs.Relation order (0=outside, 1=inside, 2=crosses) while
// search.spatialRelation uses (0=inside, 1=outside, 2=crosses), so the
// conversion is an explicit switch rather than a raw cast.
type spatialVisitorBridge struct {
	v spatialIntersectVisitor
}

func (b *spatialVisitorBridge) Visit(docID int) error { return b.v.Visit(docID) }

func (b *spatialVisitorBridge) VisitByPackedValue(docID int, packedValue []byte) error {
	return b.v.VisitWithPackedValue(docID, packedValue)
}

func (b *spatialVisitorBridge) Compare(minPackedValue, maxPackedValue []byte) int {
	switch b.v.Compare(minPackedValue, maxPackedValue) {
	case spatialCellOutsideQuery:
		return 0 // codecs.RelationCellOutsideQuery
	case spatialCellInsideQuery:
		return 1 // codecs.RelationCellInsideQuery
	default:
		return 2 // codecs.RelationCellCrossesQuery
	}
}

func (b *spatialVisitorBridge) Grow(count int) { b.v.Grow(count) }

var _ index.PointTreeIntersectVisitor = (*spatialVisitorBridge)(nil)

// relationScorerSupplier is the ScorerSupplier returned by the
// fall-through branch of SpatialQuery.getScorerSupplier. It owns
// the per-leaf point source, the SpatialVisitor and the query
// relation; Get dispatches to one of three internal builders
// (sparse / dense / contains-dense) based on the relation and the
// segment's value density.
//
// Mirrors the inner abstract class
// SpatialQuery.RelationScorerSupplier.
type relationScorerSupplier struct {
	source        spatialPointSource
	visitor       SpatialVisitor
	queryRelation document.QueryRelation
	maxDoc        int
	score         float32
	scoreMode     ScoreMode
	cost          int64

	topLevelScoring bool
}

// Get returns the constant-score scorer for the current leaf. The
// scorer dispatch mirrors RelationScorerSupplier.getScorer in the
// Java reference: INTERSECTS → sparse, CONTAINS → contains-dense,
// WITHIN/DISJOINT → dense when there are multivalued docs,
// otherwise sparse.
func (s *relationScorerSupplier) Get(_ int64) (Scorer, error) {
	var iter DocIdSetIterator
	var err error
	switch s.queryRelation {
	case document.QueryRelationIntersects:
		iter, err = s.getSparseIterator()
	case document.QueryRelationContains:
		iter, err = s.getContainsDenseIterator()
	case document.QueryRelationWithin, document.QueryRelationDisjoint:
		if s.source.GetDocCount() == s.source.SizeAsInt() {
			iter, err = s.getSparseIterator()
		} else {
			iter, err = s.getDenseIterator()
		}
	default:
		return nil, fmt.Errorf("search: unsupported query relation %v", s.queryRelation)
	}
	if err != nil {
		return nil, err
	}
	if iter == nil {
		iter = NewEmptyDocIdSetIterator()
	}
	return NewConstantScoreScorer(s.score, s.scoreMode, iter), nil
}

// Cost returns a lazy estimate of the number of matching docs.
// The estimate is cached after the first call because EstimateDocCount
// can be expensive.
func (s *relationScorerSupplier) Cost() int64 {
	if s.cost == -1 {
		visitor := newEstimateVisitor(s.visitor, s.queryRelation)
		c, err := s.source.EstimateDocCount(visitor)
		if err != nil || c < 0 {
			c = 0
		}
		s.cost = c
	}
	return s.cost
}

// SetTopLevelScoringClause is a no-op for this supplier today;
// recorded so callers can inspect it if needed in tests.
func (s *relationScorerSupplier) SetTopLevelScoringClause() {
	s.topLevelScoring = true
}

// getSparseIterator mirrors RelationScorerSupplier.getSparseScorer.
// The three branches (inverse-dense / dense / sparse-builder) are
// preserved verbatim from the Java reference; each builds a
// DocIdSetIterator that the caller wraps in a ConstantScoreScorer.
func (s *relationScorerSupplier) getSparseIterator() (DocIdSetIterator, error) {
	if s.queryRelation == document.QueryRelationDisjoint &&
		s.source.GetDocCount() == s.maxDoc &&
		s.source.GetDocCount() == s.source.SizeAsInt() &&
		s.Cost() > int64(s.maxDoc)/2 {
		result, err := util.NewFixedBitSet(s.maxDoc)
		if err != nil {
			return nil, err
		}
		setAllBits(result, s.maxDoc)
		cost := []int64{int64(s.maxDoc)}
		visitor := newInverseDenseVisitor(s.visitor, s.queryRelation, result, cost)
		if err := s.source.Intersect(visitor); err != nil {
			return nil, err
		}
		return newUtilToSearchDISIAdapter(util.NewBitSetIterator(result, cost[0])), nil
	}
	if s.source.GetDocCount() < (s.source.SizeAsInt() >> 2) {
		result, err := util.NewFixedBitSet(s.maxDoc)
		if err != nil {
			return nil, err
		}
		cost := []int64{0}
		visitor := newIntersectsDenseVisitor(s.visitor, s.queryRelation, result, cost)
		if err := s.source.Intersect(visitor); err != nil {
			return nil, err
		}
		if cost[0] == 0 {
			return NewEmptyDocIdSetIterator(), nil
		}
		return newUtilToSearchDISIAdapter(util.NewBitSetIterator(result, cost[0])), nil
	}
	builder := util.NewDocIdSetBuilder(s.maxDoc)
	visitor := newSparseVisitor(s.visitor, s.queryRelation, builder)
	if err := s.source.Intersect(visitor); err != nil {
		return nil, err
	}
	set, err := builder.Build()
	if err != nil {
		return nil, err
	}
	if set == nil {
		return NewEmptyDocIdSetIterator(), nil
	}
	utilIter := set.Iterator()
	if utilIter == nil {
		return NewEmptyDocIdSetIterator(), nil
	}
	return newUtilToSearchDISIAdapter(utilIter), nil
}

// getDenseIterator mirrors RelationScorerSupplier.getDenseScorer.
// The two branches (one-tree-walk when every doc has a value, two
// when not) match the Java reference precisely.
func (s *relationScorerSupplier) getDenseIterator() (DocIdSetIterator, error) {
	result, err := util.NewFixedBitSet(s.maxDoc)
	if err != nil {
		return nil, err
	}
	var cost []int64
	if s.source.GetDocCount() == s.maxDoc {
		cost = []int64{int64(s.source.SizeAsInt())}
		setAllBits(result, s.maxDoc)
		visitor := newInverseDenseVisitor(s.visitor, s.queryRelation, result, cost)
		if err := s.source.Intersect(visitor); err != nil {
			return nil, err
		}
	} else {
		cost = []int64{0}
		excluded, err := util.NewFixedBitSet(s.maxDoc)
		if err != nil {
			return nil, err
		}
		visitor := newDenseVisitor(s.visitor, s.queryRelation, result, excluded, cost)
		if err := s.source.Intersect(visitor); err != nil {
			return nil, err
		}
		if err := result.AndNot(excluded); err != nil {
			return nil, err
		}
		// Remove false positives. We only care about inner nodes;
		// the shallow inverse visitor walks them and clears any
		// doc whose cell did not match.
		shallow := newShallowInverseDenseVisitor(s.visitor, s.queryRelation, result)
		if err := s.source.Intersect(shallow); err != nil {
			return nil, err
		}
	}
	if cost[0] == 0 {
		return NewEmptyDocIdSetIterator(), nil
	}
	return newUtilToSearchDISIAdapter(util.NewBitSetIterator(result, cost[0])), nil
}

// getContainsDenseIterator mirrors
// RelationScorerSupplier.getContainsDenseScorer. The contains
// branch can only be reached through a single tree walk; the
// excluded bitset captures docs whose cell answered NOTWITHIN.
func (s *relationScorerSupplier) getContainsDenseIterator() (DocIdSetIterator, error) {
	result, err := util.NewFixedBitSet(s.maxDoc)
	if err != nil {
		return nil, err
	}
	excluded, err := util.NewFixedBitSet(s.maxDoc)
	if err != nil {
		return nil, err
	}
	cost := []int64{0}
	visitor := newContainsDenseVisitor(s.visitor, s.queryRelation, result, excluded, cost)
	if err := s.source.Intersect(visitor); err != nil {
		return nil, err
	}
	if err := result.AndNot(excluded); err != nil {
		return nil, err
	}
	if cost[0] == 0 {
		return NewEmptyDocIdSetIterator(), nil
	}
	return newUtilToSearchDISIAdapter(util.NewBitSetIterator(result, cost[0])), nil
}

// Ensure relationScorerSupplier implements ScorerSupplier.
var _ ScorerSupplier = (*relationScorerSupplier)(nil)

// setAllBits is the Go equivalent of FixedBitSet.set(0, maxDoc).
// util.FixedBitSet does not expose a range-set helper today; a
// per-bit loop is correct (every bit unset → set is a pure write)
// and avoids the indirection of allocating an intermediate bitset.
func setAllBits(fbs *util.FixedBitSet, maxDoc int) {
	for i := 0; i < maxDoc; i++ {
		fbs.Set(i)
	}
}

// errSpatialUnsupportedRelation is the structural sibling of the
// Java reference's IllegalArgumentException for unknown query
// relations. Surfaced as an error (not a panic) because the value
// can flow from user input on some shape queries.
var errSpatialUnsupportedRelation = errors.New("search: unsupported spatial query relation")
