// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// PhraseWeight is the Weight implementation for PhraseQuery.
// This is the Go port of Lucene's org.apache.lucene.search.PhraseWeight.
type PhraseWeight struct {
	*BaseWeight
	query       *PhraseQuery
	searcher    *IndexSearcher
	needsScores bool
	similarity  Similarity
	simScorer   SimScorer
}

// NewPhraseWeight creates a new PhraseWeight.
func NewPhraseWeight(query *PhraseQuery, searcher *IndexSearcher, needsScores bool) (*PhraseWeight, error) {
	w := &PhraseWeight{
		BaseWeight:  NewBaseWeight(query),
		query:       query,
		searcher:    searcher,
		needsScores: needsScores,
		similarity:  NewClassicSimilarity(),
	}

	if needsScores && len(query.terms) > 0 {
		// Get collection statistics
		collectionStats := w.getCollectionStats(searcher)
		// Get term statistics for the first term
		termStats := w.getTermStats(searcher)
		// Create the similarity scorer
		w.simScorer = w.similarity.Scorer(collectionStats, termStats)
	}

	return w, nil
}

// getCollectionStats returns collection statistics for the phrase's field.
func (w *PhraseWeight) getCollectionStats(searcher *IndexSearcher) *CollectionStatistics {
	reader := searcher.GetIndexReader()
	return NewCollectionStatistics(w.query.field, reader.MaxDoc(), reader.NumDocs(), -1, -1)
}

// getTermStats returns term statistics for the phrase.
func (w *PhraseWeight) getTermStats(searcher *IndexSearcher) *TermStatistics {
	if len(w.query.terms) == 0 {
		return nil
	}
	// Use the first term for statistics
	term := w.query.terms[0]
	docFreq := 0
	if reader, ok := searcher.GetIndexReader().(index.IndexReaderInterface); ok {
		if leafReader, ok := reader.(*index.LeafReader); ok {
			terms, err := leafReader.Terms(w.query.field)
			if err == nil && terms != nil {
				docFreq, _ = terms.GetDocCount()
			}
		}
	}
	return NewTermStatistics(term, docFreq, -1)
}

// Scorer creates a scorer for this weight.
func (w *PhraseWeight) Scorer(reader index.IndexReaderInterface) (Scorer, error) {
	// Get the leaf contexts
	leaves, err := reader.Leaves()
	if err != nil {
		return nil, err
	}

	// Try to get postings from the first leaf reader
	for _, leaf := range leaves {
		leafReader := leaf.LeafReader()
		if leafReader == nil {
			continue
		}

		// Get the terms for the field
		terms, err := leafReader.Terms(w.query.field)
		if err != nil {
			return nil, err
		}
		if terms == nil {
			continue
		}

		// For phrase queries, we need to find documents containing all terms
		// in the correct positions. This is a simplified implementation.
		// A full implementation would use phrase positions and slop.

		// Get postings for the first term
		if len(w.query.terms) == 0 {
			return nil, nil
		}

		termsEnum, err := terms.GetIterator()
		if err != nil {
			return nil, err
		}

		// Seek to the first term
		found, err := termsEnum.SeekExact(w.query.terms[0])
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, nil
		}

		// Get the postings enum for the term
		postingsEnum, err := termsEnum.Postings(0)
		if err != nil {
			return nil, err
		}
		if postingsEnum == nil {
			return nil, nil
		}

		// Create and return the scorer
		// For phrase queries with slop > 0, we'd need a SloppyPhraseScorer
		if w.query.slop > 0 {
			return NewSloppyPhraseScorer(w, postingsEnum, w.simScorer, w.query.slop), nil
		}
		return NewPhraseScorer(w, postingsEnum, w.simScorer), nil
	}

	return nil, nil
}

// GetValueForNormalization returns the value for normalization.
func (w *PhraseWeight) GetValueForNormalization() float32 {
	return 1.0
}

// Normalize normalizes this weight.
func (w *PhraseWeight) Normalize(norm float32) {}

// Ensure PhraseWeight implements Weight
var _ Weight = (*PhraseWeight)(nil)

// PhraseScorer is a scorer for exact phrase queries.
type PhraseScorer struct {
	*BaseScorer
	postings  index.PostingsEnum
	simScorer SimScorer
	doc       int
}

// NewPhraseScorer creates a new PhraseScorer.
func NewPhraseScorer(weight Weight, postings index.PostingsEnum, simScorer SimScorer) *PhraseScorer {
	return &PhraseScorer{
		BaseScorer: NewBaseScorer(weight),
		postings:   postings,
		simScorer:  simScorer,
		doc:        -1,
	}
}

// DocID returns the current document ID.
func (s *PhraseScorer) DocID() int {
	return s.doc
}

// NextDoc advances to the next document.
func (s *PhraseScorer) NextDoc() (int, error) {
	doc, err := s.postings.NextDoc()
	if err != nil {
		return NO_MORE_DOCS, err
	}
	s.doc = doc
	return doc, nil
}

// Advance advances to the target document.
func (s *PhraseScorer) Advance(target int) (int, error) {
	doc, err := s.postings.Advance(target)
	if err != nil {
		return NO_MORE_DOCS, err
	}
	s.doc = doc
	return doc, nil
}

// Cost returns the estimated cost.
func (s *PhraseScorer) Cost() int64 {
	return s.postings.Cost()
}

// DocIDRunEnd returns the end of the current run.
func (s *PhraseScorer) DocIDRunEnd() int {
	return s.doc + 1
}

// Score returns the score for the current document.
func (s *PhraseScorer) Score() float32 {
	if s.simScorer != nil {
		return s.simScorer.Score(s.doc, 1.0)
	}
	return 1.0
}

// GetMaxScore returns the maximum score for documents up to the given doc.
func (s *PhraseScorer) GetMaxScore(upTo int) float32 {
	return 1.0
}

// Ensure PhraseScorer implements Scorer
var _ Scorer = (*PhraseScorer)(nil)

// SloppyPhraseScorer is a scorer for sloppy phrase queries.
type SloppyPhraseScorer struct {
	*PhraseScorer
	slop int
}

// NewSloppyPhraseScorer creates a new SloppyPhraseScorer.
func NewSloppyPhraseScorer(weight Weight, postings index.PostingsEnum, simScorer SimScorer, slop int) *SloppyPhraseScorer {
	return &SloppyPhraseScorer{
		PhraseScorer: NewPhraseScorer(weight, postings, simScorer),
		slop:         slop,
	}
}

// Ensure SloppyPhraseScorer implements Scorer
var _ Scorer = (*SloppyPhraseScorer)(nil)
