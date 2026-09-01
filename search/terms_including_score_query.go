package search

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TermsIncludingScoreQuery mirrors Lucene's org.apache.lucene.search.join.TermsIncludingScoreQuery.
type TermsIncludingScoreQuery struct {
	scoreMode                 ScoreMode
	toField                   string
	multipleValuesPerDocument bool
	terms                     *util.BytesRefHash
	scores                    []float32
	ords                      []int
	fromQuery                 Query
	fromField                 string
	topReaderContextId        interface{}
	ramBytesUsed              int64
}

func NewTermsIncludingScoreQuery(
	scoreMode ScoreMode,
	toField string,
	multipleValuesPerDocument bool,
	terms *util.BytesRefHash,
	scores []float32,
	fromField string,
	fromQuery Query,
	indexReaderContextId interface{},
) *TermsIncludingScoreQuery {
	ords := terms.Sort()

	ram := int64(util.RamUsageEstimator.ShallowSizeOfInstance(TermsIncludingScoreQuery{}))
	ram += int64(util.RamUsageEstimator.SizeOfObject(fromField))
	ram += int64(util.RamUsageEstimator.SizeOfObject(fromQuery, util.RamUsageEstimator.QueryDefaultRamBytesUsed))
	ram += int64(util.RamUsageEstimator.SizeOfObject(ords))
	ram += int64(util.RamUsageEstimator.SizeOfObject(scores))
	ram += int64(util.RamUsageEstimator.SizeOfObject(terms))
	ram += int64(util.RamUsageEstimator.SizeOfObject(toField))

	return &TermsIncludingScoreQuery{
		scoreMode:                 scoreMode,
		toField:                   toField,
		multipleValuesPerDocument: multipleValuesPerDocument,
		terms:                     terms,
		scores:                    scores,
		ords:                      ords,
		fromQuery:                 fromQuery,
		fromField:                 fromField,
		topReaderContextId:        indexReaderContextId,
		ramBytesUsed:              ram,
	}
}

func (q *TermsIncludingScoreQuery) ToString(field string) string {
	return fmt.Sprintf("TermsIncludingScoreQuery{field=%s;fromQuery=%v}", q.toField, q.fromQuery)
}

func (q *TermsIncludingScoreQuery) Visit(visitor QueryVisitor) {
	if visitor.AcceptField(q.toField) {
		visitor.VisitLeaf(q)
	}
}

func (q *TermsIncludingScoreQuery) Equals(other Query) bool {
	if other == nil {
		return false
	}
	o, ok := other.(*TermsIncludingScoreQuery)
	if !ok {
		return false
	}
	return q.scoreMode == o.scoreMode &&
		q.toField == o.toField &&
		q.fromField == o.fromField &&
		q.fromQuery == o.fromQuery &&
		q.topReaderContextId == o.topReaderContextId
}

func (q *TermsIncludingScoreQuery) HashCode() int {
	// Simple hash for demonstration
	return 0
}

func (q *TermsIncludingScoreQuery) RamBytesUsed() int64 {
	return q.ramBytesUsed
}

func (q *TermsIncludingScoreQuery) CreateWeight(searcher IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	if !scoreMode.NeedsScores() {
		// We don't need scores, quickly change to TermsQuery
		termsQuery := NewTermsQuery(q.toField, q.terms, q.fromField, q.fromQuery, q.topReaderContextId)
		rewritten, err := searcher.Rewrite(termsQuery)
		if err != nil {
			return nil, err
		}
		return rewritten.CreateWeight(searcher, ScoreModeCompleteNoScores, boost)
	}

	return &termsIncludingScoreWeight{
		query: q,
		boost: boost,
	}, nil
}

type termsIncludingScoreWeight struct {
	query *TermsIncludingScoreQuery
	boost float32
}

func (w *termsIncludingScoreWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	terms, err := context.Reader().Terms(w.query.toField)
	if err != nil || terms == nil {
		return Explanation{}, fmt.Errorf("no terms for field %s", w.query.toField)
	}

	segmentTermsEnum := terms.Iterator()
	var spare util.BytesRef
	var postingsEnum index.PostingsEnum

	for i := 0; i < w.query.terms.Size(); i++ {
		term := w.query.terms.Get(w.query.ords[i], &spare)
		if segmentTermsEnum.SeekExact(term) {
			postingsEnum = segmentTermsEnum.Postings(postingsEnum, index.PostingsEnumNone)
			if postingsEnum.Advance(doc) == doc {
				score := w.query.scores[w.query.ords[i]]
				if w.boost == 1.0 {
					return Explanation{
						Text:  fmt.Sprintf("Score based on join value %s", segmentTermsEnum.Term().UTF8ToString()),
						Value: score,
					}, nil
				}
				return Explanation{
					Text:  fmt.Sprintf("Score based on join value %s^%f", segmentTermsEnum.Term().UTF8ToString(), w.boost),
					Value: score * w.boost,
				}, nil
			}
		}
	}
	return Explanation{}, fmt.Errorf("not a match")
}

func (w *termsIncludingScoreWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	terms, err := context.Reader().Terms(w.query.toField)
	if err != nil || terms == nil {
		return nil, nil
	}

	cost := int64(context.Reader().MaxDoc()) * int64(w.query.terms.Size())
	segmentTermsEnum := terms.Iterator()

	var scorer Scorer
	if w.query.multipleValuesPerDocument {
		scorer = newMVInOrderScorer(segmentTermsEnum, context.Reader().MaxDoc(), cost, w.boost, w.query)
	} else {
		scorer = newSVInOrderScorer(segmentTermsEnum, context.Reader().MaxDoc(), cost, w.boost, w.query)
	}
	return NewDefaultScorerSupplier(scorer), nil
}

func (w *termsIncludingScoreWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return true
}

type svInOrderScorer struct {
	matchingDocsIterator DocIdSetIterator
	scores               []float32
	cost                 int64
	boost                float32
}

func newSVInOrderScorer(termsEnum index.TermsEnum, maxDoc int, cost int64, boost float32, q *TermsIncludingScoreQuery) *svInOrderScorer {
	matchingDocs := util.NewBitSet(maxDoc)
	scores := make([]float32, maxDoc)

	var spare util.BytesRef
	var postingsEnum index.PostingsEnum
	for i := 0; i < q.terms.Size(); i++ {
		term := q.terms.Get(q.ords[i], &spare)
		if termsEnum.SeekExact(term) {
			postingsEnum = termsEnum.Postings(postingsEnum, index.PostingsEnumNone)
			score := q.scores[q.ords[i]]
			for doc := postingsEnum.NextDoc(); doc != NO_MORE_DOCS; doc = postingsEnum.NextDoc() {
				matchingDocs.Set(doc)
				scores[doc] = score
			}
		}
	}

	return &svInOrderScorer{
		matchingDocsIterator: util.NewBitSetIterator(matchingDocs, cost),
		scores:               scores,
		cost:                 cost,
		boost:                boost,
	}
}

func (s *svInOrderScorer) Score() (float32, error) {
	return s.scores[s.DocID()] * s.boost, nil
}

func (s *svInOrderScorer) GetMaxScore(upTo int) (float32, error) {
	return 3.402823466e+38, nil
}

func (s *svInOrderScorer) DocID() int {
	return s.matchingDocsIterator.DocID()
}

func (s *svInOrderScorer) Iterator() DocIdSetIterator {
	return s.matchingDocsIterator
}

func (s *svInOrderScorer) TwoPhaseIterator() TwoPhaseIterator {
	return nil
}

func (s *svInOrderScorer) AdvanceShallow(target int) (int, error) {
	return DefaultAdvanceShallow(target)
}

func (s *svInOrderScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *DocAndFloatFeatureBuffer) error {
	return DefaultNextDocsAndScores(s, upTo, liveDocs, buffer)
}

type mvInOrderScorer struct {
	*svInOrderScorer
}

func newMVInOrderScorer(termsEnum index.TermsEnum, maxDoc int, cost int64, boost float32, q *TermsIncludingScoreQuery) *mvInOrderScorer {
	matchingDocs := util.NewBitSet(maxDoc)
	scores := make([]float32, maxDoc)

	var spare util.BytesRef
	var postingsEnum index.PostingsEnum
	for i := 0; i < q.terms.Size(); i++ {
		term := q.terms.Get(q.ords[i], &spare)
		if termsEnum.SeekExact(term) {
			postingsEnum = termsEnum.Postings(postingsEnum, index.PostingsEnumNone)
			score := q.scores[q.ords[i]]
			for doc := postingsEnum.NextDoc(); doc != NO_MORE_DOCS; doc = postingsEnum.NextDoc() {
				if !matchingDocs.Get(doc) {
					scores[doc] = score
				}
				matchingDocs.Set(doc)
			}
		}
	}

	return &mvInOrderScorer{
		svInOrderScorer: &svInOrderScorer{
			matchingDocsIterator: util.NewBitSetIterator(matchingDocs, cost),
			scores:               scores,
			cost:                 cost,
			boost:                boost,
		},
	}
}
