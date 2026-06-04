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
)

// latLonDocValuesQuery matches documents whose indexed lat/lon doc
// values satisfy a given QueryRelation with one or more LatLonGeometry
// shapes.
//
// It is the Go port of the package-private
// org.apache.lucene.document.LatLonDocValuesQuery (Lucene 10.4.0).
// Lucene keeps the class package-private and exposes it solely through
// the LatLonDocValuesField factory methods; Gocene mirrors that
// invariant by keeping the struct unexported and routing construction
// through NewLatLonDocValuesQuery.
//
// # Doc-values shape
//
// The field must be indexed with [document.NewLatLonDocValuesField],
// which packs (latitude, longitude) into a single int64 with the layout
//
//	high32(value) = encoded latitude
//	low32(value)  = encoded longitude
//
// matching the reference's setLocationValue bit layout. The query
// resolves the per-leaf [index.SortedNumericDocValues] iterator and
// inspects each indexed value with [geo.GeoEncodingUtils]-style
// predicates.
//
// # Construction-time validation
//
// Two Java IllegalArgumentException branches in the reference are
// preserved as constructor errors:
//
//   - WITHIN rejects [geo.Line] geometries (the reference does not
//     support line-WITHIN today).
//   - CONTAINS rejects every non-Point geometry.
//
// A nil field or nil relation also reports an error. The original
// throws NPE-shaped IllegalArgumentExceptions; we surface idiomatic
// errors and keep the same wording.
type latLonDocValuesQuery struct {
	*BaseQuery

	field         string
	queryRelation document.QueryRelation
	geometries    []geo.LatLonGeometry
	component2D   geo.Component2D
}

// LatLonDocValuesQueryError messages mirror Lucene's exception text.
var (
	errLatLonDocValuesQueryNilField = errors.New(
		"search: LatLonDocValuesQuery field must not be null")
	errLatLonDocValuesQueryNilRelation = errors.New(
		"search: LatLonDocValuesQuery queryRelation must not be null")
)

// NewLatLonDocValuesQuery builds a doc-values query bound to field that
// applies queryRelation against the union of geometries.
//
// Mirrors the Java constructor
// LatLonDocValuesQuery(String, ShapeField.QueryRelation, LatLonGeometry...).
// The variadic geometries slice is captured by reference for hashCode
// and equals stability; callers must not mutate it after construction.
//
// Returns an error when:
//
//   - field is empty (Java throws "field must not be null").
//   - queryRelation is the zero value and the caller did not pass a
//     valid enum (callers should always pass an explicit relation; the
//     check is here for forward compatibility once additional
//     relations are introduced).
//   - a [geo.Line] is supplied with WITHIN.
//   - a non-Point geometry is supplied with CONTAINS.
//   - the [geo.CreateLatLonGeometry] union build fails.
func NewLatLonDocValuesQuery(
	field string,
	queryRelation document.QueryRelation,
	geometries ...geo.LatLonGeometry,
) (Query, error) {
	if field == "" {
		return nil, errLatLonDocValuesQueryNilField
	}
	if queryRelation < document.QueryRelationIntersects ||
		queryRelation > document.QueryRelationDisjoint {
		return nil, errLatLonDocValuesQueryNilRelation
	}
	switch queryRelation {
	case document.QueryRelationWithin:
		for _, g := range geometries {
			if _, ok := g.(geo.Line); ok {
				return nil, fmt.Errorf(
					"search: LatLonDocValuesQuery does not support %s queries with line geometries",
					queryRelation)
			}
		}
	case document.QueryRelationContains:
		for _, g := range geometries {
			if _, ok := g.(geo.Point); !ok {
				return nil, fmt.Errorf(
					"search: LatLonDocValuesQuery does not support %s queries with non-points geometries",
					queryRelation)
			}
		}
	}
	component2D, err := geo.CreateLatLonGeometry(geometries...)
	if err != nil {
		return nil, fmt.Errorf("search: LatLonDocValuesQuery: %w", err)
	}
	return &latLonDocValuesQuery{
		BaseQuery:     &BaseQuery{},
		field:         field,
		queryRelation: queryRelation,
		geometries:    geometries,
		component2D:   component2D,
	}, nil
}

// String mirrors LatLonDocValuesQuery.toString(String). Format:
//
//	[field:]<RELATION>:geometries([geom1, geom2, ...])
//
// The "field:" prefix is suppressed when the supplied default field
// matches the query's field, exactly like the Java reference.
func (q *latLonDocValuesQuery) String(field string) string {
	var sb strings.Builder
	if q.field != field {
		sb.WriteString(q.field)
		sb.WriteByte(':')
	}
	sb.WriteString(q.queryRelation.String())
	sb.WriteByte(':')
	sb.WriteString("geometries(")
	sb.WriteByte('[')
	for i, g := range q.geometries {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("%v", g))
	}
	sb.WriteByte(']')
	sb.WriteByte(')')
	return sb.String()
}

// Equals mirrors LatLonDocValuesQuery.equals: same class, same field,
// same relation, element-wise equal geometry slice.
func (q *latLonDocValuesQuery) Equals(other Query) bool {
	o, ok := other.(*latLonDocValuesQuery)
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
		// Geometry is an interface backed by value types
		// (geo.Point, geo.Rectangle, geo.Line, geo.Polygon, ...).
		// Interface equality compares the underlying dynamic
		// values which matches Java's Arrays.equals for the
		// reference's hashCode/equals contract on these types.
		if q.geometries[i] != o.geometries[i] {
			return false
		}
	}
	return true
}

// HashCode mirrors LatLonDocValuesQuery.hashCode. The Java reference
// seeds with classHash() (a per-Class constant) and folds through
// field, queryRelation, and Arrays.hashCode(geometries). Gocene uses a
// type-stable literal seed (so distinct query classes never collide on
// equal field/relation/geometry triples) and the same FNV-derived
// string hash everyone in this package uses.
func (q *latLonDocValuesQuery) HashCode() int {
	h := classHashLatLonDocValuesQuery
	h = 31*h + stringHash(q.field)
	h = 31*h + int(q.queryRelation)
	h = 31*h + geometriesHashCodeLatLon(q.geometries)
	return h
}

// Visit mirrors LatLonDocValuesQuery.visit(QueryVisitor): descend into
// the leaf only when the visitor accepts the query's field.
func (q *latLonDocValuesQuery) Visit(visitor QueryVisitor) {
	if visitor.AcceptField(q.field) {
		visitor.VisitLeaf(q)
	}
}

// GetField returns the field this query is bound to.
func (q *latLonDocValuesQuery) GetField() string { return q.field }

// GetQueryRelation returns the relation the query asserts.
func (q *latLonDocValuesQuery) GetQueryRelation() document.QueryRelation {
	return q.queryRelation
}

// GetGeometries returns a copy of the geometries the query was built
// with. The copy keeps the query's hash identity stable against caller
// mutation.
func (q *latLonDocValuesQuery) GetGeometries() []geo.LatLonGeometry {
	out := make([]geo.LatLonGeometry, len(q.geometries))
	copy(out, q.geometries)
	return out
}

// Clone returns the query itself. The struct is logically immutable
// (geometries are captured by reference and the contract documents
// that callers must not mutate them), so a shallow clone preserves
// query identity and equals semantics.
func (q *latLonDocValuesQuery) Clone() Query { return q }

// Rewrite returns the query unchanged (it has no rewrite rules in the
// Java reference). The explicit override is required because the type
// embeds *BaseQuery: relying on the promoted BaseQuery.Rewrite would
// return the inner *BaseQuery receiver, erasing this query's
// CreateWeight override so the rewritten query would silently match
// zero documents.
func (q *latLonDocValuesQuery) Rewrite(_ IndexReader) (Query, error) { return q, nil }

// CreateWeight builds a [ConstantScoreWeight] that resolves the
// per-leaf [index.SortedNumericDocValues] iterator and wraps a
// [TwoPhaseIterator] whose Matches method performs the actual
// point-in-shape test for the configured QueryRelation.
//
// The Java reference takes a ScoreMode; Gocene's Query.CreateWeight
// signature uses a needsScores bool, so the supplier infers the mode
// (true => COMPLETE, false => COMPLETE_NO_SCORES) and propagates it
// to the ConstantScoreScorer.
func (q *latLonDocValuesQuery) CreateWeight(_ *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	mode := COMPLETE_NO_SCORES
	if needsScores {
		mode = COMPLETE
	}
	// Pre-compute the predicate once. CONTAINS does not use the
	// predicate (it works directly on the decoded lat/lon and the
	// per-geometry Component2D list), matching the Java fast path
	// that skips the predicate build for CONTAINS.
	var predicate *geo.Component2DPredicate
	if q.queryRelation != document.QueryRelationContains {
		p := geo.CreateComponentPredicate(q.component2D)
		predicate = &p
	}
	// CONTAINS evaluates per geometry; build the per-geometry
	// Component2D list once. Mirrors the Java contains() helper.
	var containsComponents []geo.Component2D
	if q.queryRelation == document.QueryRelationContains {
		containsComponents = make([]geo.Component2D, len(q.geometries))
		for i, g := range q.geometries {
			c2d, err := geo.CreateLatLonGeometry(g)
			if err != nil {
				return nil, fmt.Errorf(
					"search: LatLonDocValuesQuery contains component: %w", err)
			}
			containsComponents[i] = c2d
		}
	}

	supplier := func(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
		values, err := leafSortedNumeric(ctx, q.field)
		if err != nil {
			return nil, err
		}
		if values == nil {
			return nil, nil
		}
		maxDoc := 0
		if r := ctx.LeafReader(); r != nil {
			maxDoc = r.MaxDoc()
		}
		approx := newSortedNumericApproximation(values, maxDoc)
		var matchFn func(docID int) (bool, error)
		switch q.queryRelation {
		case document.QueryRelationIntersects:
			matchFn = func(docID int) (bool, error) {
				return matchesIntersects(values, predicate, docID)
			}
		case document.QueryRelationWithin:
			matchFn = func(docID int) (bool, error) {
				return matchesWithin(values, predicate, docID)
			}
		case document.QueryRelationDisjoint:
			matchFn = func(docID int) (bool, error) {
				return matchesDisjoint(values, predicate, docID)
			}
		case document.QueryRelationContains:
			matchFn = func(docID int) (bool, error) {
				return matchesContains(values, containsComponents, docID)
			}
		default:
			return nil, fmt.Errorf(
				"search: LatLonDocValuesQuery invalid query relationship: [%s]",
				q.queryRelation)
		}
		tpi := NewTwoPhaseIterator(approx, func() (bool, error) {
			return matchFn(approx.DocID())
		})
		return NewConstantScoreScorerSupplier(
			boost,
			mode,
			approx.Cost(),
			func(_ int64) (DocIdSetIterator, error) {
				return tpi.AsDocIdSetIterator(), nil
			},
		), nil
	}

	cacheable := func(ctx *index.LeafReaderContext) bool {
		return index.IsDocValuesCacheable(ctx, q.field)
	}

	return NewConstantScoreWeight(q, boost, supplier, cacheable), nil
}

// Ensure latLonDocValuesQuery implements Query.
var _ Query = (*latLonDocValuesQuery)(nil)

// classHashLatLonDocValuesQuery seeds the type-stable hash for this
// query. The literal ("LlDV") makes the seed visually self-describing
// and distinct from every other classHash in the package.
const classHashLatLonDocValuesQuery = 0x4c6c_4456 // "LlDV"

// geometriesHashCodeLatLon mirrors java.util.Arrays.hashCode on a
// LatLonGeometry[]: seed at 1, fold each element through 31*h +
// element-hash, where element-hash uses %v as a stable identity proxy
// for the value-typed geometries this package ships.
func geometriesHashCodeLatLon(geoms []geo.LatLonGeometry) int {
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

// matchesIntersects mirrors the Java intersects() TwoPhaseIterator
// match: a doc matches when at least one of its lat/lon values is
// inside the shape. Iterates the doc's packed long values and decodes
// the high 32 bits as latitude, low 32 bits as longitude (matching
// document.EncodeLatLonAsLong).
func matchesIntersects(
	values index.SortedNumericDocValues,
	predicate *geo.Component2DPredicate,
	docID int,
) (bool, error) {
	if predicate == nil {
		return false, nil
	}
	vs, err := index.DrainSortedNumeric(values, docID)
	if err != nil {
		return false, err
	}
	for _, v := range vs {
		lat := int32(uint64(v) >> 32)
		lon := int32(v & 0xFFFFFFFF)
		if predicate.Test(lat, lon) {
			return true, nil
		}
	}
	return false, nil
}

// matchesWithin mirrors within(): every lat/lon value must be inside
// the shape. An empty value slice falls through to "all matched" — the
// Java loop runs zero times and returns true.
func matchesWithin(
	values index.SortedNumericDocValues,
	predicate *geo.Component2DPredicate,
	docID int,
) (bool, error) {
	if predicate == nil {
		return false, nil
	}
	vs, err := index.DrainSortedNumeric(values, docID)
	if err != nil {
		return false, err
	}
	for _, v := range vs {
		lat := int32(uint64(v) >> 32)
		lon := int32(v & 0xFFFFFFFF)
		if !predicate.Test(lat, lon) {
			return false, nil
		}
	}
	return true, nil
}

// matchesDisjoint mirrors disjoint(): no lat/lon value may be inside
// the shape. Mirrors the Java loop that returns false as soon as a
// match is observed and true otherwise.
func matchesDisjoint(
	values index.SortedNumericDocValues,
	predicate *geo.Component2DPredicate,
	docID int,
) (bool, error) {
	if predicate == nil {
		return true, nil
	}
	vs, err := index.DrainSortedNumeric(values, docID)
	if err != nil {
		return false, err
	}
	for _, v := range vs {
		lat := int32(uint64(v) >> 32)
		lon := int32(v & 0xFFFFFFFF)
		if predicate.Test(lat, lon) {
			return false, nil
		}
	}
	return true, nil
}

// matchesContains mirrors contains(): every component2D must report a
// per-point WithinRelation other than NOTWITHIN, and at least one
// component2D must report CANDIDATE. The Java reference iterates one
// value at a time and folds the answer; we follow the same loop
// structure to preserve byte-equivalent boolean folding semantics.
func matchesContains(
	values index.SortedNumericDocValues,
	components []geo.Component2D,
	docID int,
) (bool, error) {
	vs, err := index.DrainSortedNumeric(values, docID)
	if err != nil {
		return false, err
	}
	answer := geo.WithinDisjoint
	for _, v := range vs {
		lat := geo.DecodeLatitude(int32(uint64(v) >> 32))
		lon := geo.DecodeLongitude(int32(v & 0xFFFFFFFF))
		for _, c2d := range components {
			rel := c2d.WithinPoint(lon, lat)
			if rel == geo.WithinNotWithin {
				return false, nil
			}
			if rel != geo.WithinDisjoint {
				answer = rel
			}
		}
	}
	return answer == geo.WithinCandidate, nil
}

// leafSortedNumeric resolves the per-leaf SortedNumericDocValues
// iterator for field. Uses the same narrow type assertion the rest of
// the search package uses (defaultLongDistanceLeafLookup,
// xy_point_in_geometry_query, ...) so the query is forward compatible
// with any LeafReader that exposes GetSortedNumericDocValues.
func leafSortedNumeric(ctx *index.LeafReaderContext, field string) (index.SortedNumericDocValues, error) {
	if ctx == nil {
		return nil, nil
	}
	leaf := ctx.LeafReader()
	if leaf == nil {
		return nil, nil
	}
	type docValuesReader interface {
		GetSortedNumericDocValues(field string) (index.SortedNumericDocValues, error)
	}
	r, ok := leaf.(docValuesReader)
	if !ok {
		return nil, nil
	}
	return r.GetSortedNumericDocValues(field)
}

// sortedNumericApproximation adapts a SortedNumericDocValues iterator
// into a DocIdSetIterator so it can serve as the approximation pass of
// a TwoPhaseIterator. The reference Java code constructs the
// TwoPhaseIterator directly from values (Java's
// SortedNumericDocValues extends DocIdSetIterator) — Gocene's
// SortedNumericDocValues interface deliberately omits the
// DocIdSetIterator surface (it carries per-doc Get instead of
// nextValue/docValueCount), so this adapter bridges the two contracts
// without changing the interface shape.
//
// Cost is approximated as the segment's maxDoc when known, matching
// the upper bound the Java reference reports for doc-values cost. The
// adapter does not implement DocIDRunEnd specially; it returns the
// generic DocIdSetIterator answer of "current doc only" via the
// returned NO_MORE_DOCS sentinel.
type sortedNumericApproximation struct {
	values index.SortedNumericDocValues
	cost   int64
	docID  int
}

// newSortedNumericApproximation wraps values; maxDoc seeds the cost
// estimate. A non-positive maxDoc falls back to 0 (the iterator will
// still drain correctly, only the cost hint is dampened).
func newSortedNumericApproximation(values index.SortedNumericDocValues, maxDoc int) *sortedNumericApproximation {
	cost := int64(0)
	if maxDoc > 0 {
		cost = int64(maxDoc)
	}
	return &sortedNumericApproximation{
		values: values,
		cost:   cost,
		docID:  -1,
	}
}

// DocID returns the current document id, or -1 before iteration / the
// search.NO_MORE_DOCS sentinel after exhaustion.
func (s *sortedNumericApproximation) DocID() int { return s.docID }

// NextDoc advances to the next document carrying a value and returns
// its id (or NO_MORE_DOCS when exhausted).
func (s *sortedNumericApproximation) NextDoc() (int, error) {
	id, err := s.values.NextDoc()
	if err != nil {
		return 0, err
	}
	if id == -1 || id >= int(int32(^uint32(0)>>1)) {
		// Java SortedNumericDocValues uses DocIdSetIterator.NO_MORE_DOCS
		// (Integer.MAX_VALUE = 2147483647). Gocene's index package
		// uses -1 as its NO_MORE_DOCS sentinel for doc-values
		// iterators (see index/postings_enum.go), so we accept both
		// and normalise to search.NO_MORE_DOCS for the
		// DocIdSetIterator contract.
		s.docID = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	s.docID = id
	return id, nil
}

// Advance jumps to the first document id >= target carrying a value
// and returns it (or NO_MORE_DOCS).
func (s *sortedNumericApproximation) Advance(target int) (int, error) {
	id, err := s.values.Advance(target)
	if err != nil {
		return 0, err
	}
	if id == -1 || id >= int(int32(^uint32(0)>>1)) {
		s.docID = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	s.docID = id
	return id, nil
}

// Cost returns the seeded cost estimate.
func (s *sortedNumericApproximation) Cost() int64 { return s.cost }

// DocIDRunEnd returns the current doc id + 1, matching the
// DocIdSetIterator default contract (no consecutive-run optimisation
// because doc-values iterators are sparse and unordered with respect
// to runs).
func (s *sortedNumericApproximation) DocIDRunEnd() int {
	if s.docID < 0 || s.docID == NO_MORE_DOCS {
		return s.docID
	}
	return s.docID + 1
}

// Ensure sortedNumericApproximation satisfies DocIdSetIterator.
var _ DocIdSetIterator = (*sortedNumericApproximation)(nil)
