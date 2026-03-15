// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// TermScorer scores documents using a term's postings.
//
// This is the Go port of Lucene's org.apache.lucene.search.TermScorer.
//
// TermScorer iterates over the documents matching a term and scores them
// using the provided Similarity.SimScorer.
type TermScorer struct {
	*BaseScorer
	postingsEnum index.PostingsEnum
	doc          int
	simScorer    SimScorer
}

// NewTermScorer creates a new TermScorer.
func NewTermScorer(weight Weight, postingsEnum index.PostingsEnum, simScorer SimScorer) *TermScorer {
	return &TermScorer{
		BaseScorer:   NewBaseScorer(weight),
		postingsEnum: postingsEnum,
		doc:          -1,
		simScorer:    simScorer,
	}
}

// NextDoc advances to the next document.
func (s *TermScorer) NextDoc() (int, error) {
	if s.postingsEnum == nil {
		s.doc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	nextDoc, err := s.postingsEnum.NextDoc()
	if err != nil {
		return NO_MORE_DOCS, err
	}
	s.doc = nextDoc
	return nextDoc, nil
}

// DocID returns the current document ID.
func (s *TermScorer) DocID() int {
	return s.doc
}

// Advance advances to the document at or beyond the target.
func (s *TermScorer) Advance(target int) (int, error) {
	if s.postingsEnum == nil {
		s.doc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	advancedDoc, err := s.postingsEnum.Advance(target)
	if err != nil {
		return NO_MORE_DOCS, err
	}
	s.doc = advancedDoc
	return advancedDoc, nil
}

// Score returns the score of the current document.
func (s *TermScorer) Score() float32 {
	if s.simScorer == nil {
		return 1.0
	}
	freq, err := s.postingsEnum.Freq()
	if err != nil {
		return 0.0
	}
	return s.simScorer.Score(s.doc, float32(freq))
}

// Cost returns the estimated cost of iterating through all documents.
func (s *TermScorer) Cost() int64 {
	if s.postingsEnum == nil {
		return 0
	}
	return s.postingsEnum.Cost()
}

// DocIDRunEnd returns the end of the current run of consecutive doc IDs.
func (s *TermScorer) DocIDRunEnd() int {
	return s.doc + 1
}

// Ensure TermScorer implements Scorer
var _ Scorer = (*TermScorer)(nil)
