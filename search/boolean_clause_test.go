package search

import (
	"testing"
)

type mockQuery struct{}

func (m *mockQuery) Rewrite(searcher IndexSearcher, scoreMode ScoreMode, boost float32) Query {
	return m
}

func TestBooleanClause_OccurString(t *testing.T) {
	tests := []struct {
		occur    Occur
		expected string
	}{
		{MUST, "+"},
		{SHOULD, ""},
		{MUST_NOT, "-"},
		{FILTER, "#"},
	}

	for _, tt := range tests {
		if got := tt.occur.String(); got != tt.expected {
			t.Errorf("Occur(%d).String() = %q, want %q", tt.occur, got, tt.expected)
		}
	}
}

func TestBooleanClause_Properties(t *testing.T) {
	q := &mockQuery{}

	tests := []struct {
		name        string
		occur       Occur
		isRequired  bool
		isProhibited bool
		isScoring   bool
	}{
		{"MUST", MUST, true, false, true},
		{"SHOULD", SHOULD, false, false, true},
		{"MUST_NOT", MUST_NOT, false, true, false},
		{"FILTER", FILTER, true, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clause := NewBooleanClause(q, tt.occur)
			if got := clause.IsRequired(); got != tt.isRequired {
				t.Errorf("IsRequired() = %v, want %v", got, tt.isRequired)
			}
			if got := clause.IsProhibited(); got != tt.isProhibited {
				t.Errorf("IsProhibited() = %v, want %v", got, tt.isProhibited)
			}
			if got := clause.IsScoring(); got != tt.isScoring {
				t.Errorf("IsScoring() = %v, want %v", got, tt.isScoring)
			}
		})
	}
}

func TestBooleanClause_NilQuery(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("NewBooleanClause should panic on nil query")
		}
	}()
	NewBooleanClause(nil, MUST)
}
