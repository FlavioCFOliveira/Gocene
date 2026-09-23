// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Port of
// lucene/join/src/test/org/apache/lucene/search/join/TestParentBlockJoinByteKnnVectorQuery.java
// (Apache Lucene 10.5.0), which extends ParentBlockJoinKnnVectorQueryTestCase.

func newParentBlockJoinByteKnnVectorQueryTestCase() *parentBlockJoinKnnVectorQueryTestCase {
	return &parentBlockJoinKnnVectorQueryTestCase{
		getParentJoinKnnQuery: func(t testing.TB, fieldName string, queryVector []float32, childFilter search.Query, k int,
			parentBitSet BitSetProducer) search.Query {
			return NewDiversifyingChildrenByteKnnVectorQuery(fieldName, fromFloat(t, queryVector), k, childFilter, parentBitSet)
		},
		getKnnVectorField: func(t testing.TB, name string, vector []float32) document.IndexableField {
			f, err := document.NewKnnByteVectorFieldEuclidean(name, fromFloat(t, vector))
			if err != nil {
				t.Fatal(err)
			}
			return f
		},
		getKnnVectorFieldWithSimilarity: func(t testing.TB, name string, vector []float32,
			vectorSimilarityFunction spi.VectorSimilarityFunction) document.IndexableField {
			f, err := document.NewKnnByteVectorField(name, fromFloat(t, vector), vectorSimilarityFunction)
			if err != nil {
				t.Fatal(err)
			}
			return f
		},
		randomVector: func(dim int) []float32 {
			v := randomBinaryTerm(random(), dim)
			// clip at -127 to avoid overflow
			for i := range v {
				if int8(v[i]) == -128 {
					v[i] = byte(0x81) // -127
				}
			}
			v1 := make([]float32, len(v))
			vi := 0
			for i := range v {
				v1[vi] = float32(int8(v[i]))
				vi++
			}
			return v1
		},
	}
}

func TestParentBlockJoinByteKnnVectorQueryVectorEncodingMismatch(t *testing.T) {
	c := newParentBlockJoinByteKnnVectorQueryTestCase()
	d := newDirectory()
	defer mustClose(t, d)
	func() {
		iwc := index.NewIndexWriterConfig()
		iwc.SetMergePolicy(newMergePolicyNoMock(t))
		w := mustNewIndexWriter(t, d, iwc)
		defer mustClose(t, w)
		mustAddDocuments(t, w,
			newTestDocument(c.getKnnVectorFieldWithSimilarity(t, "field", []float32{1, 1}, util.CosineSim)),
			makeKnnParent(t, []int{1}))
	}()
	reader := mustOpenDirectoryReader(t, d)
	defer mustClose(t, reader)
	searcher := newSearcher(t, reader)
	parentFilter := knnParentFilter(t, reader)
	kvq := NewDiversifyingChildrenFloatKnnVectorQuery("field", []float32{1, 2}, 2, nil, parentFilter)
	if _, err := searcher.Search(kvq, 3); err == nil {
		t.Fatal("expected IllegalStateException")
	}
}

func TestParentBlockJoinByteKnnVectorQueryToString(t *testing.T) {
	c := newParentBlockJoinByteKnnVectorQueryTestCase()
	// test without filter
	query := c.getParentJoinKnnQuery(t, "field", []float32{0, 1}, nil, 10, nil)
	assertStringEquals(t, "DiversifyingChildrenByteKnnVectorQuery:field[0,...][10]", joinQueryToString(query, "ignored"))
	// test with filter
	filter := search.NewTermQuery(index.NewTerm("id", "text"))
	query = c.getParentJoinKnnQuery(t, "field", []float32{0, 1}, filter, 10, nil)
	assertStringEquals(t, "DiversifyingChildrenByteKnnVectorQuery:field[0,...][10][id:text]", joinQueryToString(query, "ignored"))
}

func fromFloat(t testing.TB, queryVector []float32) []byte {
	query := make([]byte, len(queryVector))
	for i := range queryVector {
		if util.AssertsEnabled() && queryVector[i] != float32(int8(queryVector[i])) {
			t.Fatal(util.NewAssertionError("queryVector[i] == (byte) queryVector[i]"))
		}
		query[i] = byte(int8(queryVector[i]))
	}
	return query
}

func TestParentBlockJoinByteKnnVectorQueryEmptyIndex(t *testing.T) {
	newParentBlockJoinByteKnnVectorQueryTestCase().testEmptyIndex(t)
}

func TestParentBlockJoinByteKnnVectorQueryIndexWithNoVectorsNorParents(t *testing.T) {
	newParentBlockJoinByteKnnVectorQueryTestCase().testIndexWithNoVectorsNorParents(t)
}

func TestParentBlockJoinByteKnnVectorQueryIndexWithNoParents(t *testing.T) {
	newParentBlockJoinByteKnnVectorQueryTestCase().testIndexWithNoParents(t)
}

func TestParentBlockJoinByteKnnVectorQueryFilterWithNoVectorMatches(t *testing.T) {
	newParentBlockJoinByteKnnVectorQueryTestCase().testFilterWithNoVectorMatches(t)
}

func TestParentBlockJoinByteKnnVectorQueryScoringWithMultipleChildren(t *testing.T) {
	newParentBlockJoinByteKnnVectorQueryTestCase().testScoringWithMultipleChildren(t)
}

func TestParentBlockJoinByteKnnVectorQuerySkewedIndex(t *testing.T) {
	newParentBlockJoinByteKnnVectorQueryTestCase().testSkewedIndex(t)
}

func TestParentBlockJoinByteKnnVectorQueryTimeout(t *testing.T) {
	newParentBlockJoinByteKnnVectorQueryTestCase().testTimeout(t)
}

func TestParentBlockJoinByteKnnVectorQueryTwoSegments(t *testing.T) {
	newParentBlockJoinByteKnnVectorQueryTestCase().testTwoSegments(t)
}

func TestParentBlockJoinByteKnnVectorQueryRandom(t *testing.T) {
	newParentBlockJoinByteKnnVectorQueryTestCase().testRandom(t)
}
