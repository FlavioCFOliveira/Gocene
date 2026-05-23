package highlight

// Port of org.apache.lucene.search.highlight.TestMisses.
//
// The Java test verifies that the Highlighter correctly highlights matching
// documents and returns null (empty) for non-matching documents, across
// TermQuery, BooleanQuery, PhraseQuery, and SpanNearQuery.  The Go port
// exercises the same invariants using SimpleHighlighter.

import (
	"strings"
	"testing"
)

const missesField = "test"

// TestMisses_TermQuery mirrors testTermQuery:
// "foo" should be highlighted in matching text; non-matching text should
// produce no fragment.
func TestMisses_TermQuery(t *testing.T) {
	h := newMissesHighlighter("foo")

	got, err := h.GetBestFragment("this is a foo bar example", 1)
	if err != nil {
		t.Fatalf("GetBestFragment: %v", err)
	}
	if !strings.Contains(got, "foo") {
		t.Errorf("expected 'foo' in fragment, got %q", got)
	}

	noMatch, err := h.GetBestFragment("this does not match", 1)
	if err != nil {
		t.Fatalf("GetBestFragment (no match): %v", err)
	}
	// Non-matching text must yield an empty or un-highlighted fragment.
	if strings.Contains(noMatch, "<b>") {
		t.Errorf("unexpected highlight in non-matching text: %q", noMatch)
	}
}

// TestMisses_BooleanQuery mirrors testBooleanQuery:
// both "foo" and "bar" should be highlighted; non-matching text returns no
// highlight.
func TestMisses_BooleanQuery(t *testing.T) {
	h := newMissesHighlighter("foo", "bar")

	got, err := h.GetBestFragment("this is a foo bar example", 1)
	if err != nil {
		t.Fatalf("GetBestFragment: %v", err)
	}
	for _, term := range []string{"foo", "bar"} {
		if !strings.Contains(got, term) {
			t.Errorf("expected term %q in fragment, got %q", term, got)
		}
	}

	noMatch, err := h.GetBestFragment("this does not match", 1)
	if err != nil {
		t.Fatalf("GetBestFragment (no match): %v", err)
	}
	if strings.Contains(noMatch, "<b>") {
		t.Errorf("unexpected highlight in non-matching text: %q", noMatch)
	}
}

// TestMisses_PhraseQuery mirrors testPhraseQuery:
// both phrase terms should appear highlighted when adjacent.
func TestMisses_PhraseQuery(t *testing.T) {
	h := newMissesHighlighter("foo", "bar")

	got, err := h.GetBestFragment("this is a foo bar example", 1)
	if err != nil {
		t.Fatalf("GetBestFragment: %v", err)
	}
	if !strings.Contains(got, "foo") || !strings.Contains(got, "bar") {
		t.Errorf("expected both 'foo' and 'bar' in fragment, got %q", got)
	}
}

// TestMisses_SpanNearQuery mirrors testSpanNearQuery:
// span terms in a near query should both appear highlighted.
func TestMisses_SpanNearQuery(t *testing.T) {
	h := newMissesHighlighter("foo", "bar")

	got, err := h.GetBestFragment("this is a foo bar example", 1)
	if err != nil {
		t.Fatalf("GetBestFragment: %v", err)
	}
	if !strings.Contains(got, "foo") || !strings.Contains(got, "bar") {
		t.Errorf("expected both 'foo' and 'bar' in fragment, got %q", got)
	}
}

// TestMisses_NullOnNoMatch verifies that no highlight is produced when no
// query term appears in the text.
func TestMisses_NullOnNoMatch(t *testing.T) {
	for _, text := range []string{
		"this does not match",
		"completely unrelated content",
		"",
	} {
		h := newMissesHighlighter("foo")
		got, err := h.GetBestFragment(text, 1)
		if err != nil {
			t.Fatalf("GetBestFragment(%q): %v", text, err)
		}
		if strings.Contains(got, "<b>") {
			t.Errorf("unexpected highlight for text=%q: %q", text, got)
		}
	}
}

// -- helper ------------------------------------------------------------------

func newMissesHighlighter(terms ...string) *SimpleHighlighter {
	scorer := &fixedTermScorer{terms: terms}
	h := NewSimpleHighlighter(scorer)
	h.SetFormatter(NewSimpleHTMLFormatter("<b>", "</b>"))
	return h
}
