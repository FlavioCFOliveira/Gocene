// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Ported from Apache Lucene 10.5.0:
//   lucene/core/src/java/org/apache/lucene/search/TermScorer.java

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TermScorer is a Scorer for documents matching a Term.
//
// Mirrors org.apache.lucene.search.TermScorer (final class).
type TermScorer struct {
	BaseScorer
	postingsEnum  index.PostingsEnum
	iterator      DocIdSetIterator
	scorer        SimScorer
	bulkScorer    BulkSimScorer
	norms         index.NumericDocValues
	impactsDisi   *ImpactsDISI
	maxScoreCache *MaxScoreCache
}

// NewTermScorer constructs a TermScorer that will iterate all documents.
//
// Mirrors TermScorer(PostingsEnum, SimScorer, NumericDocValues). Gocene's
// index.ImpactsSource.GetImpacts returns an error, so the eager materialisation
// of the first Impacts snapshot that MaxScoreCache needs is surfaced here;
// the Java constructor itself cannot fail.
func NewTermScorer(postingsEnum index.PostingsEnum, scorer SimScorer, norms index.NumericDocValues) (*TermScorer, error) {
	impactsEnum := index.NewSlowImpactsEnum(postingsEnum)
	src, err := newIndexImpactsSource(impactsEnum)
	if err != nil {
		return nil, err
	}
	return &TermScorer{
		postingsEnum:  postingsEnum,
		iterator:      postingsEnum,
		maxScoreCache: NewMaxScoreCache(src, newSimImpactScorer(scorer)),
		impactsDisi:   nil,
		scorer:        scorer,
		norms:         norms,
		bulkScorer:    asBulkSimScorer(scorer),
	}, nil
}

// NewTermScorerWithImpacts constructs a TermScorer that will use impacts to
// skip blocks of non-competitive documents.
//
// Mirrors TermScorer(ImpactsEnum, SimScorer, NumericDocValues, boolean).
func NewTermScorerWithImpacts(
	impactsEnum spi.ImpactsEnum,
	scorer SimScorer,
	norms index.NumericDocValues,
	topLevelScoringClause bool,
) (*TermScorer, error) {
	src, err := newSPIImpactsSource(impactsEnum)
	if err != nil {
		return nil, err
	}
	t := &TermScorer{
		postingsEnum:  impactsEnum,
		maxScoreCache: NewMaxScoreCache(src, newSimImpactScorer(scorer)),
		scorer:        scorer,
		norms:         norms,
		bulkScorer:    asBulkSimScorer(scorer),
	}
	if topLevelScoringClause {
		t.impactsDisi = NewImpactsDISI(impactsEnum, t.maxScoreCache)
		t.iterator = t.impactsDisi
	} else {
		t.impactsDisi = nil
		t.iterator = impactsEnum
	}
	return t, nil
}

// asBulkSimScorer returns scorer.AsBulkSimScorer(), or nil for a nil scorer.
//
// Lucene calls scorer.asBulkSimScorer() unconditionally; Gocene admits a nil
// SimScorer on the paths where scores are not needed (see TermScorer.Score,
// which returns 0 for a nil scorer), so the nil case is carried here rather
// than panicking during construction.
func asBulkSimScorer(scorer SimScorer) BulkSimScorer {
	if scorer == nil {
		return nil
	}
	return scorer.AsBulkSimScorer()
}

// DocID returns the doc ID that is currently being scored.
//
// Mirrors TermScorer.docID().
func (s *TermScorer) DocID() int {
	return s.postingsEnum.DocID()
}

// Freq returns the term frequency in the current document.
//
// Mirrors TermScorer.freq().
func (s *TermScorer) Freq() (int, error) {
	return s.postingsEnum.Freq()
}

// Iterator returns a DocIdSetIterator over matching documents.
//
// Mirrors TermScorer.iterator().
func (s *TermScorer) Iterator() DocIdSetIterator {
	return s.iterator
}

// Score returns the score of the current document.
//
// Mirrors TermScorer.score().
func (s *TermScorer) Score() (float32, error) {
	if s.scorer == nil {
		return 0, nil
	}
	postingsEnum := s.postingsEnum
	norms := s.norms

	var norm int64 = 1
	if norms != nil {
		ok, err := norms.AdvanceExact(postingsEnum.DocID())
		if err != nil {
			return 0, err
		}
		if ok {
			norm, err = norms.LongValue()
			if err != nil {
				return 0, err
			}
		}
	}
	freq, err := postingsEnum.Freq()
	if err != nil {
		return 0, err
	}
	return s.scorer.Score104(float32(freq), norm), nil
}

// SmoothingScore returns the smoothing score of the current document.
//
// Mirrors TermScorer.smoothingScore(int).
func (s *TermScorer) SmoothingScore(docID int) (float32, error) {
	if s.scorer == nil {
		return 0, nil
	}
	var norm int64 = 1
	if s.norms != nil {
		ok, err := s.norms.AdvanceExact(docID)
		if err != nil {
			return 0, err
		}
		if ok {
			norm, err = s.norms.LongValue()
			if err != nil {
				return 0, err
			}
		}
	}
	return s.scorer.Score104(0, norm), nil
}

// AdvanceShallow advances to the block of documents that contains target.
//
// Mirrors TermScorer.advanceShallow(int).
func (s *TermScorer) AdvanceShallow(target int) (int, error) {
	return s.maxScoreCache.AdvanceShallow(target)
}

// GetMaxScore returns the maximum score up to and including upTo.
//
// Mirrors TermScorer.getMaxScore(int).
func (s *TermScorer) GetMaxScore(upTo int) (float32, error) {
	return s.maxScoreCache.GetMaxScore(upTo)
}

// SetMinCompetitiveScore tells the scorer that hits scoring below minScore are
// not competitive.
//
// Mirrors TermScorer.setMinCompetitiveScore(float).
func (s *TermScorer) SetMinCompetitiveScore(minScore float32) error {
	if s.impactsDisi != nil {
		s.impactsDisi.SetMinCompetitiveScore(minScore)
	}
	return nil
}

// NextDocsAndScores returns a new batch of doc IDs and scores, starting at the
// current doc ID and ending before upTo.
//
// PORTING GAP. Lucene 10.5.0's TermScorer overrides nextDocsAndScores with a
// bulk path built on PostingsEnum.nextPostings(int, DocAndFloatFeatureBuffer),
// DocAndFloatFeatureBuffer.apply(Bits) and NumericDocValues.longValues(...),
// none of which exist on Gocene's index.PostingsEnum / DocAndFloatFeatureBuffer
// yet. Until they do, this carries the inherited default body of
// Scorer.nextDocsAndScores, which yields exactly the same (doc, score) pairs
// one document at a time.
func (s *TermScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *DocAndFloatFeatureBuffer) error {
	return DefaultNextDocsAndScores(s, upTo, liveDocs, buffer)
}

// Compile-time assertion: TermScorer satisfies the Scorer contract.
var _ Scorer = (*TermScorer)(nil)
