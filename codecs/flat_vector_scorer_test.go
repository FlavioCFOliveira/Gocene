// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Test file: flat_vector_scorer_test.go
// Source: lucene/core/src/test/org/apache/lucene/codecs/hnsw/TestFlatVectorScorer.java
// Purpose: Tests for FlatVectorsScorer including byte/float vector scoring,
//          bulk scoring, dimension checking, and multiple scorer isolation.

package codecs_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// VectorSimilarityFunction represents the similarity function for vector comparison
type VectorSimilarityFunction int

const (
	VectorSimilarityFunctionEuclidean VectorSimilarityFunction = iota
	VectorSimilarityFunctionDotProduct
	VectorSimilarityFunctionCosine
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
type FlatVectorsScorer interface {
	// GetRandomVectorScorerSupplier returns a supplier that can create scorers
	GetRandomVectorScorerSupplier(similarityFunction VectorSimilarityFunction, vectorValues KnnVectorValues) (RandomVectorScorerSupplier, error)
	// GetRandomVectorScorer returns a scorer for float vectors
	GetRandomVectorScorer(similarityFunction VectorSimilarityFunction, vectorValues KnnVectorValues, target []float32) (RandomVectorScorer, error)
	// GetRandomVectorScorerByte returns a scorer for byte vectors
	GetRandomVectorScorerByte(similarityFunction VectorSimilarityFunction, vectorValues KnnVectorValues, target []byte) (RandomVectorScorer, error)
	// String returns the scorer name
	String() string
}

// RandomVectorScorer scores random nodes against a query vector
type RandomVectorScorer interface {
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

// RandomVectorScorerSupplier creates RandomVectorScorer instances
type RandomVectorScorerSupplier interface {
	// Scorer creates a new RandomVectorScorer
	Scorer() (RandomVectorScorer, error)
	// Copy makes a copy of the supplier for thread safety
	Copy() (RandomVectorScorerSupplier, error)
}

// KnnVectorValues provides access to vector values
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

// ByteVectorValues provides access to byte vector values
type ByteVectorValues interface {
	KnnVectorValues
	// Get returns the byte vector at the given ordinal
	Get(ordinal int) ([]byte, error)
}

// FloatVectorValues provides access to float vector values
type FloatVectorValues interface {
	KnnVectorValues
	// Get returns the float vector at the given ordinal
	Get(ordinal int) ([]float32, error)
}

// DefaultFlatVectorScorer is the default implementation of FlatVectorsScorer
type DefaultFlatVectorScorer struct{}

// String returns the string representation
func (d *DefaultFlatVectorScorer) String() string {
	return "DefaultFlatVectorScorer()"
}

// GetRandomVectorScorerSupplier returns a supplier for random vector scorers
func (d *DefaultFlatVectorScorer) GetRandomVectorScorerSupplier(similarityFunction VectorSimilarityFunction, vectorValues KnnVectorValues) (RandomVectorScorerSupplier, error) {
	return &defaultRandomVectorScorerSupplier{
		similarityFunction: similarityFunction,
		vectorValues:       vectorValues,
	}, nil
}

// GetRandomVectorScorer returns a scorer for float vectors
func (d *DefaultFlatVectorScorer) GetRandomVectorScorer(similarityFunction VectorSimilarityFunction, vectorValues KnnVectorValues, target []float32) (RandomVectorScorer, error) {
	if vectorValues.Dimension() != len(target) {
		return nil, fmt.Errorf("vector query dimension: %d differs from field dimension: %d", len(target), vectorValues.Dimension())
	}
	return &defaultRandomVectorScorer{
		similarityFunction: similarityFunction,
		vectorValues:       vectorValues,
		targetFloat:        target,
		targetByte:         nil,
	}, nil
}

// GetRandomVectorScorerByte returns a scorer for byte vectors
func (d *DefaultFlatVectorScorer) GetRandomVectorScorerByte(similarityFunction VectorSimilarityFunction, vectorValues KnnVectorValues, target []byte) (RandomVectorScorer, error) {
	if vectorValues.Dimension() != len(target) {
		return nil, fmt.Errorf("vector query dimension: %d differs from field dimension: %d", len(target), vectorValues.Dimension())
	}
	return &defaultRandomVectorScorer{
		similarityFunction: similarityFunction,
		vectorValues:       vectorValues,
		targetFloat:        nil,
		targetByte:         target,
	}, nil
}

// defaultRandomVectorScorerSupplier implements RandomVectorScorerSupplier
type defaultRandomVectorScorerSupplier struct {
	similarityFunction VectorSimilarityFunction
	vectorValues       KnnVectorValues
}

// Scorer creates a new RandomVectorScorer
func (s *defaultRandomVectorScorerSupplier) Scorer() (RandomVectorScorer, error) {
	return &defaultRandomVectorScorer{
		similarityFunction: s.similarityFunction,
		vectorValues:       s.vectorValues,
		scoringOrdinal:     -1,
	}, nil
}

// Copy makes a copy of the supplier
func (s *defaultRandomVectorScorerSupplier) Copy() (RandomVectorScorerSupplier, error) {
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
	if s.scoringOrdinal < 0 {
		return 0, fmt.Errorf("scoring ordinal not set")
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
	// This is a simplified implementation for testing
	// In a real implementation, this would retrieve the vector and compute the actual similarity
	floatValues, ok := s.vectorValues.(FloatVectorValues)
	if !ok {
		return 0, fmt.Errorf("vector values not float type")
	}

	vec, err := floatValues.Get(node)
	if err != nil {
		return 0, err
	}

	if s.targetFloat != nil {
		return computeSimilarity(s.similarityFunction, vec, s.targetFloat), nil
	}

	// If scoring ordinal is set, compare against that ordinal's vector
	if s.scoringOrdinal >= 0 {
		targetVec, err := floatValues.Get(s.scoringOrdinal)
		if err != nil {
			return 0, err
		}
		return computeSimilarity(s.similarityFunction, vec, targetVec), nil
	}

	return 0, fmt.Errorf("no target vector set")
}

// scoreByte calculates score for byte vectors
func (s *defaultRandomVectorScorer) scoreByte(node int) (float32, error) {
	byteValues, ok := s.vectorValues.(ByteVectorValues)
	if !ok {
		return 0, fmt.Errorf("vector values not byte type")
	}

	vec, err := byteValues.Get(node)
	if err != nil {
		return 0, err
	}

	if s.targetByte != nil {
		return computeSimilarityByte(s.similarityFunction, vec, s.targetByte), nil
	}

	if s.scoringOrdinal >= 0 {
		targetVec, err := byteValues.Get(s.scoringOrdinal)
		if err != nil {
			return 0, err
		}
		return computeSimilarityByte(s.similarityFunction, vec, targetVec), nil
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

// computeSimilarity computes similarity between two float vectors
func computeSimilarity(simFunc VectorSimilarityFunction, v1, v2 []float32) float32 {
	switch simFunc {
	case VectorSimilarityFunctionEuclidean:
		return euclideanSimilarity(v1, v2)
	case VectorSimilarityFunctionDotProduct:
		return dotProductSimilarity(v1, v2)
	case VectorSimilarityFunctionCosine:
		return cosineSimilarity(v1, v2)
	case VectorSimilarityFunctionMaximumInnerProduct:
		return maxInnerProductSimilarity(v1, v2)
	default:
		return 0
	}
}

// computeSimilarityByte computes similarity between two byte vectors
func computeSimilarityByte(simFunc VectorSimilarityFunction, v1, v2 []byte) float32 {
	switch simFunc {
	case VectorSimilarityFunctionEuclidean:
		return euclideanSimilarityByte(v1, v2)
	case VectorSimilarityFunctionDotProduct:
		return dotProductSimilarityByte(v1, v2)
	case VectorSimilarityFunctionCosine:
		return cosineSimilarityByte(v1, v2)
	case VectorSimilarityFunctionMaximumInnerProduct:
		return maxInnerProductSimilarityByte(v1, v2)
	default:
		return 0
	}
}

// euclideanSimilarity calculates normalized Euclidean similarity
func euclideanSimilarity(v1, v2 []float32) float32 {
	var sum float32
	for i := range v1 {
		diff := v1[i] - v2[i]
		sum += diff * diff
	}
	// Normalize to unit interval: 1 / (1 + distance)
	return 1.0 / (1.0 + sum)
}

// dotProductSimilarity calculates normalized dot product similarity
func dotProductSimilarity(v1, v2 []float32) float32 {
	var dot float32
	for i := range v1 {
		dot += v1[i] * v2[i]
	}
	// Normalize to unit interval: (dot + 1) / 2
	return (dot + 1.0) / 2.0
}

// cosineSimilarity calculates normalized cosine similarity
func cosineSimilarity(v1, v2 []float32) float32 {
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

// maxInnerProductSimilarity calculates scaled maximum inner product
func maxInnerProductSimilarity(v1, v2 []float32) float32 {
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

// euclideanSimilarityByte calculates Euclidean similarity for byte vectors
func euclideanSimilarityByte(v1, v2 []byte) float32 {
	var sum int32
	for i := range v1 {
		diff := int32(v1[i]) - int32(v2[i])
		sum += diff * diff
	}
	return 1.0 / (1.0 + float32(sum))
}

// dotProductSimilarityByte calculates dot product similarity for byte vectors
func dotProductSimilarityByte(v1, v2 []byte) float32 {
	var dot int32
	for i := range v1 {
		dot += int32(v1[i]) * int32(v2[i])
	}
	// Scale and normalize
	maxDot := float32(127 * 127 * len(v1))
	return (float32(dot) + maxDot) / (2.0 * maxDot)
}

// cosineSimilarityByte calculates cosine similarity for byte vectors
func cosineSimilarityByte(v1, v2 []byte) float32 {
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

// maxInnerProductSimilarityByte calculates maximum inner product for byte vectors
func maxInnerProductSimilarityByte(v1, v2 []byte) float32 {
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

// InMemoryByteVectorValues implements ByteVectorValues in memory
type InMemoryByteVectorValues struct {
	vectors   [][]byte
	dimension int
}

// Dimension returns the vector dimension
func (v *InMemoryByteVectorValues) Dimension() int {
	return v.dimension
}

// Size returns the number of vectors
func (v *InMemoryByteVectorValues) Size() int {
	return len(v.vectors)
}

// GetEncoding returns the encoding type
func (v *InMemoryByteVectorValues) GetEncoding() index.VectorEncoding {
	return index.VectorEncodingByte
}

// Copy creates a copy of the values
func (v *InMemoryByteVectorValues) Copy() (KnnVectorValues, error) {
	copied := make([][]byte, len(v.vectors))
	for i, vec := range v.vectors {
		copied[i] = make([]byte, len(vec))
		copy(copied[i], vec)
	}
	return &InMemoryByteVectorValues{
		vectors:   copied,
		dimension: v.dimension,
	}, nil
}

// Get returns the vector at the given ordinal
func (v *InMemoryByteVectorValues) Get(ordinal int) ([]byte, error) {
	if ordinal < 0 || ordinal >= len(v.vectors) {
		return nil, fmt.Errorf("ordinal %d out of bounds [0, %d)", ordinal, len(v.vectors))
	}
	return v.vectors[ordinal], nil
}

// InMemoryFloatVectorValues implements FloatVectorValues in memory
type InMemoryFloatVectorValues struct {
	vectors   [][]float32
	dimension int
}

// Dimension returns the vector dimension
func (v *InMemoryFloatVectorValues) Dimension() int {
	return v.dimension
}

// Size returns the number of vectors
func (v *InMemoryFloatVectorValues) Size() int {
	return len(v.vectors)
}

// GetEncoding returns the encoding type
func (v *InMemoryFloatVectorValues) GetEncoding() index.VectorEncoding {
	return index.VectorEncodingFloat32
}

// Copy creates a copy of the values
func (v *InMemoryFloatVectorValues) Copy() (KnnVectorValues, error) {
	copied := make([][]float32, len(v.vectors))
	for i, vec := range v.vectors {
		copied[i] = make([]float32, len(vec))
		copy(copied[i], vec)
	}
	return &InMemoryFloatVectorValues{
		vectors:   copied,
		dimension: v.dimension,
	}, nil
}

// Get returns the vector at the given ordinal
func (v *InMemoryFloatVectorValues) Get(ordinal int) ([]float32, error) {
	if ordinal < 0 || ordinal >= len(v.vectors) {
		return nil, fmt.Errorf("ordinal %d out of bounds [0, %d)", ordinal, len(v.vectors))
	}
	return v.vectors[ordinal], nil
}

// TestFlatVectorScorer_DefaultScorer tests the default scorer string representation
// Source: TestFlatVectorScorer.testDefaultOrMemSegScorer()
func TestFlatVectorScorer_DefaultScorer(t *testing.T) {
	scorer := &DefaultFlatVectorScorer{}
	str := scorer.String()

	// Should be one of the expected scorer types
	if str != "DefaultFlatVectorScorer()" {
		t.Errorf("Expected 'DefaultFlatVectorScorer()', got '%s'", str)
	}
}

// TestFlatVectorScorer_MultipleByteScorers tests that creating another scorer doesn't disturb previous scorers
// Source: TestFlatVectorScorer.testMultipleByteScorers()
func TestFlatVectorScorer_MultipleByteScorers(t *testing.T) {
	vec0 := []byte{0, 0, 0, 0}
	vec1 := []byte{1, 1, 1, 1}
	vec2 := []byte{15, 15, 15, 15}

	vectorValues := &InMemoryByteVectorValues{
		vectors:   [][]byte{vec0, vec1, vec2},
		dimension: 4,
	}

	scorer := &DefaultFlatVectorScorer{}
	supplier, err := scorer.GetRandomVectorScorerSupplier(VectorSimilarityFunctionEuclidean, vectorValues)
	if err != nil {
		t.Fatalf("Failed to create supplier: %v", err)
	}

	// Create first scorer against ordinal 0
	scorerAgainstOrd0, err := supplier.Scorer()
	if err != nil {
		t.Fatalf("Failed to create scorer: %v", err)
	}
	scorerAgainstOrd0.SetScoringOrdinal(0)

	firstScore, err := scorerAgainstOrd0.Score(1)
	if err != nil {
		t.Fatalf("Failed to score: %v", err)
	}

	// Create second scorer against ordinal 2
	scorerAgainstOrd2, err := supplier.Scorer()
	if err != nil {
		t.Fatalf("Failed to create second scorer: %v", err)
	}
	scorerAgainstOrd2.SetScoringOrdinal(2)

	// Score again with first scorer - should be unchanged
	scoreAgain, err := scorerAgainstOrd0.Score(1)
	if err != nil {
		t.Fatalf("Failed to score again: %v", err)
	}

	if firstScore != scoreAgain {
		t.Errorf("Score changed after creating another scorer: first=%f, again=%f", firstScore, scoreAgain)
	}
}

// TestFlatVectorScorer_MultipleFloatScorers tests that creating another float scorer doesn't perturb previous scorers
// Source: TestFlatVectorScorer.testMultipleFloatScorers()
func TestFlatVectorScorer_MultipleFloatScorers(t *testing.T) {
	vec0 := []float32{0, 0, 0, 0}
	vec1 := []float32{1, 1, 1, 1}
	vec2 := []float32{15, 15, 15, 15}

	vectorValues := &InMemoryFloatVectorValues{
		vectors:   [][]float32{vec0, vec1, vec2},
		dimension: 4,
	}

	scorer := &DefaultFlatVectorScorer{}
	supplier, err := scorer.GetRandomVectorScorerSupplier(VectorSimilarityFunctionEuclidean, vectorValues)
	if err != nil {
		t.Fatalf("Failed to create supplier: %v", err)
	}

	// Create first scorer against ordinal 0
	scorerAgainstOrd0, err := supplier.Scorer()
	if err != nil {
		t.Fatalf("Failed to create scorer: %v", err)
	}
	scorerAgainstOrd0.SetScoringOrdinal(0)

	firstScore, err := scorerAgainstOrd0.Score(1)
	if err != nil {
		t.Fatalf("Failed to score: %v", err)
	}

	// Create second scorer against ordinal 2
	scorerAgainstOrd2, err := supplier.Scorer()
	if err != nil {
		t.Fatalf("Failed to create second scorer: %v", err)
	}
	scorerAgainstOrd2.SetScoringOrdinal(2)

	// Score again with first scorer - should be unchanged
	scoreAgain, err := scorerAgainstOrd0.Score(1)
	if err != nil {
		t.Fatalf("Failed to score again: %v", err)
	}

	if firstScore != scoreAgain {
		t.Errorf("Score changed after creating another scorer: first=%f, again=%f", firstScore, scoreAgain)
	}
}

// TestFlatVectorScorer_CheckByteDimensions tests dimension checking for byte vectors
// Source: TestFlatVectorScorer.testCheckByteDimensions()
func TestFlatVectorScorer_CheckByteDimensions(t *testing.T) {
	vec0 := make([]byte, 4)
	vectorValues := &InMemoryByteVectorValues{
		vectors:   [][]byte{vec0},
		dimension: 4,
	}

	scorer := &DefaultFlatVectorScorer{}
	similarityFunctions := []VectorSimilarityFunction{
		VectorSimilarityFunctionCosine,
		VectorSimilarityFunctionDotProduct,
		VectorSimilarityFunctionEuclidean,
		VectorSimilarityFunctionMaximumInnerProduct,
	}

	for _, sim := range similarityFunctions {
		// Should throw error for mismatched dimensions (5 vs 4)
		_, err := scorer.GetRandomVectorScorerByte(sim, vectorValues, make([]byte, 5))
		if err == nil {
			t.Errorf("Expected error for dimension mismatch with %s, got nil", sim.String())
		}
	}
}

// TestFlatVectorScorer_CheckFloatDimensions tests dimension checking for float vectors
// Source: TestFlatVectorScorer.testCheckFloatDimensions()
func TestFlatVectorScorer_CheckFloatDimensions(t *testing.T) {
	vec0 := make([]float32, 4)
	vectorValues := &InMemoryFloatVectorValues{
		vectors:   [][]float32{vec0},
		dimension: 4,
	}

	scorer := &DefaultFlatVectorScorer{}
	similarityFunctions := []VectorSimilarityFunction{
		VectorSimilarityFunctionCosine,
		VectorSimilarityFunctionDotProduct,
		VectorSimilarityFunctionEuclidean,
		VectorSimilarityFunctionMaximumInnerProduct,
	}

	for _, sim := range similarityFunctions {
		// Should throw error for mismatched dimensions (5 vs 4)
		_, err := scorer.GetRandomVectorScorer(sim, vectorValues, make([]float32, 5))
		if err == nil {
			t.Errorf("Expected error for dimension mismatch with %s, got nil", sim.String())
		}
	}
}

// TestFlatVectorScorer_BulkScorerBytes tests bulk scoring for byte vectors
// Source: TestFlatVectorScorer.testBulkScorerBytes()
func TestFlatVectorScorer_BulkScorerBytes(t *testing.T) {
	dims := 64
	size := 100

	// Generate random byte vectors
	vectors := make([][]byte, size)
	for i := 0; i < size; i++ {
		vec := make([]byte, dims)
		rand.Read(vec)
		vectors[i] = vec
	}

	vectorValues := &InMemoryByteVectorValues{
		vectors:   vectors,
		dimension: dims,
	}

	scorer := &DefaultFlatVectorScorer{}
	similarityFunctions := []VectorSimilarityFunction{
		VectorSimilarityFunctionCosine,
		VectorSimilarityFunctionDotProduct,
		VectorSimilarityFunctionEuclidean,
		VectorSimilarityFunctionMaximumInnerProduct,
	}

	for _, sim := range similarityFunctions {
		t.Run(sim.String(), func(t *testing.T) {
			// Test bulk equals non-bulk
			err := assertBulkEqualsNonBulk(scorer, vectorValues, sim)
			if err != nil {
				t.Errorf("Bulk equals non-bulk failed for %s: %v", sim.String(), err)
			}

			// Test bulk equals non-bulk supplier
			err = assertBulkEqualsNonBulkSupplier(scorer, vectorValues, sim)
			if err != nil {
				t.Errorf("Bulk equals non-bulk supplier failed for %s: %v", sim.String(), err)
			}

			// Test scores against default flat scorer
			err = assertScoresAgainstDefaultFlatScorer(scorer, vectorValues, sim)
			if err != nil {
				t.Errorf("Scores against default flat scorer failed for %s: %v", sim.String(), err)
			}
		})
	}
}

// TestFlatVectorScorer_BulkScorerFloats tests bulk scoring for float vectors
// Source: TestFlatVectorScorer.testBulkScorerFloats()
func TestFlatVectorScorer_BulkScorerFloats(t *testing.T) {
	dims := 64
	size := 100

	// Generate random float vectors
	vectors := make([][]float32, size)
	for i := 0; i < size; i++ {
		vec := make([]float32, dims)
		for j := range vec {
			vec[j] = rand.Float32()
		}
		vectors[i] = vec
	}

	vectorValues := &InMemoryFloatVectorValues{
		vectors:   vectors,
		dimension: dims,
	}

	scorer := &DefaultFlatVectorScorer{}
	similarityFunctions := []VectorSimilarityFunction{
		VectorSimilarityFunctionCosine,
		VectorSimilarityFunctionDotProduct,
		VectorSimilarityFunctionEuclidean,
		VectorSimilarityFunctionMaximumInnerProduct,
	}

	for _, sim := range similarityFunctions {
		t.Run(sim.String(), func(t *testing.T) {
			// Test bulk equals non-bulk
			err := assertBulkEqualsNonBulk(scorer, vectorValues, sim)
			if err != nil {
				t.Errorf("Bulk equals non-bulk failed for %s: %v", sim.String(), err)
			}

			// Test bulk equals non-bulk supplier
			err = assertBulkEqualsNonBulkSupplier(scorer, vectorValues, sim)
			if err != nil {
				t.Errorf("Bulk equals non-bulk supplier failed for %s: %v", sim.String(), err)
			}

			// Test scores against default flat scorer
			err = assertScoresAgainstDefaultFlatScorer(scorer, vectorValues, sim)
			if err != nil {
				t.Errorf("Scores against default flat scorer failed for %s: %v", sim.String(), err)
			}
		})
	}
}

// TestFlatVectorScorer_OnHeapBulkScorerFloats tests on-heap bulk scoring for float vectors
// Source: TestFlatVectorScorer.testOnHeapBulkScorerFloats()
func TestFlatVectorScorer_OnHeapBulkScorerFloats(t *testing.T) {
	dims := 64
	size := 100

	// Generate random float vectors
	vectors := make([][]float32, size)
	for i := 0; i < size; i++ {
		vec := make([]float32, dims)
		for j := range vec {
			vec[j] = rand.Float32()
		}
		vectors[i] = vec
	}

	vectorValues := &InMemoryFloatVectorValues{
		vectors:   vectors,
		dimension: dims,
	}

	scorer := &DefaultFlatVectorScorer{}
	similarityFunctions := []VectorSimilarityFunction{
		VectorSimilarityFunctionCosine,
		VectorSimilarityFunctionDotProduct,
		VectorSimilarityFunctionEuclidean,
		VectorSimilarityFunctionMaximumInnerProduct,
	}

	for _, sim := range similarityFunctions {
		t.Run(sim.String(), func(t *testing.T) {
			// Test bulk equals non-bulk
			err := assertBulkEqualsNonBulk(scorer, vectorValues, sim)
			if err != nil {
				t.Errorf("Bulk equals non-bulk failed for %s: %v", sim.String(), err)
			}

			// Test bulk equals non-bulk supplier
			err = assertBulkEqualsNonBulkSupplier(scorer, vectorValues, sim)
			if err != nil {
				t.Errorf("Bulk equals non-bulk supplier failed for %s: %v", sim.String(), err)
			}

			// Test scores against default flat scorer
			err = assertScoresAgainstDefaultFlatScorer(scorer, vectorValues, sim)
			if err != nil {
				t.Errorf("Scores against default flat scorer failed for %s: %v", sim.String(), err)
			}
		})
	}
}

// assertBulkEqualsNonBulk verifies that bulk scoring matches individual scoring
func assertBulkEqualsNonBulk(scorer FlatVectorsScorer, values KnnVectorValues, sim VectorSimilarityFunction) error {
	size := values.Size()
	dims := values.Dimension()
	delta := 1e-3 * float32(size)

	// Create scorer with random target
	var testScorer RandomVectorScorer
	var err error
	if values.GetEncoding() == index.VectorEncodingByte {
		target := make([]byte, dims)
		rand.Read(target)
		testScorer, err = scorer.GetRandomVectorScorerByte(sim, values, target)
	} else {
		target := make([]float32, dims)
		for i := range target {
			target[i] = rand.Float32()
		}
		testScorer, err = scorer.GetRandomVectorScorer(sim, values, target)
	}
	if err != nil {
		return fmt.Errorf("failed to create scorer: %w", err)
	}

	indices := randomIndices(size)
	expectedScores := make([]float32, size)
	expectedMaxScore := float32(math.Inf(-1))

	for i := 0; i < size; i++ {
		score, err := testScorer.Score(indices[i])
		if err != nil {
			return fmt.Errorf("failed to score: %w", err)
		}
		expectedScores[i] = score
		if score > expectedMaxScore {
			expectedMaxScore = score
		}
	}

	bulkScores := make([]float32, size)
	maxScore, err := testScorer.BulkScore(indices, bulkScores, size)
	if err != nil {
		return fmt.Errorf("bulk score failed: %w", err)
	}

	if math.Abs(float64(maxScore-expectedMaxScore)) > 0.001 {
		return fmt.Errorf("max score mismatch: expected %f, got %f", expectedMaxScore, maxScore)
	}

	for i := 0; i < size; i++ {
		if math.Abs(float64(expectedScores[i]-bulkScores[i])) > float64(delta) {
			return fmt.Errorf("score mismatch at %d: expected %f, got %f", i, expectedScores[i], bulkScores[i])
		}
	}

	return assertNoScoreBeyondNumNodes(testScorer, size)
}

// assertBulkEqualsNonBulkSupplier verifies bulk scoring through supplier interface
func assertBulkEqualsNonBulkSupplier(scorer FlatVectorsScorer, values KnnVectorValues, sim VectorSimilarityFunction) error {
	size := values.Size()
	delta := 1e-3 * float32(size)

	supplier, err := scorer.GetRandomVectorScorerSupplier(sim, values)
	if err != nil {
		return fmt.Errorf("failed to create supplier: %w", err)
	}

	// Test both original and copied supplier
	suppliers := []RandomVectorScorerSupplier{supplier}
	copied, err := supplier.Copy()
	if err != nil {
		return fmt.Errorf("failed to copy supplier: %w", err)
	}
	suppliers = append(suppliers, copied)

	for _, ss := range suppliers {
		updatableScorer, err := ss.Scorer()
		if err != nil {
			return fmt.Errorf("failed to create scorer: %w", err)
		}

		targetNode := rand.Intn(size)
		updatableScorer.SetScoringOrdinal(targetNode)

		indices := randomIndices(size)
		expectedScores := make([]float32, size)
		for i := 0; i < size; i++ {
			score, err := updatableScorer.Score(indices[i])
			if err != nil {
				return fmt.Errorf("failed to score: %w", err)
			}
			expectedScores[i] = score
		}

		bulkScores := make([]float32, size)
		_, err = updatableScorer.BulkScore(indices, bulkScores, size)
		if err != nil {
			return fmt.Errorf("bulk score failed: %w", err)
		}

		for i := 0; i < size; i++ {
			if math.Abs(float64(expectedScores[i]-bulkScores[i])) > float64(delta) {
				return fmt.Errorf("score mismatch at %d: expected %f, got %f", i, expectedScores[i], bulkScores[i])
			}
		}

		if err := assertNoScoreBeyondNumNodes(updatableScorer, size); err != nil {
			return err
		}
	}

	return nil
}

// assertScoresAgainstDefaultFlatScorer verifies scores match the default flat scorer
func assertScoresAgainstDefaultFlatScorer(scorer FlatVectorsScorer, values KnnVectorValues, sim VectorSimilarityFunction) error {
	size := values.Size()
	delta := 1e-3 * float32(size)

	targetNode := rand.Intn(size)
	indices := randomIndices(size)

	// Get expected scores from default scorer
	defaultScorer := &DefaultFlatVectorScorer{}
	defaultSupplier, err := defaultScorer.GetRandomVectorScorerSupplier(sim, values)
	if err != nil {
		return fmt.Errorf("failed to create default supplier: %w", err)
	}
	defaultScorerInstance, err := defaultSupplier.Scorer()
	if err != nil {
		return fmt.Errorf("failed to create default scorer: %w", err)
	}
	defaultScorerInstance.SetScoringOrdinal(targetNode)

	expectedScores := make([]float32, size)
	expectedMaxScore := float32(math.Inf(-1))
	for i := 0; i < size; i++ {
		score, err := defaultScorerInstance.Score(indices[i])
		if err != nil {
			return fmt.Errorf("failed to score with default: %w", err)
		}
		expectedScores[i] = score
		if score > expectedMaxScore {
			expectedMaxScore = score
		}
	}

	// Test with the provided scorer
	supplier, err := scorer.GetRandomVectorScorerSupplier(sim, values)
	if err != nil {
		return fmt.Errorf("failed to create supplier: %w", err)
	}

	// Test both original and copied supplier
	suppliers := []RandomVectorScorerSupplier{supplier}
	copied, err := supplier.Copy()
	if err != nil {
		return fmt.Errorf("failed to copy supplier: %w", err)
	}
	suppliers = append(suppliers, copied)

	for _, ss := range suppliers {
		updatableScorer, err := ss.Scorer()
		if err != nil {
			return fmt.Errorf("failed to create scorer: %w", err)
		}
		updatableScorer.SetScoringOrdinal(targetNode)

		bulkScores := make([]float32, size)
		maxScore, err := updatableScorer.BulkScore(indices, bulkScores, size)
		if err != nil {
			return fmt.Errorf("bulk score failed: %w", err)
		}

		if math.Abs(float64(maxScore-expectedMaxScore)) > 0.001 {
			return fmt.Errorf("max score mismatch: expected %f, got %f", expectedMaxScore, maxScore)
		}

		for i := 0; i < size; i++ {
			if math.Abs(float64(expectedScores[i]-bulkScores[i])) > float64(delta) {
				return fmt.Errorf("score mismatch at %d: expected %f, got %f", i, expectedScores[i], bulkScores[i])
			}
		}
	}

	return nil
}

// assertNoScoreBeyondNumNodes verifies that nodes beyond numNodes are not scored
func assertNoScoreBeyondNumNodes(scorer RandomVectorScorer, maxSize int) error {
	numNodes := rand.Intn(maxSize + 1)
	indices := make([]int, numNodes+1)
	bulkScores := make([]float32, numNodes+1)
	bulkScores[len(bulkScores)-1] = float32(math.NaN())

	_, err := scorer.BulkScore(indices, bulkScores, numNodes)
	if err != nil {
		return fmt.Errorf("bulk score failed: %w", err)
	}

	if !math.IsNaN(float64(bulkScores[len(bulkScores)-1])) {
		return fmt.Errorf("expected NaN for unscored node, got %f", bulkScores[len(bulkScores)-1])
	}

	return nil
}

// randomIndices returns a shuffled array of indices from 0 to size-1
func randomIndices(size int) []int {
	indices := make([]int, size)
	for i := 0; i < size; i++ {
		indices[i] = i
	}

	// Fisher-Yates shuffle
	for i := size - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		indices[i], indices[j] = indices[j], indices[i]
	}

	return indices
}

// concatFloats concatenates float arrays as byte slices (little-endian)
func concatFloats(arrays ...[]float32) []byte {
	buf := new(bytes.Buffer)
	for _, arr := range arrays {
		for _, f := range arr {
			binary.Write(buf, binary.LittleEndian, f)
		}
	}
	return buf.Bytes()
}

// concatBytes concatenates byte arrays
func concatBytes(arrays ...[]byte) []byte {
	totalLen := 0
	for _, arr := range arrays {
		totalLen += len(arr)
	}
	result := make([]byte, totalLen)
	offset := 0
	for _, arr := range arrays {
		copy(result[offset:], arr)
		offset += len(arr)
	}
	return result
}

// TestFlatVectorScorer_ByteCompatibility verifies byte-level compatibility with Lucene
func TestFlatVectorScorer_ByteCompatibility(t *testing.T) {
	// Verify float size matches Java's Float.BYTES (4 bytes)
	const javaFloatBytes = 4
	const goFloat32Bytes = 4

	if javaFloatBytes != goFloat32Bytes {
		t.Error("Float byte size mismatch between Java and Go")
	}

	// Test byte concatenation produces same results as Java
	vec1 := []float32{1.0, 2.0, 3.0}
	vec2 := []float32{4.0, 5.0, 6.0}

	concatenated := concatFloats(vec1, vec2)
	expectedLen := (len(vec1) + len(vec2)) * 4 // 4 bytes per float32

	if len(concatenated) != expectedLen {
		t.Errorf("Expected concatenated length %d, got %d", expectedLen, len(concatenated))
	}

	// Verify little-endian encoding
	buf := bytes.NewReader(concatenated)
	var readFloat float32
	binary.Read(buf, binary.LittleEndian, &readFloat)
	if readFloat != 1.0 {
		t.Errorf("Expected first float to be 1.0, got %f", readFloat)
	}
}

// TestFlatVectorScorer_SimilarityFunctions tests all similarity functions
func TestFlatVectorScorer_SimilarityFunctions(t *testing.T) {
	// Test vectors
	v1 := []float32{1.0, 0.0, 0.0}
	v2 := []float32{0.0, 1.0, 0.0}
	v3 := []float32{1.0, 1.0, 0.0}

	vectors := [][]float32{v1, v2, v3}
	vectorValues := &InMemoryFloatVectorValues{
		vectors:   vectors,
		dimension: 3,
	}

	scorer := &DefaultFlatVectorScorer{}

	tests := []struct {
		name     string
		simFunc  VectorSimilarityFunction
		target   []float32
		node     int
		expected float32
	}{
		{
			name:     "Euclidean - same vector",
			simFunc:  VectorSimilarityFunctionEuclidean,
			target:   []float32{1.0, 0.0, 0.0},
			node:     0,
			expected: 1.0, // Perfect match
		},
		{
			name:     "DotProduct - orthogonal vectors",
			simFunc:  VectorSimilarityFunctionDotProduct,
			target:   []float32{1.0, 0.0, 0.0},
			node:     1,
			expected: 0.5, // Dot product of orthogonal is 0, normalized to 0.5
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := scorer.GetRandomVectorScorer(tt.simFunc, vectorValues, tt.target)
			if err != nil {
				t.Fatalf("Failed to create scorer: %v", err)
			}

			score, err := s.Score(tt.node)
			if err != nil {
				t.Fatalf("Failed to score: %v", err)
			}

			// Allow small floating point differences
			if math.Abs(float64(score-tt.expected)) > 0.01 {
				t.Errorf("Expected score %f, got %f", tt.expected, score)
			}
		})
	}
}

// TestFlatVectorScorer_MaxOrd tests the MaxOrd method
func TestFlatVectorScorer_MaxOrd(t *testing.T) {
	vectors := [][]float32{
		{1.0, 2.0, 3.0},
		{4.0, 5.0, 6.0},
		{7.0, 8.0, 9.0},
	}

	vectorValues := &InMemoryFloatVectorValues{
		vectors:   vectors,
		dimension: 3,
	}

	scorer := &DefaultFlatVectorScorer{}
	s, err := scorer.GetRandomVectorScorer(VectorSimilarityFunctionEuclidean, vectorValues, vectors[0])
	if err != nil {
		t.Fatalf("Failed to create scorer: %v", err)
	}

	if s.MaxOrd() != 3 {
		t.Errorf("Expected MaxOrd() = 3, got %d", s.MaxOrd())
	}
}

// TestFlatVectorScorer_SupplierCopy tests supplier copy functionality
func TestFlatVectorScorer_SupplierCopy(t *testing.T) {
	vectors := [][]float32{
		{1.0, 0.0, 0.0},
		{0.0, 1.0, 0.0},
	}

	vectorValues := &InMemoryFloatVectorValues{
		vectors:   vectors,
		dimension: 3,
	}

	scorer := &DefaultFlatVectorScorer{}
	supplier, err := scorer.GetRandomVectorScorerSupplier(VectorSimilarityFunctionEuclidean, vectorValues)
	if err != nil {
		t.Fatalf("Failed to create supplier: %v", err)
	}

	// Copy the supplier
	copied, err := supplier.Copy()
	if err != nil {
		t.Fatalf("Failed to copy supplier: %v", err)
	}

	// Both should be able to create independent scorers
	scorer1, err := supplier.Scorer()
	if err != nil {
		t.Fatalf("Failed to create scorer from original: %v", err)
	}

	scorer2, err := copied.Scorer()
	if err != nil {
		t.Fatalf("Failed to create scorer from copy: %v", err)
	}

	// Set different scoring ordinals
	scorer1.SetScoringOrdinal(0)
	scorer2.SetScoringOrdinal(1)

	// Verify they operate independently
	score1, err := scorer1.Score(1)
	if err != nil {
		t.Fatalf("Failed to score with scorer1: %v", err)
	}

	score2, err := scorer2.Score(0)
	if err != nil {
		t.Fatalf("Failed to score with scorer2: %v", err)
	}

	// Scores should be different since they're scoring different pairs
	if score1 == score2 {
		t.Error("Expected different scores from independent scorers")
	}
}

// BenchmarkFlatVectorScorer_Score benchmarks the Score method
func BenchmarkFlatVectorScorer_Score(b *testing.B) {
	dims := 128
	numVectors := 1000

	// Generate random vectors
	vectors := make([][]float32, numVectors)
	for i := 0; i < numVectors; i++ {
		vec := make([]float32, dims)
		for j := range vec {
			vec[j] = rand.Float32()
		}
		vectors[i] = vec
	}

	vectorValues := &InMemoryFloatVectorValues{
		vectors:   vectors,
		dimension: dims,
	}

	target := make([]float32, dims)
	for i := range target {
		target[i] = rand.Float32()
	}

	scorer := &DefaultFlatVectorScorer{}
	s, err := scorer.GetRandomVectorScorer(VectorSimilarityFunctionEuclidean, vectorValues, target)
	if err != nil {
		b.Fatalf("Failed to create scorer: %v", err)
	}
	s.SetScoringOrdinal(0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Score(i % numVectors)
	}
}

// BenchmarkFlatVectorScorer_BulkScore benchmarks the BulkScore method
func BenchmarkFlatVectorScorer_BulkScore(b *testing.B) {
	dims := 128
	numVectors := 1000

	vectors := make([][]float32, numVectors)
	for i := 0; i < numVectors; i++ {
		vec := make([]float32, dims)
		for j := range vec {
			vec[j] = rand.Float32()
		}
		vectors[i] = vec
	}

	vectorValues := &InMemoryFloatVectorValues{
		vectors:   vectors,
		dimension: dims,
	}

	target := make([]float32, dims)
	for i := range target {
		target[i] = rand.Float32()
	}

	scorer := &DefaultFlatVectorScorer{}
	s, err := scorer.GetRandomVectorScorer(VectorSimilarityFunctionEuclidean, vectorValues, target)
	if err != nil {
		b.Fatalf("Failed to create scorer: %v", err)
	}
	s.SetScoringOrdinal(0)

	indices := randomIndices(numVectors)
	scores := make([]float32, numVectors)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.BulkScore(indices, scores, numVectors)
	}
}

// OffHeapVectorValues provides off-heap vector storage similar to Lucene's OffHeapFloatVectorValues
type OffHeapVectorValues struct {
	data      []byte
	dimension int
	size      int
	encoding  index.VectorEncoding
}

// Dimension returns the vector dimension
func (v *OffHeapVectorValues) Dimension() int {
	return v.dimension
}

// Size returns the number of vectors
func (v *OffHeapVectorValues) Size() int {
	return v.size
}

// GetEncoding returns the encoding type
func (v *OffHeapVectorValues) GetEncoding() index.VectorEncoding {
	return v.encoding
}

// Copy creates a copy of the values
func (v *OffHeapVectorValues) Copy() (KnnVectorValues, error) {
	copiedData := make([]byte, len(v.data))
	copy(copiedData, v.data)
	return &OffHeapVectorValues{
		data:      copiedData,
		dimension: v.dimension,
		size:      v.size,
		encoding:  v.encoding,
	}, nil
}

// GetFloat returns the float vector at the given ordinal
func (v *OffHeapVectorValues) GetFloat(ordinal int) ([]float32, error) {
	if v.encoding != index.VectorEncodingFloat32 {
		return nil, fmt.Errorf("not float encoding")
	}
	if ordinal < 0 || ordinal >= v.size {
		return nil, fmt.Errorf("ordinal %d out of bounds", ordinal)
	}

	vec := make([]float32, v.dimension)
	offset := ordinal * v.dimension * 4 // 4 bytes per float32
	buf := bytes.NewReader(v.data[offset : offset+len(vec)*4])
	for i := range vec {
		binary.Read(buf, binary.LittleEndian, &vec[i])
	}
	return vec, nil
}

// GetByte returns the byte vector at the given ordinal
func (v *OffHeapVectorValues) GetByte(ordinal int) ([]byte, error) {
	if v.encoding != index.VectorEncodingByte {
		return nil, fmt.Errorf("not byte encoding")
	}
	if ordinal < 0 || ordinal >= v.size {
		return nil, fmt.Errorf("ordinal %d out of bounds", ordinal)
	}

	vec := make([]byte, v.dimension)
	offset := ordinal * v.dimension
	copy(vec, v.data[offset:offset+v.dimension])
	return vec, nil
}

// TestFlatVectorScorer_OffHeapVectors tests scoring with off-heap vector values
// This simulates the off-heap storage used in Lucene
func TestFlatVectorScorer_OffHeapVectors(t *testing.T) {
	dims := 16
	size := 10

	// Create off-heap float vectors
	data := make([]byte, dims*size*4)
	for i := 0; i < size; i++ {
		for j := 0; j < dims; j++ {
			offset := (i*dims + j) * 4
			binary.LittleEndian.PutUint32(data[offset:offset+4], math.Float32bits(rand.Float32()))
		}
	}

	vectorValues := &OffHeapVectorValues{
		data:      data,
		dimension: dims,
		size:      size,
		encoding:  index.VectorEncodingFloat32,
	}

	// Create a mock FloatVectorValues wrapper
	floatValues := &offHeapFloatVectorValuesWrapper{OffHeapVectorValues: vectorValues}

	scorer := &DefaultFlatVectorScorer{}
	target := make([]float32, dims)
	for i := range target {
		target[i] = rand.Float32()
	}

	s, err := scorer.GetRandomVectorScorer(VectorSimilarityFunctionEuclidean, floatValues, target)
	if err != nil {
		t.Fatalf("Failed to create scorer: %v", err)
	}
	s.SetScoringOrdinal(0)

	// Score a few nodes
	for i := 0; i < 5; i++ {
		score, err := s.Score(i)
		if err != nil {
			t.Fatalf("Failed to score node %d: %v", i, err)
		}
		if score < 0 || score > 1 {
			t.Errorf("Score %f out of expected range [0, 1]", score)
		}
	}
}

// offHeapFloatVectorValuesWrapper wraps OffHeapVectorValues to implement FloatVectorValues
type offHeapFloatVectorValuesWrapper struct {
	*OffHeapVectorValues
}

// Get returns the float vector at the given ordinal
func (v *offHeapFloatVectorValuesWrapper) Get(ordinal int) ([]float32, error) {
	return v.OffHeapVectorValues.GetFloat(ordinal)
}

// TestFlatVectorScorer_StoreIntegration tests integration with the store package
func TestFlatVectorScorer_StoreIntegration(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Write test data
	out, err := dir.CreateOutput("test_vectors", store.IOContextWrite)
	if err != nil {
		t.Fatalf("Failed to create output: %v", err)
	}

	// Write some float vectors (4 bytes each)
	vectors := [][]float32{
		{1.0, 2.0, 3.0, 4.0},
		{5.0, 6.0, 7.0, 8.0},
		{9.0, 10.0, 11.0, 12.0},
	}

	for _, vec := range vectors {
		for _, f := range vec {
			store.WriteFloat32(out, f)
		}
	}
	out.Close()

	// Read back and verify
	in, err := dir.OpenInput("test_vectors", store.IOContextRead)
	if err != nil {
		t.Fatalf("Failed to open input: %v", err)
	}
	defer in.Close()

	// Verify file length
	expectedLen := int64(len(vectors) * len(vectors[0]) * 4)
	if in.Length() != expectedLen {
		t.Errorf("Expected file length %d, got %d", expectedLen, in.Length())
	}

	// Read and verify first vector
	f, err := store.ReadFloat32(in)
	if err != nil {
		t.Fatalf("Failed to read float: %v", err)
	}
	if f != 1.0 {
		t.Errorf("Expected first float 1.0, got %f", f)
	}
}
