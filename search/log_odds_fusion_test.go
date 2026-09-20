package search

import (
	"github.com/FlavioCFOliveira/Gocene/util"
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/stretchr/testify/assert"
)

// mockScorer is a simple scorer for testing.
type mockScorer struct {
	docIDVal int
	scoreVal float32
	maxScore float32
}

func (m *mockScorer) NextDoc() (int, error) {
	return m.docIDVal, nil
}

func (m *mockScorer) Score() float32 {
	return m.scoreVal
}

func (m *mockScorer) DocID() int {
	return m.docIDVal
}

func (m *mockScorer) Iterator() DocIdSetIterator {
	return &mockIterator{docID: m.docIDVal}
}

func (m *mockScorer) Advance(target int) (int, error) {
	if m.docIDVal >= target {
		return m.docIDVal, nil
	}
	return NO_MORE_DOCS, nil
}

func (m *mockScorer) GetMaxScore(upTo int) float32 {
	if m.docIDVal <= upTo {
		return m.maxScore
	}
	return 0
}

type mockIterator struct {
	docID int
}

func (i *mockIterator) NextDoc() (int, error) {
	return i.docID, nil
}

func (i *mockIterator) Advance(target int) (int, error) {
	if i.docID >= target {
		return i.docID, nil
	}
	return NO_MORE_DOCS, nil
}

func (i *mockIterator) DocID() int {
	return i.docID
}

func (i *mockIterator) Cost() int64 {
	return 1
}

func (i *mockIterator) DocIDRunEnd() (int, error) {
	return -1, nil
}

func TestLogOddsFusionNumericalUtils(t *testing.T) {
	t.Run("clampProbability", func(t *testing.T) {
		assert.Equal(t, float32(1e-7), clampProbability(0))
		assert.Equal(t, float32(1e-7), clampProbability(-1.0))
		assert.Equal(t, float32(1.0-1e-7), clampProbability(1.0))
		assert.Equal(t, float32(1.0-1e-7), clampProbability(2.0))
		assert.Equal(t, float32(0.5), clampProbability(0.5))
	})

	t.Run("logit", func(t *testing.T) {
		// logit(0.5) = log(0.5/0.5) = log(1) = 0
		assert.InDelta(t, 0.0, logit(0.5), 1e-6)
		// logit(0.75) = log(0.75/0.25) = log(3) approx 1.0986
		assert.InDelta(t, 1.0986, logit(0.75), 1e-4)
	})

	t.Run("sigmoid", func(t *testing.T) {
		// sigmoid(0) = 1/(1+1) = 0.5
		assert.InDelta(t, 0.5, logOddsFusionScorerSigmoid(0), 1e-6)
		// sigmoid(1.0986) approx 0.75
		assert.InDelta(t, 0.75, logOddsFusionScorerSigmoid(1.0986), 1e-4)
	})

	t.Run("softplus", func(t *testing.T) {
		// softplus(0) = log(1+exp(0)) = log(2) approx 0.6931
		assert.InDelta(t, 0.6931, softplus(0), 1e-4)
		// softplus(21) approx 21
		assert.InDelta(t, 21.0, softplus(21.0), 1e-6)
	})
}

func TestLogOddsFusionScorer_Score(t *testing.T) {
	subScorers := []Scorer{
		&mockScorer{docIDVal: 1, scoreVal: 0.8, maxScore: 0.8},
		&mockScorer{docIDVal: 1, scoreVal: 0.6, maxScore: 0.6},
	}

	// alpha = 0.5, uniform weights
	scorer := NewLogOddsFusionScorer(subScorers, 2, 0.5, nil, nil, nil, COMPLETE, 1)

	// Mock a topList by creating a DisiWrapper chain
	w1 := &DisiWrapper{scorer: subScorers[0], doc: 1}
	w2 := &DisiWrapper{scorer: subScorers[1], doc: 1}
	w1.next = w2

	score := scorer.scoreTopList(w1)

	// Calculation:
	// logit(0.8) = log(0.8/0.2) = log(4) = 1.3863
	// logit(0.6) = log(0.6/0.4) = log(1.5) = 0.4055
	// softplus(1.3863) = log(1 + exp(1.3863)) = log(1 + 4) = log(5) = 1.6094
	// softplus(0.4055) = log(1 + exp(0.4055)) = log(1 + 1.5) = log(2.5) = 0.9163
	// meanLogit = (1.6094 + 0.9163) / 2 = 2.5257 / 2 = 1.2628
	// scalingFactor = 2^0.5 = 1.4142
	// scaledLogit = 1.2628 * 1.4142 = 1.7859
	// score = sigmoid(1.7859) = 1 / (1 + exp(-1.7859)) = 1 / (1 + 0.1676) = 1 / 1.1676 = 0.8565

	assert.InDelta(t, 0.8565, score, 1e-4)
}

func TestLogOddsFusionScorer_WeightedScore(t *testing.T) {
	subScorers := []Scorer{
		&mockScorer{docIDVal: 1, scoreVal: 0.8, maxScore: 0.8},
		&mockScorer{docIDVal: 1, scoreVal: 0.6, maxScore: 0.6},
	}

	weights := []float32{0.7, 0.3}
	scorer := NewLogOddsFusionScorer(subScorers, 2, 0.5, weights, nil, nil, COMPLETE, 1)

	w1 := &DisiWrapper{scorer: subScorers[0], doc: 1}
	w2 := &DisiWrapper{scorer: subScorers[1], doc: 1}
	w1.next = w2

	score := scorer.scoreTopList(w1)

	// Calculation:
	// gated1 = 1.6094, gated2 = 0.9163
	// weightedSum = 0.7 * 1.6094 + 0.3 * 0.9163 = 1.1266 + 0.2749 = 1.4015
	// scalingFactor = 1.4142
	// scaledLogit = 1.4015 * 1.4142 = 1.9820
	// score = sigmoid(1.9820) = 1 / (1 + exp(-1.9820)) = 1 / (1 + 0.1378) = 0.8788

	assert.InDelta(t, 0.8788, score, 1e-4)
}

func TestLogOddsFusionScorer_NormalizedScore(t *testing.T) {
	subScorers := []Scorer{
		&mockScorer{docIDVal: 1, scoreVal: 0.8, maxScore: 0.8},
		&mockScorer{docIDVal: 1, scoreVal: 0.6, maxScore: 0.6},
	}

	logitMin := []float32{0, 0}
	logitMax := []float32{2.0, 2.0}
	scorer := NewLogOddsFusionScorer(subScorers, 2, 0.5, nil, logitMin, logitMax, COMPLETE, 1)

	w1 := &DisiWrapper{scorer: subScorers[0], doc: 1}
	w2 := &DisiWrapper{scorer: subScorers[1], doc: 1}
	w1.next = w2

	score := scorer.scoreTopList(w1)

	// Calculation:
	// logit(0.8) = 1.3863, logit(0.6) = 0.4055
	// gated1 = clamp((1.3863 - 0) / 2, 0, 1) = 0.6932
	// gated2 = clamp((0.4055 - 0) / 2, 0, 1) = 0.2028
	// meanLogit = (0.6932 + 0.2028) / 2 = 0.8960 / 2 = 0.4480
	// scalingFactor = 1.4142
	// scaledLogit = 0.4480 * 1.4142 = 0.6336
	// score = sigmoid(0.6336) = 1 / (1 + exp(-0.6336)) = 1 / (1 + 0.5306) = 0.6533

	assert.InDelta(t, 0.6533, score, 1e-4)
}

func TestLogOddsFusionQuery_Rewrite(t *testing.T) {
	q1 := &mockQuery{}
	q2 := &mockQuery{}
	q3 := &mockQuery{}

	clauses := []Query{q1, q2, q3}
	q, _ := NewLogOddsFusionQuery(clauses, 0.5, nil, nil, nil)

	// Mock a searcher
	searcher := &mockSearcher{}

	// case 1: no changes
	rewritten, _ := q.Rewrite(searcher)
	assert.Equal(t, q, rewritten)
}

type mockQuery struct {
	rewriteTo Query
}

func (m *mockQuery) CreateWeight(s *IndexSearcher, sm ScoreMode, b float32) (Weight, error) {
	return &mockWeight{}, nil
}

func (m *mockQuery) Rewrite(s *IndexSearcher) (Query, error) {
	if m.rewriteTo != nil {
		return m.rewriteTo, nil
	}
	return m, nil
}

func (m *mockQuery) Visit(v QueryVisitor)     {}
func (m *mockQuery) ToString(f string) string { return "mock" }

type mockWeight struct{}

func (m *mockWeight) Matches(ctx *index.LeafReaderContext, doc int) (Matches, error) {
	return nil, nil
}

func (m *mockWeight) ScorerSupplier(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
	return &mockScorerSupplier{}, nil
}

func (m *mockWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return true
}

func (m *mockWeight) Explain(ctx *index.LeafReaderContext, doc int) (Explanation, error) {
	return nil, nil
}

type mockScorerSupplier struct{}

func (m *mockScorerSupplier) Get(leadCost int64) (Scorer, error) {
	return &mockScorer{docIDVal: 1}, nil
}

func (m *mockScorerSupplier) Cost() int64 {
	return 1
}

func (m *mockScorerSupplier) SetTopLevelScoringClause() {}

type mockSearcher struct {
	*IndexSearcher
}

func (m *mockSearcher) CreateWeight(q Query, sm ScoreMode, b float32) (Weight, error) {
	return q.CreateWeight(m, sm, b)
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene
// 10.5.0, which every subclass inherits unless it overrides it.
func (i *mockIterator) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(i, upTo, bitSet, offset)
}
