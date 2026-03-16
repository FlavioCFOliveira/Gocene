package join

import (
	"testing"
)

func TestNewToChildBlockJoinQuery(t *testing.T) {
	parentQuery := &MockQuery{}
	parentFilter := &MockQuery{}

	q := NewToChildBlockJoinQuery(parentQuery, parentFilter)

	if q == nil {
		t.Fatal("Expected ToChildBlockJoinQuery to be created")
	}

	if q.GetParentQuery() != parentQuery {
		t.Error("Expected parent query to match")
	}

	if q.GetParentFilter() != parentFilter {
		t.Error("Expected parent filter to match")
	}
}

func TestToChildBlockJoinQueryClone(t *testing.T) {
	parentQuery := &MockQuery{}
	parentFilter := &MockQuery{}

	q := NewToChildBlockJoinQuery(parentQuery, parentFilter)
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
}

func TestToChildBlockJoinQueryEquals(t *testing.T) {
	parentQuery := &MockQuery{}
	parentFilter := &MockQuery{}

	q1 := NewToChildBlockJoinQuery(parentQuery, parentFilter)
	q2 := NewToChildBlockJoinQuery(parentQuery, parentFilter)

	if !q1.Equals(q2) {
		t.Error("Expected Equals to return true for identical queries")
	}

	// Different type
	if q1.Equals(&MockQuery{}) {
		t.Error("Expected Equals to return false for different type")
	}
}

func TestToChildBlockJoinQueryHashCode(t *testing.T) {
	parentQuery := &MockQuery{}
	parentFilter := &MockQuery{}

	q := NewToChildBlockJoinQuery(parentQuery, parentFilter)
	hash := q.HashCode()

	if hash == 0 {
		t.Error("Expected non-zero HashCode")
	}
}

func TestToChildBlockJoinQueryString(t *testing.T) {
	parentQuery := &MockQuery{}
	parentFilter := &MockQuery{}

	q := NewToChildBlockJoinQuery(parentQuery, parentFilter)
	str := q.String()

	if str == "" {
		t.Error("Expected non-empty String representation")
	}
}

func TestNewToChildBlockJoinWeight(t *testing.T) {
	w := NewToChildBlockJoinWeight(nil, nil)

	if w == nil {
		t.Fatal("Expected ToChildBlockJoinWeight to be created")
	}
}

func TestToChildBlockJoinWeightGetValueForNormalization(t *testing.T) {
	w := NewToChildBlockJoinWeight(nil, nil)
	// Should not panic with nil weights
	_ = w.GetValueForNormalization()
}

func TestToChildBlockJoinWeightNormalize(t *testing.T) {
	w := NewToChildBlockJoinWeight(nil, nil)
	// Should not panic with nil weights
	w.Normalize(1.0)
}
