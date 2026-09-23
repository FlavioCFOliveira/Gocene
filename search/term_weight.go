// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Ported from Apache Lucene 10.5.0:
//   lucene/core/src/java/org/apache/lucene/search/TermQuery.java (inner class TermWeight)

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TermWeight is the Weight for a TermQuery.
//
// Mirrors org.apache.lucene.search.TermQuery.TermWeight.
type TermWeight struct {
	BaseWeight
	term       *index.Term
	similarity Similarity
	simScorer  SimScorer
	termStates *index.TermStates
	scoreMode  ScoreMode
}

// NewTermWeight builds the Weight for the supplied term.
//
// Mirrors TermWeight(IndexSearcher, ScoreMode, float, TermStates), which throws
// IllegalStateException when scores are needed without TermStates and IOException
// from the statistics lookups.
func NewTermWeight(searcher *IndexSearcher, term *index.Term, scoreMode ScoreMode, boost float32, termStates *index.TermStates) (*TermWeight, error) {
	if scoreMode.NeedsScores() && termStates == nil {
		panic("termStates are required when scores are needed")
	}
	tw := &TermWeight{
		BaseWeight: BaseWeight{query: NewTermQuery(term)},
		term:       term,
		scoreMode:  scoreMode,
		termStates: termStates,
		similarity: searcher.GetSimilarity(),
	}

	var collectionStats *CollectionStatistics
	var termStats *TermStatistics
	if scoreMode.NeedsScores() {
		var err error
		collectionStats, err = searcher.CollectionStatistics(term.Field)
		if err != nil {
			return nil, err
		}
		if termStates.DocFreq() > 0 {
			ts := searcher.TermStatistics(term, termStates.DocFreq(), termStates.TotalTermFreq())
			termStats = &ts
		} else {
			termStats = nil
		}
	} else {
		// we do not need the actual stats, use fake stats with docFreq=maxDoc=ttf=1
		collectionStats = NewCollectionStatistics(term.Field, 1, 1, 1, 1)
		termStats = NewTermStatistics(term, 1, 1)
	}

	if termStats == nil {
		// term doesn't exist in any segment, we won't use similarity at all
		tw.simScorer = nil
	} else {
		// Assigning a dummy simScorer in case score is not needed to avoid unnecessary
		// float[] allocations in case default BM25Scorer is used.
		// See: https://github.com/apache/lucene/issues/12297
		if scoreMode.NeedsScores() {
			tw.simScorer = tw.similarity.Scorer104(boost, collectionStats, termStats)
		} else {
			// Assigning a dummy scorer as this is not expected to be called since
			// scores are not needed.
			tw.simScorer = &dummySimScorer{}
		}
	}

	return tw, nil
}

// dummySimScorer is the anonymous Similarity.SimScorer whose score(freq, norm)
// returns 0f, assigned by TermWeight when scores are not needed.
type dummySimScorer struct{}

// Score104 mirrors the anonymous SimScorer.score(float, long), which returns 0f.
func (d *dummySimScorer) Score104(freq float32, norm int64) float32 {
	return 0
}

// AsBulkSimScorer mirrors the concrete body of Similarity.SimScorer.asBulkSimScorer()
// in Apache Lucene 10.5.0: new DefaultBulkSimScorer(this).
func (d *dummySimScorer) AsBulkSimScorer() BulkSimScorer {
	return NewDefaultBulkSimScorer(d)
}

// Explain104 mirrors the concrete body of Similarity.SimScorer.explain(Explanation, long)
// in Apache Lucene 10.5.0: Explanation.match(score(freq.getValue().floatValue(), norm),
// "score(freq=" + freq.getValue() + "), with freq of:", freq).
func (d *dummySimScorer) Explain104(freq Explanation, norm int64) Explanation {
	e := NewExplanation(true, d.Score104(freq.GetValue(), norm),
		fmt.Sprintf("score(freq=%s), with freq of:", formatFloatGeneric(freq.GetValue())))
	e.AddDetail(freq)
	return e
}

// String mirrors TermWeight.toString().
func (tw *TermWeight) String() string {
	return "weight(" + queryToString(tw.BaseWeight.GetQuery(), "") + ")"
}

// ScorerSupplier returns a ScorerSupplier for the given leaf, or nil when the
// term is absent from it.
//
// Mirrors TermWeight.scorerSupplier(LeafReaderContext).
func (tw *TermWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	if tw.termStates == nil {
		return nil, nil
	}
	stateSupplier, err := tw.termStates.Get(context)
	if err != nil {
		return nil, err
	}
	if stateSupplier == nil {
		return nil, nil
	}
	return &termScorerSupplier{
		weight:        tw,
		context:       context,
		stateSupplier: stateSupplier,
	}, nil
}

// termScorerSupplier is the anonymous ScorerSupplier returned by
// TermWeight.scorerSupplier.
type termScorerSupplier struct {
	BaseScorerSupplier
	weight        *TermWeight
	context       *index.LeafReaderContext
	stateSupplier util.IOSupplier[index.TermState]

	termsEnum             index.TermsEnum
	topLevelScoringClause bool
}

// getTermsEnum lazily positions a TermsEnum on the weight's term.
//
// Mirrors the anonymous ScorerSupplier's private getTermsEnum().
func (s *termScorerSupplier) getTermsEnum() (index.TermsEnum, error) {
	if s.termsEnum == nil {
		state, err := s.stateSupplier()
		if err != nil {
			return nil, err
		}
		if state == nil {
			return nil, nil
		}
		terms, err := s.context.LeafReader().Terms(s.weight.term.Field)
		if err != nil {
			return nil, err
		}
		te, err := terms.Iterator()
		if err != nil {
			return nil, err
		}
		if err := index.SeekExactWithState(te, s.weight.term, state); err != nil {
			return nil, err
		}
		s.termsEnum = te
	}
	return s.termsEnum, nil
}

// Get builds the Scorer.
//
// Mirrors the anonymous ScorerSupplier.get(long).
func (s *termScorerSupplier) Get(leadCost int64) (Scorer, error) {
	termsEnum, err := s.getTermsEnum()
	if err != nil {
		return nil, err
	}
	if termsEnum == nil {
		return NewConstantScoreScorer(0, s.weight.scoreMode, Empty()), nil
	}

	var norms index.NumericDocValues
	if s.weight.scoreMode.NeedsScores() {
		norms, err = s.context.LeafReader().GetNormValues(s.weight.term.Field)
		if err != nil {
			return nil, err
		}
	}

	if s.weight.scoreMode == ScoreModeTopScores {
		impacts, err := termsEnum.Impacts(index.PostingsFlagFreqs)
		if err != nil {
			return nil, err
		}
		return NewTermScorerWithImpacts(impacts, s.weight.simScorer, norms, s.topLevelScoringClause)
	}
	flags := index.PostingsFlagNone
	if s.weight.scoreMode.NeedsScores() {
		flags = index.PostingsFlagFreqs
	}
	postings, err := termsEnum.Postings(flags)
	if err != nil {
		return nil, err
	}
	return NewTermScorer(postings, s.weight.simScorer, norms)
}

// BulkScorer returns a scorer optimized for bulk scoring.
//
// Mirrors the anonymous ScorerSupplier.bulkScorer().
func (s *termScorerSupplier) BulkScorer() (BulkScorer, error) {
	scorer, err := s.Get(math.MaxInt64)
	if err != nil {
		return nil, err
	}
	if !s.weight.scoreMode.NeedsScores() {
		iterator := scorer.Iterator()
		return NewConstantScoreScorerSupplierFromIterator(0, s.weight.scoreMode, iterator).BulkScorer()
	}
	return NewBatchScoreBulkScorer(scorer), nil
}

// Cost returns the docFreq of the term in this leaf, or 0 when it is absent.
//
// Mirrors the anonymous ScorerSupplier.cost().
func (s *termScorerSupplier) Cost() int64 {
	te, err := s.getTermsEnum()
	if err != nil || te == nil {
		return 0
	}
	docFreq, err := te.DocFreq()
	if err != nil {
		return 0
	}
	return int64(docFreq)
}

// SetTopLevelScoringClause marks this supplier as producing collector-visible scores.
//
// Mirrors the anonymous ScorerSupplier.setTopLevelScoringClause().
func (s *termScorerSupplier) SetTopLevelScoringClause() error {
	s.topLevelScoringClause = true
	return nil
}

// IsCacheable mirrors TermWeight.isCacheable(LeafReaderContext), whose body is
// `return true`.
func (tw *TermWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return true
}

// getTermsEnum returns a TermsEnum positioned at this weight's Term, or nil if
// the term does not exist in the given context.
//
// Mirrors TermWeight.getTermsEnum(LeafReaderContext).
func (tw *TermWeight) getTermsEnum(context *index.LeafReaderContext) (index.TermsEnum, error) {
	if tw.termStates == nil {
		return nil, nil
	}
	supplier, err := tw.termStates.Get(context)
	if err != nil {
		return nil, err
	}
	var state index.TermState
	if supplier != nil {
		state, err = supplier()
		if err != nil {
			return nil, err
		}
	}
	if state == nil { // term is not present in that reader
		return nil, nil
	}
	terms, err := context.LeafReader().Terms(tw.term.Field)
	if err != nil {
		return nil, err
	}
	termsEnum, err := terms.Iterator()
	if err != nil {
		return nil, err
	}
	if err := index.SeekExactWithState(termsEnum, tw.term, state); err != nil {
		return nil, err
	}
	return termsEnum, nil
}

// Explain produces an Explanation for the given document.
//
// Mirrors TermWeight.explain(LeafReaderContext, int).
func (tw *TermWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	scorer, err := tw.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer != nil {
		newDoc, err := scorer.Iterator().Advance(doc)
		if err != nil {
			return nil, err
		}
		if newDoc == doc {
			termScorer, ok := scorer.(*TermScorer)
			if !ok {
				return NoMatchExplanation("no matching term"), nil
			}
			freq, err := termScorer.Freq()
			if err != nil {
				return nil, err
			}
			norms, err := context.LeafReader().GetNormValues(tw.term.Field)
			if err != nil {
				return nil, err
			}
			var norm int64 = 1
			if norms != nil {
				ok, err := norms.AdvanceExact(doc)
				if err != nil {
					return nil, err
				}
				if ok {
					norm, err = norms.LongValue()
					if err != nil {
						return nil, err
					}
				}
			}
			freqExplanation := MatchExplanation(float32(freq), "freq, occurrences of term within document")
			scoreExplanation := tw.simScorer.Explain104(freqExplanation, norm)
			return MatchExplanationWithDetails(
				scoreExplanation.GetValue(),
				fmt.Sprintf("weight(%s in %d) [%s], result of:",
					queryToString(tw.BaseWeight.GetQuery(), ""), doc, tw.similarityName()),
				scoreExplanation), nil
		}
	}
	return NoMatchExplanation("no matching term"), nil
}

// similarityName returns the descriptive name of the similarity backing this
// weight for use in explanations. It stands in for Java's
// similarity.getClass().getSimpleName().
func (tw *TermWeight) similarityName() string {
	if tw.similarity == nil {
		return "Similarity"
	}
	if s, ok := tw.similarity.(interface{ String() string }); ok {
		return s.String()
	}
	return "Similarity"
}

// Count returns the number of documents matching this weight in the given leaf,
// or -1 when the count cannot be computed in sub-linear time.
//
// Mirrors TermWeight.count(LeafReaderContext).
func (tw *TermWeight) Count(context *index.LeafReaderContext) (int, error) {
	if !context.LeafReader().HasDeletions() {
		termsEnum, err := tw.getTermsEnum(context)
		if err != nil {
			return 0, err
		}
		// termsEnum is not nil if term state is available
		if termsEnum != nil {
			return termsEnum.DocFreq()
		}
		// the term cannot be found in the dictionary so the count is 0
		return 0, nil
	}
	return tw.BaseWeight.Count(context)
}

var _ Weight = (*TermWeight)(nil)

// Scorer renders the final Weight.scorer(LeafReaderContext): the scorer of
// scorerSupplier(context).get(Long.MAX_VALUE), or nil when no document
// matches. It is restated because the embedded BaseWeight.Scorer would call
// BaseWeight.ScorerSupplier, not this type's override.
func (tw *TermWeight) Scorer(context *index.LeafReaderContext) (Scorer, error) {
	scorerSupplier, err := tw.ScorerSupplier(context)
	if err != nil || scorerSupplier == nil {
		return nil, err
	}
	return scorerSupplier.Get(math.MaxInt64)
}

// BulkScorer renders the final Weight.bulkScorer(LeafReaderContext):
// scorerSupplier(context), marked as the top-level scoring clause, supplies
// the bulk scorer; nil when no document matches. It is restated because the
// embedded BaseWeight.BulkScorer would call BaseWeight.ScorerSupplier, not
// this type's override.
func (tw *TermWeight) BulkScorer(context *index.LeafReaderContext) (BulkScorer, error) {
	scorerSupplier, err := tw.ScorerSupplier(context)
	if err != nil || scorerSupplier == nil {
		// No docs match
		return nil, err
	}
	if err := scorerSupplier.SetTopLevelScoringClause(); err != nil {
		return nil, err
	}
	return scorerSupplier.BulkScorer()
}
