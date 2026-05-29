// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TermScorer scores documents using a term's postings.
//
// This is the Go port of Lucene's org.apache.lucene.search.TermScorer.
//
// TermScorer iterates over the documents matching a term and scores them
// using the provided Similarity.SimScorer.
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
	// liveDocs filters out deleted documents. When nil, every document the
	// postings enumerate is live. Mirrors the acceptDocs/liveDocs intersection
	// Lucene applies around a TermScorer; without it, documents deleted via a
	// persisted .liv file would still match (rmp #4753).
	liveDocs util.Bits
}

// NewTermScorer creates a new TermScorer with no live-docs filtering.
func NewTermScorer(weight Weight, postingsEnum index.PostingsEnum, simScorer SimScorer) *TermScorer {
	return NewTermScorerWithLiveDocs(weight, postingsEnum, simScorer, nil)
}

// NewTermScorerWithLiveDocs creates a TermScorer that skips documents not set
// in liveDocs. A nil liveDocs means all documents are live.
func NewTermScorerWithLiveDocs(weight Weight, postingsEnum index.PostingsEnum, simScorer SimScorer, liveDocs util.Bits) *TermScorer {
	return &TermScorer{
		BaseScorer:   NewBaseScorer(weight),
		postingsEnum: postingsEnum,
		doc:          -1,
		simScorer:    simScorer,
		liveDocs:     liveDocs,
	}
}

// isLive reports whether docID is a live (non-deleted) document. A nil liveDocs
// or an out-of-range docID (past the bitset length) is treated as live, matching
// Bits.get semantics for the sentinel-free range.
func (s *TermScorer) isLive(docID int) bool {
	if s.liveDocs == nil {
		return true
	}
	if docID < 0 || docID >= s.liveDocs.Length() {
		return true
	}
	return s.liveDocs.Get(docID)
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

// NextDoc advances to the next live document.
func (s *TermScorer) NextDoc() (int, error) {
	if s.postingsEnum == nil {
		s.doc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	for {
		nextDoc, err := s.postingsEnum.NextDoc()
		if err != nil {
			return NO_MORE_DOCS, err
		}
		s.doc = postingsDocToSearchDoc(nextDoc)
		if s.doc == NO_MORE_DOCS || s.isLive(s.doc) {
			return s.doc, nil
		}
	}
}

// DocID returns the current document ID.
func (s *TermScorer) DocID() int {
	return s.doc
}

// Advance advances to the first live document at or beyond the target.
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
	if s.doc == NO_MORE_DOCS || s.isLive(s.doc) {
		return s.doc, nil
	}
	// Landed on a deleted doc: walk forward to the next live one.
	return s.NextDoc()
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
