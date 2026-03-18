package highlight

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

func TestNewSimpleHighlighter(t *testing.T) {
	scorer := NewSimpleFragmentScorer([]string{"test"})
	h := NewSimpleHighlighter(scorer)

	if h == nil {
		t.Fatal("Expected SimpleHighlighter to be created")
	}
}

func TestSimpleHighlighterGetBestFragment(t *testing.T) {
	scorer := NewSimpleFragmentScorer([]string{"test"})
	h := NewSimpleHighlighter(scorer)

	// Test empty text
	fragment, err := h.GetBestFragment("", 1)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if fragment != "" {
		t.Errorf("Expected empty fragment, got '%s'", fragment)
	}

	// Test with text
	text := "This is a test. This is only a test."
	fragment, err = h.GetBestFragment(text, 1)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if fragment == "" {
		t.Error("Expected non-empty fragment")
	}
}

func TestSimpleHighlighterGetBestFragments(t *testing.T) {
	scorer := NewSimpleFragmentScorer([]string{"test"})
	h := NewSimpleHighlighter(scorer)

	text := "This is a test. This is only a test. Here is another test."
	fragments, err := h.GetBestFragments(text, 2)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(fragments) == 0 {
		t.Error("Expected at least one fragment")
	}
}

func TestSimpleHighlighterSetters(t *testing.T) {
	scorer := NewSimpleFragmentScorer([]string{"test"})
	h := NewSimpleHighlighter(scorer)

	// Test SetTextFragmenter
	fragmenter := NewSimpleFragmenter(50)
	h.SetTextFragmenter(fragmenter)

	// Test SetFormatter
	formatter := NewSimpleHTMLFormatter("<em>", "</em>")
	h.SetFormatter(formatter)

	// Test SetMaxDocBytesToAnalyze
	h.SetMaxDocBytesToAnalyze(1000)
}

func TestNewSimpleFragmenter(t *testing.T) {
	f := NewSimpleFragmenter(100)

	if f == nil {
		t.Fatal("Expected SimpleFragmenter to be created")
	}
}

func TestSimpleFragmenterGetFragments(t *testing.T) {
	f := NewSimpleFragmenter(50)

	// Test empty text
	fragments := f.GetFragments("", 1)
	if len(fragments) != 0 {
		t.Errorf("Expected 0 fragments for empty text, got %d", len(fragments))
	}

	// Test with text
	text := "This is a test. This is only a test. Here is another sentence."
	fragments = f.GetFragments(text, 3)
	if len(fragments) == 0 {
		t.Error("Expected at least one fragment")
	}
}

func TestNewSimpleHTMLFormatter(t *testing.T) {
	f := NewSimpleHTMLFormatter("<b>", "</b>")

	if f == nil {
		t.Fatal("Expected SimpleHTMLFormatter to be created")
	}
}

func TestSimpleHTMLFormatterHighlight(t *testing.T) {
	f := NewSimpleHTMLFormatter("<b>", "</b>")

	text := "This is a test sentence with test words."
	terms := []string{"test"}
	result := f.Highlight(text, terms)

	if result == "" {
		t.Error("Expected non-empty result")
	}

	// Check that tags were added
	if !contains(result, "<b>") {
		t.Error("Expected result to contain opening tag")
	}
	if !contains(result, "</b>") {
		t.Error("Expected result to contain closing tag")
	}
}

func TestSimpleHTMLFormatterHighlightCaseInsensitive(t *testing.T) {
	f := NewSimpleHTMLFormatter("<b>", "</b>")

	text := "This is a TEST sentence."
	terms := []string{"test"}
	result := f.Highlight(text, terms)

	if !contains(result, "<b>") {
		t.Error("Expected case-insensitive highlighting")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestNewSimpleFragmentScorer(t *testing.T) {
	terms := []string{"test", "example"}
	s := NewSimpleFragmentScorer(terms)

	if s == nil {
		t.Fatal("Expected SimpleFragmentScorer to be created")
	}

	if len(s.GetQueryTerms()) != len(terms) {
		t.Errorf("Expected %d terms, got %d", len(terms), len(s.GetQueryTerms()))
	}
}

func TestSimpleFragmentScorerGetFragmentScore(t *testing.T) {
	scorer := NewSimpleFragmentScorer([]string{"test"})

	// Fragment with term
	score := scorer.GetFragmentScore("This is a test sentence.")
	if score != 1.0 {
		t.Errorf("Expected score 1.0, got %f", score)
	}

	// Fragment without term
	score = scorer.GetFragmentScore("This is a sentence.")
	if score != 0 {
		t.Errorf("Expected score 0, got %f", score)
	}

	// Fragment with multiple occurrences
	score = scorer.GetFragmentScore("Test test test.")
	if score != 3.0 {
		t.Errorf("Expected score 3.0, got %f", score)
	}
}

func TestNewHighlighterFactory(t *testing.T) {
	query := &MockQuery{}
	factory := NewHighlighterFactory(query, "content")

	if factory == nil {
		t.Fatal("Expected HighlighterFactory to be created")
	}
}

func TestHighlighterFactoryCreateHighlighter(t *testing.T) {
	query := &MockQuery{}
	factory := NewHighlighterFactory(query, "content")

	highlighter, err := factory.CreateHighlighter()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if highlighter == nil {
		t.Fatal("Expected Highlighter to be created")
	}
}

// MockQuery is a simple mock query for testing
type MockQuery struct{}

func (q *MockQuery) Rewrite(reader search.IndexReader) (search.Query, error) { return q, nil }
func (q *MockQuery) Clone() search.Query                                     { return &MockQuery{} }
func (q *MockQuery) Equals(other search.Query) bool                          { _, ok := other.(*MockQuery); return ok }
func (q *MockQuery) HashCode() int                                           { return 0 }
func (q *MockQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	return nil, nil
}
