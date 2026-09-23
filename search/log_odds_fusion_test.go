package search

import (
	"errors"
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
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

func (m *mockScorer) Score() (float32, error) {
	return m.scoreVal, nil
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

func (m *mockScorer) GetMaxScore(upTo int) (float32, error) {
	if m.docIDVal <= upTo {
		return m.maxScore, nil
	}
	return 0, nil
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
	w1 := NewDisiWrapper(subScorers[0], false)
	w1.doc = 1
	w2 := NewDisiWrapper(subScorers[1], false)
	w2.doc = 1
	w1.next = w2

	score, err := scorer.scoreTopList(w1)
	if err != nil {
		t.Fatalf("scoreTopList: %v", err)
	}

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

	w1 := NewDisiWrapper(subScorers[0], false)
	w1.doc = 1
	w2 := NewDisiWrapper(subScorers[1], false)
	w2.doc = 1
	w1.next = w2

	score, err := scorer.scoreTopList(w1)
	if err != nil {
		t.Fatalf("scoreTopList: %v", err)
	}

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

	w1 := NewDisiWrapper(subScorers[0], false)
	w1.doc = 1
	w2 := NewDisiWrapper(subScorers[1], false)
	w2.doc = 1
	w1.next = w2

	score, err := scorer.scoreTopList(w1)
	if err != nil {
		t.Fatalf("scoreTopList: %v", err)
	}

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
	q1 := &logOddsMockQuery{}
	q2 := &logOddsMockQuery{}
	q3 := &logOddsMockQuery{}

	clauses := []Query{q1, q2, q3}
	q, _ := NewLogOddsFusionQuery(clauses, 0.5, nil, nil, nil)

	// Mock a searcher
	searcher := &mockSearcher{IndexSearcher: NewIndexSearcher(newEmptyMultiReader(t))}

	// case 1: no changes
	rewritten, _ := q.Rewrite(searcher.IndexSearcher)
	assert.Equal(t, q, rewritten)
}

type logOddsMockQuery struct {
	rewriteTo Query
}

func (m *logOddsMockQuery) CreateWeight(s *IndexSearcher, sm ScoreMode, b float32) (Weight, error) {
	return &mockWeight{query: m}, nil
}

func (m *logOddsMockQuery) Rewrite(s *IndexSearcher) (Query, error) {
	if m.rewriteTo != nil {
		return m.rewriteTo, nil
	}
	return m, nil
}

// Equals is abstract in Lucene's Query; this double compares by identity.
func (m *logOddsMockQuery) Equals(other spi.Query) bool {
	o, ok := other.(*logOddsMockQuery)
	return ok && o == m
}

// HashCode is abstract in Lucene's Query; identity equality admits a
// constant hash.
func (m *logOddsMockQuery) HashCode() int {
	return 31
}

func (m *logOddsMockQuery) Visit(v QueryVisitor)     {}
func (m *logOddsMockQuery) ToString(f string) string { return "mock" }

type mockWeight struct {
	query Query
}

// GetQuery returns the parent query, as Lucene's Weight.getQuery() does.
func (m *mockWeight) GetQuery() Query {
	return m.query
}

// Scorer carries the default body Lucene gives Weight.scorer: the scorer
// supplier's scorer for a lead cost of Long.MAX_VALUE.
func (m *mockWeight) Scorer(ctx *index.LeafReaderContext) (Scorer, error) {
	supplier, err := m.ScorerSupplier(ctx)
	if err != nil || supplier == nil {
		return nil, err
	}
	return supplier.Get(math.MaxInt64)
}

// BulkScorer carries the default body Lucene gives Weight.bulkScorer: the
// scorer supplier's bulk scorer.
func (m *mockWeight) BulkScorer(ctx *index.LeafReaderContext) (BulkScorer, error) {
	supplier, err := m.ScorerSupplier(ctx)
	if err != nil || supplier == nil {
		return nil, err
	}
	return supplier.BulkScorer()
}

// Count carries the default body Lucene gives Weight.count: -1.
func (m *mockWeight) Count(ctx *index.LeafReaderContext) (int, error) {
	return -1, nil
}

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

func (m *mockScorerSupplier) SetTopLevelScoringClause() error { return nil }

// AdvanceShallow carries the default body Lucene gives Scorer.AdvanceShallow.
func (m *mockScorer) AdvanceShallow(target int) (int, error) {
	return DefaultAdvanceShallow(target)
}

// GetChildren carries the default body Lucene gives Scorer.GetChildren.
func (m *mockScorer) GetChildren() ([]ChildScorable, error) {
	return nil, nil
}

// NextDocsAndScores carries the default body Lucene gives Scorer.NextDocsAndScores.
func (m *mockScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *DocAndFloatFeatureBuffer) error {
	return DefaultNextDocsAndScores(m, upTo, liveDocs, buffer)
}

// SetMinCompetitiveScore carries the default body Lucene gives Scorer.SetMinCompetitiveScore.
func (m *mockScorer) SetMinCompetitiveScore(minScore float32) error {
	return nil
}

// SmoothingScore carries the default body Lucene gives Scorer.SmoothingScore.
func (m *mockScorer) SmoothingScore(docID int) (float32, error) {
	return 0, nil
}

// TwoPhaseIterator carries the default body Lucene gives Scorer.TwoPhaseIterator.
func (m *mockScorer) TwoPhaseIterator() *TwoPhaseIterator {
	return DefaultTwoPhaseIterator()
}

// BulkScorer is abstract in Lucene's ScorerSupplier; this double does not support it.
func (m *mockScorerSupplier) BulkScorer() (BulkScorer, error) {
	return nil, errors.New("mockScorerSupplier.BulkScorer: unsupported operation")
}

type mockSearcher struct {
	*IndexSearcher
}

func (m *mockSearcher) CreateWeight(q Query, sm ScoreMode, b float32) (Weight, error) {
	return q.CreateWeight(m.IndexSearcher, sm, b)
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene
// 10.5.0, which every subclass inherits unless it overrides it.
func (i *mockIterator) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(i, upTo, bitSet, offset)
}
