package search

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Scorer exposes an iterator over documents matching a query in increasing order of doc id.
type Scorer interface {
	Scorable
	// DocID returns the doc ID that is currently being scored.
	DocID() int

	// Iterator returns a DocIdSetIterator over matching documents.
	Iterator() DocIdSetIterator

	// TwoPhaseIterator returns a TwoPhaseIterator view of this Scorer.
	TwoPhaseIterator() TwoPhaseIterator

	// AdvanceShallow advances to the block of documents that contains target.
	AdvanceShallow(target int) (int, error)

	// GetMaxScore returns the maximum score that documents between the last advanceShallow and upTo included.
	GetMaxScore(upTo int) (float32, error)

	// NextDocsAndScores returns a new batch of doc IDs and scores.
	NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *DocAndFloatFeatureBuffer) error
}

// TwoPhaseIterator is returned by Scorer.TwoPhaseIterator() to expose an approximation of a DocIdSetIterator.
type TwoPhaseIterator interface {
	// Approximation returns a DocIdSetIterator view of the provided TwoPhaseIterator.
	Approximation() DocIdSetIterator

	// Matches returns whether the current doc ID that Approximation() is on matches.
	Matches() (bool, error)

	// MatchCost returns an estimate of the expected cost to determine that a single document matches.
	MatchCost() float32

	// DocIDRunEnd returns the end of the run of consecutive doc IDs that match this TwoPhaseIterator.
	DocIDRunEnd() (int, error)

	// IntoBitSet loads the doc IDs that both belong to the Approximation() and Matches() match.
	IntoBitSet(upTo int, bitSet util.BitSet, offset int) error
}

// DocAndFloatFeatureBuffer is a wrapper around parallel arrays storing doc IDs and their corresponding features.
type DocAndFloatFeatureBuffer struct {
	Docs     []int
	Features []float32
	Size     int
}

func NewDocAndFloatFeatureBuffer() *DocAndFloatFeatureBuffer {
	return &DocAndFloatFeatureBuffer{}
}

func (b *DocAndFloatFeatureBuffer) GrowNoCopy(minSize int) {
	if len(b.Docs) < minSize {
		newDocs := make([]int, minSize)
		copy(newDocs, b.Docs)
		b.Docs = newDocs
		b.Features = make([]float32, len(b.Docs))
	}
}

func (b *DocAndFloatFeatureBuffer) Apply(liveDocs util.Bits) {
	newSize := 0
	for i := 0; i < b.Size; i++ {
		if liveDocs.Get(b.Docs[i]) {
			b.Docs[newSize] = b.Docs[i]
			b.Features[newSize] = b.Features[i]
			newSize++
		}
	}
	b.Size = newSize
}
