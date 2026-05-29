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

	// Max is a scoring mode, so doScores must be true.
	if !scorer.doScores {
		t.Error("Expected doScores true for ScoreMode Max")
	}

	if scorer.boost != 1.0 {
		t.Errorf("Expected boost 1.0, got %f", scorer.boost)
	}

	if scorer.parentBits != parentsBits {
		t.Error("Expected parent bits to match")
	}
}

// TestToChildBlockJoinScorerPrevSetBit verifies the FixedBitSet.PrevSetBit
// primitive the scorer now uses to locate the start of a child block (replacing
// the former O(n) findPreviousParent helper). The first child of a block ending
// at parent doc p is parentBits.PrevSetBit(p-1)+1.
func TestToChildBlockJoinScorerPrevSetBit(t *testing.T) {
	parentsBits := NewFixedBitSet(100)
	// Mark documents 9, 19, 29, ... as parents
	for i := 9; i < 100; i += 10 {
		parentsBits.Set(i)
	}

	// Previous parent strictly before doc 25 is 19.
	if prev := parentsBits.PrevSetBit(24); prev != 19 {
		t.Errorf("PrevSetBit(24) = %d, want 19", prev)
	}
	// Previous parent strictly before doc 10 is 9.
	if prev := parentsBits.PrevSetBit(9); prev != 9 {
		t.Errorf("PrevSetBit(9) = %d, want 9", prev)
	}
	// No parent at or before doc 4.
	if prev := parentsBits.PrevSetBit(4); prev != -1 {
		t.Errorf("PrevSetBit(4) = %d, want -1", prev)
	}
}

func TestToChildBlockJoinScorerScore(t *testing.T) {
	parentsBits := NewFixedBitSet(100)

	weight := &ToChildBlockJoinWeight{}

	// ScoreMode None disables score propagation (doScores == false).
	scorerNone := NewToChildBlockJoinScorer(weight, nil, parentsBits, None, 1.0)
	if scorerNone.doScores {
		t.Error("Expected doScores false for ScoreMode None")
	}

	// Any other score mode propagates the parent score (doScores == true).
	scorerMax := NewToChildBlockJoinScorer(weight, nil, parentsBits, Max, 1.0)
	if !scorerMax.doScores {
		t.Error("Expected doScores true for ScoreMode Max")
	}
}
