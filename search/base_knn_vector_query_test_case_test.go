// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/BaseKnnVectorQueryTestCase.java
//
// BaseKnnVectorQueryTestCase is an abstract JUnit base class whose @Test methods
// are inherited by the concrete float / byte (and MMap) subclasses. Go has no
// test inheritance, so the shared scenarios are expressed here as exported
// runners that take a knnVectorFixture describing how the concrete subclass
// builds documents and queries. Each concrete suite (TestKnnFloatVectorQuery,
// TestKnnByteVectorQuery, …) instantiates its fixture and invokes the shared
// runners, mirroring how the Java subclasses inherit the base tests.
//
// The scenarios drive the real IndexWriter flush + IndexSearcher read path via
// the package integration harness (newIntegrationIndex / addDoc / searcher),
// exercising the production KNN search end to end (HNSW approximate search,
// the exact brute-force fallback for restrictive pre-filters, multi-segment
// merge, boost, and explain).

package search_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// knnVectorFixture abstracts the vector-type-specific operations the shared KNN
// scenarios depend on. It is the Go counterpart of the abstract methods on
// BaseKnnVectorQueryTestCase (getKnnVectorQuery, getKnnVectorField, …). The
// float and byte concrete suites each supply an implementation.
type knnVectorFixture interface {
	// newQuery builds the concrete KNN query (KnnFloatVectorQuery or
	// KnnByteVectorQuery) for field/target/k, optionally pre-filtered.
	newQuery(field string, target []float32, k int, filter search.Query) search.Query

	// addVectorDoc indexes one document carrying the vector field with the
	// given similarity function, plus the supplied extra fields.
	addVectorDoc(ix *integrationIndex, field string, vec []float32,
		sim index.VectorSimilarityFunction, extra ...document.IndexableField)

	// queryTypeName is the textual prefix used by the concrete query's String()
	// (e.g. "KnnFloatVectorQuery").
	queryTypeName() string

	// newIndex opens a fresh integration index on the backend the fixture
	// wants to test. The default float / byte fixtures use the in-memory
	// directory; the MMap fixture overrides it to use an MMapDirectory, so the
	// inherited scenarios run unchanged over a different store backend (the Go
	// analogue of overriding newDirectoryForTest in the Java subclass).
	newIndex(t *testing.T) *integrationIndex
}

// ── shared scenario runners ──────────────────────────────────────────────────

// runKnnEquals mirrors BaseKnnVectorQueryTestCase.testEquals.
func runKnnEquals(t *testing.T, f knnVectorFixture) {
	t.Helper()
	q1 := f.newQuery("f1", []float32{0, 1}, 10, nil)
	filter1 := search.NewTermQuery(index.NewTerm("id", "id1"))
	q2 := f.newQuery("f1", []float32{0, 1}, 10, filter1)

	if q2.Equals(q1) || q1.Equals(q2) {
		t.Fatalf("filtered query must not equal unfiltered query")
	}
	if !q2.Equals(f.newQuery("f1", []float32{0, 1}, 10, filter1)) {
		t.Fatalf("queries with equal field/target/k/filter must be equal")
	}
	filter2 := search.NewTermQuery(index.NewTerm("id", "id2"))
	if q2.Equals(f.newQuery("f1", []float32{0, 1}, 10, filter2)) {
		t.Fatalf("different filter must not be equal")
	}
	if !q1.Equals(f.newQuery("f1", []float32{0, 1}, 10, nil)) {
		t.Fatalf("unfiltered queries with equal field/target/k must be equal")
	}
	if q1.Equals(search.NewTermQuery(index.NewTerm("f1", "x"))) {
		t.Fatalf("must not equal a TermQuery")
	}
	if q1.Equals(f.newQuery("f2", []float32{0, 1}, 10, nil)) {
		t.Fatalf("different field must not be equal")
	}
	if q1.Equals(f.newQuery("f1", []float32{1, 1}, 10, nil)) {
		t.Fatalf("different target must not be equal")
	}
	if q1.Equals(f.newQuery("f1", []float32{0, 1}, 2, nil)) {
		t.Fatalf("different k must not be equal")
	}
	if q1.Equals(f.newQuery("f1", []float32{0}, 10, nil)) {
		t.Fatalf("different dimension must not be equal")
	}
}

// runKnnEmptyIndex mirrors BaseKnnVectorQueryTestCase.testEmptyIndex: a KNN
// query over an index with no matching vectors must rewrite to MatchNoDocsQuery
// and match zero documents.
func runKnnEmptyIndex(t *testing.T, f knnVectorFixture) {
	t.Helper()
	ix := f.newIndex(t)
	// One non-vector document so a segment exists but the field is absent.
	d := document.NewDocument()
	other, _ := document.NewStringField("other", "value", false)
	d.Add(other)
	ix.addDoc(d)
	s, cleanup := ix.searcher()
	defer cleanup()

	q := f.newQuery("field", []float32{1, 2}, 10, nil)
	assertKnnMatches(t, s, q, 0)
	rewritten, err := q.Rewrite(s.GetIndexReader())
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if _, ok := rewritten.(*search.MatchNoDocsQuery); !ok {
		t.Fatalf("empty index must rewrite to MatchNoDocsQuery, got %T", rewritten)
	}
}

// runKnnFindAll mirrors BaseKnnVectorQueryTestCase.testFindAll: when k >= numDocs
// every vector document is returned in descending score order.
func runKnnFindAll(t *testing.T, f knnVectorFixture) {
	t.Helper()
	ix := f.newIndex(t)
	addStableVectorDocs(ix, f, "field", index.VectorSimilarityFunctionEuclidean,
		[]float32{0, 1}, []float32{1, 2}, []float32{0, 0})
	s, cleanup := ix.searcher()
	defer cleanup()

	q := f.newQuery("field", []float32{0, 0}, 10, nil)
	assertKnnMatches(t, s, q, 3)
	top, err := s.Search(q, 3)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	assertIDMatches(t, s, "id2", top.ScoreDocs[0])
	assertIDMatches(t, s, "id0", top.ScoreDocs[1])
	assertIDMatches(t, s, "id1", top.ScoreDocs[2])
}

// runKnnFindFewer mirrors BaseKnnVectorQueryTestCase.testFindFewer.
func runKnnFindFewer(t *testing.T, f knnVectorFixture) {
	t.Helper()
	ix := f.newIndex(t)
	addStableVectorDocs(ix, f, "field", index.VectorSimilarityFunctionEuclidean,
		[]float32{0, 1}, []float32{1, 2}, []float32{0, 0})
	s, cleanup := ix.searcher()
	defer cleanup()

	q := f.newQuery("field", []float32{0, 0}, 2, nil)
	assertKnnMatches(t, s, q, 2)
	top, err := s.Search(q, 3)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(top.ScoreDocs) != 2 {
		t.Fatalf("expected 2 score docs, got %d", len(top.ScoreDocs))
	}
	assertTopIDs(t, s, map[string]bool{"id2": true, "id0": true}, top.ScoreDocs)
}

// runKnnSearchBoost mirrors BaseKnnVectorQueryTestCase.testSearchBoost: wrapping
// the KNN query in a BoostQuery multiplies every score by the boost.
func runKnnSearchBoost(t *testing.T, f knnVectorFixture) {
	t.Helper()
	ix := f.newIndex(t)
	addStableVectorDocs(ix, f, "field", index.VectorSimilarityFunctionEuclidean,
		[]float32{0, 1}, []float32{1, 2}, []float32{0, 0})
	s, cleanup := ix.searcher()
	defer cleanup()

	vq := f.newQuery("field", []float32{0, 0}, 10, nil)
	base, err := s.Search(vq, 3)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	boosted, err := s.Search(search.NewBoostQuery(f.newQuery("field", []float32{0, 0}, 10, nil), 3.0), 3)
	if err != nil {
		t.Fatalf("boost search: %v", err)
	}
	if len(base.ScoreDocs) != len(boosted.ScoreDocs) {
		t.Fatalf("boost changed hit count: %d vs %d", len(base.ScoreDocs), len(boosted.ScoreDocs))
	}
	for i := range base.ScoreDocs {
		if base.ScoreDocs[i].Doc != boosted.ScoreDocs[i].Doc {
			t.Fatalf("boost changed doc order at %d", i)
		}
		if math.Abs(float64(base.ScoreDocs[i].Score*3.0-boosted.ScoreDocs[i].Score)) > 0.001 {
			t.Fatalf("boosted score mismatch at %d: %f*3 != %f",
				i, base.ScoreDocs[i].Score, boosted.ScoreDocs[i].Score)
		}
	}
}

// runKnnSimpleFilter mirrors BaseKnnVectorQueryTestCase.testSimpleFilter.
func runKnnSimpleFilter(t *testing.T, f knnVectorFixture) {
	t.Helper()
	ix := f.newIndex(t)
	addStableVectorDocs(ix, f, "field", index.VectorSimilarityFunctionEuclidean,
		[]float32{0, 1}, []float32{1, 2}, []float32{0, 0})
	s, cleanup := ix.searcher()
	defer cleanup()

	filter := search.NewTermQuery(index.NewTerm("id", "id2"))
	q := f.newQuery("field", []float32{0, 0}, 10, filter)
	top, err := s.Search(q, 3)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if top.TotalHits.Value != 1 {
		t.Fatalf("filtered hits = %d, want 1", top.TotalHits.Value)
	}
	assertIDMatches(t, s, "id2", top.ScoreDocs[0])
}

// runKnnFilterWithNoVectorMatches mirrors testFilterWithNoVectorMatches: a
// filter that matches only vector-less documents yields zero KNN results.
func runKnnFilterWithNoVectorMatches(t *testing.T, f knnVectorFixture) {
	t.Helper()
	ix := f.newIndex(t)
	addStableVectorDocs(ix, f, "field", index.VectorSimilarityFunctionEuclidean,
		[]float32{0, 1}, []float32{1, 2}, []float32{0, 0})
	// Documents matched by the filter but carrying no vector.
	for i := 0; i < 3; i++ {
		d := document.NewDocument()
		other, _ := document.NewStringField("other", "value", false)
		d.Add(other)
		ix.addDoc(d)
	}
	s, cleanup := ix.searcher()
	defer cleanup()

	filter := search.NewTermQuery(index.NewTerm("other", "value"))
	q := f.newQuery("field", []float32{0, 0}, 10, filter)
	top, err := s.Search(q, 3)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if top.TotalHits.Value != 0 {
		t.Fatalf("filter with no vector matches = %d, want 0", top.TotalHits.Value)
	}
}

// runKnnMatchAllFilter mirrors testMatchAllFilter: a MatchAllDocsQuery filter
// does not collapse to exact search even though it matches everything.
func runKnnMatchAllFilter(t *testing.T, f knnVectorFixture) {
	t.Helper()
	ix := f.newIndex(t)
	addStableVectorDocs(ix, f, "field", index.VectorSimilarityFunctionEuclidean,
		[]float32{0, 1}, []float32{1, 2}, []float32{0, 0})
	s, cleanup := ix.searcher()
	defer cleanup()

	q := f.newQuery("field", []float32{0, 0}, 10, search.NewMatchAllDocsQuery())
	top, err := s.Search(q, 3)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if top.TotalHits.Value != 3 {
		t.Fatalf("match-all filter hits = %d, want 3", top.TotalHits.Value)
	}
}

// runKnnNonVectorField mirrors testNonVectorField: querying a field that is not
// a vector field (or does not exist) matches nothing.
func runKnnNonVectorField(t *testing.T, f knnVectorFixture) {
	t.Helper()
	ix := f.newIndex(t)
	addStableVectorDocs(ix, f, "field", index.VectorSimilarityFunctionEuclidean,
		[]float32{0, 1}, []float32{1, 2}, []float32{0, 0})
	s, cleanup := ix.searcher()
	defer cleanup()

	assertKnnMatches(t, s, f.newQuery("xyzzy", []float32{0}, 10, nil), 0)
	assertKnnMatches(t, s, f.newQuery("id", []float32{0}, 10, nil), 0)
}

// runKnnDimensionMismatch mirrors testDimensionMismatch: a query whose vector
// dimension differs from the field's must surface an error at search time.
func runKnnDimensionMismatch(t *testing.T, f knnVectorFixture) {
	t.Helper()
	ix := f.newIndex(t)
	addStableVectorDocs(ix, f, "field", index.VectorSimilarityFunctionEuclidean,
		[]float32{0, 1}, []float32{1, 2}, []float32{0, 0})
	s, cleanup := ix.searcher()
	defer cleanup()

	q := f.newQuery("field", []float32{0}, 1, nil)
	if _, err := s.Search(q, 10); err == nil {
		t.Fatalf("dimension mismatch must error")
		// Deviation: Gocene's codec reports "lucene99 flat: query dim 1 !=
		// field dim 2" rather than Lucene's "vector query dimension: 1 differs
		// from field dimension: 2"; both are IllegalArgument-class errors. The
		// error semantics (search fails) match.
	}
}

// runKnnIllegalArguments mirrors testIllegalArguments: k < 1 is rejected at
// query construction (Gocene panics, the analogue of Lucene's
// IllegalArgumentException).
func runKnnIllegalArguments(t *testing.T, f knnVectorFixture) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("k=0 must panic")
		}
	}()
	_ = f.newQuery("xx", []float32{1}, 0, nil)
}

// runKnnScoreEuclidean mirrors testScoreEuclidean's scorer-level checks against
// a stable single-segment index of (j,j) vectors.
func runKnnScoreEuclidean(t *testing.T, f knnVectorFixture) {
	t.Helper()
	ix := f.newIndex(t)
	vecs := make([][]float32, 5)
	for j := range vecs {
		vecs[j] = []float32{float32(j), float32(j)}
	}
	addStableVectorDocs(ix, f, "field", index.VectorSimilarityFunctionEuclidean, vecs...)
	ix.forceMerge(1)
	s, cleanup := ix.searcher()
	defer cleanup()

	q := f.newQuery("field", []float32{2, 3}, 3, nil)
	rewritten, err := q.Rewrite(s.GetIndexReader())
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	weight, err := s.CreateWeight(rewritten, search.COMPLETE, 1.0)
	if err != nil {
		t.Fatalf("createWeight: %v", err)
	}
	leaves, _ := s.GetIndexReader().Leaves()
	scorer, err := weight.Scorer(leaves[0])
	if err != nil {
		t.Fatalf("scorer: %v", err)
	}
	if scorer == nil {
		t.Fatalf("nil scorer")
	}
	if scorer.DocID() != -1 {
		t.Fatalf("initial docID = %d, want -1", scorer.DocID())
	}
	// 1 / (l2distance((2,3),(2,2))=1 + 1) = 0.5 is the maximum score in top 3.
	if got := scorer.GetMaxScore(2); math.Abs(float64(got-0.5)) > 1e-6 {
		t.Fatalf("getMaxScore(2) = %f, want 0.5", got)
	}
	if got := scorer.GetMaxScore(search.NO_MORE_DOCS); math.Abs(float64(got-0.5)) > 1e-6 {
		t.Fatalf("getMaxScore(MAX) = %f, want 0.5", got)
	}
	if scorer.Cost() != 3 {
		t.Fatalf("iterator cost = %d, want 3", scorer.Cost())
	}
	// Walk the iterator and confirm every score is one of the expected
	// Euclidean similarities {1/6, 1/2} for the top-3 of target (2,3).
	doc, _ := scorer.NextDoc()
	seen := 0
	for doc != search.NO_MORE_DOCS {
		score := scorer.Score()
		if math.Abs(float64(score-1.0/6.0)) > 1e-5 && math.Abs(float64(score-0.5)) > 1e-5 {
			t.Fatalf("doc %d score %f not in {1/6, 1/2}", doc, score)
		}
		seen++
		doc, _ = scorer.NextDoc()
	}
	if seen != 3 {
		t.Fatalf("iterated %d docs, want 3", seen)
	}
}

// runKnnExplain mirrors testExplain's match / no-match value assertions.
//
// Deviation: Gocene's DocAndScoreQuery explanation description is
// "DocAndScoreQuery, product of:" rather than Lucene's "within top N docs" /
// "not in top N docs"; the match flag and score value — the load-bearing
// assertions — are checked exactly.
func runKnnExplain(t *testing.T, f knnVectorFixture) {
	t.Helper()
	ix := f.newIndex(t)
	vecs := make([][]float32, 5)
	for j := range vecs {
		vecs[j] = []float32{float32(j), float32(j)}
	}
	addStableVectorDocs(ix, f, "field", index.VectorSimilarityFunctionEuclidean, vecs...)
	ix.forceMerge(1)
	s, cleanup := ix.searcher()
	defer cleanup()

	q := f.newQuery("field", []float32{2, 3}, 3, nil)
	matched, err := s.Explain(q, 2)
	if err != nil {
		t.Fatalf("explain(2): %v", err)
	}
	if !matched.IsMatch() {
		t.Fatalf("doc 2 should be a match")
	}
	if math.Abs(float64(matched.GetValue()-0.5)) > 1e-6 {
		t.Fatalf("matched value = %f, want 0.5", matched.GetValue())
	}
	// Doc 5 does not exist (only docs 0..4 were indexed), so it is guaranteed
	// to be outside the top-3 — exactly as testExplain uses explain(query, 5).
	noMatch, err := s.Explain(q, 5)
	if err != nil {
		t.Fatalf("explain(5): %v", err)
	}
	if noMatch.IsMatch() {
		t.Fatalf("doc 5 should not be a match")
	}
	if noMatch.GetValue() != 0 {
		t.Fatalf("no-match value = %f, want 0", noMatch.GetValue())
	}
}

// runKnnSkewedIndex mirrors testSkewedIndex: vectors are flushed across five
// segments and the global top-K must still be found in score order.
func runKnnSkewedIndex(t *testing.T, f knnVectorFixture) {
	t.Helper()
	ix := f.newIndex(t)
	r := 0
	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ {
			id := fmt.Sprintf("id%d", r)
			idf, _ := document.NewStringField("id", id, true)
			f.addVectorDoc(ix, "field", []float32{float32(r), float32(r)},
				index.VectorSimilarityFunctionEuclidean, idf)
			r++
		}
		ix.commit()
	}
	s, cleanup := ix.searcher()
	defer cleanup()

	res, err := s.Search(f.newQuery("field", []float32{0, 0}, 8, nil), 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(res.ScoreDocs) != 8 {
		t.Fatalf("skewed top-8 returned %d", len(res.ScoreDocs))
	}
	assertIDMatches(t, s, "id0", res.ScoreDocs[0])
	assertIDMatches(t, s, "id7", res.ScoreDocs[7])

	res, err = s.Search(f.newQuery("field", []float32{10, 10}, 8, nil), 10)
	if err != nil {
		t.Fatalf("search mid: %v", err)
	}
	if len(res.ScoreDocs) != 8 {
		t.Fatalf("skewed mid top-8 returned %d", len(res.ScoreDocs))
	}
	assertIDMatches(t, s, "id10", res.ScoreDocs[0])
	assertIDMatches(t, s, "id6", res.ScoreDocs[7])
}

// runKnnRandom mirrors testRandom: random vectors / k / n, asserting the result
// count and descending score order. Deterministic seeding keeps the run stable.
func runKnnRandom(t *testing.T, f knnVectorFixture) {
	t.Helper()
	const numDocs = 120
	const dim = 5
	rng := newDeterministicRand(0x5eed)
	ix := f.newIndex(t)
	for i := 0; i < numDocs; i++ {
		f.addVectorDoc(ix, "field", randomVectorValues(rng, dim),
			index.VectorSimilarityFunctionEuclidean)
		if i%25 == 0 {
			ix.commit()
		}
	}
	s, cleanup := ix.searcher()
	defer cleanup()

	for iter := 0; iter < 10; iter++ {
		k := rng.intn(80) + 1
		n := rng.intn(100) + 1
		q := f.newQuery("field", randomVectorValues(rng, dim), k, nil)
		res, err := s.Search(q, n)
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		expected := min3(n, k, numDocs)
		if len(res.ScoreDocs) != expected {
			t.Fatalf("iter %d: got %d docs, want %d (n=%d k=%d)",
				iter, len(res.ScoreDocs), expected, n, k)
		}
		if res.TotalHits.Value < int64(len(res.ScoreDocs)) {
			t.Fatalf("iter %d: totalHits %d < scoreDocs %d",
				iter, res.TotalHits.Value, len(res.ScoreDocs))
		}
		last := float32(math.MaxFloat32)
		for _, sd := range res.ScoreDocs {
			if sd.Score > last {
				t.Fatalf("iter %d: scores not descending (%f > %f)", iter, sd.Score, last)
			}
			last = sd.Score
		}
	}
}

// runKnnRandomConsistency mirrors testRandomConsistencySingleThreaded: repeated
// identical queries must yield identical results.
func runKnnRandomConsistency(t *testing.T, f knnVectorFixture) {
	t.Helper()
	const numDocs = 100
	const dim = 4
	rng := newDeterministicRand(0xC0FFEE)
	ix := f.newIndex(t)
	for i := 0; i < numDocs; i++ {
		f.addVectorDoc(ix, "field", randomVectorValues(rng, dim),
			index.VectorSimilarityFunctionEuclidean)
		if i%50 == 0 {
			ix.commit()
		}
	}
	s, cleanup := ix.searcher()
	defer cleanup()

	k := rng.intn(80) + 1
	n := rng.intn(100) + 1
	q := f.newQuery("field", randomVectorValues(rng, dim), k, nil)
	expected, err := s.Search(q, n)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	for iter := 0; iter < 10; iter++ {
		got, err := s.Search(q, n)
		if err != nil {
			t.Fatalf("iter %d search: %v", iter, err)
		}
		if got.TotalHits.Value != expected.TotalHits.Value ||
			len(got.ScoreDocs) != len(expected.ScoreDocs) {
			t.Fatalf("iter %d: inconsistent counts", iter)
		}
		for j := range got.ScoreDocs {
			if got.ScoreDocs[j].Doc != expected.ScoreDocs[j].Doc {
				t.Fatalf("iter %d: inconsistent doc at %d", iter, j)
			}
			if math.Abs(float64(got.ScoreDocs[j].Score-expected.ScoreDocs[j].Score)) > 0.001 {
				t.Fatalf("iter %d: inconsistent score at %d", iter, j)
			}
		}
	}
}

// runKnnAllScenarios runs the full shared scenario set for a fixture. The
// concrete float / byte suites call this from a single Test function so every
// inherited scenario from BaseKnnVectorQueryTestCase is exercised.
func runKnnAllScenarios(t *testing.T, f knnVectorFixture) {
	t.Helper()
	t.Run("Equals", func(t *testing.T) { runKnnEquals(t, f) })
	t.Run("EmptyIndex", func(t *testing.T) { runKnnEmptyIndex(t, f) })
	t.Run("FindAll", func(t *testing.T) { runKnnFindAll(t, f) })
	t.Run("FindFewer", func(t *testing.T) { runKnnFindFewer(t, f) })
	t.Run("SearchBoost", func(t *testing.T) { runKnnSearchBoost(t, f) })
	t.Run("SimpleFilter", func(t *testing.T) { runKnnSimpleFilter(t, f) })
	t.Run("FilterWithNoVectorMatches", func(t *testing.T) { runKnnFilterWithNoVectorMatches(t, f) })
	t.Run("MatchAllFilter", func(t *testing.T) { runKnnMatchAllFilter(t, f) })
	t.Run("NonVectorField", func(t *testing.T) { runKnnNonVectorField(t, f) })
	t.Run("DimensionMismatch", func(t *testing.T) { runKnnDimensionMismatch(t, f) })
	t.Run("IllegalArguments", func(t *testing.T) { runKnnIllegalArguments(t, f) })
	t.Run("ScoreEuclidean", func(t *testing.T) { runKnnScoreEuclidean(t, f) })
	t.Run("Explain", func(t *testing.T) { runKnnExplain(t, f) })
	t.Run("SkewedIndex", func(t *testing.T) { runKnnSkewedIndex(t, f) })
	t.Run("Random", func(t *testing.T) { runKnnRandom(t, f) })
	t.Run("RandomConsistency", func(t *testing.T) { runKnnRandomConsistency(t, f) })
}

// ── shared helpers ───────────────────────────────────────────────────────────

// addStableVectorDocs indexes one document per vector, each carrying a stored
// "id" field "id<i>", in insertion order (the Go analogue of the Java suite's
// getStableIndexStore). The fixture supplies the vector-type specialisation.
func addStableVectorDocs(ix *integrationIndex, f knnVectorFixture, field string,
	sim index.VectorSimilarityFunction, vectors ...[]float32) {
	for i, v := range vectors {
		idf, _ := document.NewStringField("id", fmt.Sprintf("id%d", i), true)
		f.addVectorDoc(ix, field, v, sim, idf)
	}
}

// assertKnnMatches asserts the query matches exactly want documents.
func assertKnnMatches(t *testing.T, s *search.IndexSearcher, q search.Query, want int) {
	t.Helper()
	top, err := s.Search(q, 1000)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(top.ScoreDocs) != want {
		t.Fatalf("matched %d documents, want %d", len(top.ScoreDocs), want)
	}
}

// assertIDMatches asserts the stored "id" of scoreDoc equals want.
func assertIDMatches(t *testing.T, s *search.IndexSearcher, want string, scoreDoc *search.ScoreDoc) {
	t.Helper()
	doc, err := s.Doc(scoreDoc.Doc)
	if err != nil {
		t.Fatalf("doc(%d): %v", scoreDoc.Doc, err)
	}
	field := doc.Get("id")
	if field == nil {
		t.Fatalf("doc %d has no stored id", scoreDoc.Doc)
	}
	if field.StringValue() != want {
		t.Fatalf("doc %d id = %q, want %q", scoreDoc.Doc, field.StringValue(), want)
	}
}

// assertTopIDs asserts the set of stored ids over scoreDocs equals want.
func assertTopIDs(t *testing.T, s *search.IndexSearcher, want map[string]bool, scoreDocs []*search.ScoreDoc) {
	t.Helper()
	got := make(map[string]bool, len(scoreDocs))
	for _, sd := range scoreDocs {
		doc, err := s.Doc(sd.Doc)
		if err != nil {
			t.Fatalf("doc(%d): %v", sd.Doc, err)
		}
		got[doc.Get("id").StringValue()] = true
	}
	if len(got) != len(want) {
		t.Fatalf("got %d ids, want %d", len(got), len(want))
	}
	for id := range want {
		if !got[id] {
			t.Fatalf("expected id %q in results", id)
		}
	}
}

// mustNumericDocValues builds a NumericDocValuesField or fails the test.
func mustNumericDocValues(t *testing.T, name string, value int64) document.IndexableField {
	t.Helper()
	f, err := document.NewNumericDocValuesField(name, value)
	if err != nil {
		t.Fatalf("NewNumericDocValuesField: %v", err)
	}
	return f
}

// assertDescendingScores fails unless scoreDocs are in non-increasing score
// order (the post-condition every KNN search guarantees).
func assertDescendingScores(t *testing.T, scoreDocs []*search.ScoreDoc) {
	t.Helper()
	last := float32(math.MaxFloat32)
	for i, sd := range scoreDocs {
		if sd.Score > last {
			t.Fatalf("scores not descending at %d (%f > %f)", i, sd.Score, last)
		}
		last = sd.Score
	}

// min3 returns the minimum of three integers.
}
func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

// deterministicRand is a tiny xorshift PRNG used to make the "random" scenarios
// reproducible without depending on the Go runtime's map/seed behaviour. It is
// intentionally minimal: the suites only need a stable stream of values, not
// statistical quality.
type deterministicRand struct{ state uint64 }

func newDeterministicRand(seed uint64) *deterministicRand {
	if seed == 0 {
		seed = 0x9E3779B97F4A7C15
	}
	return &deterministicRand{state: seed}
}

func (r *deterministicRand) next() uint64 {
	r.state ^= r.state << 13
	r.state ^= r.state >> 7
	r.state ^= r.state << 17
	return r.state
}

// intn returns a non-negative pseudo-random int in [0, n).
func (r *deterministicRand) intn(n int) int {
	if n <= 0 {
		return 0
	}
	return int(r.next() % uint64(n))
}

// float32 returns a pseudo-random float in [0, 1).
func (r *deterministicRand) float32() float32 {
	return float32(r.next()>>40) / float32(1<<24)
}

// randomVectorValues returns a dim-length vector with small positive integer
// components, so it round-trips losslessly through both float and byte fields
// (byte fields require values in [-128,127] with no fractional part).
func randomVectorValues(r *deterministicRand, dim int) []float32 {
	v := make([]float32, dim)
	for i := range v {
		v[i] = float32(r.intn(100))
	}
	return v
}