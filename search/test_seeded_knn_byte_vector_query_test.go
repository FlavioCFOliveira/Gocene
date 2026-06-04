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

// TestSeededKnnByteVectorQuery_SeedWithTimeout is deferred for the same reason
// as the float counterpart.
func TestSeededKnnByteVectorQuery_SeedWithTimeout(t *testing.T) {
	t.Fatalf("testSeedWithTimeout requires IndexSearcher.setTimeout + IndexSearcher.count + the HNSW seeded entry-point strategy (KnnSearchStrategy.Seeded), none of which are wired in Gocene yet")
}
