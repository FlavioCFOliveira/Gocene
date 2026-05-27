package queryparser

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

func TestNewStandardQueryParser(t *testing.T) {
	p := NewStandardQueryParser()

	if p == nil {
		t.Fatal("Expected StandardQueryParser to be created")
	}

	if p.GetDefaultField() != "" {
		t.Errorf("Expected empty default field, got '%s'", p.GetDefaultField())
	}

	if p.GetDefaultOperator() != OR {
		t.Error("Expected default operator OR")
	}
}

func TestStandardQueryParserSetters(t *testing.T) {
	p := NewStandardQueryParser()

	// Test SetDefaultField
	p.SetDefaultField("content")
	if p.GetDefaultField() != "content" {
		t.Errorf("Expected default field 'content', got '%s'", p.GetDefaultField())
	}

	// Test SetDefaultOperator
	p.SetDefaultOperator(AND)
	if p.GetDefaultOperator() != AND {
		t.Error("Expected default operator AND")
	}

	// Test SetAllowLeadingWildcard
	p.SetAllowLeadingWildcard(true)
	if !p.GetAllowLeadingWildcard() {
		t.Error("Expected allow leading wildcard to be true")
	}

	// Test SetPhraseSlop
	p.SetPhraseSlop(5)
	if p.GetPhraseSlop() != 5 {
		t.Errorf("Expected phrase slop 5, got %d", p.GetPhraseSlop())
	}
}

func TestStandardQueryParserParse(t *testing.T) {
	p := NewStandardQueryParser()
	p.SetDefaultField("content")

	// Test empty query
	_, err := p.Parse("")
	if err == nil {
		t.Error("Expected error for empty query")
	}

	// Test simple term
	query, err := p.Parse("test")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if query == nil {
		t.Error("Expected query to be created")
	}

	// Test phrase
	query, err = p.Parse("\"hello world\"")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if query == nil {
		t.Error("Expected query to be created")
	}

	// Test fielded query
	query, err = p.Parse("title:test")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if query == nil {
		t.Error("Expected query to be created")
	}
}

func TestStandardQueryParserParseWithField(t *testing.T) {
	p := NewStandardQueryParser()

	query, err := p.ParseWithField("title", "test")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if query == nil {
		t.Error("Expected query to be created")
	}
}

func TestBooleanOperatorString(t *testing.T) {
	if AND.String() != "AND" {
		t.Error("Expected AND.String() to return 'AND'")
	}
	if OR.String() != "OR" {
		t.Error("Expected OR.String() to return 'OR'")
	}
}

// TestStandardQueryParser_RangeQueryReturnsTermRange exercises the
// previously stubbed parseRange path. Both inclusive ([..]) and exclusive
// ({..}) forms must yield TermRangeQuery instances; mixed brackets must
// preserve the inclusivity of each end independently.
func TestStandardQueryParser_RangeQueryReturnsTermRange(t *testing.T) {
	p := NewStandardQueryParser()
	p.SetDefaultField("title")

	cases := []struct {
		input              string
		incLower, incUpper bool
	}{
		{"title:[a TO z]", true, true},
		{"title:{a TO z}", false, false},
		{"title:[a TO z}", true, false},
		{"title:{a TO z]", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			q, err := p.Parse(tc.input)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tc.input, err)
			}
			rq, ok := q.(*search.TermRangeQuery)
			if !ok {
				t.Fatalf("Parse(%q) = %T, want *search.TermRangeQuery", tc.input, q)
			}
			if got := rq.IncludesLower(); got != tc.incLower {
				t.Errorf("IncludesLower = %v, want %v", got, tc.incLower)
			}
			if got := rq.IncludesUpper(); got != tc.incUpper {
				t.Errorf("IncludesUpper = %v, want %v", got, tc.incUpper)
			}
			if string(rq.LowerTerm()) != "a" {
				t.Errorf("LowerTerm = %q, want %q", rq.LowerTerm(), "a")
			}
			if string(rq.UpperTerm()) != "z" {
				t.Errorf("UpperTerm = %q, want %q", rq.UpperTerm(), "z")
			}
		})
	}
}

// TestStandardQueryParser_FuzzyQuery checks that "term~" and "term~N" both
// yield FuzzyQuery instances (previously stubbed back to TermQuery).
func TestStandardQueryParser_FuzzyQuery(t *testing.T) {
	p := NewStandardQueryParser()
	p.SetDefaultField("title")

	for _, input := range []string{"title:roam~", "title:roam~1", "title:roam~2"} {
		t.Run(input, func(t *testing.T) {
			q, err := p.Parse(input)
			if err != nil {
				t.Fatalf("Parse(%q): %v", input, err)
			}
			if _, ok := q.(*search.FuzzyQuery); !ok {
				t.Fatalf("Parse(%q) = %T, want *search.FuzzyQuery", input, q)
			}
		})
	}
}

// TestStandardQueryParser_WildcardQuery checks that '*' and '?' bearing
// terms produce WildcardQuery instances (previously stubbed back to
// TermQuery).
func TestStandardQueryParser_WildcardQuery(t *testing.T) {
	p := NewStandardQueryParser()
	p.SetDefaultField("title")

	for _, input := range []string{"title:foo*", "title:f?o", "title:b*r"} {
		t.Run(input, func(t *testing.T) {
			q, err := p.Parse(input)
			if err != nil {
				t.Fatalf("Parse(%q): %v", input, err)
			}
			if _, ok := q.(*search.WildcardQuery); !ok {
				t.Fatalf("Parse(%q) = %T, want *search.WildcardQuery", input, q)
			}
		})
	}
}
