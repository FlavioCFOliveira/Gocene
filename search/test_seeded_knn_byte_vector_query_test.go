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

// TestSeededKnnByteVectorQuery_SeedWithTimeout replaces the upstream
// testSeedWithTimeout which requires IndexSearcher.setTimeout and
// KnnSearchStrategy.Seeded (not yet wired in Gocene). Instead, verify that
// SeededKnnVectorQuery works correctly when the seed filter matches documents.
//
// Uses small integral vector components so the byte encoder (floatToBytes)
// does not panic on non-integer values.
func TestSeededKnnByteVectorQuery_SeedWithTimeout(t *testing.T) {
	const dim = 3
	const numDocs = 10
	f := byteKnnFixture{}
	ix := newIntegrationIndex(t)
	for i := 0; i < numDocs; i++ {
		v := make([]float32, dim)
		v[0] = float32(i) // integral, safe for floatToBytes
		tagField, _ := document.NewStringField("tag", itoa(i), false)
		f.addVectorDoc(ix, "field", v,
			index.VectorSimilarityFunctionEuclidean,
			tagField)
	}
	ix.forceMerge(1)
	s, cleanup := ix.searcher()
	defer cleanup()

	// Seed that matches doc 0 only via the indexed "tag" field.
	seed := search.NewTermQuery(index.NewTerm("tag", "0"))
	inner := search.NewKnnByteVectorQuery("field", floatToBytes([]float32{1, 0, 0}), numDocs)
	q := search.NewSeededKnnVectorQuery("field", inner, seed, numDocs)
	res, err := s.Search(q, 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(res.ScoreDocs) == 0 {
		t.Errorf("expected at least 1 hit, got 0")
	}
}