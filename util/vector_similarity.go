package util

// VectorSimilarityFunction defines the method used to determine the nearest neighbors.
type VectorSimilarityFunction interface {
	// CompareFloat calculates a similarity score between the two vectors. Higher scores correspond to closer vectors.
	CompareFloat(v1, v2 []float32) float32
	// CompareBytes calculates a similarity score between the two vectors.
	CompareBytes(v1, v2 []byte) float32
}

type euclideanSimilarity struct{}

func (s euclideanSimilarity) CompareFloat(v1, v2 []float32) float32 {
	return NormalizeDistanceToUnitInterval(SquareDistance(v1, v2))
}

func (s euclideanSimilarity) CompareBytes(v1, v2 []byte) float32 {
	return 1 / (1 + float32(SquareDistanceBytes(v1, v2)))
}

type dotProductSimilarity struct{}

func (s dotProductSimilarity) CompareFloat(v1, v2 []float32) float32 {
	return NormalizeToUnitInterval(ComputeDotProduct(v1, v2))
}

func (s dotProductSimilarity) CompareBytes(v1, v2 []byte) float32 {
	return DotProductScore(v1, v2)
}

type cosineSimilarity struct{}

func (s cosineSimilarity) CompareFloat(v1, v2 []float32) float32 {
	return NormalizeToUnitInterval(Cosine(v1, v2))
}

func (s cosineSimilarity) CompareBytes(v1, v2 []byte) float32 {
	return (1 + CosineBytes(v1, v2)) / 2
}

type maximumInnerProductSimilarity struct{}

func (s maximumInnerProductSimilarity) CompareFloat(v1, v2 []float32) float32 {
	return ScaleMaxInnerProductScore(ComputeDotProduct(v1, v2))
}

func (s maximumInnerProductSimilarity) CompareBytes(v1, v2 []byte) float32 {
	return ScaleMaxInnerProductScore(float32(DotProductBytes(v1, v2)))
}

var (
	EuclideanSim           VectorSimilarityFunction = euclideanSimilarity{}
	DotProductSim           VectorSimilarityFunction = dotProductSimilarity{}
	CosineSim              VectorSimilarityFunction = cosineSimilarity{}
	MaximumInnerProductSim VectorSimilarityFunction = maximumInnerProductSimilarity{}
)
