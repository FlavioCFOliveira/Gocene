// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// latLonDocValuesBoxQuery matches documents whose indexed lat/lon doc
// values intersect a geographic bounding box.
//
// It is the Go port of the package-private
// org.apache.lucene.document.LatLonDocValuesBoxQuery (Lucene 10.4.0).
// Lucene keeps the class package-private and exposes it solely through
// the LatLonDocValuesField.newSlowBoxQuery factory; Gocene mirrors
// that invariant by keeping the struct unexported and routing
// construction through NewLatLonDocValuesBoxQuery.
//
// # Doc-values shape
//
// The field must be indexed with [document.NewLatLonDocValuesField],
// which packs (latitude, longitude) into a single int64 with the
// layout
//
//	high32(value) = encoded latitude
//	low32(value)  = encoded longitude
//
// matching the reference's setLocationValue bit layout. The query
// resolves the per-leaf [index.SortedNumericDocValues] iterator and
// scans each indexed value against the rounded encoded bounding box.
//
// # Construction-time validation
//
// Mirrors the Java reference exactly: latitudes and longitudes are
// validated via [geo.CheckLatitude] / [geo.CheckLongitude] before any
// encoding so that callers get an error (rather than a panic from the
// downstream encoder) when bounds are invalid.
//
// # Dateline crossing
//
// The Java reference computes `crossesDateline = minLongitude >
// maxLongitude` BEFORE rounding the longitudes, because rounding can
// flip strict inequality at the dateline edge. Gocene preserves that
// ordering precisely; the boolean is captured pre-rounding and frozen
// into the struct.
//
// # Encoded rounding policy
//
// Same as the Java reference:
//
//   - minLatitude  = EncodeLatitudeCeil(minLat)
//   - maxLatitude  = EncodeLatitude(maxLat)
//   - minLongitude = EncodeLongitudeCeil(minLon)
//   - maxLongitude = EncodeLongitude(maxLon)
//
// Ceil on the lower edge, floor (the default Encode*) on the upper
// edge — the standard half-open bucket convention shared with the
// BKD point queries on the same field.
type latLonDocValuesBoxQuery struct {
	*BaseQuery

	field                      string
	minLatitude, maxLatitude   int32
	minLongitude, maxLongitude int32
	crossesDateline            bool
}

// latLonDocValuesBoxQuery error sentinels mirror Lucene's exception
// texts where they map cleanly to Go idioms.
var (
	errLatLonDocValuesBoxQueryNilField = errors.New(
		"search: LatLonDocValuesBoxQuery field must not be null")
)

// NewLatLonDocValuesBoxQuery builds a doc-values box query bound to
// field and asserting intersection with the bounding box
// [minLat..maxLat] x [minLon..maxLon].
//
// Mirrors the Java constructor LatLonDocValuesBoxQuery(String, double,
// double, double, double). The constructor validates latitude and
// longitude bounds explicitly via [geo.CheckLatitude] /
// [geo.CheckLongitude] (the same checks Java's GeoUtils performs) so
// invalid inputs surface as returned errors rather than panics from
// the encoder.
//
// Longitude wrap-around (minLon > maxLon) is recognised as a
// dateline-crossing query and is computed BEFORE rounding the
// longitudes to encoded form, exactly as the Java reference does.
//
// Returns an error when:
//
//   - field is empty (Java throws "field must not be null").
//   - any of the four coordinate bounds fails validation.
func NewLatLonDocValuesBoxQuery(
	field string,
	minLatitude, maxLatitude, minLongitude, maxLongitude float64,
) (Query, error) {
	if err := geo.CheckLatitude(minLatitude); err != nil {
		return nil, fmt.Errorf("search: LatLonDocValuesBoxQuery: %w", err)
	}
	if err := geo.CheckLatitude(maxLatitude); err != nil {
		return nil, fmt.Errorf("search: LatLonDocValuesBoxQuery: %w", err)
	}
	if err := geo.CheckLongitude(minLongitude); err != nil {
		return nil, fmt.Errorf("search: LatLonDocValuesBoxQuery: %w", err)
	}
	if err := geo.CheckLongitude(maxLongitude); err != nil {
		return nil, fmt.Errorf("search: LatLonDocValuesBoxQuery: %w", err)
	}
	if field == "" {
		return nil, errLatLonDocValuesBoxQueryNilField
	}
	return &latLonDocValuesBoxQuery{
		BaseQuery:       &BaseQuery{},
		field:           field,
		crossesDateline: minLongitude > maxLongitude, // pre-rounding (matches Java)
		minLatitude:     geo.EncodeLatitudeCeil(minLatitude),
		maxLatitude:     geo.EncodeLatitude(maxLatitude),
		minLongitude:    geo.EncodeLongitudeCeil(minLongitude),
		maxLongitude:    geo.EncodeLongitude(maxLongitude),
	}, nil
}

// String mirrors LatLonDocValuesBoxQuery.toString(String). Format:
//
//	[field:]box(minLat=<v>, maxLat=<v>, minLon=<v>, maxLon=<v>)
//
// The "field:" prefix is suppressed when the supplied default field
// matches the query's field, exactly like the Java reference. Decoded
// (not encoded) coordinates are emitted so the output is human-
// readable for diagnostics.
func (q *latLonDocValuesBoxQuery) String(field string) string {
	var sb strings.Builder
	if q.field != field {
		sb.WriteString(q.field)
		sb.WriteByte(':')
	}
	sb.WriteString("box(minLat=")
	fmt.Fprintf(&sb, "%v", geo.DecodeLatitude(q.minLatitude))
	sb.WriteString(", maxLat=")
	fmt.Fprintf(&sb, "%v", geo.DecodeLatitude(q.maxLatitude))
	sb.WriteString(", minLon=")
	fmt.Fprintf(&sb, "%v", geo.DecodeLongitude(q.minLongitude))
	sb.WriteString(", maxLon=")
	fmt.Fprintf(&sb, "%v", geo.DecodeLongitude(q.maxLongitude))
	sb.WriteByte(')')
	return sb.String()
}

// Equals mirrors LatLonDocValuesBoxQuery.equals: same class, same
// field, same dateline-crossing flag, and all four encoded bounds
// equal.
func (q *latLonDocValuesBoxQuery) Equals(other Query) bool {
	o, ok := other.(*latLonDocValuesBoxQuery)
	if !ok {
		return false
	}
	return q.field == o.field &&
		q.crossesDateline == o.crossesDateline &&
		q.minLatitude == o.minLatitude &&
		q.maxLatitude == o.maxLatitude &&
		q.minLongitude == o.minLongitude &&
		q.maxLongitude == o.maxLongitude
}

// HashCode mirrors LatLonDocValuesBoxQuery.hashCode. The Java
// reference seeds with classHash() (a per-Class constant) and folds
// the same six fields used by equals through 31*h + Integer.hashCode
// / Boolean.hashCode (which collapse to identity for the integers,
// and 1231/1237 for the boolean). Gocene uses a type-stable literal
// seed so distinct query classes never collide on otherwise-equal
// fields, and reproduces the same 31-fold pattern.
func (q *latLonDocValuesBoxQuery) HashCode() int {
	h := classHashLatLonDocValuesBoxQuery
	h = 31*h + stringHash(q.field)
	if q.crossesDateline {
		h = 31*h + 1231
	} else {
		h = 31*h + 1237
	}
	h = 31*h + int(q.minLatitude)
	h = 31*h + int(q.maxLatitude)
	h = 31*h + int(q.minLongitude)
	h = 31*h + int(q.maxLongitude)
	return h
}

// Visit mirrors LatLonDocValuesBoxQuery.visit(QueryVisitor): descend
// into the leaf only when the visitor accepts the query's field.
func (q *latLonDocValuesBoxQuery) Visit(visitor QueryVisitor) {
	if visitor.AcceptField(q.field) {
		visitor.VisitLeaf(q)
	}
}

// GetField returns the field this query is bound to.
func (q *latLonDocValuesBoxQuery) GetField() string { return q.field }

// CrossesDateline reports whether the bounding box wraps around the
// 180/-180 longitude meridian. Useful for tests and for diagnostic
// tooling that needs to display the query's wrap state without
// re-deriving it.
func (q *latLonDocValuesBoxQuery) CrossesDateline() bool { return q.crossesDateline }

// EncodedBounds returns the four encoded bounds in the order
// (minLat, maxLat, minLon, maxLon). The values are the post-rounding
// encoded forms, useful for assertions in tests.
func (q *latLonDocValuesBoxQuery) EncodedBounds() (int32, int32, int32, int32) {
	return q.minLatitude, q.maxLatitude, q.minLongitude, q.maxLongitude
}

// Clone returns the query itself. The struct is logically immutable
// (all fields are primitives captured at construction), so a shallow
// clone preserves query identity and equals semantics.
func (q *latLonDocValuesBoxQuery) Clone() Query { return q }

// CreateWeight builds a [ConstantScoreWeight] that resolves the
// per-leaf [index.SortedNumericDocValues] iterator and wraps a
// [TwoPhaseIterator] whose Matches method scans the indexed lat/lon
// values against the rounded encoded bounding box.
//
// The Java reference takes a ScoreMode; Gocene's Query.CreateWeight
// signature uses a needsScores bool, so the supplier infers the mode
// (true => COMPLETE, false => COMPLETE_NO_SCORES) and propagates it
// to the ConstantScoreScorer.
func (q *latLonDocValuesBoxQuery) CreateWeight(_ *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	mode := COMPLETE_NO_SCORES
	if needsScores {
		mode = COMPLETE
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
		tpi := NewTwoPhaseIterator(approx, func() (bool, error) {
			return boxMatches(
				values,
				approx.DocID(),
				q.minLatitude, q.maxLatitude,
				q.minLongitude, q.maxLongitude,
				q.crossesDateline,
			)
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

// Ensure latLonDocValuesBoxQuery implements Query.
var _ Query = (*latLonDocValuesBoxQuery)(nil)

// classHashLatLonDocValuesBoxQuery seeds the type-stable hash for
// this query. The literal ("LlBQ") makes the seed visually self-
// describing and distinct from every other classHash in the package.
const classHashLatLonDocValuesBoxQuery = 0x4c6c_4251 // "LlBQ"

// boxMatches scans a doc's packed lat/lon values and returns true as
// soon as one falls inside the encoded bounding box. Mirrors the
// Java TwoPhaseIterator.matches() body byte-for-byte:
//
//   - latitude is the high 32 bits of the packed value;
//   - longitude is the low 32 bits;
//   - latitude must lie within [minLat..maxLat];
//   - longitude must lie within [minLon..maxLon] when the box does
//     not cross the dateline, or OUTSIDE (maxLon, minLon) when it
//     does (the wrap-around region).
//
// The Java reference uses signed comparisons on int32 throughout —
// the encoded form is signed precisely so that the natural numeric
// order matches the geographic order — and so does this port.
func boxMatches(
	values index.SortedNumericDocValues,
	docID int,
	minLat, maxLat, minLon, maxLon int32,
	crossesDateline bool,
) (bool, error) {
	vs, err := index.DrainSortedNumeric(values, docID)
	if err != nil {
		return false, err
	}
	for _, v := range vs {
		lat := int32(uint64(v) >> 32)
		if lat < minLat || lat > maxLat {
			continue
		}
		lon := int32(v & 0xFFFFFFFF)
		if crossesDateline {
			if lon > maxLon && lon < minLon {
				continue
			}
		} else {
			if lon < minLon || lon > maxLon {
				continue
			}
		}
		return true, nil
	}
	return false, nil
}
