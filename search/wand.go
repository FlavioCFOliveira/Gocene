package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// WANDCoordinator manages a set of WANDScorers to efficiently find top documents.
type WANDCoordinator struct {
	scorers []*WANDScorer
	pq      *DisiPriorityQueue
}

func NewWANDCoordinator(scorers []*WANDScorer) *WANDCoordinator {
	return &WANDCoordinator{
		scorers: scorers,
		pq:      NewDisiPriorityQueue(len(scorers)),
	}
}

// Next finds the next document that could potentially beat the current top scores.
func (wc *WANDCoordinator) Next(minScore float32) int {
	// Simplified WAND: find the minimum docID among current candidates.
	// In a full implementation, we would use block-maxes to skip docs.
	minDoc := -1
	for _, s := range wc.scorers {
		doc := s.DocID()
		if doc != -1 && (minDoc == -1 || doc < minDoc) {
			minDoc = doc
		}
	}
	
	if minDoc == -1 {
		return -1
	}

	// Advance all scorers to minDoc
	for _, s := range wc.scorers {
		s.Advance(minDoc)
	}

	return minDoc
}

func (wc *WANDCoordinator) Score(doc int) float32 {
	var total float32
	for _, s := range wc.scorers {
		if s.DocID() == doc {
			total += s.Score()
		}
	}
	return total
}
