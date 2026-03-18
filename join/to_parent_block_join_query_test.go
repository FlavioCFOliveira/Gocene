package join

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// MockQuery is a simple mock query for testing
type MockQuery struct{}

func (q *MockQuery) Rewrite(reader search.IndexReader) (search.Query, error) { return q, nil }
func (q *MockQuery) Clone() search.Query                                     { return &MockQuery{} }
func (q *MockQuery) Equals(other search.Query) bool                          { _, ok := other.(*MockQuery); return ok }
func (q *MockQuery) HashCode() int                                           { return 42 }
func (q *MockQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	return nil, nil
}

func TestNewToParentBlockJoinQuery(t *testing.T) {
	childQuery := &MockQuery{}
	parentsFilter := NewMockBitSetProducer(100)

	q := NewToParentBlockJoinQuery(childQuery, parentsFilter, Max)

	if q == nil {
		t.Fatal("Expected ToParentBlockJoinQuery to be created")
	}

	if q.GetChildQuery() != childQuery {
		t.Error("Expected child query to match")
	}

	if q.GetParentsFilter() != parentsFilter {
		t.Error("Expected parents filter to match")
	}

	if q.GetScoreMode() != Max {
		t.Errorf("Expected score mode Max, got %v", q.GetScoreMode())
	}
}

func TestToParentBlockJoinQueryClone(t *testing.T) {
	childQuery := &MockQuery{}
	parentsFilter := NewMockBitSetProducer(100)

	q := NewToParentBlockJoinQuery(childQuery, parentsFilter, Avg)
	cloned := q.Clone()

	if cloned == nil {
		t.Fatal("Expected Clone to return non-nil")
	}

	clonedQ, ok := cloned.(*ToParentBlockJoinQuery)
	if !ok {
		t.Fatal("Expected Clone to return *ToParentBlockJoinQuery")
	}

	if clonedQ.GetChildQuery() == nil {
		t.Error("Expected cloned query to have child query")
	}

	if clonedQ.GetScoreMode() != Avg {
		t.Error("Expected cloned query to have same score mode")
	}
}

func TestToParentBlockJoinQueryEquals(t *testing.T) {
	childQuery := &MockQuery{}
	parentsFilter := NewMockBitSetProducer(100)

	q1 := NewToParentBlockJoinQuery(childQuery, parentsFilter, Total)
	q2 := NewToParentBlockJoinQuery(childQuery, parentsFilter, Total)

	if !q1.Equals(q2) {
		t.Error("Expected Equals to return true for identical queries")
	}

	// Different score mode
	q3 := NewToParentBlockJoinQuery(childQuery, parentsFilter, Max)
	if q1.Equals(q3) {
		t.Error("Expected Equals to return false for different score mode")
	}

	// Different type
	if q1.Equals(&MockQuery{}) {
		t.Error("Expected Equals to return false for different type")
	}
}

func TestToParentBlockJoinQueryHashCode(t *testing.T) {
	childQuery := &MockQuery{}
	parentsFilter := NewMockBitSetProducer(100)

	q := NewToParentBlockJoinQuery(childQuery, parentsFilter, None)
	hash := q.HashCode()

	if hash == 0 {
		t.Error("Expected non-zero HashCode")
	}
}

func TestToParentBlockJoinQueryString(t *testing.T) {
	childQuery := &MockQuery{}
	parentsFilter := NewMockBitSetProducer(100)

	q := NewToParentBlockJoinQuery(childQuery, parentsFilter, Avg)
	str := q.String()

	if str == "" {
		t.Error("Expected non-empty String representation")
	}
}

func TestToParentBlockJoinWeightCreation(t *testing.T) {
	childQuery := &MockQuery{}
	parentsFilter := NewMockBitSetProducer(100)

	q := NewToParentBlockJoinQuery(childQuery, parentsFilter, Max)
	w := NewToParentBlockJoinWeight(q, nil, parentsFilter, Max, 1.0)

	if w == nil {
		t.Fatal("Expected ToParentBlockJoinWeight to be created")
	}

	if w.GetScoreMode() != Max {
		t.Errorf("Expected score mode Max, got %v", w.GetScoreMode())
	}

	if w.GetQuery() != q {
		t.Error("Expected query to match")
	}

	if w.GetParentsFilter() != parentsFilter {
		t.Error("Expected parents filter to match")
	}
}

func TestToParentBlockJoinScorerCreation(t *testing.T) {
	parentsBits := NewFixedBitSet(100)
	// Mark some documents as parents (every 10th document)
	for i := 9; i < 100; i += 10 {
		parentsBits.Set(i)
	}

	weight := &ToParentBlockJoinWeight{}
	scorer := NewToParentBlockJoinScorer(weight, nil, parentsBits, Max, 1.0)

	if scorer == nil {
		t.Fatal("Expected ToParentBlockJoinScorer to be created")
	}

	if scorer.scoreMode != Max {
		t.Errorf("Expected score mode Max, got %v", scorer.scoreMode)
	}

	if scorer.boost != 1.0 {
		t.Errorf("Expected boost 1.0, got %f", scorer.boost)
	}

	if scorer.parentsBits != parentsBits {
		t.Error("Expected parents bits to match")
	}
}

func TestToParentBlockJoinScorerFindParent(t *testing.T) {
	parentsBits := NewFixedBitSet(100)
	// Mark documents 9, 19, 29, ... as parents
	for i := 9; i < 100; i += 10 {
		parentsBits.Set(i)
	}

	weight := &ToParentBlockJoinWeight{}
	scorer := NewToParentBlockJoinScorer(weight, nil, parentsBits, Max, 1.0)

	// Test finding parent
	parent := scorer.findParent(5)
	if parent != 9 {
		t.Errorf("Expected parent of 5 to be 9, got %d", parent)
	}

	// Test finding parent after parent
	parent = scorer.findParent(15)
	if parent != 19 {
		t.Errorf("Expected parent of 15 to be 19, got %d", parent)
	}

	// Test finding parent at exact position
	parent = scorer.findParent(9)
	if parent != 9 {
		t.Errorf("Expected parent of 9 to be 9, got %d", parent)
	}

	// Test finding parent beyond last - should find the last parent
	parent = scorer.findParent(95)
	if parent != 99 {
		t.Errorf("Expected parent of 95 to be 99 (last parent), got %d", parent)
	}
}

func TestToParentBlockJoinScorerAccumulateScore(t *testing.T) {
	parentsBits := NewFixedBitSet(100)

	weight := &ToParentBlockJoinWeight{}

	// Test with Avg score mode
	scorerAvg := NewToParentBlockJoinScorer(weight, nil, parentsBits, Avg, 1.0)
	scorerAvg.accumulateScore(10.0)
	scorerAvg.accumulateScore(20.0)
	if scorerAvg.accumulatedScore != 30.0 {
		t.Errorf("Expected accumulated score 30.0 for Avg, got %f", scorerAvg.accumulatedScore)
	}
	if scorerAvg.childCount != 2 {
		t.Errorf("Expected child count 2, got %d", scorerAvg.childCount)
	}

	// Test with Max score mode
	scorerMax := NewToParentBlockJoinScorer(weight, nil, parentsBits, Max, 1.0)
	scorerMax.accumulateScore(10.0)
	scorerMax.accumulateScore(20.0)
	if scorerMax.accumulatedScore != 20.0 {
		t.Errorf("Expected accumulated score 20.0 for Max, got %f", scorerMax.accumulatedScore)
	}

	// Test with Min score mode
	scorerMin := NewToParentBlockJoinScorer(weight, nil, parentsBits, Min, 1.0)
	scorerMin.accumulateScore(20.0)
	scorerMin.accumulateScore(10.0)
	if scorerMin.accumulatedScore != 10.0 {
		t.Errorf("Expected accumulated score 10.0 for Min, got %f", scorerMin.accumulatedScore)
	}
}

func TestToParentBlockJoinScorerScore(t *testing.T) {
	parentsBits := NewFixedBitSet(100)

	weight := &ToParentBlockJoinWeight{}

	// Test with score mode None
	scorerNone := NewToParentBlockJoinScorer(weight, nil, parentsBits, None, 1.0)
	if scorerNone.scoreMode != None {
		t.Error("Expected score mode None")
	}

	// Test with Avg score mode
	scorerAvg := NewToParentBlockJoinScorer(weight, nil, parentsBits, Avg, 1.0)
	scorerAvg.accumulatedScore = 30.0
	scorerAvg.childCount = 2
	score := scorerAvg.Score()
	if score != 15.0 {
		t.Errorf("Expected score 15.0 for Avg mode, got %f", score)
	}

	// Test with boost
	scorerAvg2 := NewToParentBlockJoinScorer(weight, nil, parentsBits, Avg, 2.0)
	scorerAvg2.accumulatedScore = 30.0
	scorerAvg2.childCount = 2
	score = scorerAvg2.Score()
	if score != 30.0 {
		t.Errorf("Expected score 30.0 for Avg mode with boost 2.0, got %f", score)
	}
}
