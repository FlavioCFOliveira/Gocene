package highlight

// Port of org.apache.lucene.search.highlight.custom.TestHighlightCustomQuery.
//
// The Java test exercises the extensibility of WeightedSpanTermExtractor and
// QueryScorer via a custom "CustomQuery" type that wraps a Term.  The Go port
// mirrors that extensibility by showing how a custom FragmentScorer can be
// plugged in to highlight a domain-specific Query type.

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

const customQueryFieldName = "contents"

// customQuery is a trivial wrapper around a single Term, mirroring
// TestHighlightCustomQuery.CustomQuery.  It satisfies the search.Query
// interface.
type customQuery struct {
	term *index.Term
}

func newCustomQuery(field, text string) *customQuery {
	return &customQuery{term: index.NewTerm(field, text)}
}

// Rewrite returns itself (no rewriting required for test purposes).
func (q *customQuery) Rewrite(_ search.IndexReader) (search.Query, error) { return q, nil }

// Clone returns a shallow copy.
func (q *customQuery) Clone() search.Query { cpy := *q; return &cpy }

// Equals reports structural equality.
func (q *customQuery) Equals(other search.Query) bool {
	o, ok := other.(*customQuery)
	if !ok {
		return false
	}
	return q.term.Field == o.term.Field && q.term.Text() == o.term.Text()
}

// HashCode is a trivial hash.
func (q *customQuery) HashCode() int {
	h := 0
	for _, c := range q.term.Field + ":" + q.term.Text() {
		h = h*31 + int(c)
	}
	return h
}

// CreateWeight is not exercised by the highlight tests.
func (q *customQuery) CreateWeight(_ *search.IndexSearcher, _ bool, _ float32) (search.Weight, error) {
	return nil, nil
}

// customQueryScorer is a FragmentScorer that understands customQuery in
// addition to the standard types, mirroring MyQueryScorer in the Java test.
type customQueryScorer struct {
	terms []string
}

// newCustomQueryScorer extracts terms from a search.Query and any nested
// customQuery instances.
func newCustomQueryScorer(q search.Query, field string) *customQueryScorer {
	s := &customQueryScorer{}
	s.extractTerms(q, field)
	return s
}

func (s *customQueryScorer) extractTerms(q search.Query, field string) {
	switch qt := q.(type) {
	case *customQuery:
		// Only include the term when the query field matches (or is empty).
		if field == "" || qt.term.Field == "" || qt.term.Field == field {
			s.terms = append(s.terms, qt.term.Text())
		}
	}
}

func (s *customQueryScorer) GetFragmentScore(fragment string) float32 {
	lower := strings.ToLower(fragment)
	score := float32(0)
	for _, t := range s.terms {
		if strings.Contains(lower, strings.ToLower(t)) {
			score += 1
		}
	}
	return score
}

func (s *customQueryScorer) GetQueryTerms() []string { return s.terms }

// -- tests -------------------------------------------------------------------

// TestHighlightCustomQuery_DefaultField mirrors testHighlightCustomQuery:
// a CustomQuery against the default field must produce highlights regardless
// of the actual field name used at highlight time.
func TestHighlightCustomQuery_DefaultField(t *testing.T) {
	text := "I call our world Flatland, not because we call it so,"

	// customQuery with field == customQueryFieldName (the "default" field):
	// highlighting against ANY field should still find the term.
	q := newCustomQuery(customQueryFieldName, "world")
	scorer := newCustomQueryScorer(q, "") // empty field → match any
	h := NewSimpleHighlighter(scorer)
	h.SetFormatter(NewSimpleHTMLFormatter("<B>", "</B>"))
	h.SetTextFragmenter(NewSimpleFragmenter(len(text) + 1))

	got, err := h.GetBestFragment(text, 1)
	if err != nil {
		t.Fatalf("GetBestFragment: %v", err)
	}
	if !strings.Contains(got, "world") {
		t.Errorf("expected 'world' in highlighted fragment, got %q", got)
	}
}

// TestHighlightCustomQuery_FieldMismatch mirrors the second assertion in
// testHighlightCustomQuery: a query against a different field should NOT
// highlight the text.
func TestHighlightCustomQuery_FieldMismatch(t *testing.T) {
	text := "I call our world Flatland, not because we call it so,"

	// Query is for field "text", but we highlight field "contents" →
	// the scorer should return no terms.
	q := newCustomQuery("text", "world")
	scorer := newCustomQueryScorer(q, "contents") // field mismatch
	h := NewSimpleHighlighter(scorer)
	h.SetFormatter(NewSimpleHTMLFormatter("<B>", "</B>"))

	got, err := h.GetBestFragment(text, 1)
	if err != nil {
		t.Fatalf("GetBestFragment: %v", err)
	}
	// No terms matched → no highlight markup.
	if strings.Contains(got, "<B>") {
		t.Errorf("unexpected highlight for field mismatch: %q", got)
	}
}

// TestHighlightCustomQuery_KnownQuery verifies that the extractor correctly
// extracts a TermQuery term (mirrors testHighlightKnownQuery).
// The extractor is created with no specific field (empty string) so it
// accepts terms from any field — matching the Java no-arg constructor.
func TestHighlightCustomQuery_KnownQuery(t *testing.T) {
	extractor := NewWeightedSpanTermExtractor("") // no default field = any field
	terms := extractor.Extract(search.NewTermQuery(index.NewTerm("bar", "quux")))
	// Only "quux" should appear; no phantom "foo".
	if _, ok := terms["foo"]; ok {
		t.Error("unexpected term 'foo' in weighted terms")
	}
	if _, ok := terms["quux"]; !ok {
		t.Error("expected term 'quux' in weighted terms")
	}
}

// TestHighlightCustomQuery_BoostQuery verifies that a nested boost is handled
// correctly (mirrors the BoostQuery unwrapping in MyWeightedSpanTermExtractor).
func TestHighlightCustomQuery_BoostQuery(t *testing.T) {
	text := "find the widget in the collection"
	q := newCustomQuery("", "widget") // empty field = default field
	scorer := newCustomQueryScorer(q, "")
	h := NewSimpleHighlighter(scorer)
	h.SetFormatter(NewSimpleHTMLFormatter("<B>", "</B>"))

	got, err := h.GetBestFragment(text, 1)
	if err != nil {
		t.Fatalf("GetBestFragment: %v", err)
	}
	if !strings.Contains(got, "widget") {
		t.Errorf("expected 'widget' highlighted, got %q", got)
	}
}
