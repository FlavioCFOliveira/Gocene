package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TermWeight is the weight for a TermQuery.
type TermWeight struct {
	BaseWeight
	term       *index.Term
	similarity Similarity
	simScorer  SimScorer
	termStates *index.TermStates
	scoreMode  ScoreMode
}

func NewTermWeight(searcher *IndexSearcher, term *index.Term, scoreMode ScoreMode, boost float32, termStates *index.TermStates) *TermWeight {
	tw := &TermWeight{
		BaseWeight: BaseWeight{query: NewTermQuery(term)},
		term:       term,
		scoreMode:  scoreMode,
		termStates: termStates,
	}

	tw.similarity = searcher.GetSimilarity()

	var collectionStats *CollectionStatistics
	var termStats *TermStatistics

	if scoreMode.NeedsScores() {
		collectionStats = tw.getCollectionStats(searcher)
		if termStates != nil && termStates.DocFreq() > 0 {
			termStats = tw.getTermStats(searcher, termStates.DocFreq(), termStates.TotalTermFreq())
		}
	} else {
		collectionStats = NewCollectionStatistics(term.Field(), 1, 1, 1, 1)
		termStats = NewTermStatistics(term, 1, 1)
	}

	if termStats == nil {
		tw.simScorer = nil
	} else {
		if scoreMode.NeedsScores() {
			tw.simScorer = tw.similarity.Scorer104(boost, collectionStats, termStats)
		} else {
			// Dummy simScorer
			tw.simScorer = &dummySimScorer{}
		}
	}

	return tw
}

type dummySimScorer struct{}

func (d *dummySimScorer) Score(freq float32, norm int64) float32 {
	return 0
}

func (tw *TermWeight) getCollectionStats(searcher *IndexSearcher) *CollectionStatistics {
	reader := searcher.GetReader()
	// In a real implementation, we would get these from the reader/index.
	// For now, we use approximations or placeholder values.
	return NewCollectionStatistics(tw.term.Field(), reader.MaxDoc(), reader.NumDocs(), -1, -1)
}

func (tw *TermWeight) getTermStats(searcher *IndexSearcher, docFreq int, totalTermFreq int64) *TermStatistics {
	return NewTermStatistics(tw.term, docFreq, totalTermFreq)
}

func (tw *TermWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	if tw.termStates == nil {
		return nil, nil
	}

	stateSupplier := tw.termStates.Get(context)
	if stateSupplier == nil {
		return nil, nil
	}

	return &termScorerSupplier{
		tw:            tw,
		stateSupplier: stateSupplier,
		context:       context,
	}
}

type termScorerSupplier struct {
	tw            *TermWeight
	stateSupplier func() *index.TermState
	context       *index.LeafReaderContext
}

func (s *termScorerSupplier) Get(weightIndex int) (Scorer, error) {
	state := s.stateSupplier()
	if state == nil {
		return nil, nil
	}

	termsEnum := s.context.Reader().Terms(s.tw.term.Field()).Iterator()
	termsEnum.SeekExact(s.tw.term.Bytes(), state)

	var scorer Scorer
	if s.tw.scoreMode == TOP_SCORES {
		scorer = NewTermScorer(termsEnum.Impacts(index.PostingsEnumFreqs), s.tw.simScorer, s.context.Reader().GetNormValues(s.tw.term.Field()), TOP_SCORES)
	} else {
		flags := index.PostingsEnumNone
		if s.tw.scoreMode.NeedsScores() {
			flags = index.PostingsEnumFreqs
		}
		scorer = NewTermScorer(termsEnum.Postings(nil, flags), s.tw.simScorer, s.context.Reader().GetNormValues(s.tw.term.Field()), s.tw.scoreMode)
	}

	return scorer, nil
}

func (s *termScorerSupplier) GetMatchCost() float32 {
	state := s.stateSupplier()
	if state == nil {
		return 0
	}
	// docFreq is the cost
	return float32(state.DocFreq())
}

func (s *termScorerSupplier) GetDocCount() int {
	state := s.stateSupplier()
	if state == nil {
		return 0
	}
	return state.DocFreq()
}

func (tw *TermWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	scorer, err := tw.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return NewExplanation("no matching term", 0), nil
	}

	termScorer := scorer.(*TermScorer)
	if termScorer.Advance(doc) != doc {
		return NewExplanation("no matching term", 0), nil
	}

	freq := termScorer.Freq()
	var norm int64 = 1
	norms := context.Reader().GetNormValues(tw.term.Field())
	if norms != nil && norms.AdvanceExact(doc) {
		norm = norms.LongValue()
	}

	freqExp := NewExplanation(fmt.Sprintf("freq, occurrences of term within document: %d", freq), float32(freq))
	scoreExp := tw.simScorer.Explain(freqExp, norm)

	return NewExplanation(
		fmt.Sprintf("weight(%s in %d) [%T], result of:", tw.BaseWeight.GetQuery(), doc, tw.similarity),
		scoreExp.Value,
	), nil
}

func (tw *TermWeight) Count(context *index.LeafReaderContext) (int, error) {
	if !context.Reader().HasDeletions() {
		termsEnum := tw.getTermsEnum(context)
		if termsEnum != nil {
			return termsEnum.DocFreq(), nil
		}
		return 0, nil
	}
	return tw.BaseWeight.Count(context)
}

func (tw *TermWeight) getTermsEnum(context *index.LeafReaderContext) index.TermsEnum {
	if tw.termStates == nil {
		return nil
	}
	supplier := tw.termStates.Get(context)
	if supplier == nil {
		return nil
	}
	state := supplier()
	if state == nil {
		return nil
	}
	termsEnum := context.Reader().Terms(tw.term.Field()).Iterator()
	termsEnum.SeekExact(tw.term.Bytes(), state)
	return termsEnum
}

var _ Weight = (*TermWeight)(nil)
