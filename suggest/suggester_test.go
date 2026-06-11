package suggest

import "testing"

func TestSuggestion_New(t *testing.T) {
	s := NewSuggestion("term", 100)
	if s == nil {
		t.Fatal("NewSuggestion returned nil")
	}
	if s.Term != "term" {
		t.Fatalf("Term=%q", s.Term)
	}
	if s.Weight != 100 {
		t.Fatalf("Weight=%d", s.Weight)
	}
}

func TestInMemorySuggester_New(t *testing.T) {
	sug := NewInMemorySuggester()
	if sug == nil {
		t.Fatal("NewInMemorySuggester returned nil")
	}
}

func TestInMemorySorter_New(t *testing.T) {
	sorter := NewInMemorySorter()
	if sorter == nil {
		t.Fatal("NewInMemorySorter returned nil")
	}
}

func TestCompletionQuery_New(t *testing.T) {
	q := NewCompletionQuery("prefix", "field", 5)
	if q == nil {
		t.Fatal("NewCompletionQuery returned nil")
	}
}
