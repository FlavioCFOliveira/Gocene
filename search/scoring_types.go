package search

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ScorerSupplier is created by Weight.ScorerSupplier() to expose cost and count
// information before creating the actual Scorer.
type ScorerSupplier interface {
	// Get returns a Scorer for the given weight index.
	Get(weightIndex int) (Scorer, error)

	// GetMatchCost returns an estimate of the expected cost to determine
	// that a single document matches.
	GetMatchCost() float32

	// GetDocCount returns the number of documents that match this weight.
	GetDocCount() int
}

// BulkScorer is an optimized version of Scorer that can produce multiple
// doc IDs and scores in a single call.
type BulkScorer interface {
	// NextDoc returns the next matching doc ID, and fills the buffer with
	// docs and scores up to upTo.
	NextDoc(upTo int, liveDocs util.Bits, buffer *DocAndFloatFeatureBuffer) (int, error)

	// Score returns the score of the document returned by NextDoc.
	Score() float32

	// DocID returns the doc ID of the document returned by NextDoc.
	DocID() int

	// Iterator returns a DocIdSetIterator over matching documents.
	Iterator() DocIdSetIterator
}

// DefaultBulkScorer is a simple implementation of BulkScorer that wraps a Scorer.
type DefaultBulkScorer struct {
	scorer Scorer
}

func NewDefaultBulkScorer(scorer Scorer) *DefaultBulkScorer {
	return &DefaultBulkScorer{scorer: scorer}
}

func (b *DefaultBulkScorer) NextDoc(upTo int, liveDocs util.Bits, buffer *DocAndFloatFeatureBuffer) (int, error) {
	if b.scorer == nil {
		return -1, nil
	}
	doc, err := b.scorer.Advance(upTo)
	if err != nil {
		return -1, err
	}
	if doc >= upTo {
		return -1, nil
	}

	// In a real Lucene implementation, BulkScorer would fill the buffer.
	// This default implementation just advances one by one.
	b.scorer.Advance(doc)
	buffer.Docs = append(buffer.Docs, doc)
	buffer.Features = append(buffer.Features, b.scorer.Score())
	buffer.Size++

	return doc, nil
}

func (b *DefaultBulkScorer) Score() float32 {
	if b.scorer == nil {
		return 0
	}
	return b.scorer.Score()
}

func (b *DefaultBulkScorer) DocID() int {
	if b.scorer == nil {
		return -1
	}
	return b.scorer.DocID()
}

func (b *DefaultBulkScorer) Iterator() DocIdSetIterator {
	if b.scorer == nil {
		return nil
	}
	return b.scorer.Iterator()
}

// Explanation provides a detailed breakdown of how a score was computed.
type Explanation struct {
	Description string
	Value       float32
	Details     []*Explanation
}

func NewExplanation(description string, value float32) *Explanation {
	return &Explanation{
		Description: description,
		Value:       value,
	}
}

func (e *Explanation) AddDetail(detail *Explanation) {
	e.Details = append(e.Details, detail)
}

func (e *Explanation) String() string {
	return fmt.Sprintf("%s: %f", e.Description, e.Value)
}

// Matches is used to describe why a document matches a Weight.
type Matches interface {
	// GetMatches returns the matches for the given document.
	GetMatches() string
}
