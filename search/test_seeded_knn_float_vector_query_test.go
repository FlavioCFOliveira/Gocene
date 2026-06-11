// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestSeededKnnFloatVectorQuery.java
//
// TestSeededKnnFloatVectorQuery extends BaseKnnVectorQueryTestCase, wrapping
// every KnnFloatVectorQuery in a SeededKnnVectorQuery seeded by MatchNoDocsQuery
// (so the seed contributes no entry points and the search falls back to the
// standard approximate/exact path). The Go port supplies a seeded fixture and
// runs the full inherited scenario set through it, then ports the
// seeded-specific scenarios to the extent Gocene's KNN surface supports.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// seededFloatKnnFixture wraps floatKnnFixture's queries in a
// SeededKnnVectorQuery seeded by MatchNoDocsQuery.
type seededFloatKnnFixture struct {
	floatKnnFixture
}

func (seededFloatKnnFixture) newQuery(field string, target []float32, k int, filter search.Query) search.Query {
	var inner search.Query
	if filter == nil {
		inner = search.NewKnnFloatVectorQuery(field, target, k)
	} else {
		inner = search.NewKnnFloatVectorQueryWithFilter(field, target, k, filter)
	}
	return search.NewSeededKnnVectorQuery(field, inner, search.NewMatchNoDocsQuery(), k)
}

// TestSeededKnnFloatVectorQuery runs the inherited BaseKnnVectorQueryTestCase
// scenario set through the seeded-by-MatchNone float query. Because the seed
// matches no documents, the seeded query must produce exactly the same results
// as the underlying KnnFloatVectorQuery — which the shared scenarios assert.
func TestSeededKnnFloatVectorQuery(t *testing.T) {
	runKnnAllScenarios(t, seededFloatKnnFixture{})
}

// TestSeededKnnFloatVectorQuery_RandomWithSeed ports the portion of
// testRandomWithSeed that Gocene's KNN surface supports: with a MatchNoDocs
// seed (no entry points), the seeded query falls back to full approximate
// search and returns min(n, k, numDocs) hits in descending score order, exactly
// matching the underlying KnnFloatVectorQuery.
//
// Deviation: the upstream test also asserts (a) that the Seeded HNSW
// entry-point strategy is invoked exactly once per leaf when the seed matches
// documents, via AssertingSeededKnnVectorQuery / KnnSearchStrategy.Seeded entry
// counting, and (b) timeout-collector wrapping via searcher.setTimeout. Those
// rely on the seeded entry-point strategy and IndexSearcher timeout plumbing,
// which are not yet wired in Gocene (see the suite-level deferral below).
func TestSeededKnnFloatVectorQuery_RandomWithSeed(t *testing.T) {
	const numDocs = 300
	const dim = 5
	rng := newDeterministicRand(0xA11CE)
	f := floatKnnFixture{}
	ix := newIntegrationIndex(t)
	numWithVector := 0
	for i := 0; i < numDocs; i++ {
		if rng.intn(2) == 0 {
			f.addVectorDoc(ix, "field", randomVectorValues(rng, dim),
				index.VectorSimilarityFunctionEuclidean,
				mustNumericDocValues(t, "tag", int64(i)))
			numWithVector++
		} else {
			doc := document.NewDocument()
			doc.Add(mustNumericDocValues(t, "tag", int64(i)))
			ix.addDoc(doc)
		}
	}
	ix.forceMerge(1)
	s, cleanup := ix.searcher()
	defer cleanup()

	for iter := 0; iter < 10; iter++ {
		k := rng.intn(10) + 1
		n := rng.intn(100) + 1
		inner := search.NewKnnFloatVectorQuery("field", randomVectorValues(rng, dim), k)
		// No seed documents -> falls back on full approximate search.
		q := search.NewSeededKnnVectorQuery("field", inner, search.NewMatchNoDocsQuery(), k)
		res, err := s.Search(q, n)
		if err != nil {
			t.Fatalf("iter %d: search: %v", iter, err)
		}
		expected := min3(n, k, numWithVector)
		if len(res.ScoreDocs) != expected {
			t.Fatalf("iter %d: got %d docs, want %d", iter, len(res.ScoreDocs), expected)
		}
		assertDescendingScores(t, res.ScoreDocs)
	}
}

// TestSeededKnnFloatVectorQuery_SeedWithTimeout is a structural test for the
// SeedWithTimeout scenario — it verifies the SeededKnnVectorQuery type
// construction and accessor behaviour. The full timeout-integration test
// (IndexSearcher.setTimeout + IndexSearcher.count + HNSW seeded entry-point
// strategy) is deferred until those subsystems are wired.
func TestSeededKnnFloatVectorQuery_SeedWithTimeout(t *testing.T) {
	inner := search.NewKnnFloatVectorQuery("vec", []float32{1, 2, 3}, 10)
	seed := search.NewMatchAllDocsQuery()
	q := search.NewSeededKnnVectorQuery("vec", inner, seed, 100)
	if q.GetField() != "vec" {
		t.Fatalf("got field %q, want %q", q.GetField(), "vec")
	}
	if q.MaxK() != 100 {
		t.Fatalf("got maxK %d, want 100", q.MaxK())
	}
	if q.Inner() != inner {
		t.Fatalf("Inner() returned different query")
	}
	if q.Seed() != seed {
		t.Fatalf("Seed() returned different query")
	}
	s := q.String()
	if s != "SeededKnnVectorQuery(field=vec, maxK=100)" {
		t.Fatalf("unexpected String: %q", s)
	}
	// Equals / HashCode
	q2 := search.NewSeededKnnVectorQuery("vec", inner.Clone(), search.NewMatchAllDocsQuery(), 100)
	if !q.Equals(q2) {
		t.Fatal("equal queries should be Equal")
	}
	if q.HashCode() != q2.HashCode() {
		t.Fatal("equal queries should have same HashCode")
	}
	// Clone
	clone := q.Clone()
	if !q.Equals(clone) {
		t.Fatal("clone should Equal original")
	}
	// Different field not Equal
	q3 := search.NewSeededKnnVectorQuery("other", inner, seed, 100)
	if q.Equals(q3) {
		t.Fatal("different field should not be Equal")
	}

	// Compile-time interface compliance.
	var _ search.Query = q
	_ = q
}
