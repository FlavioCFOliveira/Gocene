package queries

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

func TestCommonTermsQuery_New(t *testing.T) {
	q, err := NewCommonTermsQuery(search.MUST, search.SHOULD, 0.5)
	if err != nil {
		t.Fatalf("NewCommonTermsQuery: %v", err)
	}
	if q == nil {
		t.Fatal("returned nil")
	}
}

func TestCommonTermsQuery_MustNotRejected(t *testing.T) {
	_, err := NewCommonTermsQuery(search.MUST_NOT, search.SHOULD, 0.5)
	if err == nil {
		t.Fatal("expected error for MUST_NOT highFreq")
	}
}

func TestCommonTermsQuery_AddTerm(t *testing.T) {
	q, _ := NewCommonTermsQuery(search.SHOULD, search.MUST, 0.3)
	if err := q.Add(index.NewTerm("body", "hello")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	s := q.String()
	if s == "" {
		t.Error("String() returned empty after Add")
	}
}

func TestCommonTermsQuery_MinMustMatch(t *testing.T) {
	q, _ := NewCommonTermsQuery(search.SHOULD, search.MUST, 0.5)
	if err := q.Add(index.NewTerm("f", "a")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	_ = q.String()
}
