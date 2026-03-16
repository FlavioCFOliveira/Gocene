// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// VectorSimilarityFunction represents the similarity function for vector comparison
type VectorSimilarityFunction int

const (
	// VectorSimilarityFunctionEuclidean uses squared Euclidean distance
	VectorSimilarityFunctionEuclidean VectorSimilarityFunction = iota
	// VectorSimilarityFunctionDotProduct uses dot product similarity
	VectorSimilarityFunctionDotProduct
	// VectorSimilarityFunctionCosine uses cosine similarity
	VectorSimilarityFunctionCosine
	// VectorSimilarityFunctionMaximumInnerProduct uses maximum inner product
	VectorSimilarityFunctionMaximumInnerProduct
)

// String returns the string representation of the similarity function
func (vsf VectorSimilarityFunction) String() string {
	switch vsf {
	case VectorSimilarityFunctionEuclidean:
		return "EUCLIDEAN"
	case VectorSimilarityFunctionDotProduct:
		return "DOT_PRODUCT"
	case VectorSimilarityFunctionCosine:
		return "COSINE"
	case VectorSimilarityFunctionMaximumInnerProduct:
		return "MAXIMUM_INNER_PRODUCT"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", vsf)
	}
}

// FlatVectorsScorer provides mechanisms to score vectors stored in a flat file
// This is the Go port of Lucene's FlatVectorsScorer
type FlatVectorsScorer interface {
	// GetRandomVectorScorerSupplier returns a supplier that can create scorers
	GetRandomVectorScorerSupplier(similarityFunction VectorSimilarityFunction, vectorValues KnnVectorValues) (FlatRandomVectorScorerSupplier, error)
	// GetRandomVectorScorer returns a scorer for float vectors
	GetRandomVectorScorer(similarityFunction VectorSimilarityFunction, vectorValues KnnVectorValues, target []float32) (FlatRandomVectorScorer, error)
	// GetRandomVectorScorerByte returns a scorer for byte vectors
	GetRandomVectorScorerByte(similarityFunction VectorSimilarityFunction, vectorValues KnnVectorValues, target []byte) (FlatRandomVectorScorer, error)
	// String returns the scorer name
	String() string
}

// FlatRandomVectorScorer scores random nodes against a query vector
// This is the Go port of Lucene's RandomVectorScorer for flat vectors
type FlatRandomVectorScorer interface {
	// Score returns the score between the query and the provided node
	Score(node int) (float32, error)
	// BulkScore scores multiple nodes and stores results in the scores array
	// Returns the maximum score value
	BulkScore(nodes []int, scores []float32, numNodes int) (float32, error)
	// MaxOrd returns the maximum ordinal for this scorer
	MaxOrd() int
	// SetScoringOrdinal sets the ordinal to score against (for updatable scorers)
	SetScoringOrdinal(node int) error
}

// FlatRandomVectorScorerSupplier creates FlatRandomVectorScorer instances
// This is the Go port of Lucene's RandomVectorScorerSupplier for flat vectors
type FlatRandomVectorScorerSupplier interface {
	// Scorer creates a new FlatRandomVectorScorer
	Scorer() (FlatRandomVectorScorer, error)
	// Copy makes a copy of the supplier for thread safety
	Copy() (FlatRandomVectorScorerSupplier, error)
}

// KnnVectorValues provides access to vector values
// This is the Go port of Lucene's KnnVectorValues
type KnnVectorValues interface {
	// Dimension returns the dimension of the vectors
	Dimension() int
	// Size returns the number of vectors
	Size() int
	// GetEncoding returns the vector encoding type
	GetEncoding() index.VectorEncoding
	// Copy creates a copy of the values
	Copy() (KnnVectorValues, error)
}

// FlatByteVectorValues provides access to byte vector values for flat vector scoring
type FlatByteVectorValues interface {
	KnnVectorValues
	// Get returns the byte vector at the given ordinal
	Get(ordinal int) ([]byte, error)
}

// FlatFloatVectorValues provides access to float vector values for flat vector scoring
type FlatFloatVectorValues interface {
	KnnVectorValues
	// Get returns the float vector at the given ordinal
	Get(ordinal int) ([]float32, error)
}

// DefaultFlatVectorScorer is the default implementation of FlatVectorsScorer
type DefaultFlatVectorScorer struct{}

// NewDefaultFlatVectorScorer creates a new DefaultFlatVectorScorer
func NewDefaultFlatVectorScorer() *DefaultFlatVectorScorer {
	return &DefaultFlatVectorScorer{}
}

// String returns the string representation
func (d *DefaultFlatVectorScorer) String() string {
	return "DefaultFlatVectorScorer()"
}

// GetRandomVectorScorerSupplier returns a supplier for random vector scorers
func (d *DefaultFlatVectorScorer) GetRandomVectorScorerSupplier(similarityFunction VectorSimilarityFunction, vectorValues KnnVectorValues) (FlatRandomVectorScorerSupplier, error) {
	return &defaultRandomVectorScorerSupplier{
		similarityFunction: similarityFunction,
		vectorValues:       vectorValues,
	}, nil
}

// GetRandomVectorScorer returns a scorer for float vectors
func (d *DefaultFlatVectorScorer) GetRandomVectorScorer(similarityFunction VectorSimilarityFunction, vectorValues KnnVectorValues, target []float32) (FlatRandomVectorScorer, error) {
	if vectorValues.Dimension() != len(target) {
		return nil, fmt.Errorf("vector query dimension: %d differs from field dimension: %d", len(target), vectorValues.Dimension())
	}
	return &defaultRandomVectorScorer{
		similarityFunction: similarityFunction,
		vectorValues:       vectorValues,
		targetFloat:        target,
		targetByte:         nil,
		scoringOrdinal:     -1,
	}, nil
}

// GetRandomVectorScorerByte returns a scorer for byte vectors
func (d *DefaultFlatVectorScorer) GetRandomVectorScorerByte(similarityFunction VectorSimilarityFunction, vectorValues KnnVectorValues, target []byte) (FlatRandomVectorScorer, error) {
	if vectorValues.Dimension() != len(target) {
		return nil, fmt.Errorf("vector query dimension: %d differs from field dimension: %d", len(target), vectorValues.Dimension())
	}
	return &defaultRandomVectorScorer{
		similarityFunction: similarityFunction,
		vectorValues:       vectorValues,
		targetFloat:        nil,
		targetByte:         target,
		scoringOrdinal:     -1,
	}, nil
}

// defaultRandomVectorScorerSupplier implements FlatRandomVectorScorerSupplier
type defaultRandomVectorScorerSupplier struct {
	similarityFunction VectorSimilarityFunction
	vectorValues       KnnVectorValues
}

// Scorer creates a new FlatRandomVectorScorer
func (s *defaultRandomVectorScorerSupplier) Scorer() (FlatRandomVectorScorer, error) {
	return &defaultRandomVectorScorer{
		similarityFunction: s.similarityFunction,
		vectorValues:       s.vectorValues,
		scoringOrdinal:     -1,
	}, nil
}

// Copy makes a copy of the supplier
func (s *defaultRandomVectorScorerSupplier) Copy() (FlatRandomVectorScorerSupplier, error) {
	copiedValues, err := s.vectorValues.Copy()
	if err != nil {
		return nil, err
	}
	return &defaultRandomVectorScorerSupplier{
		similarityFunction: s.similarityFunction,
		vectorValues:       copiedValues,
	}, nil
}

// defaultRandomVectorScorer implements RandomVectorScorer
type defaultRandomVectorScorer struct {
	similarityFunction VectorSimilarityFunction
	vectorValues       KnnVectorValues
	targetFloat        []float32
	targetByte         []byte
	scoringOrdinal     int
}

// Score returns the score for the given node
func (s *defaultRandomVectorScorer) Score(node int) (float32, error) {
	if s.scoringOrdinal < 0 && s.targetFloat == nil && s.targetByte == nil {
		return 0, fmt.Errorf("scoring ordinal not set and no target vector provided")
	}

	// Calculate score based on similarity function
	switch s.vectorValues.GetEncoding() {
	case index.VectorEncodingFloat32:
		return s.scoreFloat(node)
	case index.VectorEncodingByte:
		return s.scoreByte(node)
	default:
		return 0, fmt.Errorf("unknown vector encoding: %v", s.vectorValues.GetEncoding())
	}
}

// scoreFloat calculates score for float vectors
func (s *defaultRandomVectorScorer) scoreFloat(node int) (float32, error) {
	floatValues, ok := s.vectorValues.(FlatFloatVectorValues)
	if !ok {
		return 0, fmt.Errorf("vector values not float type")
	}

	vec, err := floatValues.Get(node)
	if err != nil {
		return 0, err
	}

	if s.targetFloat != nil {
		return ComputeSimilarity(s.similarityFunction, vec, s.targetFloat), nil
	}

	// If scoring ordinal is set, compare against that ordinal's vector
	if s.scoringOrdinal >= 0 {
		targetVec, err := floatValues.Get(s.scoringOrdinal)
		if err != nil {
			return 0, err
		}
		return ComputeSimilarity(s.similarityFunction, vec, targetVec), nil
	}

	return 0, fmt.Errorf("no target vector set")
}

// scoreByte calculates score for byte vectors
func (s *defaultRandomVectorScorer) scoreByte(node int) (float32, error) {
	byteValues, ok := s.vectorValues.(FlatByteVectorValues)
	if !ok {
		return 0, fmt.Errorf("vector values not byte type")
	}

	vec, err := byteValues.Get(node)
	if err != nil {
		return 0, err
	}

	if s.targetByte != nil {
		return ComputeSimilarityByte(s.similarityFunction, vec, s.targetByte), nil
	}

	if s.scoringOrdinal >= 0 {
		targetVec, err := byteValues.Get(s.scoringOrdinal)
		if err != nil {
			return 0, err
		}
		return ComputeSimilarityByte(s.similarityFunction, vec, targetVec), nil
	}

	return 0, fmt.Errorf("no target vector set")
}

// BulkScore scores multiple nodes
func (s *defaultRandomVectorScorer) BulkScore(nodes []int, scores []float32, numNodes int) (float32, error) {
	if numNodes == 0 {
		return float32(math.Inf(-1)), nil
	}

	maxScore := float32(math.Inf(-1))
	for i := 0; i < numNodes; i++ {
		score, err := s.Score(nodes[i])
		if err != nil {
			return 0, err
		}
		scores[i] = score
		if score > maxScore {
			maxScore = score
		}
	}
	return maxScore, nil
}

// MaxOrd returns the maximum ordinal
func (s *defaultRandomVectorScorer) MaxOrd() int {
	return s.vectorValues.Size()
}

// SetScoringOrdinal sets the scoring ordinal
func (s *defaultRandomVectorScorer) SetScoringOrdinal(node int) error {
	s.scoringOrdinal = node
	return nil
}

// ComputeSimilarity computes similarity between two float vectors
func ComputeSimilarity(simFunc VectorSimilarityFunction, v1, v2 []float32) float32 {
	switch simFunc {
	case VectorSimilarityFunctionEuclidean:
		return EuclideanSimilarity(v1, v2)
	case VectorSimilarityFunctionDotProduct:
		return DotProductSimilarity(v1, v2)
	case VectorSimilarityFunctionCosine:
		return CosineSimilarity(v1, v2)
	case VectorSimilarityFunctionMaximumInnerProduct:
		return MaxInnerProductSimilarity(v1, v2)
	default:
		return 0
	}
}

// ComputeSimilarityByte computes similarity between two byte vectors
func ComputeSimilarityByte(simFunc VectorSimilarityFunction, v1, v2 []byte) float32 {
	switch simFunc {
	case VectorSimilarityFunctionEuclidean:
		return EuclideanSimilarityByte(v1, v2)
	case VectorSimilarityFunctionDotProduct:
		return DotProductSimilarityByte(v1, v2)
	case VectorSimilarityFunctionCosine:
		return CosineSimilarityByte(v1, v2)
	case VectorSimilarityFunctionMaximumInnerProduct:
		return MaxInnerProductSimilarityByte(v1, v2)
	default:
		return 0
	}
}

// EuclideanSimilarity calculates normalized Euclidean similarity
func EuclideanSimilarity(v1, v2 []float32) float32 {
	var sum float32
	for i := range v1 {
		diff := v1[i] - v2[i]
		sum += diff * diff
	}
	// Normalize to unit interval: 1 / (1 + distance)
	return 1.0 / (1.0 + sum)
}

// DotProductSimilarity calculates normalized dot product similarity
func DotProductSimilarity(v1, v2 []float32) float32 {
	var dot float32
	for i := range v1 {
		dot += v1[i] * v2[i]
	}
	// Normalize to unit interval: (dot + 1) / 2
	return (dot + 1.0) / 2.0
}

// CosineSimilarity calculates normalized cosine similarity
func CosineSimilarity(v1, v2 []float32) float32 {
	var dot, norm1, norm2 float32
	for i := range v1 {
		dot += v1[i] * v2[i]
		norm1 += v1[i] * v1[i]
		norm2 += v2[i] * v2[i]
	}
	if norm1 == 0 || norm2 == 0 {
		return 0
	}
	// Normalize to unit interval: (cosine + 1) / 2
	return (dot/(sqrt(norm1)*sqrt(norm2)) + 1.0) / 2.0
}

// MaxInnerProductSimilarity calculates scaled maximum inner product
func MaxInnerProductSimilarity(v1, v2 []float32) float32 {
	var dot float32
	for i := range v1 {
		dot += v1[i] * v2[i]
	}
	// Scale to [0, 1] using sigmoid-like function
	if dot < 0 {
		return 1.0 / (1.0 - dot)
	}
	return dot + 1.0
}

// EuclideanSimilarityByte calculates Euclidean similarity for byte vectors
func EuclideanSimilarityByte(v1, v2 []byte) float32 {
	var sum int32
	for i := range v1 {
		diff := int32(v1[i]) - int32(v2[i])
		sum += diff * diff
	}
	return 1.0 / (1.0 + float32(sum))
}

// DotProductSimilarityByte calculates dot product similarity for byte vectors
func DotProductSimilarityByte(v1, v2 []byte) float32 {
	var dot int32
	for i := range v1 {
		dot += int32(v1[i]) * int32(v2[i])
	}
	// Scale and normalize
	maxDot := float32(127 * 127 * len(v1))
	return (float32(dot) + maxDot) / (2.0 * maxDot)
}

// CosineSimilarityByte calculates cosine similarity for byte vectors
func CosineSimilarityByte(v1, v2 []byte) float32 {
	var dot, norm1, norm2 int32
	for i := range v1 {
		dot += int32(v1[i]) * int32(v2[i])
		norm1 += int32(v1[i]) * int32(v1[i])
		norm2 += int32(v2[i]) * int32(v2[i])
	}
	if norm1 == 0 || norm2 == 0 {
		return 0.5 // Neutral score for zero vectors
	}
	cosine := float32(dot) / (sqrt(float32(norm1)) * sqrt(float32(norm2)))
	return (cosine + 1.0) / 2.0
}

// MaxInnerProductSimilarityByte calculates maximum inner product for byte vectors
func MaxInnerProductSimilarityByte(v1, v2 []byte) float32 {
	var dot int32
	for i := range v1 {
		dot += int32(v1[i]) * int32(v2[i])
	}
	// Scale to [0, 1]
	if dot < 0 {
		return 1.0 / (1.0 - float32(dot)/1000.0)
	}
	return float32(dot)/1000.0 + 1.0
}

// sqrt calculates square root
func sqrt(x float32) float32 {
	return float32(math.Sqrt(float64(x)))
}
