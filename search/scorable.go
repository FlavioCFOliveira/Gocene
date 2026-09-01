package search

import "io"

// Scorable allows access to the score of a Query.
type Scorable interface {
	// Score returns the score of the current document matching the query.
	Score() (float32, error)

	// SmoothingScore returns the smoothing score of the current document matching the query.
	SmoothingScore(docID int) (float32, error)

	// SetMinCompetitiveScore tells the scorer that its iterator may safely ignore all documents
	// whose score is less than the given minScore.
	SetMinCompetitiveScore(minScore float32) error

	// GetChildren returns child sub-scorers positioned on the current document.
	GetChildren() ([]ChildScorable, error)
}

// ChildScorable is a child Scorer and its relationship to its parent.
type ChildScorable struct {
	Child        Scorable
	Relationship string
}
