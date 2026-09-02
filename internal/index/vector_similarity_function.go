package index

// VectorSimilarityFunction describes the method used during indexing and searching of vectors.
type VectorSimilarityFunction int

const (
	VectorSimilarityFunctionEuclidean VectorSimilarityFunction = iota
	VectorSimilarityFunctionDotProduct
	VectorSimilarityFunctionCosine
	VectorSimilarityFunctionMaximumInnerProduct
)
