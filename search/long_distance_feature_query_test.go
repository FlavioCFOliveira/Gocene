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

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestLongDistanceFeatureQuery_EqualsAndHashCode mirrors
// TestLongDistanceFeatureQuery.testEqualsAndHashcode (Lucene 10.4.0).
// The Java test exercises the factory built around BoostQuery; in
// Gocene we test both the wrapped result of the factory and the bare
// LongDistanceFeatureQuery underneath so the hash and equality
// invariants are visible at both layers.
func TestLongDistanceFeatureQuery_EqualsAndHashCode(t *testing.T) {
	mustQuery := func(t *testing.T, field string, weight float32, origin, pivot int64) Query {
		t.Helper()
		q, err := LongFieldNewDistanceFeatureQuery(field, weight, origin, pivot)
		if err != nil {
			t.Fatalf("LongFieldNewDistanceFeatureQuery returned error: %v", err)
		}
		return q
	}

	q1 := mustQuery(t, "foo", 3, 10, 5)
	q2 := mustQuery(t, "foo", 3, 10, 5)

	if !q1.Equals(q2) || !q2.Equals(q1) {
		t.Fatalf("expected q1 == q2 for identical inputs")
	}
	if q1.HashCode() != q2.HashCode() {
		t.Fatalf("expected hash(q1) == hash(q2): %d vs %d", q1.HashCode(), q2.HashCode())
	}

	q3 := mustQuery(t, "bar", 3, 10, 5)
	if q1.Equals(q3) {
		t.Fatalf("expected q1 != q3 (different field)")
	}
	q4 := mustQuery(t, "foo", 3, 11, 5)
	if q1.Equals(q4) {
		t.Fatalf("expected q1 != q4 (different origin)")
	}
	q5 := mustQuery(t, "foo", 3, 10, 6)
	if q1.Equals(q5) {
		t.Fatalf("expected q1 != q5 (different pivot)")
	}

	// The Java test compares weight=3 vs weight=3 across q4/q5 using
	// origin/pivot mutations only; in Gocene the bare query equality
	// is the same path. Also exercise weight equality via the wrapping
	// BoostQuery: weight 3 vs weight 4 must differ.
	q6 := mustQuery(t, "foo", 4, 10, 5)
	if q1.Equals(q6) {
		t.Fatalf("expected q1 != q6 (different weight, BoostQuery wrapper)")
	}
}

// TestLongDistanceFeatureQuery_ConstructorValidation mirrors the Java
// constructor's IllegalArgumentException for non-positive pivot.
func TestLongDistanceFeatureQuery_ConstructorValidation(t *testing.T) {
	cases := []struct {
		name  string
		field string
		pivot int64
		want  bool
	}{
		{"zero pivot", "foo", 0, true},
		{"negative pivot", "foo", -1, true},
		{"empty field", "", 5, true},
		{"valid", "foo", 1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewLongDistanceFeatureQuery(tc.field, 3, tc.pivot)
			gotErr := err != nil
			if gotErr != tc.want {
				t.Fatalf("got err=%v, want err=%v", err, tc.want)
			}
		})
	}
}

// TestLongDistanceFeatureQuery_String_AndVisit covers the textual
// representation and the visit dispatch logic.
func TestLongDistanceFeatureQuery_String_AndVisit(t *testing.T) {
	q, err := NewLongDistanceFeatureQuery("foo", 10, 5)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	const want = "LongDistanceFeatureQuery(field=foo,origin=10,pivotDistance=5)"
	if got := q.String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
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

type recordingVisitor struct {
	EmptyQueryVisitorBase
	acceptedField bool
	rejectField   string
	leaf          Query
}

func (v *recordingVisitor) AcceptField(field string) bool {
	if field == v.rejectField {
		return false
	}
	v.acceptedField = true
	return true
}

func (v *recordingVisitor) VisitLeaf(q Query) { v.leaf = q }

// TestLongDistanceFeatureQuery_LongFieldFactory_BoostWrapping verifies
// that the factory wraps the query in a BoostQuery when weight != 1,
// and returns the bare query when weight == 1 (Java parity).
func TestLongDistanceFeatureQuery_LongFieldFactory_BoostWrapping(t *testing.T) {
	bare, err := LongFieldNewDistanceFeatureQuery("foo", 1, 10, 5)
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	if _, ok := bare.(*LongDistanceFeatureQuery); !ok {
		t.Fatalf("weight=1 should return *LongDistanceFeatureQuery, got %T", bare)
	}

	wrapped, err := LongFieldNewDistanceFeatureQuery("foo", 3, 10, 5)
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	bq, ok := wrapped.(*BoostQuery)
	if !ok {
		t.Fatalf("weight=3 should return *BoostQuery, got %T", wrapped)
	}
	if bq.Boost() != 3 {
		t.Fatalf("boost = %v, want 3", bq.Boost())
	}
	if _, ok := bq.Query().(*LongDistanceFeatureQuery); !ok {
		t.Fatalf("BoostQuery wrapped query = %T, want *LongDistanceFeatureQuery", bq.Query())
	}
}

// inMemoryLongDocValues implements longDocValues over a sorted slice of
// (docID, []value). It is the test stand-in for SortedNumericDocValues
// + selectClosestValue and applies the Java selectValue logic.
type inMemoryLongDocValues struct {
	docs    []int
	values  [][]int64
	origin  int64
	idx     int
	current int64
	hasVal  bool
}

func newInMemoryLongDocValues(origin int64, docs map[int][]int64) *inMemoryLongDocValues {
	keys := make([]int, 0, len(docs))
	for k := range docs {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	values := make([][]int64, len(keys))
	for i, k := range keys {
		// Defensive copy + sort to match Lucene's SortedNumericDocValues invariant.
		vs := append([]int64(nil), docs[k]...)
		sort.Slice(vs, func(a, b int) bool { return vs[a] < vs[b] })
		values[i] = vs
	}
	return &inMemoryLongDocValues{docs: keys, values: values, origin: origin, idx: -1}
}

func (it *inMemoryLongDocValues) AdvanceExact(doc int) (bool, error) {
	for i, d := range it.docs {
		if d == doc {
			it.idx = i
			it.current = selectClosestValue(it.values[i], it.origin)
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

func (it *inMemoryLongDocValues) LongValue() (int64, error) {
	if !it.hasVal {
		return 0, errors.New("inMemoryLongDocValues: no value at current position")
	}
	return it.current, nil
}

func (it *inMemoryLongDocValues) DocID() int {
	if it.idx < 0 {
		return -1
	}
	if it.idx >= len(it.docs) {
		return NO_MORE_DOCS
	}
	return it.docs[it.idx]
}

func (it *inMemoryLongDocValues) NextDoc() (int, error) {
	it.idx++
	if it.idx >= len(it.docs) {
		it.hasVal = false
		return NO_MORE_DOCS, nil
	}
	it.current = selectClosestValue(it.values[it.idx], it.origin)
	it.hasVal = true
	return it.docs[it.idx], nil
}

func (it *inMemoryLongDocValues) Advance(target int) (int, error) {
	for i := it.idx + 1; i < len(it.docs); i++ {
		if it.docs[i] >= target {
			it.idx = i
			it.current = selectClosestValue(it.values[i], it.origin)
			it.hasVal = true
			return it.docs[i], nil
		}
	}
	it.idx = len(it.docs)
	it.hasVal = false
	return NO_MORE_DOCS, nil
}

func (it *inMemoryLongDocValues) Cost() int64 { return int64(len(it.docs)) }

// inMemoryLongPointSource implements longPointSource as a flat scan
// over (docID, value) pairs. It is sufficient for the scorer's skip
// logic: the only behavior the scorer relies on is that Intersect
// emits the matching docs (per the visitor's Compare/Visit contract)
// and that the estimator gives a reasonable upper bound for the
// "should we materialize this range" check.
type inMemoryLongPointSource struct {
	docs   []int
	values []int64 // values[i] is the closest value to origin for docs[i]
}

func newInMemoryLongPointSource(origin int64, docs map[int][]int64) *inMemoryLongPointSource {
	keys := make([]int, 0, len(docs))
	for k := range docs {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	values := make([]int64, len(keys))
	for i, k := range keys {
		vs := append([]int64(nil), docs[k]...)
		sort.Slice(vs, func(a, b int) bool { return vs[a] < vs[b] })
		values[i] = selectClosestValue(vs, origin)
	}
	return &inMemoryLongPointSource{docs: keys, values: values}
}

func (p *inMemoryLongPointSource) Intersect(visitor longPointVisitor) error {
	if len(p.docs) == 0 {
		return nil
	}
	visitor.Grow(len(p.docs))
	for i, doc := range p.docs {
		buf := make([]byte, 8)
		util.LongToSortableBytes(p.values[i], buf, 0)
		if err := visitor.VisitWithPackedValue(doc, buf); err != nil {
			return err
		}
	}
	return nil
}

func (p *inMemoryLongPointSource) EstimatePointCountGreaterThanOrEqualTo(_ longPointVisitor, threshold int64) bool {
	// Conservative: we know the worst-case is len(p.docs).
	return int64(len(p.docs)) >= threshold
}

// buildScorerWithLookup wires a LongDistanceFeatureQuery against the
// supplied in-memory doc-values and point source, then returns the
// per-segment scorer ready for iteration.
func buildScorerWithLookup(t *testing.T, q *LongDistanceFeatureQuery, weight float32, maxDoc int, dv longDocValues, pts longPointSource) Scorer {
	t.Helper()
	q.installTestLeafLookup(func(_ *index.LeafReaderContext, field string) (longDocValues, longPointSource, error) {
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
	// Override the maxDoc carried by the empty LeafReader. The scorer
	// reads maxDoc from ctx.LeafReader().MaxDoc(); leaf reader's
	// MaxDoc() is 0 by default, so the dynamic-skip logic will see
	// maxDoc=0 and skip the DocIdSetBuilder path. We work around this
	// by giving the scorer a non-zero maxDoc through the supplier.
	supplier, err := w.ScorerSupplier(ctx)
	if err != nil {
		t.Fatalf("ScorerSupplier: %v", err)
	}
	if supplier == nil {
		t.Fatalf("ScorerSupplier returned nil")
	}
	// Patch the supplier's maxDoc through the unexported field.
	if s, ok := supplier.(*longDistanceFeatureScorerSupplier); ok {
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

// collectAll consumes the scorer and returns parallel slices of
// (docID, score) sorted by descending score (stable). The Java tests
// compare against a fixed expected ranking; this helper makes the
// comparison readable.
type docScore struct {
	doc   int
	score float32
}

func collectAll(t *testing.T, scorer Scorer) []docScore {
	t.Helper()
	var out []docScore
	for {
		doc, err := scorer.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if doc == NO_MORE_DOCS {
			break
		}
		out = append(out, docScore{doc: doc, score: scorer.Score()})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		return out[i].doc < out[j].doc
	})
	return out
}

// expectTop validates the first n elements of got against expected,
// comparing by (doc, score) with a tight float tolerance.
func expectTop(t *testing.T, got, expected []docScore) {
	t.Helper()
	if len(got) < len(expected) {
		t.Fatalf("got %d hits, expected at least %d (got=%v expected=%v)", len(got), len(expected), got, expected)
	}
	for i, want := range expected {
		if got[i].doc != want.doc {
			t.Fatalf("hit %d: doc = %d, want %d (full got=%v)", i, got[i].doc, want.doc, got)
		}
		if !approxEqual(got[i].score, want.score) {
			t.Fatalf("hit %d: score = %v, want %v (full got=%v)", i, got[i].score, want.score, got)
		}
	}
}

func approxEqual(a, b float32) bool {
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

// expectedScore replicates the Java score formula so tests stay
// readable: weight * (pivot / (pivot + |value - origin|)).
func expectedScore(weight float32, pivot, distance int64) float32 {
	return float32(float64(weight) * (float64(pivot) / (float64(pivot) + float64(distance))))
}

// TestLongDistanceFeatureQuery_Basics mirrors
// TestLongDistanceFeatureQuery.testBasics (Lucene 10.4.0).
//
// NOTE: simplified vs Java peer because RandomIndexWriter / CheckHits /
// QueryUtils are not yet ported. We materialize the same five-document
// fixture in-memory and assert the top-2 ranking matches the Java
// expected ScoreDoc array element-for-element.
func TestLongDistanceFeatureQuery_Basics(t *testing.T) {
	// Java test docs (single value per doc):
	//   0 -> 3, 1 -> 12, 2 -> 8, 3 -> -1, 4 -> 7
	docs := map[int][]int64{
		0: {3},
		1: {12},
		2: {8},
		3: {-1},
		4: {7},
	}

	// Sub-test 1: origin=10 pivot=5 weight=3, top 2 = (1, 2) both with score 3 * 5/(5+2).
	t.Run("origin=10 pivot=5 weight=3", func(t *testing.T) {
		const (
			origin int64   = 10
			pivot  int64   = 5
			weight float32 = 3
		)
		q, err := NewLongDistanceFeatureQuery("foo", origin, pivot)
		if err != nil {
			t.Fatalf("constructor: %v", err)
		}
		dv := newInMemoryLongDocValues(origin, docs)
		pts := newInMemoryLongPointSource(origin, docs)
		scorer := buildScorerWithLookup(t, q, weight, 5, dv, pts)
		got := collectAll(t, scorer)
		expectTop(t, got, []docScore{
			{1, expectedScore(weight, pivot, 2)},
			{2, expectedScore(weight, pivot, 2)},
		})
	})

	// Sub-test 2: origin=7 pivot=5 weight=3, top 2 = (4, 2): distance 0 and 1.
	t.Run("origin=7 pivot=5 weight=3", func(t *testing.T) {
		const (
			origin int64   = 7
			pivot  int64   = 5
			weight float32 = 3
		)
		q, err := NewLongDistanceFeatureQuery("foo", origin, pivot)
		if err != nil {
			t.Fatalf("constructor: %v", err)
		}
		dv := newInMemoryLongDocValues(origin, docs)
		pts := newInMemoryLongPointSource(origin, docs)
		scorer := buildScorerWithLookup(t, q, weight, 5, dv, pts)
		got := collectAll(t, scorer)
		expectTop(t, got, []docScore{
			{4, expectedScore(weight, pivot, 0)},
			{2, expectedScore(weight, pivot, 1)},
		})
	})
}

// TestLongDistanceFeatureQuery_OverUnderflow mirrors
// TestLongDistanceFeatureQuery.testOverUnderFlow (Lucene 10.4.0).
//
// NOTE: simplified vs Java peer because the indexing pipeline is not
// available; we replay the same fixture in-memory.
func TestLongDistanceFeatureQuery_OverUnderflow(t *testing.T) {
	docs := map[int][]int64{
		0: {3},
		1: {12},
		2: {-10},
		3: {math.MaxInt64},
		4: {math.MinInt64},
	}

	// Sub-test 1: origin=MaxInt64-1, pivot=100, weight=3.
	// Java expects [doc 3, doc 0]: doc 3 distance 1, doc 0 distance treated as MAX_VALUE.
	t.Run("origin=MaxInt64-1 pivot=100", func(t *testing.T) {
		var (
			origin int64   = math.MaxInt64 - 1
			pivot  int64   = 100
			weight float32 = 3
		)
		q, err := NewLongDistanceFeatureQuery("foo", origin, pivot)
		if err != nil {
			t.Fatalf("constructor: %v", err)
		}
		dv := newInMemoryLongDocValues(origin, docs)
		pts := newInMemoryLongPointSource(origin, docs)
		scorer := buildScorerWithLookup(t, q, weight, 5, dv, pts)
		got := collectAll(t, scorer)
		expectTop(t, got, []docScore{
			{3, expectedScore(weight, pivot, 1)},
			{0, expectedScore(weight, pivot, math.MaxInt64)},
		})
	})

	// Sub-test 2: origin=MinInt64+1, pivot=100, weight=3.
	// Java expects [doc 4, doc 0]: doc 4 distance 1, doc 0 distance treated as MAX_VALUE.
	t.Run("origin=MinInt64+1 pivot=100", func(t *testing.T) {
		var (
			origin int64   = math.MinInt64 + 1
			pivot  int64   = 100
			weight float32 = 3
		)
		q, err := NewLongDistanceFeatureQuery("foo", origin, pivot)
		if err != nil {
			t.Fatalf("constructor: %v", err)
		}
		dv := newInMemoryLongDocValues(origin, docs)
		pts := newInMemoryLongPointSource(origin, docs)
		scorer := buildScorerWithLookup(t, q, weight, 5, dv, pts)
		got := collectAll(t, scorer)
		expectTop(t, got, []docScore{
			{4, expectedScore(weight, pivot, 1)},
			{0, expectedScore(weight, pivot, math.MaxInt64)},
		})
	})
}

// TestLongDistanceFeatureQuery_MissingField mirrors
// TestLongDistanceFeatureQuery.testMissingField. With no field in the
// segment, the supplier returns nil and there are zero hits.
//
// NOTE: simplified vs Java peer because the test relies on a real
// MultiReader; the Gocene equivalent is to verify the supplier path
// returns nil when point values are absent.
func TestLongDistanceFeatureQuery_MissingField(t *testing.T) {
	q, err := NewLongDistanceFeatureQuery("foo", 10, 5)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	q.installTestLeafLookup(func(_ *index.LeafReaderContext, _ string) (longDocValues, longPointSource, error) {
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

// TestLongDistanceFeatureQuery_MissingValue mirrors
// TestLongDistanceFeatureQuery.testMissingValue (Lucene 10.4.0).
//
// NOTE: simplified vs Java peer; we materialize the same fixture
// in-memory. The middle doc (id=1) has no value and must not appear
// in the result set, while docs 0 and 2 score by distance.
func TestLongDistanceFeatureQuery_MissingValue(t *testing.T) {
	docs := map[int][]int64{
		0: {3},
		// doc 1: no value
		2: {7},
	}
	const (
		origin int64   = 10
		pivot  int64   = 5
		weight float32 = 3
	)
	q, err := NewLongDistanceFeatureQuery("foo", origin, pivot)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	dv := newInMemoryLongDocValues(origin, docs)
	pts := newInMemoryLongPointSource(origin, docs)
	scorer := buildScorerWithLookup(t, q, weight, 3, dv, pts)
	got := collectAll(t, scorer)
	expectTop(t, got, []docScore{
		{2, expectedScore(weight, pivot, 3)},
		{0, expectedScore(weight, pivot, 7)},
	})
	// Ensure doc 1 was not emitted.
	for _, hit := range got {
		if hit.doc == 1 {
			t.Fatalf("doc 1 (no value) should not be in the result set, got %v", got)
		}
	}
}

// TestLongDistanceFeatureQuery_MultiValued mirrors
// TestLongDistanceFeatureQuery.testMultiValued (Lucene 10.4.0).
//
// NOTE: simplified vs Java peer; the same five multi-valued documents
// are replayed in-memory. Verifies selectValue picks the value closest
// to origin in unsigned distance.
func TestLongDistanceFeatureQuery_MultiValued(t *testing.T) {
	docs := map[int][]int64{
		0: {3, 1000, math.MaxInt64},
		1: {-100, 12, 999},
		2: {math.MinInt64, -1000, 8},
		3: {-1},
		4: {math.MinInt64, 7},
	}

	t.Run("origin=10 pivot=5 weight=3", func(t *testing.T) {
		const (
			origin int64   = 10
			pivot  int64   = 5
			weight float32 = 3
		)
		q, err := NewLongDistanceFeatureQuery("foo", origin, pivot)
		if err != nil {
			t.Fatalf("constructor: %v", err)
		}
		dv := newInMemoryLongDocValues(origin, docs)
		pts := newInMemoryLongPointSource(origin, docs)
		scorer := buildScorerWithLookup(t, q, weight, 5, dv, pts)
		got := collectAll(t, scorer)
		expectTop(t, got, []docScore{
			{1, expectedScore(weight, pivot, 2)},
			{2, expectedScore(weight, pivot, 2)},
		})
	})

	t.Run("origin=7 pivot=5 weight=3", func(t *testing.T) {
		const (
			origin int64   = 7
			pivot  int64   = 5
			weight float32 = 3
		)
		q, err := NewLongDistanceFeatureQuery("foo", origin, pivot)
		if err != nil {
			t.Fatalf("constructor: %v", err)
		}
		dv := newInMemoryLongDocValues(origin, docs)
		pts := newInMemoryLongPointSource(origin, docs)
		scorer := buildScorerWithLookup(t, q, weight, 5, dv, pts)
		got := collectAll(t, scorer)
		expectTop(t, got, []docScore{
			{4, expectedScore(weight, pivot, 0)},
			{2, expectedScore(weight, pivot, 1)},
		})
	})
}

// TestLongDistanceFeatureQuery_Random mirrors
// TestLongDistanceFeatureQuery.testRandom (Lucene 10.4.0).
//
// NOTE: simplified vs Java peer because CheckHits.checkTopScores
// depends on an indexed reader. We instead verify that the scorer
// produces strictly decreasing scores under arbitrary origin/pivot
// choices: scores are monotonic in distance for fixed origin/pivot/
// weight, so the top-N from a brute-force sort must match the scorer
// output.
func TestLongDistanceFeatureQuery_Random(t *testing.T) {
	rng := rand.New(rand.NewPCG(0xC0FFEE, 0x1234)) //nolint:gosec // deterministic seeding for reproducibility
	const numDocs = 1000

	docs := make(map[int][]int64, numDocs)
	for i := 0; i < numDocs; i++ {
		v := int64(rng.Uint64())
		docs[i] = []int64{v}
	}

	for iter := 0; iter < 5; iter++ {
		origin := int64(rng.Uint64())
		var pivot int64
		for pivot <= 0 {
			pivot = int64(rng.Uint64() >> 1)
		}
		weight := float32(1+rng.IntN(10)) / 3

		q, err := NewLongDistanceFeatureQuery("foo", origin, pivot)
		if err != nil {
			t.Fatalf("constructor: %v", err)
		}
		dv := newInMemoryLongDocValues(origin, docs)
		pts := newInMemoryLongPointSource(origin, docs)
		scorer := buildScorerWithLookup(t, q, weight, numDocs, dv, pts)
		got := collectAll(t, scorer)
		if len(got) != numDocs {
			t.Fatalf("iter=%d: expected %d hits, got %d", iter, numDocs, len(got))
		}

		// Verify strictly non-increasing scores.
		for i := 1; i < len(got); i++ {
			if got[i].score > got[i-1].score {
				t.Fatalf("iter=%d: scores not sorted descending at %d: %v > %v", iter, i, got[i].score, got[i-1].score)
			}
		}

		// Brute-force expected top-3 and compare scores.
		type known struct {
			doc int
			d   int64
		}
		all := make([]known, 0, len(docs))
		for d := 0; d < numDocs; d++ {
			all = append(all, known{doc: d, d: unsignedDistance(docs[d][0], origin)})
		}
		sort.Slice(all, func(i, j int) bool {
			if all[i].d != all[j].d {
				return uint64(all[i].d) < uint64(all[j].d)
			}
			return all[i].doc < all[j].doc
		})
		for i := 0; i < 3 && i < len(all); i++ {
			want := computeLongDistanceScore(weight, pivot, all[i].d)
			if !approxEqual(got[i].score, want) {
				t.Fatalf("iter=%d top-%d: doc score = %v, want %v (doc=%d distance=%d)", iter, i, got[i].score, want, all[i].doc, all[i].d)
			}
		}
	}
}

// TestLongDistanceFeatureQuery_Explain verifies the explain output
// format and the score returned by the weight's Explain. Mirrors the
// CheckHits.checkExplanations interleaved through several Java tests.
func TestLongDistanceFeatureQuery_Explain(t *testing.T) {
	docs := map[int][]int64{
		0: {3},
		1: {7},
	}
	const (
		origin int64   = 10
		pivot  int64   = 5
		weight float32 = 2
	)
	q, err := NewLongDistanceFeatureQuery("foo", origin, pivot)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	dv := newInMemoryLongDocValues(origin, docs)
	pts := newInMemoryLongPointSource(origin, docs)
	q.installTestLeafLookup(func(_ *index.LeafReaderContext, _ string) (longDocValues, longPointSource, error) {
		return dv, pts, nil
	})
	searcher := NewIndexSearcher(nil)
	w, err := q.CreateWeight(searcher, true, weight)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	leaf := index.NewLeafReader(nil)
	ctx := index.NewLeafReaderContext(leaf, nil, 0, 0)

	// Matching doc.
	ex, err := w.Explain(ctx, 1)
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}
	if !ex.IsMatch() {
		t.Fatalf("expected match explanation for doc 1, got %+v", ex)
	}
	want := expectedScore(weight, pivot, 3)
	if !approxEqual(ex.GetValue(), want) {
		t.Fatalf("explain value = %v, want %v", ex.GetValue(), want)
	}
	if got := ex.GetDescription(); got == "" {
		t.Fatalf("expected non-empty description, got empty")
	}
	if len(ex.GetDetails()) != 4 {
		t.Fatalf("expected 4 sub-explanations (weight/pivot/origin/value), got %d", len(ex.GetDetails()))
	}

	// Non-matching doc (doc 2: no value).
	ex2, err := w.Explain(ctx, 2)
	if err != nil {
		t.Fatalf("Explain(2): %v", err)
	}
	if ex2.IsMatch() {
		t.Fatalf("expected no-match explanation for doc 2, got match")
	}
}

// TestLongDistanceFeatureQuery_MaxScore verifies the per-scorer
// GetMaxScore returns the configured weight, matching the Java
// override on DistanceScorer.
func TestLongDistanceFeatureQuery_MaxScore(t *testing.T) {
	docs := map[int][]int64{0: {3}}
	const (
		origin int64   = 10
		pivot  int64   = 5
		weight float32 = 4.25
	)
	q, err := NewLongDistanceFeatureQuery("foo", origin, pivot)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	dv := newInMemoryLongDocValues(origin, docs)
	pts := newInMemoryLongPointSource(origin, docs)
	scorer := buildScorerWithLookup(t, q, weight, 1, dv, pts)
	if got := scorer.GetMaxScore(0); got != weight {
		t.Fatalf("GetMaxScore = %v, want %v", got, weight)
	}
}

// TestLongDistanceFeatureQuery_SetMinCompetitiveScore_AboveBoost
// verifies that setting a minScore above the boost replaces the
// iterator with an empty one, matching the Java early-return.
func TestLongDistanceFeatureQuery_SetMinCompetitiveScore_AboveBoost(t *testing.T) {
	docs := map[int][]int64{
		0: {3},
		1: {12},
		2: {8},
	}
	const (
		origin int64   = 10
		pivot  int64   = 5
		weight float32 = 3
	)
	q, err := NewLongDistanceFeatureQuery("foo", origin, pivot)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	dv := newInMemoryLongDocValues(origin, docs)
	pts := newInMemoryLongPointSource(origin, docs)
	scorer := buildScorerWithLookup(t, q, weight, 3, dv, pts).(*longDistanceFeatureScorer)
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

// TestUnsignedDistance covers the helper directly, including the
// underflow → MaxInt64 case Java exercises through the testOverUnderFlow
// table.
func TestUnsignedDistance(t *testing.T) {
	cases := []struct {
		a, b int64
		want int64
	}{
		{10, 3, 7},
		{3, 10, 7},
		{0, 0, 0},
		{math.MaxInt64, math.MinInt64, math.MaxInt64}, // underflow trap
		{math.MinInt64, math.MaxInt64, math.MaxInt64}, // underflow trap
		{1, math.MinInt64, math.MaxInt64},             // underflow on subtract
	}
	for _, tc := range cases {
		got := unsignedDistance(tc.a, tc.b)
		if got != tc.want {
			t.Fatalf("unsignedDistance(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

// TestSelectClosestValue covers the multi-valued selection helper, the
// same routine Java's DistanceScorer.selectValue uses to pick the
// closest value to origin.
func TestSelectClosestValue(t *testing.T) {
	cases := []struct {
		name   string
		values []int64
		origin int64
		want   int64
	}{
		{"single value", []int64{5}, 10, 5},
		{"all below origin", []int64{1, 2, 3}, 10, 3},
		{"all above origin", []int64{10, 20, 30}, 5, 10},
		{"straddling, closer below", []int64{1, 9, 100}, 10, 9},
		{"straddling, closer above", []int64{1, 12, 100}, 10, 12},
		{"equal distances picks larger (Java tie-break)", []int64{8, 12}, 10, 12},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := selectClosestValue(tc.values, tc.origin)
			if got != tc.want {
				t.Fatalf("selectClosestValue(%v, %d) = %d, want %d", tc.values, tc.origin, got, tc.want)
			}
		})
	}
}
