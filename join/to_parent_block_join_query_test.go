package join

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// MockQuery is a simple mock query for testing
type MockQuery struct{}

func (q *MockQuery) Rewrite(reader search.IndexReader) (search.Query, error) { return q, nil }
func (q *MockQuery) Clone() search.Query                                      { return &MockQuery{} }
func (q *MockQuery) Equals(other search.Query) bool                          { _, ok := other.(*MockQuery); return ok }
func (q *MockQuery) HashCode() int                                           { return 42 }
func (q *MockQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	return nil, nil
}

func TestNewToParentBlockJoinQuery(t *testing.T) {
	childQuery := &MockQuery{}
	parentFilter := &MockQuery{}

	q := NewToParentBlockJoinQuery(childQuery, parentFilter, Max)

	if q == nil {
		t.Fatal("Expected ToParentBlockJoinQuery to be created")
	}

	if q.GetChildQuery() != childQuery {
		t.Error("Expected child query to match")
	}

	if q.GetParentFilter() != parentFilter {
		t.Error("Expected parent filter to match")
	}

	if q.GetScoreMode() != Max {
		t.Errorf("Expected score mode Max, got %v", q.GetScoreMode())
	}
}

func TestToParentBlockJoinQueryClone(t *testing.T) {
	childQuery := &MockQuery{}
	parentFilter := &MockQuery{}

	q := NewToParentBlockJoinQuery(childQuery, parentFilter, Avg)
	cloned := q.Clone()

	if cloned == nil {
		t.Fatal("Expected Clone to return non-nil")
	}

	clonedQ, ok := cloned.(*ToParentBlockJoinQuery)
	if !ok {
		t.Fatal("Expected Clone to return *ToParentBlockJoinQuery")
	}

	if clonedQ.GetScoreMode() != Avg {
		t.Error("Expected cloned query to have same score mode")
	}
}

func TestToParentBlockJoinQueryEquals(t *testing.T) {
	childQuery := &MockQuery{}
	parentFilter := &MockQuery{}

	q1 := NewToParentBlockJoinQuery(childQuery, parentFilter, Total)
	q2 := NewToParentBlockJoinQuery(childQuery, parentFilter, Total)

	if !q1.Equals(q2) {
		t.Error("Expected Equals to return true for identical queries")
	}

	// Different score mode
	q3 := NewToParentBlockJoinQuery(childQuery, parentFilter, Max)
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
	parentFilter := &MockQuery{}

	q := NewToParentBlockJoinQuery(childQuery, parentFilter, None)
	hash := q.HashCode()

	if hash == 0 {
		t.Error("Expected non-zero HashCode")
	}
}

func TestToParentBlockJoinQueryString(t *testing.T) {
	childQuery := &MockQuery{}
	parentFilter := &MockQuery{}

	q := NewToParentBlockJoinQuery(childQuery, parentFilter, Avg)
	str := q.String()

	if str == "" {
		t.Error("Expected non-empty String representation")
	}
}

func TestNewToParentBlockJoinWeight(t *testing.T) {
	// This is a basic test since we can't easily create real weights
	w := NewToParentBlockJoinWeight(nil, nil, Max)

	if w == nil {
		t.Fatal("Expected ToParentBlockJoinWeight to be created")
	}

	if w.scoreMode != Max {
		t.Errorf("Expected score mode Max, got %v", w.scoreMode)
	}
}

func TestToParentBlockJoinWeightGetValueForNormalization(t *testing.T) {
	w := NewToParentBlockJoinWeight(nil, nil, Max)
	// Should not panic with nil weights
	_ = w.GetValueForNormalization()
}

func TestToParentBlockJoinWeightNormalize(t *testing.T) {
	w := NewToParentBlockJoinWeight(nil, nil, Max)
	// Should not panic with nil weights
	w.Normalize(1.0)
}
