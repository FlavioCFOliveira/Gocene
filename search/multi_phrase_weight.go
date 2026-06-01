// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// MultiPhraseWeight is the Weight implementation for MultiPhraseQuery.
//
// It is the Go port of Apache Lucene 10.4.0's MultiPhraseQuery.MultiPhraseWeight
// (the relevant slice). For each phrase position whose term array holds a single
// term, the per-leaf PostingsEnum is read directly; for a position with multiple
// terms, the postings of every term at that position are merged through a
// UnionPostingsEnum so the slot behaves as a position-level disjunction. The
// merged per-position enums are then fed into the same exact / sloppy phrase
// scorers PhraseQuery uses, so MultiPhraseQuery and PhraseQuery share identical
// position-matching semantics.
type MultiPhraseWeight struct {
	*BaseWeight
	query       *MultiPhraseQuery
	searcher    *IndexSearcher
	needsScores bool
	similarity  Similarity
	simScorer   SimScorer
}

// NewMultiPhraseWeight creates a new MultiPhraseWeight.
func NewMultiPhraseWeight(query *MultiPhraseQuery, searcher *IndexSearcher, needsScores bool) (*MultiPhraseWeight, error) {
	w := &MultiPhraseWeight{
		BaseWeight:  NewBaseWeight(query),
		query:       query,
		searcher:    searcher,
		needsScores: needsScores,
		similarity:  NewClassicSimilarity(),
	}
	if needsScores && len(query.termArrays) > 0 {
		reader := searcher.GetIndexReader()
		collectionStats := NewCollectionStatistics(query.field, reader.MaxDoc(), reader.NumDocs(), -1, -1)
		var termStats *TermStatistics
		if len(query.termArrays[0]) > 0 {
			termStats = NewTermStatistics(query.termArrays[0][0], reader.NumDocs(), -1)
		}
		w.simScorer = w.similarity.Scorer(collectionStats, termStats)
	}
	return w, nil
}

// Scorer creates a scorer for this weight over a single leaf.
//
// It opens one PostingsEnum (or UnionPostingsEnum, for multi-term positions)
// per phrase position. If any position has no matching postings in this leaf
// the leaf cannot produce a phrase hit, so a nil scorer is returned.
func (w *MultiPhraseWeight) Scorer(context *index.LeafReaderContext) (Scorer, error) {
	leafReader := context.LeafReader()
	if leafReader == nil {
		return nil, nil
	}
	if len(w.query.termArrays) == 0 {
		return nil, nil
	}

	terms, err := leafReader.Terms(w.query.field)
	if err != nil {
		return nil, err
	}
	if terms == nil {
		return nil, nil
	}

	n := len(w.query.termArrays)
	postings := make([]index.PostingsEnum, n)
	for i, termArray := range w.query.termArrays {
		if len(termArray) == 0 {
			return nil, nil
		}
		subs := make([]index.PostingsEnum, 0, len(termArray))
		for _, term := range termArray {
			// Each slot needs its own TermsEnum so duplicate terms across
			// positions (e.g. "a (a b)") obtain independent PostingsEnums.
			termsEnum, err := terms.GetIterator()
			if err != nil {
				return nil, err
			}
			found, err := termsEnum.SeekExact(term)
			if err != nil {
				return nil, err
			}
			if !found {
				continue // this term is absent from the leaf; skip it
			}
			pe, err := termsEnum.Postings(index.PostingsFlagPositions)
			if err != nil {
				return nil, err
			}
			if pe == nil {
				continue
			}
			subs = append(subs, pe)
		}
		if len(subs) == 0 {
			// No term at this position exists in the leaf: no phrase can match.
			return nil, nil
		}
		if len(subs) == 1 {
			postings[i] = subs[0]
		} else {
			postings[i] = NewUnionPostingsEnum(subs)
		}
	}

	queryPositions := w.query.GetPositions()
	if w.query.slop == 0 {
		return NewPhraseScorer(w, postings, queryPositions, w.simScorer), nil
	}
	return NewSloppyPhraseScorer(w, postings, queryPositions, w.simScorer, w.query.slop), nil
}

// ScorerSupplier creates a scorer supplier for this weight.
func (w *MultiPhraseWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return NewScorerSupplierAdapter(scorer), nil
}

// BulkScorer creates a bulk scorer for efficient bulk scoring.
func (w *MultiPhraseWeight) BulkScorer(context *index.LeafReaderContext) (BulkScorer, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return NewDefaultBulkScorer(scorer), nil
}

// Explain returns an explanation of the score for the given document, mirroring
// the PhraseWeight.explain shape.
func (w *MultiPhraseWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer != nil {
		advanced, err := scorer.Advance(doc)
		if err != nil {
			return nil, err
		}
		if advanced == doc {
			score := scorer.Score()
			var freq float32
			if pfs, ok := scorer.(phraseFreqScorer); ok {
				freq = pfs.PhraseFreq()
			}
			scoreExpl := MatchExplanation(score, "score(phraseFreq), product of:")
			scoreExpl.AddDetail(MatchExplanation(freq, fmt.Sprintf("phraseFreq=%v", freq)))
			desc := fmt.Sprintf("weight(%s in %d) [%s], result of:", w.GetQuery(), doc, "ClassicSimilarity")
			result := MatchExplanation(score, desc)
			result.AddDetail(scoreExpl)
			return result, nil
		}
	}
	return NoMatchExplanation("no matching terms"), nil
}

// IsCacheable reports whether this weight can be cached for the given leaf.
func (w *MultiPhraseWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return true
}

// Count returns -1 (no sub-linear count is available).
func (w *MultiPhraseWeight) Count(context *index.LeafReaderContext) (int, error) {
	return -1, nil
}

// Matches returns the matches for a specific document (not yet implemented).
func (w *MultiPhraseWeight) Matches(context *index.LeafReaderContext, doc int) (Matches, error) {
	return nil, nil
}

// Ensure MultiPhraseWeight implements Weight.
var _ Weight = (*MultiPhraseWeight)(nil)
