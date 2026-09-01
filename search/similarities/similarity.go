package similarities

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Similarity defines the components of Lucene scoring.
type Similarity interface {
	// ComputeNorm computes the normalization value for a field at index-time.
	ComputeNorm(state *index.FieldInvertState) int64

	// Scorer computes any collection-level weight needed for scoring a query.
	Scorer(boost float32, collectionStats CollectionStatistics, termStats ...TermStatistics) SimScorer
	
	// GetDiscountOverlaps returns true if overlap tokens are discounted from the document's length.
	GetDiscountOverlaps() bool
}

// SimScorer is the internal scoring engine for a Similarity.
type SimScorer interface {
	// Score computes the score for a single document.
	Score(freq float32, norm int64) float32

	// AsBulkSimScorer returns a BulkSimScorer for efficient bulk-computation of scores.
	AsBulkSimScorer() BulkSimScorer

	// Explain provides an explanation of the score.
	Explain(freq search.Explanation, norm int64) search.Explanation
}

// BulkSimScorer is a specialization of SimScorer for bulk-computation.
type BulkSimScorer interface {
	// Score bulk-computes scores.
	Score(size int, freqs []float32, norms []int64, scores []float32)
}

// BaseSimilarity provides a default implementation of Similarity.
type BaseSimilarity struct {
	discountOverlaps bool
}

func NewBaseSimilarity(discountOverlaps bool) *BaseSimilarity {
	return &BaseSimilarity{discountOverlaps: discountOverlaps}
}

func (s *BaseSimilarity) GetDiscountOverlaps() bool {
	return s.discountOverlaps
}

func (s *BaseSimilarity) ComputeNorm(state *index.FieldInvertState) int64 {
	var numTerms int
	if state.IndexOptions == index.Docs {
		numTerms = state.GetUniqueTermCount()
	} else if s.discountOverlaps {
		numTerms = state.GetLength() - state.GetNumOverlap()
	} else {
		numTerms = state.GetLength()
	}
	return util.IntToByte4(numTerms)
}

// DefaultBulkSimScorer is the default implementation of BulkSimScorer.
type DefaultBulkSimScorer struct {
	scorer SimScorer
}

func NewDefaultBulkSimScorer(scorer SimScorer) *DefaultBulkSimScorer {
	return &DefaultBulkSimScorer{scorer: scorer}
}

func (b *DefaultBulkSimScorer) Score(size int, freqs []float32, norms []int64, scores []float32) {
	for i := 0; i < size; i++ {
		scores[i] = b.scorer.Score(freqs[i], norms[i])
	}
}

// CollectionStatistics represents collection-level statistics.
type CollectionStatistics interface {
	SumTotalTermFreq() int64
	DocCount() int
}

// TermStatistics represents term-level statistics.
type TermStatistics interface {
	DocFreq() int
}
