// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestSeededKnnByteVectorQuery.java
//
// Byte analogue of TestSeededKnnFloatVectorQuery: every KnnByteVectorQuery is
// wrapped in a SeededKnnVectorQuery seeded by MatchNoDocsQuery, and the full
// inherited BaseKnnVectorQueryTestCase scenario set runs through it.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// seededByteKnnFixture wraps byteKnnFixture's queries in a SeededKnnVectorQuery
// seeded by MatchNoDocsQuery.
type seededByteKnnFixture struct {
	byteKnnFixture
}

func (seededByteKnnFixture) newQuery(field string, target []float32, k int, filter search.Query) search.Query {
	b := floatToBytes(target)
	var inner search.Query
	if filter == nil {
		inner = search.NewKnnByteVectorQuery(field, b, k)
	} else {
		inner = search.NewKnnByteVectorQueryWithFilter(field, b, k, filter)
	}
	return search.NewSeededKnnVectorQuery(field, inner, search.NewMatchNoDocsQuery(), k)
}

// TestSeededKnnByteVectorQuery runs the inherited scenario set through the
// seeded-by-MatchNone byte query.
func TestSeededKnnByteVectorQuery(t *testing.T) {
	runKnnAllScenarios(t, seededByteKnnFixture{})
}

// TestSeededKnnByteVectorQuery_RandomWithSeed ports the no-seed fallback path of
// testRandomWithSeed for byte vectors (see the float counterpart for the
// deferred entry-point/timeout assertions).
func TestSeededKnnByteVectorQuery_RandomWithSeed(t *testing.T) {
	const numDocs = 300
	const dim = 5
	rng := newDeterministicRand(0xB17E5)
	f := byteKnnFixture{}
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
		inner := search.NewKnnByteVectorQuery("field", floatToBytes(randomVectorValues(rng, dim)), k)
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

// TestSeededKnnByteVectorQuery_SeedWithTimeout is a structural test for the
// SeedWithTimeout scenario — it verifies the SeededKnnVectorQuery type
// construction and accessor behaviour using a byte vector inner query.
// The full timeout-integration test is deferred until those subsystems are wired.
func TestSeededKnnByteVectorQuery_SeedWithTimeout(t *testing.T) {
	b := floatToBytes([]float32{1, 2, 3})
	inner := search.NewKnnByteVectorQuery("vec", b, 10)
	seed := search.NewMatchAllDocsQuery()
	q := search.NewSeededKnnVectorQuery("vec", inner, seed, 50)
	if q.GetField() != "vec" {
		t.Fatalf("got field %q, want %q", q.GetField(), "vec")
	}
	if q.MaxK() != 50 {
		t.Fatalf("got maxK %d, want 50", q.MaxK())
	}
	if q.Inner() != inner {
		t.Fatalf("Inner() returned different query")
	}
	if q.Seed() != seed {
		t.Fatalf("Seed() returned different query")
	}
	s := q.String()
	if s != "SeededKnnVectorQuery(field=vec, maxK=50)" {
		t.Fatalf("unexpected String: %q", s)
	}
	// Equals / HashCode
	q2 := search.NewSeededKnnVectorQuery("vec", inner.Clone(), search.NewMatchAllDocsQuery(), 50)
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
	// Different maxK not Equal
	q3 := search.NewSeededKnnVectorQuery("vec", inner, seed, 100)
	if q.Equals(q3) {
		t.Fatal("different maxK should not be Equal")
	}

	// Compile-time interface compliance.
	var _ search.Query = q
	_ = q
}
