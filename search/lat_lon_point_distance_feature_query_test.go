// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"math"
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestLatLonPointDistanceFeatureQuery_ConstructorValidation mirrors
// the Java constructor's IllegalArgumentException for bad lat/lon and
// non-positive pivot.
func TestLatLonPointDistanceFeatureQuery_ConstructorValidation(t *testing.T) {
	cases := []struct {
		name           string
		field          string
		lat, lon       float64
		pivot          float64
		wantErr        bool
		wantSubstrings []string
	}{
		{"valid", "loc", 38.7, -9.1, 1000, false, nil},
		{"empty field", "", 0, 0, 100, true, []string{"field"}},
		{"latitude too high", "loc", 91, 0, 100, true, []string{"latitude"}},
		{"latitude too low", "loc", -91, 0, 100, true, []string{"latitude"}},
		{"longitude too high", "loc", 0, 181, 100, true, []string{"longitude"}},
		{"longitude too low", "loc", 0, -181, 100, true, []string{"longitude"}},
		{"latitude NaN", "loc", math.NaN(), 0, 100, true, []string{"latitude"}},
		{"longitude NaN", "loc", 0, math.NaN(), 100, true, []string{"longitude"}},
		{"zero pivot", "loc", 0, 0, 0, true, []string{"pivot"}},
		{"negative pivot", "loc", 0, 0, -1, true, []string{"pivot"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewLatLonPointDistanceFeatureQuery(tc.field, tc.lat, tc.lon, tc.pivot)
			gotErr := err != nil
			if gotErr != tc.wantErr {
				t.Fatalf("got err=%v, want err=%v", err, tc.wantErr)
			}
		})
	}
}

// TestLatLonPointDistanceFeatureQuery_EqualsAndHashCode covers Java
// parity for the equality / hash contract over (field, originLat,
// originLon, pivotDistance).
func TestLatLonPointDistanceFeatureQuery_EqualsAndHashCode(t *testing.T) {
	mustQuery := func(t *testing.T, field string, lat, lon, pivot float64) *LatLonPointDistanceFeatureQuery {
		t.Helper()
		q, err := NewLatLonPointDistanceFeatureQuery(field, lat, lon, pivot)
		if err != nil {
			t.Fatalf("NewLatLonPointDistanceFeatureQuery: %v", err)
		}
		return q
	}

	q1 := mustQuery(t, "foo", 38.7, -9.1, 1000)
	q2 := mustQuery(t, "foo", 38.7, -9.1, 1000)
	if !q1.Equals(q2) || !q2.Equals(q1) {
		t.Fatalf("expected q1 == q2 for identical inputs")
	}
	if q1.HashCode() != q2.HashCode() {
		t.Fatalf("expected hash(q1) == hash(q2): %d vs %d", q1.HashCode(), q2.HashCode())
	}

	cases := []struct {
		name string
		q    *LatLonPointDistanceFeatureQuery
	}{
		{"different field", mustQuery(t, "bar", 38.7, -9.1, 1000)},
		{"different originLat", mustQuery(t, "foo", 39.7, -9.1, 1000)},
		{"different originLon", mustQuery(t, "foo", 38.7, -10.1, 1000)},
		{"different pivot", mustQuery(t, "foo", 38.7, -9.1, 2000)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if q1.Equals(tc.q) {
				t.Fatalf("expected q1 != %s", tc.name)
			}
		})
	}

	// Cross-type guard: a different Query implementation must not
	// compare equal even if Equals is asked to coerce.
	other := &LongDistanceFeatureQuery{}
	if q1.Equals(other) {
		t.Fatalf("LatLonPointDistanceFeatureQuery.Equals returned true for a LongDistanceFeatureQuery")
	}
}

// TestLatLonPointDistanceFeatureQuery_String_AndVisit covers the
// textual representation and the visit dispatch logic.
func TestLatLonPointDistanceFeatureQuery_String_AndVisit(t *testing.T) {
	q, err := NewLatLonPointDistanceFeatureQuery("foo", 38.7, -9.1, 1000)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	got := q.String()
	wantSubstrings := []string{
		"LatLonPointDistanceFeatureQuery(",
		"field=foo",
		"originLat=38.7",
		"originLon=-9.1",
		"pivotDistance=1000",
	}
	for _, s := range wantSubstrings {
		if !latLonContains(got, s) {
			t.Fatalf("String() = %q; missing substring %q", got, s)
		}
	}

	visited := &recordingVisitor{}
	q.Visit(visited)
	if !visited.acceptedField {
		t.Fatalf("expected AcceptField(\"foo\") to be invoked")
	}
	if visited.leaf != q {
		t.Fatalf("expected VisitLeaf to receive the query, got %v", visited.leaf)
	}

	rejecting := &recordingVisitor{rejectField: "foo"}
	q.Visit(rejecting)
	if rejecting.leaf != nil {
		t.Fatalf("expected VisitLeaf not to fire when AcceptField returns false")
	}
}

// latLonContains is a small substring helper. We avoid importing
// strings to keep the test imports tight and consistent with the
// long-distance test file; the file-scoped name avoids collision
// with the package's existing `contains` helper.
func latLonContains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// TestLatLonPointDistanceFeatureQuery_Clone verifies Clone returns an
// independent shallow copy that compares equal to the original.
func TestLatLonPointDistanceFeatureQuery_Clone(t *testing.T) {
	q, err := NewLatLonPointDistanceFeatureQuery("loc", 38.7, -9.1, 5000)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	c := q.Clone()
	if c == q {
		t.Fatalf("Clone returned the same pointer; expected a copy")
	}
	if !c.Equals(q) {
		t.Fatalf("Clone result does not Equal original")
	}
	cc, ok := c.(*LatLonPointDistanceFeatureQuery)
	if !ok {
		t.Fatalf("Clone returned %T, want *LatLonPointDistanceFeatureQuery", c)
	}
	if cc.Field() != q.Field() || cc.OriginLat() != q.OriginLat() ||
		cc.OriginLon() != q.OriginLon() || cc.PivotDistance() != q.PivotDistance() {
		t.Fatalf("Clone copy diverges from original: %+v vs %+v", cc, q)
	}
}

// TestLatLonPointDistanceFeatureQuery_Rewrite verifies Rewrite is a
// no-op for this query (returns itself).
func TestLatLonPointDistanceFeatureQuery_Rewrite(t *testing.T) {
	q, err := NewLatLonPointDistanceFeatureQuery("loc", 38.7, -9.1, 5000)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	got, err := q.Rewrite(nil)
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	if got != q {
		t.Fatalf("Rewrite returned %v, want %v (same pointer)", got, q)
	}
}

// TestLatLonPointDistanceFeatureQuery_Accessors smoke-tests the
// exported getters so refactors to the underlying field names cannot
// silently flip values.
func TestLatLonPointDistanceFeatureQuery_Accessors(t *testing.T) {
	q, err := NewLatLonPointDistanceFeatureQuery("loc", 38.7223, -9.1393, 1234.5)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	if got, want := q.Field(), "loc"; got != want {
		t.Errorf("Field() = %q, want %q", got, want)
	}
	if got, want := q.OriginLat(), 38.7223; got != want {
		t.Errorf("OriginLat() = %v, want %v", got, want)
	}
	if got, want := q.OriginLon(), -9.1393; got != want {
		t.Errorf("OriginLon() = %v, want %v", got, want)
	}
	if got, want := q.PivotDistance(), 1234.5; got != want {
		t.Errorf("PivotDistance() = %v, want %v", got, want)
	}
}

// encodeLatLonValue packs (lat, lon) degrees into the 64-bit Lucene
// SortedNumericDocValues encoding: (latBits<<32) | (lonBits & 0xFFFFFFFF).
func encodeLatLonValue(lat, lon float64) int64 {
	latBits := uint64(uint32(geo.EncodeLatitude(lat))) //nolint:gosec // signed→unsigned reinterpret is intentional for bit-packing
	lonBits := uint64(uint32(geo.EncodeLongitude(lon)))
	return int64((latBits << 32) | lonBits) //nolint:gosec // 64-bit signed view of the bit pattern
}

// inMemoryLatLonPointDocValues implements latLonPointDocValues over a
// sorted map of docID -> []encodedValue. It applies the
// selectClosestLatLonValue helper to mirror Lucene's multi-value
// selection.
type inMemoryLatLonPointDocValues struct {
	docs      []int
	values    [][]int64
	originLat float64
	originLon float64
	idx       int
	current   int64
	hasVal    bool
}

func newInMemoryLatLonPointDocValues(originLat, originLon float64, docs map[int][]int64) *inMemoryLatLonPointDocValues {
	keys := make([]int, 0, len(docs))
	for k := range docs {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	values := make([][]int64, len(keys))
	for i, k := range keys {
		values[i] = append([]int64(nil), docs[k]...)
	}
	return &inMemoryLatLonPointDocValues{
		docs:      keys,
		values:    values,
		originLat: originLat,
		originLon: originLon,
		idx:       -1,
	}
}

func (it *inMemoryLatLonPointDocValues) AdvanceExact(doc int) (bool, error) {
	for i, d := range it.docs {
		if d == doc {
			it.idx = i
			it.current = selectClosestLatLonValue(it.values[i], it.originLat, it.originLon)
			it.hasVal = true
			return true, nil
		}
		if d > doc {
			it.idx = i
			it.hasVal = false
			return false, nil
		}
	}
	it.idx = len(it.docs)
	it.hasVal = false
	return false, nil
}

func (it *inMemoryLatLonPointDocValues) EncodedValue() (int64, error) {
	if !it.hasVal {
		return 0, errors.New("inMemoryLatLonPointDocValues: no value at current position")
	}
	return it.current, nil
}

func (it *inMemoryLatLonPointDocValues) DocID() int {
	if it.idx < 0 {
		return -1
	}
	if it.idx >= len(it.docs) {
		return NO_MORE_DOCS
	}
	return it.docs[it.idx]
}

func (it *inMemoryLatLonPointDocValues) NextDoc() (int, error) {
	it.idx++
	if it.idx >= len(it.docs) {
		it.hasVal = false
		return NO_MORE_DOCS, nil
	}
	it.current = selectClosestLatLonValue(it.values[it.idx], it.originLat, it.originLon)
	it.hasVal = true
	return it.docs[it.idx], nil
}

func (it *inMemoryLatLonPointDocValues) Advance(target int) (int, error) {
	for i := it.idx + 1; i < len(it.docs); i++ {
		if it.docs[i] >= target {
			it.idx = i
			it.current = selectClosestLatLonValue(it.values[i], it.originLat, it.originLon)
			it.hasVal = true
			return it.docs[i], nil
		}
	}
	it.idx = len(it.docs)
	it.hasVal = false
	return NO_MORE_DOCS, nil
}

func (it *inMemoryLatLonPointDocValues) Cost() int64 { return int64(len(it.docs)) }

// inMemoryLatLonPointPointSource is a flat-scan
// latLonPointDistanceFeaturePointSource fixture. Per doc it emits the
// closest-to-origin encoded value through VisitWithPackedValue, after
// running the visitor's Compare on the single-point cell so the
// dateline / bounding-box logic is exercised.
type inMemoryLatLonPointPointSource struct {
	docs   []int
	values []int64 // closest-to-origin packed (lat, lon) per doc
}

func newInMemoryLatLonPointPointSource(originLat, originLon float64, docs map[int][]int64) *inMemoryLatLonPointPointSource {
	keys := make([]int, 0, len(docs))
	for k := range docs {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	values := make([]int64, len(keys))
	for i, k := range keys {
		values[i] = selectClosestLatLonValue(docs[k], originLat, originLon)
	}
	return &inMemoryLatLonPointPointSource{docs: keys, values: values}
}

func (p *inMemoryLatLonPointPointSource) Intersect(visitor latLonPointDistanceFeaturePointVisitor) error {
	if len(p.docs) == 0 {
		return nil
	}
	visitor.Grow(len(p.docs))
	for i, doc := range p.docs {
		buf := packLatLonBytes(p.values[i])
		// Run Compare on the degenerate min==max cell so the
		// dateline / range logic is exercised; if the cell is
		// outside, skip the per-point visit (mirroring how a real
		// BKD walker would behave).
		rel := visitor.Compare(buf, buf)
		if rel == latLonPointDistanceFeatureCellOutsideQuery {
			continue
		}
		if err := visitor.VisitWithPackedValue(doc, buf); err != nil {
			return err
		}
	}
	return nil
}

func (p *inMemoryLatLonPointPointSource) EstimatePointCountGreaterThanOrEqualTo(_ latLonPointDistanceFeaturePointVisitor, threshold int64) bool {
	return int64(len(p.docs)) >= threshold
}

// packLatLonBytes lays out the 8-byte (lat||lon) sortable-bytes
// payload from a 64-bit encoded value. Mirrors the Lucene wire form.
func packLatLonBytes(encoded int64) []byte {
	buf := make([]byte, 2*latLonPointBytesPerDim)
	util.IntToSortableBytes(int32(encoded>>32), buf, 0)
	util.IntToSortableBytes(int32(encoded&0xFFFFFFFF), buf, latLonPointBytesPerDim)
	return buf
}

// buildLatLonScorer wires a LatLonPointDistanceFeatureQuery against
// the supplied in-memory doc-values and point source, then returns
// the per-segment scorer ready for iteration.
func buildLatLonScorer(t *testing.T, q *LatLonPointDistanceFeatureQuery, weight float32, maxDoc int, dv latLonPointDocValues, pts latLonPointDistanceFeaturePointSource) Scorer {
	t.Helper()
	q.installTestLeafLookup(func(_ *index.LeafReaderContext, field string) (latLonPointDocValues, latLonPointDistanceFeaturePointSource, error) {
		if field != q.Field() {
			return nil, nil, nil
		}
		return dv, pts, nil
	})
	searcher := NewIndexSearcher(nil)
	w, err := q.CreateWeight(searcher, true, weight)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	leaf := index.NewLeafReader(nil)
	ctx := index.NewLeafReaderContext(leaf, nil, 0, 0)
	supplier, err := w.ScorerSupplier(ctx)
	if err != nil {
		t.Fatalf("ScorerSupplier: %v", err)
	}
	if supplier == nil {
		t.Fatalf("ScorerSupplier returned nil")
	}
	if s, ok := supplier.(*latLonPointDistanceFeatureScorerSupplier); ok {
		s.maxDoc = maxDoc
	}
	scorer, err := supplier.Get(int64(maxDoc))
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if scorer == nil {
		t.Fatalf("Get returned nil scorer")
	}
	return scorer
}

// latLonExpectedScore replicates the Java score formula so tests stay
// readable: weight * (pivot / (pivot + distanceMeters)).
func latLonExpectedScore(weight float32, pivot, distanceMeters float64) float32 {
	return float32(float64(weight) * (pivot / (pivot + distanceMeters)))
}

// haversineMeters resolves the Lucene-style haversine distance for a
// (lat, lon) pair encoded as the 64-bit packed value used by the
// scorer. Exists so tests can compute the same distance the scorer
// computes without re-encoding/decoding inline.
func haversineMeters(encoded int64, originLat, originLon float64) float64 {
	lat := geo.DecodeLatitude(int32(encoded >> 32))
	lon := geo.DecodeLongitude(int32(encoded & 0xFFFFFFFF))
	return util.HaversinMeters(originLat, originLon, lat, lon)
}

// TestLatLonPointDistanceFeatureQuery_Basics exercises a small
// hand-built fixture: 4 docs with single lat/lon points, origin at
// Lisbon, pivot at 100 km. The closest doc must rank first; the
// emitted score must match the Java formula to within float32
// rounding.
func TestLatLonPointDistanceFeatureQuery_Basics(t *testing.T) {
	const (
		originLat = 38.7223
		originLon = -9.1393
	)
	// Approximate distances from Lisbon (km): Cascais ~25, Sintra ~30,
	// Porto ~275, Faro ~280. Encoded as the Lucene packed format.
	docs := map[int][]int64{
		0: {encodeLatLonValue(38.6979, -9.4215)}, // Cascais
		1: {encodeLatLonValue(38.8009, -9.3848)}, // Sintra
		2: {encodeLatLonValue(41.1579, -8.6291)}, // Porto
		3: {encodeLatLonValue(37.0194, -7.9304)}, // Faro
	}
	const (
		pivot  = 100_000.0 // 100 km
		weight = 3.0
	)
	q, err := NewLatLonPointDistanceFeatureQuery("loc", originLat, originLon, pivot)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	dv := newInMemoryLatLonPointDocValues(originLat, originLon, docs)
	pts := newInMemoryLatLonPointPointSource(originLat, originLon, docs)
	scorer := buildLatLonScorer(t, q, weight, len(docs), dv, pts)

	type docScore struct {
		doc   int
		score float32
	}
	var got []docScore
	for {
		doc, err := scorer.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if doc == NO_MORE_DOCS {
			break
		}
		got = append(got, docScore{doc: doc, score: scorer.Score()})
	}
	if len(got) != len(docs) {
		t.Fatalf("expected %d hits, got %d (%v)", len(docs), len(got), got)
	}

	// Compute the brute-force expected ranking from the same fixture
	// using the live haversine, then sort by descending score.
	expected := make([]docScore, 0, len(docs))
	for d, vs := range docs {
		dist := haversineMeters(vs[0], originLat, originLon)
		expected = append(expected, docScore{doc: d, score: latLonExpectedScore(weight, pivot, dist)})
	}
	sort.SliceStable(expected, func(i, j int) bool {
		if expected[i].score != expected[j].score {
			return expected[i].score > expected[j].score
		}
		return expected[i].doc < expected[j].doc
	})

	// Sort the scorer output the same way so docID tie-breaks match.
	sort.SliceStable(got, func(i, j int) bool {
		if got[i].score != got[j].score {
			return got[i].score > got[j].score
		}
		return got[i].doc < got[j].doc
	})

	for i := range expected {
		if got[i].doc != expected[i].doc {
			t.Fatalf("hit %d: doc = %d, want %d (got=%v expected=%v)", i, got[i].doc, expected[i].doc, got, expected)
		}
		if !latLonApproxEqual(got[i].score, expected[i].score) {
			t.Fatalf("hit %d: score = %v, want %v (got=%v expected=%v)", i, got[i].score, expected[i].score, got, expected)
		}
	}

	// Sanity: Sintra (doc 1, ~23 km) is marginally closer to Lisbon
	// than Cascais (doc 0, ~25 km), so doc 1 must rank first under a
	// 100 km pivot.
	if got[0].doc != 1 {
		t.Fatalf("expected doc 1 (Sintra) to rank first, got %d", got[0].doc)
	}
}

func latLonApproxEqual(a, b float32) bool {
	if a == b {
		return true
	}
	const eps = 1e-6
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < eps
}

// TestLatLonPointDistanceFeatureQuery_MissingField verifies the
// supplier short-circuits to nil when the field has no doc/point
// values, mirroring the Java early-return.
func TestLatLonPointDistanceFeatureQuery_MissingField(t *testing.T) {
	q, err := NewLatLonPointDistanceFeatureQuery("loc", 38.7, -9.1, 1000)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	q.installTestLeafLookup(func(_ *index.LeafReaderContext, _ string) (latLonPointDocValues, latLonPointDistanceFeaturePointSource, error) {
		return nil, nil, nil
	})
	searcher := NewIndexSearcher(nil)
	w, err := q.CreateWeight(searcher, true, 1)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	leaf := index.NewLeafReader(nil)
	ctx := index.NewLeafReaderContext(leaf, nil, 0, 0)
	scorer, err := w.Scorer(ctx)
	if err != nil {
		t.Fatalf("Scorer: %v", err)
	}
	if scorer != nil {
		t.Fatalf("expected nil scorer when field is missing, got %T", scorer)
	}
}

// TestLatLonPointDistanceFeatureQuery_MissingValue exercises a fixture
// where one doc has no value: it must not appear in the emitted hits.
func TestLatLonPointDistanceFeatureQuery_MissingValue(t *testing.T) {
	const (
		originLat = 38.7223
		originLon = -9.1393
		pivot     = 100_000.0
		weight    = 2.0
	)
	docs := map[int][]int64{
		0: {encodeLatLonValue(38.6979, -9.4215)}, // Cascais
		// doc 1: no value
		2: {encodeLatLonValue(38.8009, -9.3848)}, // Sintra
	}
	q, err := NewLatLonPointDistanceFeatureQuery("loc", originLat, originLon, pivot)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	dv := newInMemoryLatLonPointDocValues(originLat, originLon, docs)
	pts := newInMemoryLatLonPointPointSource(originLat, originLon, docs)
	scorer := buildLatLonScorer(t, q, weight, 3, dv, pts)
	seen := map[int]bool{}
	for {
		doc, err := scorer.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if doc == NO_MORE_DOCS {
			break
		}
		seen[doc] = true
	}
	if seen[1] {
		t.Fatalf("doc 1 (no value) should not appear in hits, got %v", seen)
	}
	if !seen[0] || !seen[2] {
		t.Fatalf("expected docs 0 and 2 to appear, got %v", seen)
	}
}

// TestLatLonPointDistanceFeatureQuery_MultiValued exercises the
// selectClosestLatLonValue path: a doc with multiple (lat, lon)
// points should score using the point closest to origin (smallest
// haversine sort key).
func TestLatLonPointDistanceFeatureQuery_MultiValued(t *testing.T) {
	const (
		originLat = 38.7223
		originLon = -9.1393
		pivot     = 100_000.0
		weight    = 3.0
	)
	// Doc 0 carries (Porto, Cascais, Faro): the closest to Lisbon is
	// Cascais.
	docs := map[int][]int64{
		0: {
			encodeLatLonValue(41.1579, -8.6291), // Porto
			encodeLatLonValue(38.6979, -9.4215), // Cascais (closest)
			encodeLatLonValue(37.0194, -7.9304), // Faro
		},
	}
	q, err := NewLatLonPointDistanceFeatureQuery("loc", originLat, originLon, pivot)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	dv := newInMemoryLatLonPointDocValues(originLat, originLon, docs)
	pts := newInMemoryLatLonPointPointSource(originLat, originLon, docs)
	scorer := buildLatLonScorer(t, q, weight, 1, dv, pts)

	doc, err := scorer.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	if doc != 0 {
		t.Fatalf("expected doc 0, got %d", doc)
	}
	got := scorer.Score()
	cascais := encodeLatLonValue(38.6979, -9.4215)
	want := latLonExpectedScore(weight, pivot, haversineMeters(cascais, originLat, originLon))
	if !latLonApproxEqual(got, want) {
		t.Fatalf("score = %v, want %v (selectClosestLatLonValue mismatch)", got, want)
	}
}

// TestLatLonPointDistanceFeatureQuery_Explain verifies the explain
// output: matching docs report match=true, the score value matches
// the Java formula, and the sub-explanations cover the 7 leaves the
// Java reference emits.
func TestLatLonPointDistanceFeatureQuery_Explain(t *testing.T) {
	const (
		originLat = 38.7223
		originLon = -9.1393
		pivot     = 100_000.0
		weight    = 2.0
	)
	docs := map[int][]int64{
		0: {encodeLatLonValue(38.6979, -9.4215)}, // Cascais
		1: {encodeLatLonValue(38.8009, -9.3848)}, // Sintra
	}
	q, err := NewLatLonPointDistanceFeatureQuery("loc", originLat, originLon, pivot)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	dv := newInMemoryLatLonPointDocValues(originLat, originLon, docs)
	pts := newInMemoryLatLonPointPointSource(originLat, originLon, docs)
	q.installTestLeafLookup(func(_ *index.LeafReaderContext, _ string) (latLonPointDocValues, latLonPointDistanceFeaturePointSource, error) {
		return dv, pts, nil
	})
	searcher := NewIndexSearcher(nil)
	w, err := q.CreateWeight(searcher, true, weight)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	leaf := index.NewLeafReader(nil)
	ctx := index.NewLeafReaderContext(leaf, nil, 0, 0)

	ex, err := w.Explain(ctx, 0)
	if err != nil {
		t.Fatalf("Explain(0): %v", err)
	}
	if !ex.IsMatch() {
		t.Fatalf("expected match explanation for doc 0, got %+v", ex)
	}
	wantScore := latLonExpectedScore(weight, pivot, haversineMeters(docs[0][0], originLat, originLon))
	if !latLonApproxEqual(ex.GetValue(), wantScore) {
		t.Fatalf("explain value = %v, want %v", ex.GetValue(), wantScore)
	}
	// Java reference emits 7 leaves: weight, pivot, originLat,
	// originLon, current lat, current lon, distance.
	if len(ex.GetDetails()) != 7 {
		t.Fatalf("expected 7 sub-explanations, got %d", len(ex.GetDetails()))
	}

	// Non-matching doc (doc 5: no value).
	ex2, err := w.Explain(ctx, 5)
	if err != nil {
		t.Fatalf("Explain(5): %v", err)
	}
	if ex2.IsMatch() {
		t.Fatalf("expected no-match explanation for doc 5, got match")
	}
}

// TestLatLonPointDistanceFeatureQuery_MaxScore verifies the per-scorer
// GetMaxScore returns the configured weight, matching the Java
// override on DistanceScorer.
func TestLatLonPointDistanceFeatureQuery_MaxScore(t *testing.T) {
	const (
		originLat = 38.7
		originLon = -9.1
		pivot     = 1000.0
	)
	docs := map[int][]int64{0: {encodeLatLonValue(38.6979, -9.4215)}}
	q, err := NewLatLonPointDistanceFeatureQuery("loc", originLat, originLon, pivot)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	const weight float32 = 4.25
	dv := newInMemoryLatLonPointDocValues(originLat, originLon, docs)
	pts := newInMemoryLatLonPointPointSource(originLat, originLon, docs)
	scorer := buildLatLonScorer(t, q, weight, 1, dv, pts)
	if got := scorer.GetMaxScore(0); got != weight {
		t.Fatalf("GetMaxScore = %v, want %v", got, weight)
	}
}

// TestLatLonPointDistanceFeatureQuery_SetMinCompetitiveScore_AboveBoost
// verifies that setting a minScore above the boost replaces the
// iterator with an empty one, matching the Java early-return.
func TestLatLonPointDistanceFeatureQuery_SetMinCompetitiveScore_AboveBoost(t *testing.T) {
	const (
		originLat = 38.7
		originLon = -9.1
		pivot     = 1000.0
		weight    = 3.0
	)
	docs := map[int][]int64{
		0: {encodeLatLonValue(38.6979, -9.4215)},
		1: {encodeLatLonValue(38.8009, -9.3848)},
	}
	q, err := NewLatLonPointDistanceFeatureQuery("loc", originLat, originLon, pivot)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	dv := newInMemoryLatLonPointDocValues(originLat, originLon, docs)
	pts := newInMemoryLatLonPointPointSource(originLat, originLon, docs)
	scorer := buildLatLonScorer(t, q, weight, 2, dv, pts).(*latLonPointDistanceFeatureScorer)
	if err := scorer.SetMinCompetitiveScore(weight + 1); err != nil {
		t.Fatalf("SetMinCompetitiveScore: %v", err)
	}
	doc, err := scorer.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	if doc != NO_MORE_DOCS {
		t.Fatalf("expected NO_MORE_DOCS after minScore > boost, got %d", doc)
	}
}

// TestLatLonPointDistanceFeatureQuery_Random fuzz-tests the scorer
// against arbitrary origins / pivots / weights: the emitted ranking
// must agree with a brute-force haversine sort.
func TestLatLonPointDistanceFeatureQuery_Random(t *testing.T) {
	rng := rand.New(rand.NewPCG(0xC0FFEE, 0xBEEF)) //nolint:gosec // deterministic seeding for reproducibility
	const numDocs = 200
	docs := make(map[int][]int64, numDocs)
	for i := 0; i < numDocs; i++ {
		lat := -90 + rng.Float64()*180
		lon := -180 + rng.Float64()*360
		docs[i] = []int64{encodeLatLonValue(lat, lon)}
	}

	for iter := 0; iter < 5; iter++ {
		originLat := -90 + rng.Float64()*180
		originLon := -180 + rng.Float64()*360
		pivot := 100 + rng.Float64()*1_000_000
		weight := float32(1+rng.IntN(10)) / 3

		q, err := NewLatLonPointDistanceFeatureQuery("loc", originLat, originLon, pivot)
		if err != nil {
			t.Fatalf("iter=%d constructor: %v", iter, err)
		}
		dv := newInMemoryLatLonPointDocValues(originLat, originLon, docs)
		pts := newInMemoryLatLonPointPointSource(originLat, originLon, docs)
		scorer := buildLatLonScorer(t, q, weight, numDocs, dv, pts)

		type hit struct {
			doc   int
			score float32
		}
		var got []hit
		for {
			doc, err := scorer.NextDoc()
			if err != nil {
				t.Fatalf("iter=%d NextDoc: %v", iter, err)
			}
			if doc == NO_MORE_DOCS {
				break
			}
			got = append(got, hit{doc: doc, score: scorer.Score()})
		}
		if len(got) != numDocs {
			t.Fatalf("iter=%d expected %d hits, got %d", iter, numDocs, len(got))
		}

		// Build the expected ranking via brute-force haversine sort.
		type known struct {
			doc  int
			dist float64
		}
		all := make([]known, 0, numDocs)
		for d := 0; d < numDocs; d++ {
			all = append(all, known{doc: d, dist: haversineMeters(docs[d][0], originLat, originLon)})
		}
		sort.Slice(all, func(i, j int) bool {
			if all[i].dist != all[j].dist {
				return all[i].dist < all[j].dist
			}
			return all[i].doc < all[j].doc
		})

		sort.SliceStable(got, func(i, j int) bool {
			if got[i].score != got[j].score {
				return got[i].score > got[j].score
			}
			return got[i].doc < got[j].doc
		})

		for i := 0; i < 3 && i < len(all); i++ {
			want := latLonExpectedScore(weight, pivot, all[i].dist)
			if !latLonApproxEqual(got[i].score, want) {
				t.Fatalf("iter=%d top-%d: doc score = %v, want %v (doc=%d dist=%v)",
					iter, i, got[i].score, want, all[i].doc, all[i].dist)
			}
		}
	}
}

// TestComputeLatLonDistanceFeatureScore covers the score formula
// helper directly: the formula must be boost * pivot/(pivot+distance)
// with float32 narrowing, and negative/NaN distances must collapse to
// zero rather than emit garbage.
func TestComputeLatLonDistanceFeatureScore(t *testing.T) {
	cases := []struct {
		name           string
		boost          float32
		pivot, dist    float64
		want           float32
		useApproxCheck bool
	}{
		{"distance zero", 2, 100, 0, 2, false},
		{"distance equals pivot", 2, 100, 100, 1, false},
		{"distance much bigger than pivot", 2, 100, 1e9, 0, true},
		{"negative distance treated as zero", 2, 100, -1, 0, false},
		{"NaN distance treated as zero", 2, 100, math.NaN(), 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := computeLatLonDistanceFeatureScore(tc.boost, tc.pivot, tc.dist)
			if tc.useApproxCheck {
				if math.Abs(float64(got-tc.want)) > 1e-6 {
					t.Fatalf("got %v, want ~%v", got, tc.want)
				}
				return
			}
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestSelectClosestLatLonValue covers the multi-valued selection
// helper directly.
func TestSelectClosestLatLonValue(t *testing.T) {
	const (
		originLat = 38.7223
		originLon = -9.1393
	)
	cascais := encodeLatLonValue(38.6979, -9.4215)
	sintra := encodeLatLonValue(38.8009, -9.3848)
	porto := encodeLatLonValue(41.1579, -8.6291)
	faro := encodeLatLonValue(37.0194, -7.9304)

	cases := []struct {
		name string
		in   []int64
		want int64
	}{
		{"single value", []int64{cascais}, cascais},
		{"two values, first closer", []int64{cascais, porto}, cascais},
		{"two values, second closer", []int64{porto, sintra}, sintra},
		{"three values, middle closer", []int64{faro, cascais, porto}, cascais},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := selectClosestLatLonValue(tc.in, originLat, originLon)
			if got != tc.want {
				t.Fatalf("selectClosestLatLonValue = %x, want %x", got, tc.want)
			}
		})
	}
}

// TestLatLonPointDistanceFeaturePointVisitor_Compare covers the
// visitor's Compare callback for both non-dateline and dateline-aware
// cells.
func TestLatLonPointDistanceFeaturePointVisitor_Compare(t *testing.T) {
	build := func(crossDateLine bool, minLat, maxLat, minLon, maxLon int32) *latLonPointDistanceFeaturePointVisitorImpl {
		return &latLonPointDistanceFeaturePointVisitorImpl{
			minLat:        minLat,
			maxLat:        maxLat,
			minLon:        minLon,
			maxLon:        maxLon,
			crossDateLine: crossDateLine,
			result:        util.NewDocIdSetBuilder(1),
			alreadyAt:     -1,
		}
	pack := func(lat, lon float64) []byte {
		buf := make([]byte, 2*latLonPointBytesPerDim)
		util.IntToSortableBytes(geo.EncodeLatitude(lat), buf, 0)
		util.IntToSortableBytes(geo.EncodeLongitude(lon), buf, latLonPointBytesPerDim)
		return buf
	}

	// Non-dateline rectangle covering Iberia roughly: lat in
	// [35, 44], lon in [-10, 5].
	v := build(false,
		geo.EncodeLatitude(35), geo.EncodeLatitude(44),
		geo.EncodeLongitude(-10), geo.EncodeLongitude(5))

	// Cell fully inside.
	if rel := v.Compare(pack(38, -5), pack(40, 0)); rel != latLonPointDistanceFeatureCellInsideQuery {
		t.Fatalf("expected INSIDE for in-range cell, got %v", rel)
	}
	// Cell fully outside (latitude too high).
	if rel := v.Compare(pack(60, 0), pack(70, 5)); rel != latLonPointDistanceFeatureCellOutsideQuery {
		t.Fatalf("expected OUTSIDE for north-of-range cell, got %v", rel)
	}
	// Cell crossing the boundary.
	if rel := v.Compare(pack(30, -5), pack(40, 0)); rel != latLonPointDistanceFeatureCellCrossesQuery {
		t.Fatalf("expected CROSSES for boundary-crossing cell, got %v", rel)
	}
}