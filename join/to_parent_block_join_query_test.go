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

	if scorer.parentBits != parentsBits {
		t.Error("Expected parent bits to match")
	}
}

// TestToParentBlockJoinScorerNextSetBit verifies the FixedBitSet.NextSetBit
// primitive used by the scorer (faithful to ParentApproximation.advance, which
// finds the parent immediately after a child via parentBits.nextSetBit(childDoc+1)).
func TestToParentBlockJoinScorerNextSetBit(t *testing.T) {
	parentsBits := NewFixedBitSet(100)
	// Mark documents 9, 19, 29, ... as parents
	for i := 9; i < 100; i += 10 {
		parentsBits.Set(i)
	}

	// Parent after child 5 is 9.
	if p := parentsBits.NextSetBit(6); p != 9 {
		t.Errorf("NextSetBit(6) = %d, want 9", p)
	}
	// Parent after child 15 is 19.
	if p := parentsBits.NextSetBit(16); p != 19 {
		t.Errorf("NextSetBit(16) = %d, want 19", p)
	}
	// Parent after child 95 is 99.
	if p := parentsBits.NextSetBit(96); p != 99 {
		t.Errorf("NextSetBit(96) = %d, want 99", p)
	}
}

// TestToParentBlockJoinScorerScore drives the real per-parent score aggregation
// through a fakeScorer child positioned before a synthetic parent. The block
// here is children {0,1,2} under parent doc 3; the scorer's Advance positions
// the child at the first child, and Score() walks the block combining child
// scores per ScoreMode (faithful to BlockJoinScorer.scoreChildDocs + Score).
func TestToParentBlockJoinScorerScore(t *testing.T) {
	parentsBits := NewFixedBitSet(8)
	parentsBits.Set(3) // single parent block: children 0,1,2 -> parent 3
	parentsBits.Set(7) // trailing parent (block-join invariant: last doc is parent)
	weight := &ToParentBlockJoinWeight{}

	// childScores: doc0=2, doc1=4, doc2=6.
	newScorer := func(mode ScoreMode, boost float32) *ToParentBlockJoinScorer {
		child := newFakeScorer([]int{0, 1, 2}, []float32{2, 4, 6}, 6)
		s := NewToParentBlockJoinScorer(weight, child, parentsBits, mode, boost)
		if _, err := s.NextDoc(); err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if s.DocID() != 3 {
			t.Fatalf("DocID = %d, want parent 3", s.DocID())
		}
		return s
	}

	cases := []struct {
		mode  ScoreMode
		boost float32
		want  float32
	}{
		{Total, 1.0, 12.0}, // 2+4+6
		{Avg, 1.0, 4.0},    // 12/3
		{Max, 1.0, 6.0},
		{Min, 1.0, 2.0},
		{Avg, 2.0, 8.0}, // (12/3) * 2
		{None, 1.0, 0.0},
	}
	for _, tc := range cases {
		s := newScorer(tc.mode, tc.boost)
		if got := s.Score(); got != tc.want {
			t.Errorf("ScoreMode %v boost %.1f: Score() = %v, want %v", tc.mode, tc.boost, got, tc.want)
		}
	}
}
