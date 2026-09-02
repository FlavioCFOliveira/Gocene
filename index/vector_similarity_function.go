//go:build ignore

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// VectorSimilarityFunction describes the method used to determine the nearest neighbors.
// Mirrors org.apache.lucene.index.VectorSimilarityFunction from Apache Lucene 10.5.0.
type VectorSimilarityFunction interface {
	// Compare calculates a similarity score between the two vectors. Higher scores correspond to closer vectors.
	CompareFloat(v1, v2 []float32) float32
	// CompareBytes calculates a similarity score between the two vectors.
	CompareBytes(v1, v2 []byte) float32
}

// Euclidean similarity function.
type euclideanSimilarity struct{}

func (s euclideanSimilarity) CompareFloat(v1, v2 []float32) float32 {
	return util.NormalizeDistanceToUnitInterval(util.SquareDistance(v1, v2))
}

func (s euclideanSimilarity) CompareBytes(v1, v2 []byte) float32 {
	return 1 / (1 + float32(util.SquareDistanceBytes(v1, v2)))
}

// DotProduct similarity function.
type dotProductSimilarity struct{}

func (s dotProductSimilarity) CompareFloat(v1, v2 []float32) float32 {
	return util.NormalizeToUnitInterval(util.DotProduct(v1, v2))
}

func (s dotProductSimilarity) CompareBytes(v1, v2 []byte) float32 {
	return util.DotProductScore(v1, v2)
}

// Cosine similarity function.
type cosineSimilarity struct{}

func (s cosineSimilarity) CompareFloat(v1, v2 []float32) float32 {
	return util.NormalizeToUnitInterval(util.Cosine(v1, v2))
}

func (s cosineSimilarity) CompareBytes(v1, v2 []byte) float32 {
	return (1 + util.CosineBytes(v1, v2)) / 2
}

// MaximumInnerProduct similarity function.
type maximumInnerProductSimilarity struct{}

func (s maximumInnerProductSimilarity) CompareFloat(v1, v2 []float32) float32 {
	return util.ScaleMaxInnerProductScore(util.DotProduct(v1, v2))
}

func (s maximumInnerProductSimilarity) CompareBytes(v1, v2 []byte) float32 {
	return util.ScaleMaxInnerProductScore(float32(util.DotProductBytes(v1, v2)))
}

var (
	// VectorSimilarityEuclidean is the Euclidean distance similarity function.
	VectorSimilarityEuclidean VectorSimilarityFunction = euclideanSimilarity{}
	// VectorSimilarityDotProduct is the Dot Product similarity function.
	VectorSimilarityDotProduct VectorSimilarityFunction = dotProductSimilarity{}
	// VectorSimilarityCosine is the Cosine similarity function.
	VectorSimilarityCosine VectorSimilarityFunction = cosineSimilarity{}
	// VectorSimilarityMaximumInnerProduct is the Maximum Inner Product similarity function.
	VectorSimilarityMaximumInnerProduct VectorSimilarityFunction = maximumInnerProductSimilarity{}
)
