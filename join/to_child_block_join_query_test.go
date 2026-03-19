package join

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// MockBitSetProducer is a mock BitSetProducer for testing
type MockBitSetProducer struct {
	bitSet *FixedBitSet
}

func NewMockBitSetProducer(size int) *MockBitSetProducer {
	return &MockBitSetProducer{
		bitSet: NewFixedBitSet(size),
	}
}

func (p *MockBitSetProducer) GetBitSet(context *index.LeafReaderContext) (*FixedBitSet, error) {
	return p.bitSet, nil
}

func TestNewToChildBlockJoinQuery(t *testing.T) {
	parentQuery := &MockQuery{}
	parentsFilter := NewMockBitSetProducer(100)

	q := NewToChildBlockJoinQuery(parentQuery, parentsFilter, Max)

	if q == nil {
		t.Fatal("Expected ToChildBlockJoinQuery to be created")
	}

	if q.GetParentQuery() != parentQuery {
		t.Error("Expected parent query to match")
	}

	if q.GetParentsFilter() != parentsFilter {
		t.Error("Expected parents filter to match")
	}

	if q.GetScoreMode() != Max {
		t.Errorf("Expected score mode Max, got %v", q.GetScoreMode())
	}
}

func TestToChildBlockJoinQueryClone(t *testing.T) {
	parentQuery := &MockQuery{}
	parentsFilter := NewMockBitSetProducer(100)

	q := NewToChildBlockJoinQuery(parentQuery, parentsFilter, Avg)
	cloned := q.Clone()

	if cloned == nil {
		t.Fatal("Expected Clone to return non-nil")
	}

	clonedQ, ok := cloned.(*ToChildBlockJoinQuery)
	if !ok {
		t.Fatal("Expected Clone to return *ToChildBlockJoinQuery")
	}

	if clonedQ.GetParentQuery() == nil {
		t.Error("Expected cloned query to have parent query")
	}

	if clonedQ.GetScoreMode() != Avg {
		t.Error("Expected cloned query to have same score mode")
	}
}

func TestToChildBlockJoinQueryEquals(t *testing.T) {
	parentQuery := &MockQuery{}
	parentsFilter := NewMockBitSetProducer(100)

	q1 := NewToChildBlockJoinQuery(parentQuery, parentsFilter, Total)
	q2 := NewToChildBlockJoinQuery(parentQuery, parentsFilter, Total)

	if !q1.Equals(q2) {
		t.Error("Expected Equals to return true for identical queries")
	}

	// Different score mode
	q3 := NewToChildBlockJoinQuery(parentQuery, parentsFilter, Max)
	if q1.Equals(q3) {
		t.Error("Expected Equals to return false for different score mode")
	}

	// Different type
	if q1.Equals(&MockQuery{}) {
		t.Error("Expected Equals to return false for different type")
	}
}

func TestToChildBlockJoinQueryHashCode(t *testing.T) {
	parentQuery := &MockQuery{}
	parentsFilter := NewMockBitSetProducer(100)

	q := NewToChildBlockJoinQuery(parentQuery, parentsFilter, None)
	hash := q.HashCode()

	if hash == 0 {
		t.Error("Expected non-zero HashCode")
	}
}

func TestToChildBlockJoinQueryString(t *testing.T) {
	parentQuery := &MockQuery{}
	parentsFilter := NewMockBitSetProducer(100)

	q := NewToChildBlockJoinQuery(parentQuery, parentsFilter, Avg)
	str := q.String()

	if str == "" {
		t.Error("Expected non-empty String representation")
	}
}

func TestToChildBlockJoinWeightCreation(t *testing.T) {
	parentQuery := &MockQuery{}
	parentsFilter := NewMockBitSetProducer(100)

	q := NewToChildBlockJoinQuery(parentQuery, parentsFilter, Max)
	w := NewToChildBlockJoinWeight(q, nil, parentsFilter, Max, 1.0)

	if w == nil {
		t.Fatal("Expected ToChildBlockJoinWeight to be created")
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

func TestToChildBlockJoinScorerCreation(t *testing.T) {
	parentsBits := NewFixedBitSet(100)
	// Mark some documents as parents (every 10th document)
	for i := 9; i < 100; i += 10 {
		parentsBits.Set(i)
	}

	// Create a mock scorer (we can't easily create a real one without full index)
	// This tests the struct creation
	weight := &ToChildBlockJoinWeight{}
	scorer := NewToChildBlockJoinScorer(weight, nil, parentsBits, Max, 1.0)

	if scorer == nil {
		t.Fatal("Expected ToChildBlockJoinScorer to be created")
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

func TestToChildBlockJoinScorerFindPreviousParent(t *testing.T) {
	parentsBits := NewFixedBitSet(100)
	// Mark documents 9, 19, 29, ... as parents
	for i := 9; i < 100; i += 10 {
		parentsBits.Set(i)
	}

	weight := &ToChildBlockJoinWeight{}
	scorer := NewToChildBlockJoinScorer(weight, nil, parentsBits, Max, 1.0)

	// Test finding previous parent
	prev := scorer.findPreviousParent(25)
	if prev != 19 {
		t.Errorf("Expected previous parent of 25 to be 19, got %d", prev)
	}

	// Test finding previous parent at boundary
	prev = scorer.findPreviousParent(10)
	if prev != 9 {
		t.Errorf("Expected previous parent of 10 to be 9, got %d", prev)
	}

	// Test finding previous parent with no parents before
	prev = scorer.findPreviousParent(5)
	if prev != -1 {
		t.Errorf("Expected previous parent of 5 to be -1, got %d", prev)
	}
}

func TestToChildBlockJoinScorerScore(t *testing.T) {
	parentsBits := NewFixedBitSet(100)

	weight := &ToChildBlockJoinWeight{}

	// Test with score mode None
	scorerNone := NewToChildBlockJoinScorer(weight, nil, parentsBits, None, 1.0)
	if scorerNone.scoreMode != None {
		t.Error("Expected score mode None")
	}

	// Test with other score modes
	scorerMax := NewToChildBlockJoinScorer(weight, nil, parentsBits, Max, 1.0)
	if scorerMax.scoreMode != Max {
		t.Error("Expected score mode Max")
	}
}
