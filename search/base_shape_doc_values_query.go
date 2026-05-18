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

// BaseShapeDocValuesQuery is the Go port of the package-private
// abstract class org.apache.lucene.document.BaseShapeDocValuesQuery
// (Lucene 10.4.0). Concrete subclasses are LatLonShapeDocValuesQuery
// (GOC-3213) and XYShapeDocValuesQuery (GOC-3225); both are still
// pending and will compose this struct via NewBaseShapeDocValuesQuery.
//
// # Compatibility deviations
//
// The Java reference is package-private under document/. Gocene
// exports BaseShapeDocValuesQuery for the same reason SpatialQuery is
// exported: Go has no equivalent of Java's package-private inheritance
// and the type must be reachable from the concrete subclass packages.
// The constructor stays in search/ for cycle-avoidance (see
// SpatialQuery for the rationale).
//
// The "abstract" hook protected ShapeDocValues getShapeDocValues
// (BytesRef) is captured as the getShapeDocValuesFn function field
// injected at construction. Subclasses pass the constructor a closure
// over their codec-specific decoder; the closure may return nil to
// signal a decode failure and short-circuit the match path.
//
// The "abstract" hook protected boolean match(ShapeDocValues) and
// protected float matchCost() are captured as matchFn / matchCostFn.
// Both default to the canonical Java implementation (matchesComponent
// composed with DISJOINT inversion / 60*100 cost respectively); the
// constructor options WithBaseShapeDocValuesMatch and
// WithBaseShapeDocValuesMatchCost let callers override them.
//
// # PointValues path vs BinaryDocValues path
//
// The parent SpatialQuery resolves matches through a visitor-driven
// PointValues source (the BKD tree path). The doc-values family
// instead reads binary doc values per leaf and runs a two-phase
// iterator. This override therefore *replaces* the parent's
// CreateWeight path entirely; the embedded *SpatialQuery is used only
// for identity / equality / hashCode / visit / class-name display.
type BaseShapeDocValuesQuery struct {
	*SpatialQuery

	getShapeDocValuesFn func(binaryValue *util.BytesRef) (*document.ShapeDocValues, error)
	matchFn             func(sdv *document.ShapeDocValues) (bool, error)
	matchCostFn         func() float32
}

// ErrBaseShapeDocValuesQueryContainsNotSupported is the Go port of
// the IllegalArgumentException the Java reference throws when a
// caller passes QueryRelation.CONTAINS. Mirrors the message
// "ShapeDocValuesBoundingBoxQuery does not yet support CONTAINS
// queries" exactly so cross-checked tests can match it.
var ErrBaseShapeDocValuesQueryContainsNotSupported = errors.New(
	"ShapeDocValuesBoundingBoxQuery does not yet support CONTAINS queries",
)

// BaseShapeDocValuesQueryOption customises construction.
type BaseShapeDocValuesQueryOption func(*BaseShapeDocValuesQuery)

// WithBaseShapeDocValuesMatch overrides the per-doc match predicate.
// The default mirrors the Java reference: matchesComponent composed
// with DISJOINT inversion. Subclasses use this hook to inject custom
// matching logic (e.g. bounding-box short-circuits).
func WithBaseShapeDocValuesMatch(fn func(sdv *document.ShapeDocValues) (bool, error)) BaseShapeDocValuesQueryOption {
	return func(q *BaseShapeDocValuesQuery) {
		if fn != nil {
			q.matchFn = fn
		}
	}
}

// WithBaseShapeDocValuesMatchCost overrides the matchCost estimate
// fed to the TwoPhaseIterator. The default is the Java reference's
// 60*100 (comparisons-per-term × averaged terms-per-doc).
func WithBaseShapeDocValuesMatchCost(fn func() float32) BaseShapeDocValuesQueryOption {
	return func(q *BaseShapeDocValuesQuery) {
		if fn != nil {
			q.matchCostFn = fn
		}
	}
}

// NewBaseShapeDocValuesQuery constructs a BaseShapeDocValuesQuery
// for the given (field, relation, queryComponent2D, geometries)
// tuple. queryComponent2D must be the pre-built tree the subclass
// produced from geometries.
//
// getShapeDocValuesFn is the per-binary-value decoder hook the
// abstract Java method getShapeDocValues(BytesRef) models. The
// closure must return a non-nil ShapeDocValues for valid payloads or
// an error for malformed ones; nil-without-error is treated as a
// non-match.
//
// Returns ErrBaseShapeDocValuesQueryContainsNotSupported when
// queryRelation == QueryRelationContains, mirroring the Java
// constructor's IllegalArgumentException.
//
// The constructor also rejects nil getShapeDocValuesFn because the
// Weight cannot be built without it; the embedded *SpatialQuery is
// configured with a placeholder spatialVisitorFactory and noop leaf
// lookup since the doc-values path bypasses both.
func NewBaseShapeDocValuesQuery(
	field string,
	queryRelation document.QueryRelation,
	queryComponent2D geo.Component2D,
	getShapeDocValuesFn func(binaryValue *util.BytesRef) (*document.ShapeDocValues, error),
	geometries []geo.Geometry,
	opts ...BaseShapeDocValuesQueryOption,
) (*BaseShapeDocValuesQuery, error) {
	if queryRelation == document.QueryRelationContains {
		return nil, ErrBaseShapeDocValuesQueryContainsNotSupported
	}
	if getShapeDocValuesFn == nil {
		return nil, errors.New("search: BaseShapeDocValuesQuery getShapeDocValuesFn must not be nil")
	}

	// The embedded SpatialQuery requires a non-nil visitor factory
	// even though the doc-values path never consults it. A no-op
	// visitor keeps the parent contract intact for callers that
	// inspect q.GetSpatialVisitor() (e.g. equality / hashCode paths).
	visitorFactory := func() SpatialVisitor {
		v := &noopBaseShapeVisitor{}
		v.BaseSpatialVisitor = NewBaseSpatialVisitor(v)
		return v
	}

	parent, err := NewSpatialQuery(
		field,
		queryRelation,
		queryComponent2D,
		visitorFactory,
		geometries,
		WithSpatialQueryDisplayClassName("BaseShapeDocValuesQuery"),
	)
	if err != nil {
		return nil, fmt.Errorf("search: BaseShapeDocValuesQuery: %w", err)
	}

	q := &BaseShapeDocValuesQuery{
		SpatialQuery:        parent,
		getShapeDocValuesFn: getShapeDocValuesFn,
	}
	q.matchFn = q.defaultMatch
	q.matchCostFn = defaultBaseShapeDocValuesMatchCost
	for _, opt := range opts {
		opt(q)
	}
	return q, nil
}

// CreateWeight overrides the parent SpatialQuery's PointValues-driven
// path with a BinaryDocValues + TwoPhaseIterator pipeline. Mirrors
// the override BaseShapeDocValuesQuery.getScorerSupplier in the Java
// reference; the Java code reaches this entry through the parent's
// createWeight, which in Gocene we override directly.
func (q *BaseShapeDocValuesQuery) CreateWeight(_ *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	mode := COMPLETE_NO_SCORES
	if needsScores {
		mode = COMPLETE
	}
	return q.createWeight(mode, boost), nil
}

// CreateWeightWithScoreMode is the ScoreMode-aware sibling exported
// for callers and tests that already hold a ScoreMode.
func (q *BaseShapeDocValuesQuery) CreateWeightWithScoreMode(_ *IndexSearcher, scoreMode ScoreMode, boost float32) Weight {
	return q.createWeight(scoreMode, boost)
}

func (q *BaseShapeDocValuesQuery) createWeight(scoreMode ScoreMode, boost float32) Weight {
	score := boost

	supplier := func(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
		if ctx == nil {
			return nil, nil
		}
		leaf := ctx.LeafReader()
		if leaf == nil {
			return nil, nil
		}

		// Resolve BinaryDocValues for the configured field. The
		// generic LeafReaderInterface does not expose
		// GetBinaryDocValues; production readers (LeafReader,
		// SegmentReader) do. Type-assert to pick up the method when
		// available, mirroring the xyPointInGeometryQuery pattern.
		type binaryDocValuesProvider interface {
			GetBinaryDocValues(field string) (index.BinaryDocValues, error)
		}
		type fieldInfosProvider interface {
			GetFieldInfos() *index.FieldInfos
		}

		bvp, ok := leaf.(binaryDocValuesProvider)
		if !ok {
			return nil, nil
		}
		values, err := bvp.GetBinaryDocValues(q.GetField())
		if err != nil {
			return nil, err
		}
		if values == nil {
			return nil, nil
		}

		fip, ok := leaf.(fieldInfosProvider)
		if !ok {
			return nil, nil
		}
		infos := fip.GetFieldInfos()
		if infos == nil {
			return nil, nil
		}
		fieldInfo := infos.GetByName(q.GetField())
		if fieldInfo == nil {
			// No docs in this segment indexed this field at all.
			return nil, nil
		}

		maxDoc := leaf.MaxDoc()
		return q.newScorerSupplier(values, maxDoc, scoreMode, score), nil
	}

	cacheable := func(ctx *index.LeafReaderContext) bool {
		return q.QueryIsCacheable(ctx)
	}

	return NewConstantScoreWeight(q, score, supplier, cacheable)
}

// newScorerSupplier wires the TwoPhaseIterator over the leaf's
// BinaryDocValues. Mirrors the inner ScorerSupplier the Java
// reference returns from getScorerSupplier.
func (q *BaseShapeDocValuesQuery) newScorerSupplier(
	values index.BinaryDocValues,
	maxDoc int,
	scoreMode ScoreMode,
	score float32,
) ScorerSupplier {
	approx := newBinaryDocValuesApproximation(values, maxDoc)
	twoPhase := NewTwoPhaseIterator(approx, func() (bool, error) {
		docID := values.DocID()
		if docID < 0 || docID == NO_MORE_DOCS {
			return false, nil
		}
		payload, err := values.Get(docID)
		if err != nil {
			return false, err
		}
		if payload == nil {
			return false, nil
		}
		ref := util.NewBytesRef(payload)
		sdv, err := q.getShapeDocValuesFn(ref)
		if err != nil {
			return false, err
		}
		if sdv == nil {
			return false, nil
		}
		return q.matchFn(sdv)
	})
	iter := NewTwoPhaseIteratorAsDocIdSetIterator(twoPhase)

	return &baseShapeDocValuesScorerSupplier{
		query:     q,
		iterator:  iter,
		twoPhase:  twoPhase,
		score:     score,
		scoreMode: scoreMode,
		maxDoc:    maxDoc,
	}
}

// MatchCost returns the matchCost estimate consumed by the
// TwoPhaseIterator. Exported so subclasses and tests can introspect
// the hook. Mirrors the Java reference's protected matchCost().
func (q *BaseShapeDocValuesQuery) MatchCost() float32 {
	return q.matchCostFn()
}

// defaultMatch mirrors BaseShapeDocValuesQuery.match in the Java
// reference: delegates to matchesComponent and inverts the result
// when the relation is DISJOINT.
func (q *BaseShapeDocValuesQuery) defaultMatch(sdv *document.ShapeDocValues) (bool, error) {
	hit, err := MatchesShapeDocValuesComponent(sdv, q.GetQueryRelation(), q.GetQueryComponent2D())
	if err != nil {
		return false, err
	}
	if q.GetQueryRelation() == document.QueryRelationDisjoint {
		return !hit, nil
	}
	return hit, nil
}

// MatchesShapeDocValuesComponent is the Go port of
// BaseShapeDocValuesQuery.matchesComponent in the Java reference: it
// asks the ShapeDocValues comparator for the relation between the
// indexed shape and the query Component2D, then folds the result for
// the WITHIN / non-WITHIN branches.
//
// Exported so subclasses living in other packages can compose it into
// custom match closures without reaching into the unexported helper.
func MatchesShapeDocValuesComponent(
	sdv *document.ShapeDocValues,
	queryRelation document.QueryRelation,
	component geo.Component2D,
) (bool, error) {
	if sdv == nil {
		return false, nil
	}
	r, err := sdv.Relate(component)
	if err != nil {
		return false, err
	}
	if r == geo.CellOutsideQuery {
		return false, nil
	}
	if queryRelation == document.QueryRelationWithin {
		return r == geo.CellInsideQuery, nil
	}
	return true, nil
}

// defaultBaseShapeDocValuesMatchCost reproduces the Java reference's
// hard-coded 60 * 100 estimate (per-term comparisons × averaged
// terms-per-doc).
func defaultBaseShapeDocValuesMatchCost() float32 { return 60 * 100 }

// baseShapeDocValuesScorerSupplier wires the two-phase iterator into
// a ConstantScoreScorer on Get, and reports maxDoc as the cost
// (matching the Java return reader.maxDoc()).
type baseShapeDocValuesScorerSupplier struct {
	query     *BaseShapeDocValuesQuery
	iterator  DocIdSetIterator
	twoPhase  *TwoPhaseIterator
	score     float32
	scoreMode ScoreMode
	maxDoc    int
}

// Get returns a ConstantScoreScorer wrapping the two-phase iterator.
// The leadCost parameter is accepted for ScorerSupplier compliance
// but unused — the Java reference also ignores it for this code path.
func (s *baseShapeDocValuesScorerSupplier) Get(_ int64) (Scorer, error) {
	return NewConstantScoreScorer(s.score, s.scoreMode, s.iterator), nil
}

// Cost returns the maxDoc of the leaf, mirroring the Java reference's
// reader.maxDoc().
func (s *baseShapeDocValuesScorerSupplier) Cost() int64 { return int64(s.maxDoc) }

// SetTopLevelScoringClause is a no-op for this supplier: the wrapped
// scorer always returns a constant score regardless of context.
func (s *baseShapeDocValuesScorerSupplier) SetTopLevelScoringClause() {}

var _ ScorerSupplier = (*baseShapeDocValuesScorerSupplier)(nil)

// binaryDocValuesApproximation adapts a BinaryDocValues iterator to
// the DocIdSetIterator surface the TwoPhaseIterator approximation
// expects. The Java reference uses BinaryDocValues directly because
// it implements DocIdSetIterator; the Gocene BinaryDocValues
// interface keeps the iterator hooks (NextDoc / Advance / DocID) and
// adds a synchronous Get(docID), so the adapter is purely structural.
type binaryDocValuesApproximation struct {
	values index.BinaryDocValues
	cost   int64
}

func newBinaryDocValuesApproximation(values index.BinaryDocValues, maxDoc int) *binaryDocValuesApproximation {
	cost := int64(maxDoc)
	if cost < 0 {
		cost = 0
	}
	return &binaryDocValuesApproximation{values: values, cost: cost}
}

// DocID returns the iterator's current document.
func (a *binaryDocValuesApproximation) DocID() int { return a.values.DocID() }

// NextDoc advances the iterator and normalises trailing errors into
// the NO_MORE_DOCS sentinel.
func (a *binaryDocValuesApproximation) NextDoc() (int, error) {
	doc, err := a.values.NextDoc()
	if err != nil {
		return NO_MORE_DOCS, err
	}
	return doc, nil
}

// Advance forwards to the underlying iterator's Advance.
func (a *binaryDocValuesApproximation) Advance(target int) (int, error) {
	doc, err := a.values.Advance(target)
	if err != nil {
		return NO_MORE_DOCS, err
	}
	return doc, nil
}

// Cost returns the maxDoc-derived cost captured at construction.
func (a *binaryDocValuesApproximation) Cost() int64 { return a.cost }

// DocIDRunEnd returns DocID()+1 because BinaryDocValues exposes no
// notion of a contiguous match run; one-doc runs are the safe
// default.
func (a *binaryDocValuesApproximation) DocIDRunEnd() int {
	doc := a.values.DocID()
	if doc < 0 || doc == NO_MORE_DOCS {
		return doc
	}
	return doc + 1
}

var _ DocIdSetIterator = (*binaryDocValuesApproximation)(nil)

// noopBaseShapeVisitor is the placeholder visitor wired into the
// embedded SpatialQuery. The doc-values path never invokes it; it
// only exists so the parent's non-nil contract is satisfied.
type noopBaseShapeVisitor struct {
	*BaseSpatialVisitor
}

func (v *noopBaseShapeVisitor) Relate(_, _ []byte) spatialRelation {
	return spatialCellCrossesQuery
}

func (v *noopBaseShapeVisitor) Intersects() func(packed []byte) bool {
	return func(_ []byte) bool { return false }
}

func (v *noopBaseShapeVisitor) Within() func(packed []byte) bool {
	return func(_ []byte) bool { return false }
}

func (v *noopBaseShapeVisitor) Contains() func(packed []byte) geo.WithinRelation {
	return func(_ []byte) geo.WithinRelation { return geo.WithinDisjoint }
}
