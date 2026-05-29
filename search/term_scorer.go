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
//
// liveDocs model: like Lucene 10.4.0, TermScorer's iterator visits EVERY
// document the postings enumerate, including documents deleted via a persisted
// .liv file. Deleted documents (acceptDocs == LeafReader.getLiveDocs()) are
// excluded by the collector layer, centrally in IndexSearcher.searchLeaf, not
// here. Filtering at the scorer would diverge from Lucene and would make
// join.QueryBitSetProducer drop deleted parents from the block-join parent
// bitset (rmp #4762).
//
// Sentinel translation: index.PostingsEnum uses index.NO_MORE_DOCS (-1) as
// its exhaustion sentinel, while search.DocIdSetIterator uses NO_MORE_DOCS
// (math.MaxInt32). TermScorer bridges the two by mapping -1 → NO_MORE_DOCS
// on every return from NextDoc and Advance.
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

// postingsDocToSearchDoc translates index.NO_MORE_DOCS (-1) to the search
// package sentinel (NO_MORE_DOCS = math.MaxInt32). All other values are
// returned as-is.
func postingsDocToSearchDoc(doc int) int {
	if doc == index.NO_MORE_DOCS {
		return NO_MORE_DOCS
	}
	return doc
}

// NextDoc advances to the next document the postings enumerate (deleted docs
// included; liveDocs is applied centrally by the collector, see the type doc).
func (s *TermScorer) NextDoc() (int, error) {
	if s.postingsEnum == nil {
		s.doc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	nextDoc, err := s.postingsEnum.NextDoc()
	if err != nil {
		return NO_MORE_DOCS, err
	}
	s.doc = postingsDocToSearchDoc(nextDoc)
	return s.doc, nil
}

// DocID returns the current document ID.
func (s *TermScorer) DocID() int {
	return s.doc
}

// Advance advances to the first document at or beyond the target (deleted docs
// included; liveDocs is applied centrally by the collector, see the type doc).
func (s *TermScorer) Advance(target int) (int, error) {
	if s.postingsEnum == nil {
		s.doc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	advancedDoc, err := s.postingsEnum.Advance(target)
	if err != nil {
		return NO_MORE_DOCS, err
	}
	s.doc = postingsDocToSearchDoc(advancedDoc)
	return s.doc, nil
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

// Freq returns the term frequency of the current document, mirroring Lucene's
// org.apache.lucene.search.TermScorer.freq(). It is used by TermWeight.Explain
// to build the frequency sub-explanation. Returns 0 if the underlying postings
// enum cannot supply a frequency.
func (s *TermScorer) Freq() (int, error) {
	if s.postingsEnum == nil {
		return 0, nil
	}
	return s.postingsEnum.Freq()
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
