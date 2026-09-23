// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/spi"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search/knn"
)

// KnnFloatVectorQuery performs a k-nearest neighbour search using float32 vectors.
//
// Mirrors org.apache.lucene.search.KnnFloatVectorQuery from Apache Lucene 10.5.0.
type KnnFloatVectorQuery struct {
	BaseKnnVectorQuery
	target []float32
}

// NewKnnFloatVectorQuery finds the k nearest documents to the target vector according
// to the vectors in the given field.
func NewKnnFloatVectorQuery(field string, target []float32, k int) *KnnFloatVectorQuery {
	return NewKnnFloatVectorQueryWithFilter(field, target, k, nil)
}

// NewKnnFloatVectorQueryWithFilter finds the k nearest documents to the target vector
// according to the vectors in the given field, subject to the provided filter.
func NewKnnFloatVectorQueryWithFilter(field string, target []float32, k int, filter Query) *KnnFloatVectorQuery {
	return NewKnnFloatVectorQueryWithStrategy(field, target, k, filter, knn.DefaultHnsw)
}

// NewKnnFloatVectorQueryWithStrategy finds the k nearest documents to the target vector
// according to the vectors in the given field, subject to the provided filter and
// using the specified search strategy.
func NewKnnFloatVectorQueryWithStrategy(
	field string,
	target []float32,
	k int,
	filter Query,
	strategy knn.KnnSearchStrategy,
) *KnnFloatVectorQuery {
	// Validate target vector (mirroring Lucene's VectorUtil.checkFinite).
	for i, v := range target {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			panic(fmt.Sprintf("target vector contains non-finite value at index %d: %f", i, v))
		}
	}

	q := &KnnFloatVectorQuery{
		target: target,
	}
	q.BaseKnnVectorQuery = NewBaseKnnVectorQuery(q, field, k, filter, strategy)
	return q
}

// ApproximateSearch executes an approximate KNN search on one leaf.
//
// Mirrors KnnFloatVectorQuery.approximateSearch(LeafReaderContext, AcceptDocs,
// int, KnnCollectorManager).
func (q *KnnFloatVectorQuery) ApproximateSearch(
	ctx *index.LeafReaderContext,
	acceptDocs AcceptDocs,
	visitedLimit int,
	collectorManager knn.KnnCollectorManager,
) (*TopDocs, error) {
	knnCollector, err := collectorManager.NewCollector(visitedLimit, q.strategy, ctx)
	if err != nil {
		return nil, err
	}
	reader := ctx.LeafReader()
	floatVectorValues, err := reader.GetFloatVectorValues(q.field)
	if err != nil {
		return nil, err
	}
	if floatVectorValues == nil {
		if err := index.CheckFloatVectorField(reader, q.field); err != nil {
			return nil, err
		}
		return emptyTopDocs(), nil
	}

	if min(knnCollector.K(), floatVectorValues.Size()) == 0 {
		return emptyTopDocs(), nil
	}

	bits, err := acceptDocs.Bits()
	if err != nil {
		return nil, err
	}
	if err := reader.SearchNearestVectorsCollector(q.field, q.target, knnCollector, bits); err != nil {
		return nil, err
	}

	results := knnCollector.TopDocs()
	if results == nil {
		return emptyTopDocs(), nil
	}
	return results, nil
}

// CreateVectorScorer returns a VectorScorer for exact brute-force search on the leaf.
//
// Mirrors AbstractKnnVectorQuery.createVectorScorer.
func (q *KnnFloatVectorQuery) CreateVectorScorer(ctx *index.LeafReaderContext, fi *index.FieldInfo) (VectorScorer, error) {
	reader := ctx.LeafReader()
	vectorValues, err := reader.GetFloatVectorValues(q.field)
	if err != nil {
		return nil, err
	}
	if vectorValues == nil {
		return nil, nil
	}

	// FloatVectorValues.Scorer returns an interface{} in Gocene; we cast it to VectorScorer.
	scorer, err := vectorValues.Scorer(q.target)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return scorer.(VectorScorer), nil
}

// ToString returns the string representation of the query.
//
// Mirrors KnnFloatVectorQuery.toString(String): the field argument is
// ignored, target[0] is rendered with Java's Float.toString and the filter
// with its toString().
func (q *KnnFloatVectorQuery) ToString(field string) string {
	var buffer strings.Builder
	buffer.WriteString("KnnFloatVectorQuery:")
	buffer.WriteString(q.field + "[" + formatJavaFloatingPoint(float64(q.target[0]), 32) + ",...]")
	buffer.WriteString("[" + strconv.Itoa(q.k) + "]")
	if q.filter != nil {
		buffer.WriteString("[" + queryToString(q.filter, "") + "]")
	}
	return buffer.String()
}

// String renders Query.toString(), which is toString("").
func (q *KnnFloatVectorQuery) String() string {
	return q.ToString("")
}

// Equals checks if two KnnFloatVectorQueries are identical.
//
// Mirrors KnnFloatVectorQuery.equals.
func (q *KnnFloatVectorQuery) Equals(other spi.Query) bool {
	if q == other {
		return true
	}
	if other == nil {
		return false
	}
	that, ok := other.(*KnnFloatVectorQuery)
	if !ok {
		return false
	}
	if !q.BaseKnnVectorQuery.EqualsBase(&that.BaseKnnVectorQuery) {
		return false
	}
	return slices.Equal(q.target, that.target)
}

// HashCode returns the hash code of the query.
//
// Mirrors KnnFloatVectorQuery.hashCode.
func (q *KnnFloatVectorQuery) HashCode() int {
	result := q.BaseKnnVectorQuery.HashCodeBase()
	// mirrors Arrays.hashCode(target)
	targetHash := 1
	for _, v := range q.target {
		// float32 to bits for hashing
		bits := math.Float32bits(v)
		targetHash = 31*targetHash + int(bits)
	}
	return 31*result + targetHash
}

// GetTargetCopy returns a copy of the target query vector.
func (q *KnnFloatVectorQuery) GetTargetCopy() []float32 {
	res := make([]float32, len(q.target))
	copy(res, q.target)
	return res
}
