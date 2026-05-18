// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SpatialQuery is the abstract base for every spatial query in the
// shape family: LatLonShapeQuery, XYShapeQuery, LatLonPointShape and
// the doc-values variants all extend it. It mirrors the
// package-private abstract class org.apache.lucene.document.
// SpatialQuery (Lucene 10.4.0).
//
// The Java reference is package-private; Gocene exports SpatialQuery
// because Go has no equivalent of Java's package-private inheritance
// — subclasses live in the same search/ package today, but the type
// is exported to keep the contract reviewable and to allow tests in
// external packages to assert against the public surface.
//
// # Composition vs inheritance
//
// SpatialQuery is a *struct* (not an interface). Concrete shape
// queries embed *SpatialQuery and provide their own SpatialVisitor
// via the spatialVisitorFactory hook captured at construction time.
// CreateWeight and the inner getScorerSupplier consume the hook
// through GetSpatialVisitor(), mirroring the Java abstract method.
//
// # Cycle-avoidance placement
//
// SpatialQuery lives in search/ rather than document/ because every
// concrete subclass needs the search-package primitives
// (ConstantScoreWeight, ScorerSupplier, BulkScorer wiring) and the
// document/ package must stay free of search/ imports to keep the
// document → search hierarchy unidirectional. ShapeField.QueryRelation
// remains in document/ and is imported here.
//
// # PointValues source
//
// The Java reference resolves point values via
// LeafReader.getPointValues(field), which returns a visitor-driven
// PointValues. Gocene's index.PointValues exposes only metadata
// today (no Intersect / EstimatePointCount hooks); to keep the
// Weight wiring functional, SpatialQuery delegates intersection to
// a spatialPointSource adapter — exactly the same noop-default
// pattern xyPointInGeometryQuery uses. Concrete subclasses or the
// upcoming PointValues port may swap the leafLookup hook to a
// non-noop adapter without changing this file.
type SpatialQuery struct {
	*BaseQuery

	field                  string
	queryRelation          document.QueryRelation
	geometries             []geo.Geometry
	queryComponent2D       geo.Component2D
	spatialVisitorFactory  func() SpatialVisitor
	queryIsCacheableHook   func(ctx *index.LeafReaderContext) bool
	leafLookup             spatialLeafLookup
	displayClassNameOption string
}

// SpatialQueryOption customises SpatialQuery construction. Options
// are applied in the order they appear in NewSpatialQuery's
// variadic tail; later options override earlier ones for the same
// field.
type SpatialQueryOption func(*SpatialQuery)

// WithSpatialQueryCacheableHook installs a per-leaf cacheability
// predicate. The default is "always cacheable" (matching the Java
// reference's queryIsCacheable default).
func WithSpatialQueryCacheableHook(hook func(ctx *index.LeafReaderContext) bool) SpatialQueryOption {
	return func(q *SpatialQuery) {
		q.queryIsCacheableHook = hook
	}
}

// WithSpatialQueryLeafLookup installs the leaf-level point source
// lookup. The default lookup returns a noop source (zero matches)
// for every leaf, mirroring the current LongDistanceFeatureQuery /
// xyPointInGeometryQuery defaults. Subclasses inject the canonical
// lookup when the PointValues port wires visitor-driven Intersect.
func WithSpatialQueryLeafLookup(lookup spatialLeafLookup) SpatialQueryOption {
	return func(q *SpatialQuery) {
		if lookup != nil {
			q.leafLookup = lookup
		}
	}
}

// WithSpatialQueryDisplayClassName overrides the class-name segment
// of String(). Subclasses use this so toString() prints
// "LatLonShapeQuery:..." rather than the generic "SpatialQuery:..."
// the base would otherwise emit.
func WithSpatialQueryDisplayClassName(name string) SpatialQueryOption {
	return func(q *SpatialQuery) {
		if name != "" {
			q.displayClassNameOption = name
		}
	}
}

// NewSpatialQuery builds a SpatialQuery for the given field /
// relation / Component2D / geometries tuple. queryComponent2D is
// the pre-built tree the concrete subclass produced from
// geometries; the constructor stores it as-is so the heavy build
// happens only once per query.
//
// spatialVisitorFactory must produce a SpatialVisitor that knows
// how to relate cells and decide per-doc inclusion for the
// subclass's geometry. The factory is called eagerly by
// CreateWeight; the returned visitor is reused across leaves.
//
// Mirrors the Java constructor
// SpatialQuery(String, QueryRelation, Geometry...) together with
// the abstract createComponent2D / getSpatialVisitor hooks.
//
// The constructor panics on a nil field or a nil queryComponent2D
// (programmer error, matching Java's IllegalArgumentException), and
// returns an error on a nil spatialVisitorFactory because the
// Weight cannot be built without it.
func NewSpatialQuery(
	field string,
	queryRelation document.QueryRelation,
	queryComponent2D geo.Component2D,
	spatialVisitorFactory func() SpatialVisitor,
	geometries []geo.Geometry,
	opts ...SpatialQueryOption,
) (*SpatialQuery, error) {
	if field == "" {
		return nil, errors.New("search: SpatialQuery field must not be empty")
	}
	if queryComponent2D == nil {
		return nil, errors.New("search: SpatialQuery queryComponent2D must not be nil")
	}
	if spatialVisitorFactory == nil {
		return nil, errors.New("search: SpatialQuery spatialVisitorFactory must not be nil")
	}
	// Defensive copy — geometries is captured for hashCode / equals
	// so a caller mutating the slice after construction would
	// corrupt the query identity.
	geomCopy := make([]geo.Geometry, len(geometries))
	copy(geomCopy, geometries)

	q := &SpatialQuery{
		BaseQuery:             &BaseQuery{},
		field:                 field,
		queryRelation:         queryRelation,
		geometries:            geomCopy,
		queryComponent2D:      queryComponent2D,
		spatialVisitorFactory: spatialVisitorFactory,
		queryIsCacheableHook:  nil, // defaults to true (Java parity)
		leafLookup:            noopSpatialLeafLookup,
	}
	for _, opt := range opts {
		opt(q)
	}
	return q, nil
}

// GetField returns the field name the query is bound to.
func (q *SpatialQuery) GetField() string { return q.field }

// GetQueryRelation returns the relation the query asserts between
// indexed shapes and the queryComponent2D tree.
func (q *SpatialQuery) GetQueryRelation() document.QueryRelation { return q.queryRelation }

// GetGeometries returns a defensive copy of the geometries used to
// build queryComponent2D. The Java reference exposes them via
// Arrays.equals; the copy here keeps callers from mutating the
// query's hashCode identity.
func (q *SpatialQuery) GetGeometries() []geo.Geometry {
	out := make([]geo.Geometry, len(q.geometries))
	copy(out, q.geometries)
	return out
}

// GetQueryComponent2D returns the pre-built Component2D tree the
// query relates against. Useful for tests; the field is not part
// of the Java public surface but mirrors the protected member.
func (q *SpatialQuery) GetQueryComponent2D() geo.Component2D { return q.queryComponent2D }

// GetSpatialVisitor calls the factory captured at construction and
// returns a fresh visitor. Mirrors the abstract getSpatialVisitor()
// on the Java reference.
func (q *SpatialQuery) GetSpatialVisitor() SpatialVisitor {
	return q.spatialVisitorFactory()
}

// Visit invokes the QueryVisitor's VisitLeaf hook when the visitor
// accepts the query's field. Mirrors Java's
// SpatialQuery.visit(QueryVisitor).
func (q *SpatialQuery) Visit(visitor QueryVisitor) {
	if visitor.AcceptField(q.field) {
		visitor.VisitLeaf(q)
	}
}

// QueryIsCacheable forwards to the per-leaf hook (defaulting to
// "true" when the hook is nil), mirroring Java's protected
// queryIsCacheable(LeafReaderContext).
func (q *SpatialQuery) QueryIsCacheable(ctx *index.LeafReaderContext) bool {
	if q.queryIsCacheableHook == nil {
		return true
	}
	return q.queryIsCacheableHook(ctx)
}

// CreateWeight builds a ConstantScoreWeight that, on every leaf,
// resolves the point source and delegates intersection to the
// SpatialVisitor produced by the factory. Mirrors the final
// override SpatialQuery.createWeight(IndexSearcher, ScoreMode,
// float).
//
// The Java reference also accepts a "needsScores" boolean instead
// of a ScoreMode; Gocene's Query.CreateWeight follows the older
// signature, so this implementation infers ScoreMode from the
// boolean (true → COMPLETE, false → COMPLETE_NO_SCORES) and
// propagates that mode to the spatial scorers. Subclasses that
// need the full ScoreMode enum can call CreateWeightWithScoreMode
// directly.
func (q *SpatialQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	mode := COMPLETE_NO_SCORES
	if needsScores {
		mode = COMPLETE
	}
	return q.CreateWeightWithScoreMode(searcher, mode, boost), nil
}

// CreateWeightWithScoreMode is the ScoreMode-aware sibling of
// CreateWeight, exported so concrete subclasses and tests can wire
// the supplier directly. The returned Weight is non-nil even when
// the field is missing from every segment — per-leaf ScorerSupplier
// returns nil for missing fields.
func (q *SpatialQuery) CreateWeightWithScoreMode(_ *IndexSearcher, scoreMode ScoreMode, boost float32) Weight {
	spatialVisitor := q.GetSpatialVisitor()
	score := boost

	supplier := func(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
		leaf := ctx.LeafReader()
		if leaf == nil {
			return nil, nil
		}
		source, fieldInfo, maxDoc, err := q.leafLookup(ctx, q.field)
		if err != nil {
			return nil, err
		}
		if source == nil || fieldInfo == nil || maxDoc <= 0 {
			return nil, nil
		}
		return q.getScorerSupplier(source, fieldInfo, maxDoc, spatialVisitor, scoreMode, score)
	}

	cacheable := func(ctx *index.LeafReaderContext) bool {
		return q.QueryIsCacheable(ctx)
	}

	return NewConstantScoreWeight(q, score, supplier, cacheable)
}

// getScorerSupplier replicates SpatialQuery.getScorerSupplier from
// the Java reference. The five branches are:
//
//  1. cell is outside the query (or fully inside on CONTAINS): no matches.
//  2. cell is fully inside the query and every doc has a value:
//     return a match-all supplier.
//  3. WITHIN / DISJOINT with sparse semantics and no hits: short-circuit.
//  4. fall back to RelationScorerSupplier.
//
// The function returns (nil, nil) for the no-matches branches —
// matching the Java null return.
func (q *SpatialQuery) getScorerSupplier(
	source spatialPointSource,
	_ *index.FieldInfo,
	maxDoc int,
	visitor SpatialVisitor,
	scoreMode ScoreMode,
	score float32,
) (ScorerSupplier, error) {
	minPacked := source.GetMinPackedValue()
	maxPacked := source.GetMaxPackedValue()
	rel := visitor.GetInnerFunction(q.queryRelation)(minPacked, maxPacked)

	switch rel {
	case spatialCellOutsideQuery:
		return nil, nil
	case spatialCellInsideQuery:
		if q.queryRelation == document.QueryRelationContains {
			return nil, nil
		}
		if source.GetDocCount() == maxDoc {
			// Every doc in the segment has a value and the value
			// cell is fully inside the query → match-all.
			return NewConstantScoreScorerSupplier(
				score,
				scoreMode,
				int64(maxDoc),
				func(_ int64) (DocIdSetIterator, error) {
					return NewRangeDocIdSetIterator(0, maxDoc), nil
				},
			), nil
		}
	}

	if q.queryRelation != document.QueryRelationIntersects &&
		q.queryRelation != document.QueryRelationContains &&
		source.GetDocCount() != source.SizeAsInt() {
		hit, err := hasAnyHits(visitor, q.queryRelation, source)
		if err != nil {
			return nil, err
		}
		if !hit {
			return nil, nil
		}
	}

	rss := &relationScorerSupplier{
		source:        source,
		visitor:       visitor,
		queryRelation: q.queryRelation,
		maxDoc:        maxDoc,
		score:         score,
		scoreMode:     scoreMode,
		cost:          -1,
	}
	return rss, nil
}

// Equals reports whether o is a SpatialQuery with the same field,
// relation and geometry slice (element-wise).
// Mirrors SpatialQuery.equalsTo on the Java reference.
func (q *SpatialQuery) Equals(other Query) bool {
	o, ok := other.(*SpatialQuery)
	if !ok {
		return false
	}
	if q.field != o.field || q.queryRelation != o.queryRelation {
		return false
	}
	if len(q.geometries) != len(o.geometries) {
		return false
	}
	for i := range q.geometries {
		// Geometry is an interface; the structural equality the
		// Java reference uses (Arrays.equals) lowers to == on the
		// element reference, which in Go is the interface equality.
		// For value-shaped geometries (geo.Rectangle, geo.Point)
		// this matches Java's reference equality semantics; for
		// pointer-shaped geometries it compares pointers.
		if q.geometries[i] != o.geometries[i] {
			return false
		}
	}
	return true
}

// HashCode mirrors SpatialQuery.hashCode: classHash seeded; folded
// through field, queryRelation, and Arrays.hashCode(geometries).
func (q *SpatialQuery) HashCode() int {
	h := classHashSpatialQuery
	h = 31*h + stringHash(q.field)
	h = 31*h + int(q.queryRelation)
	h = 31*h + geometriesHashCode(q.geometries)
	return h
}

// String mirrors SpatialQuery.toString(String): prints
// "<ClassName>:[<geom1>,<geom2>,...]" and prepends "field=<field>:"
// when the supplied default field differs from the query's field.
func (q *SpatialQuery) String(field string) string {
	var sb strings.Builder
	sb.WriteString(q.displayClassName())
	sb.WriteByte(':')
	if q.field != field {
		sb.WriteString(" field=")
		sb.WriteString(q.field)
		sb.WriteByte(':')
	}
	sb.WriteByte('[')
	for _, g := range q.geometries {
		sb.WriteString(fmt.Sprintf("%v", g))
		sb.WriteByte(',')
	}
	sb.WriteByte(']')
	return sb.String()
}

// displayClassName returns the override (when set) or the Go type
// name. Concrete subclasses set the override via
// WithSpatialQueryDisplayClassName so toString output matches the
// Java reference.
func (q *SpatialQuery) displayClassName() string {
	if q.displayClassNameOption != "" {
		return q.displayClassNameOption
	}
	return "SpatialQuery"
}

// classHashSpatialQuery seeds the hashCode with a type-stable
// constant. The value (0x53_70_51_75 — "SpQu") is distinct from
// other query class hashes so equal field/relation/geometry triples
// in different query types do not collide.
const classHashSpatialQuery = 0x5370_5175

// geometriesHashCode mirrors java.util.Arrays.hashCode on a
// Geometry[] reference: seed at 1, fold each element through
// 31*h + element-hash. Element-hash uses %v as a stable identity
// proxy (same approach as xy_point_in_geometry_query.go).
func geometriesHashCode(geoms []geo.Geometry) int {
	h := int32(1)
	for _, g := range geoms {
		gh := int32(0)
		if g != nil {
			gh = int32(stringHash(fmt.Sprintf("%v", g)))
		}
		h = 31*h + gh
	}
	return int(h)
}

// transposeSpatialRelation flips INSIDE and OUTSIDE, leaves
// CROSSES alone. Used by the DISJOINT inner function to invert a
// relate result so the rest of the pipeline can treat DISJOINT
// symmetrically with INTERSECTS. Mirrors
// SpatialQuery.transposeRelation.
func transposeSpatialRelation(r spatialRelation) spatialRelation {
	switch r {
	case spatialCellInsideQuery:
		return spatialCellOutsideQuery
	case spatialCellOutsideQuery:
		return spatialCellInsideQuery
	default:
		return spatialCellCrossesQuery
	}
}

// errCollectionTerminated is the sentinel hasAnyHits uses to
// short-circuit a BKD walk as soon as the first match is observed.
// It mirrors the control-flow role of
// org.apache.lucene.search.CollectionTerminatedException on the
// Java reference but stays an idiomatic error: callers check
// errors.Is(err, errCollectionTerminated) and convert the signal
// back into a boolean result.
var errCollectionTerminated = errors.New("spatial: collection terminated")

// hasAnyHits walks the BKD tree with a fail-fast visitor that
// throws errCollectionTerminated the moment the first match is
// observed. Mirrors SpatialQuery.hasAnyHits.
//
// The visitor's compare hook also terminates early when the cell
// is fully inside the query, because that guarantees every leaf
// in the subtree matches and re-walking them would be wasted work.
func hasAnyHits(visitor SpatialVisitor, queryRelation document.QueryRelation, source spatialPointSource) (bool, error) {
	innerFn := visitor.GetInnerFunction(queryRelation)
	leafPredicate := visitor.GetLeafPredicate(queryRelation)
	probe := &spatialHasAnyHitsVisitor{innerFn: innerFn, leafPredicate: leafPredicate}
	err := source.Intersect(probe)
	if err == nil {
		return false, nil
	}
	if errors.Is(err, errCollectionTerminated) {
		return true, nil
	}
	return false, err
}

// spatialHasAnyHitsVisitor returns errCollectionTerminated on the
// first observed match; every other hook is a passive forwarder.
type spatialHasAnyHitsVisitor struct {
	innerFn       func(min, max []byte) spatialRelation
	leafPredicate func(packed []byte) bool
}

func (v *spatialHasAnyHitsVisitor) Visit(_ int) error {
	return errCollectionTerminated
}

func (v *spatialHasAnyHitsVisitor) VisitWithPackedValue(_ int, packed []byte) error {
	if v.leafPredicate(packed) {
		return errCollectionTerminated
	}
	return nil
}

func (v *spatialHasAnyHitsVisitor) VisitIterator(_ util.DocIdSetIterator) error { return nil }

func (v *spatialHasAnyHitsVisitor) VisitIteratorWithPackedValue(_ util.DocIdSetIterator, packed []byte) error {
	if v.leafPredicate(packed) {
		return errCollectionTerminated
	}
	return nil
}

func (v *spatialHasAnyHitsVisitor) Grow(_ int) {}

func (v *spatialHasAnyHitsVisitor) Compare(minPacked, maxPacked []byte) spatialRelation {
	r := v.innerFn(minPacked, maxPacked)
	if r == spatialCellInsideQuery {
		// Surface the early-exit via the next Visit call; we cannot
		// return the sentinel from Compare itself because the
		// visitor contract requires a valid spatialRelation.
		return spatialCellInsideQuery
	}
	return r
}

var _ spatialIntersectVisitor = (*spatialHasAnyHitsVisitor)(nil)
