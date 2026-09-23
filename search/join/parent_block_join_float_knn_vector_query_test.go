// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"math"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Port of
// lucene/join/src/test/org/apache/lucene/search/join/TestParentBlockJoinFloatKnnVectorQuery.java
// (Apache Lucene 10.5.0), which extends ParentBlockJoinKnnVectorQueryTestCase.

func newParentBlockJoinFloatKnnVectorQueryTestCase() *parentBlockJoinKnnVectorQueryTestCase {
	return &parentBlockJoinKnnVectorQueryTestCase{
		getParentJoinKnnQuery: func(t testing.TB, fieldName string, queryVector []float32, childFilter search.Query, k int,
			parentBitSet BitSetProducer) search.Query {
			return NewDiversifyingChildrenFloatKnnVectorQuery(fieldName, queryVector, k, childFilter, parentBitSet)
		},
		getKnnVectorField: func(t testing.TB, name string, vector []float32) document.IndexableField {
			f, err := document.NewKnnFloatVectorFieldEuclidean(name, vector)
			if err != nil {
				t.Fatal(err)
			}
			return f
		},
		getKnnVectorFieldWithSimilarity: func(t testing.TB, name string, vector []float32,
			vectorSimilarityFunction spi.VectorSimilarityFunction) document.IndexableField {
			f, err := document.NewKnnFloatVectorField(name, vector, vectorSimilarityFunction)
			if err != nil {
				t.Fatal(err)
			}
			return f
		},
		randomVector: func(dim int) []float32 {
			v := make([]float32, dim)
			r := random()
			for i := 0; i < dim; i++ {
				v[i] = r.Float32()
			}
			return v
		},
	}
}

func TestParentBlockJoinFloatKnnVectorQueryVectorEncodingMismatch(t *testing.T) {
	c := newParentBlockJoinFloatKnnVectorQueryTestCase()
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
	kvq := NewDiversifyingChildrenByteKnnVectorQuery("field", []byte{1, 2}, 2, nil, parentFilter)
	if _, err := searcher.Search(kvq, 3); err == nil {
		t.Fatal("expected IllegalStateException")
	}
}

func TestParentBlockJoinFloatKnnVectorQueryScoreCosine(t *testing.T) {
	c := newParentBlockJoinFloatKnnVectorQueryTestCase()
	d := newDirectory()
	defer mustClose(t, d)
	func() {
		iwc := index.NewIndexWriterConfig()
		iwc.SetMergePolicy(newMergePolicyNoMock(t))
		w := mustNewIndexWriter(t, d, iwc)
		defer mustClose(t, w)
		for j := 1; j <= 5; j++ {
			mustAddDocuments(t, w,
				newTestDocument(
					c.getKnnVectorFieldWithSimilarity(t, "field", []float32{float32(j), float32(j * j)}, util.CosineSim),
					newStringField(t, "id", itoaPlain(j), true)),
				makeKnnParent(t, []int{j}))
		}
	}()
	reader := mustOpenDirectoryReader(t, d)
	defer mustClose(t, reader)
	assertIntEquals(t, 1, len(mustLeaves(t, reader)))
	searcher := search.NewIndexSearcher(reader)
	parentFilter := knnParentFilter(t, searcher.GetIndexReader())
	query := NewDiversifyingChildrenFloatKnnVectorQuery("field", []float32{2, 3}, 3, nil, parentFilter)
	/* score0 = ((2,3) * (1, 1) = 5) / (||2, 3|| * ||1, 1|| = sqrt(26)), then
	 * normalized by (1 + x) /2.
	 */
	score0 := float32((1 + (2*1+3*1)/math.Sqrt((2*2+3*3)*(1*1+1*1))) / 2)
	/* score1 = ((2,3) * (2, 4) = 16) / (||2, 3|| * ||2, 4|| = sqrt(260)), then
	 * normalized by (1 + x) /2
	 */
	score1 := float32((1 + (2*2+3*4)/math.Sqrt((2*2+3*3)*(2*2+4*4))) / 2)
	assertScorerResults(t, searcher, query, []float32{score0, score1}, []string{"1", "2"}, 2)
}

func TestParentBlockJoinFloatKnnVectorQueryToString(t *testing.T) {
	c := newParentBlockJoinFloatKnnVectorQueryTestCase()
	// test without filter
	query := c.getParentJoinKnnQuery(t, "field", []float32{0, 1}, nil, 10, nil)
	assertStringEquals(t, "DiversifyingChildrenFloatKnnVectorQuery:field[0.0,...][10]", joinQueryToString(query, "ignored"))
	// test with filter
	filter := search.NewTermQuery(index.NewTerm("id", "text"))
	query = c.getParentJoinKnnQuery(t, "field", []float32{0.0, 1.0}, filter, 10, nil)
	assertStringEquals(t, "DiversifyingChildrenFloatKnnVectorQuery:field[0.0,...][10][id:text]", joinQueryToString(query, "ignored"))
}

// itoaPlain renders Integer.toString(int).
func itoaPlain(v int) string { return strconv.Itoa(v) }

func TestParentBlockJoinFloatKnnVectorQueryEmptyIndex(t *testing.T) {
	newParentBlockJoinFloatKnnVectorQueryTestCase().testEmptyIndex(t)
}

func TestParentBlockJoinFloatKnnVectorQueryIndexWithNoVectorsNorParents(t *testing.T) {
	newParentBlockJoinFloatKnnVectorQueryTestCase().testIndexWithNoVectorsNorParents(t)
}

func TestParentBlockJoinFloatKnnVectorQueryIndexWithNoParents(t *testing.T) {
	newParentBlockJoinFloatKnnVectorQueryTestCase().testIndexWithNoParents(t)
}

func TestParentBlockJoinFloatKnnVectorQueryFilterWithNoVectorMatches(t *testing.T) {
	newParentBlockJoinFloatKnnVectorQueryTestCase().testFilterWithNoVectorMatches(t)
}

func TestParentBlockJoinFloatKnnVectorQueryScoringWithMultipleChildren(t *testing.T) {
	newParentBlockJoinFloatKnnVectorQueryTestCase().testScoringWithMultipleChildren(t)
}

func TestParentBlockJoinFloatKnnVectorQuerySkewedIndex(t *testing.T) {
	newParentBlockJoinFloatKnnVectorQueryTestCase().testSkewedIndex(t)
}

func TestParentBlockJoinFloatKnnVectorQueryTimeout(t *testing.T) {
	newParentBlockJoinFloatKnnVectorQueryTestCase().testTimeout(t)
}

func TestParentBlockJoinFloatKnnVectorQueryTwoSegments(t *testing.T) {
	newParentBlockJoinFloatKnnVectorQueryTestCase().testTwoSegments(t)
}

func TestParentBlockJoinFloatKnnVectorQueryRandom(t *testing.T) {
	newParentBlockJoinFloatKnnVectorQueryTestCase().testRandom(t)
}
