package highlight

import (
	"strings"
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

func TestNewGradientFormatter(t *testing.T) {
	f := NewGradientFormatter(0.0, 1.0, "000000", "FFFFFF")
	if f == nil {
		t.Fatal("Expected GradientFormatter to be created")
	}

	if f.minScore != 0.0 {
		t.Errorf("Expected minScore 0.0, got %f", f.minScore)
	}

	if f.maxScore != 1.0 {
		t.Errorf("Expected maxScore 1.0, got %f", f.maxScore)
	}

	if f.minForegroundColor != "000000" {
		t.Errorf("Expected minForegroundColor '000000', got '%s'", f.minForegroundColor)
	}

	if f.maxForegroundColor != "FFFFFF" {
		t.Errorf("Expected maxForegroundColor 'FFFFFF', got '%s'", f.maxForegroundColor)
	}
}

func TestNewGradientFormatterWithBackground(t *testing.T) {
	f := NewGradientFormatterWithBackground(0.0, 1.0, "000000", "FFFFFF", "FF0000", "00FF00")
	if f == nil {
		t.Fatal("Expected GradientFormatter to be created")
	}

	if f.minBackgroundColor != "FF0000" {
		t.Errorf("Expected minBackgroundColor 'FF0000', got '%s'", f.minBackgroundColor)
	}

	if f.maxBackgroundColor != "00FF00" {
		t.Errorf("Expected maxBackgroundColor '00FF00', got '%s'", f.maxBackgroundColor)
	}
}

func TestGradientFormatterHighlight(t *testing.T) {
	f := NewGradientFormatter(0.0, 1.0, "000000", "FFFFFF")

	text := "This is a test sentence with test words."
	terms := []string{"test"}
	result := f.Highlight(text, terms)

	if result == "" {
		t.Error("Expected non-empty result")
	}

	// Check that span with style was added
	if !strings.Contains(result, "<span style=\"") {
		t.Error("Expected result to contain span with style")
	}

	if !strings.Contains(result, "</span>") {
		t.Error("Expected result to contain closing span")
	}
}

func TestGradientFormatterHighlightWithBackground(t *testing.T) {
	f := NewGradientFormatterWithBackground(0.0, 1.0, "000000", "FFFFFF", "FF0000", "00FF00")

	text := "This is a test sentence."
	terms := []string{"test"}
	result := f.Highlight(text, terms)

	// Check that background-color is in the style
	if !strings.Contains(result, "background-color:") {
		t.Error("Expected result to contain background-color")
	}
}

func TestGradientFormatterInterpolateColor(t *testing.T) {
	f := NewGradientFormatter(0.0, 1.0, "000000", "FFFFFF")

	// Test mid-point interpolation
	color := f.interpolateColor("000000", "FFFFFF", 0.5)
	// 0.5 * 255 = 127.5, which truncates to 127 (0x7F)
	if color != "7F7F7F" {
		t.Errorf("Expected interpolated color '7F7F7F' for mid-point, got '%s'", color)
	}

	// Test min point
	color = f.interpolateColor("000000", "FFFFFF", 0.0)
	if color != "000000" {
		t.Errorf("Expected color '000000' for min point, got '%s'", color)
	}

	// Test max point
	color = f.interpolateColor("000000", "FFFFFF", 1.0)
	if color != "FFFFFF" {
		t.Errorf("Expected color 'FFFFFF' for max point, got '%s'", color)
	}
}

func TestHexToByte(t *testing.T) {
	tests := []struct {
		hex      string
		expected byte
	}{
		{"00", 0},
		{"FF", 255},
		{"ff", 255},
		{"80", 128},
		{"0A", 10},
		{"0a", 10},
	}

	for _, test := range tests {
		result := hexToByte(test.hex)
		if result != test.expected {
			t.Errorf("hexToByte('%s') = %d, expected %d", test.hex, result, test.expected)
		}
	}
}

func TestByteToHex(t *testing.T) {
	tests := []struct {
		b        byte
		expected string
	}{
		{0, "00"},
		{255, "FF"},
		{128, "80"},
		{10, "0A"},
	}

	for _, test := range tests {
		result := byteToHex(test.b)
		if result != test.expected {
			t.Errorf("byteToHex(%d) = '%s', expected '%s'", test.b, result, test.expected)
		}
	}
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

func TestNewNullFragmenter(t *testing.T) {
	f := NewNullFragmenter()
	if f == nil {
		t.Fatal("Expected NullFragmenter to be created")
	}
}

func TestNullFragmenterGetFragments(t *testing.T) {
	f := NewNullFragmenter()

	// Test empty text
	fragments := f.GetFragments("", 1)
	if len(fragments) != 0 {
		t.Errorf("Expected 0 fragments for empty text, got %d", len(fragments))
	}

	// Test with text - should return entire text as single fragment
	text := "This is a long text that should not be fragmented. It should be returned as a single fragment."
	fragments = f.GetFragments(text, 5)
	if len(fragments) != 1 {
		t.Errorf("Expected 1 fragment, got %d", len(fragments))
	}

	if fragments[0] != text {
		t.Errorf("Expected fragment to match original text, got '%s'", fragments[0])
	}

	// Test that maxNumFragments is ignored
	fragments = f.GetFragments(text, 1)
	if len(fragments) != 1 {
		t.Errorf("Expected 1 fragment regardless of maxNumFragments, got %d", len(fragments))
	}
}
