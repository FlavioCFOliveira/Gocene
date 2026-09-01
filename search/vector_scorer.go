package search

import (
	"io"
)

// VectorScorer computes the similarity score between a given query vector and different document vectors.
type VectorScorer interface {
	// Score computes the score for the current document ID.
	Score() (float32, error)

	// Iterator returns a DocIdSetIterator over the documents.
	Iterator() DocIdSetIterator
}

// Bulk is a bulk scorer interface to score multiple vectors at once.
type Bulk interface {
	// NextDocsAndScores score docs ids iterating to upTo documents, store the results in the provided buffer.
	NextDocsAndScores(upTo int, liveDocs interface{ Get(int) bool }, buffer *DocAndFloatFeatureBuffer) (float32, error)
}

// DocAndFloatFeatureBuffer stores the results of bulk scoring.
type DocAndFloatFeatureBuffer struct {
	Docs     []int
	Features []float32
	Size     int
}

func (b *DocAndFloatFeatureBuffer) GrowNoCopy(minSize int) {
	if len(b.Docs) < minSize {
		newDocs := make([]int, minSize)
		copy(newDocs, b.Docs)
		b.Docs = newDocs
	}
	if len(b.Features) < minSize {
		newFeatures := make([]float32, minSize)
		copy(newFeatures, b.Features)
		b.Features = newFeatures
	}
}
