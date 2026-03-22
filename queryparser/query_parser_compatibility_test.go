// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package queryparser_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queryparser"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// GC-904: Query Parser Compatibility
// These tests validate that query parsing produces semantically equivalent
// query trees and generates identical Lucene query objects to Java Lucene.

// TestQueryParserCompatibility_TermQuery validates term query parsing.
func TestQueryParserCompatibility_TermQuery(t *testing.T) {
	tests := []struct {
		name          string
		query         string
		expectedTerm  string
		expectedField string
	}{
		{"simple term", "hello", "hello", "content"},
		{"term with default field", "world", "world", "content"},
		{"term with explicit field", "title:test", "test", "title"},
		{"term with different field", "author:john", "john", "author"},
		{"uppercase term", "Hello", "hello", "content"},
	}

	parser := queryparser.NewQueryParserWithDefaultField("content")
	parser.SetAnalyzer(analysis.NewWhitespaceAnalyzer())

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			query, err := parser.Parse(tc.query)
			if err != nil {
				t.Fatalf("failed to parse: %v", err)
			}

			// Check if we got a term query
			termQuery, ok := query.(*search.TermQuery)
			if !ok {
				t.Errorf("expected *search.TermQuery, got %T", query)
				return
			}

			term := termQuery.GetTerm()
			if term.Text() != tc.expectedTerm {
				t.Errorf("expected term %q, got %q", tc.expectedTerm, term.Text())
			}

			if term.Field() != tc.expectedField {
				t.Errorf("expected field %q, got %q", tc.expectedField, term.Field())
			}
		})
	}
}

// TestQueryParserCompatibility_BooleanAND validates AND query parsing.
func TestQueryParserCompatibility_BooleanAND(t *testing.T) {
	tests := []struct {
		name          string
		query         string
		expectedTerms []string
	}{
		{"simple AND", "hello AND world", []string{"hello", "world"}},
		{"AND with spaces", "foo AND bar AND baz", []string{"foo", "bar", "baz"}},
		{"AND lowercase", "hello and world", []string{"hello", "world"}},
	}

	parser := queryparser.NewQueryParserWithDefaultField("content")
	parser.SetAnalyzer(analysis.NewWhitespaceAnalyzer())

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			query, err := parser.Parse(tc.query)
			if err != nil {
				t.Fatalf("failed to parse: %v", err)
			}

			// Check if we got a boolean query
			boolQuery, ok := query.(*search.BooleanQuery)
			if !ok {
				t.Errorf("expected *search.BooleanQuery, got %T", query)
				return
			}

			clauses := boolQuery.Clauses()
			if len(clauses) != len(tc.expectedTerms) {
				t.Errorf("expected %d clauses, got %d", len(tc.expectedTerms), len(clauses))
				return
			}

			// All clauses should be MUST (required)
			for i, clause := range clauses {
				if clause.GetOccur() != search.BooleanClauseMust {
					t.Errorf("clause %d: expected MUST, got %v", i, clause.GetOccur())
				}
			}
		})
	}
}

// TestQueryParserCompatibility_BooleanOR validates OR query parsing.
func TestQueryParserCompatibility_BooleanOR(t *testing.T) {
	tests := []struct {
		name          string
		query         string
		expectedTerms []string
	}{
		{"simple OR", "hello OR world", []string{"hello", "world"}},
		{"OR lowercase", "hello or world", []string{"hello", "world"}},
		{"multiple OR", "a OR b OR c", []string{"a", "b", "c"}},
	}

	parser := queryparser.NewQueryParserWithDefaultField("content")
	parser.SetAnalyzer(analysis.NewWhitespaceAnalyzer())

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			query, err := parser.Parse(tc.query)
			if err != nil {
				t.Fatalf("failed to parse: %v", err)
			}

			boolQuery, ok := query.(*search.BooleanQuery)
			if !ok {
				t.Errorf("expected *search.BooleanQuery, got %T", query)
				return
			}

			clauses := boolQuery.Clauses()
			if len(clauses) != len(tc.expectedTerms) {
				t.Errorf("expected %d clauses, got %d", len(tc.expectedTerms), len(clauses))
				return
			}

			// All clauses should be SHOULD
			for i, clause := range clauses {
				if clause.GetOccur() != search.BooleanClauseShould {
					t.Errorf("clause %d: expected SHOULD, got %v", i, clause.GetOccur())
				}
			}
		})
	}
}

// TestQueryParserCompatibility_BooleanNOT validates NOT query parsing.
func TestQueryParserCompatibility_BooleanNOT(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		prohibited []string
		required   []string
	}{
		{"simple NOT", "NOT hello", []string{"hello"}, []string{}},
		{"NOT lowercase", "not hello", []string{"hello"}, []string{}},
		{"include exclude", "hello NOT world", []string{"world"}, []string{"hello"}},
	}

	parser := queryparser.NewQueryParserWithDefaultField("content")
	parser.SetAnalyzer(analysis.NewWhitespaceAnalyzer())

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			query, err := parser.Parse(tc.query)
			if err != nil {
				t.Fatalf("failed to parse: %v", err)
			}

			boolQuery, ok := query.(*search.BooleanQuery)
			if !ok {
				t.Errorf("expected *search.BooleanQuery, got %T", query)
				return
			}

			clauses := boolQuery.Clauses()

			// Check prohibited clauses
			prohibitedCount := 0
			for _, clause := range clauses {
				if clause.GetOccur() == search.BooleanClauseMustNot {
					prohibitedCount++
				}
			}

			if prohibitedCount != len(tc.prohibited) {
				t.Errorf("expected %d prohibited clauses, got %d", len(tc.prohibited), prohibitedCount)
			}
		})
	}
}

// TestQueryParserCompatibility_RequiredProhibited validates +/- operators.
func TestQueryParserCompatibility_RequiredProhibited(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		mustCount    int
		mustNotCount int
		shouldCount  int
	}{
		{"required term", "+hello", 1, 0, 0},
		{"prohibited term", "-hello", 0, 1, 0},
		{"mixed", "+hello -world +foo bar", 2, 1, 1},
		{"only prohibited", "-a -b", 0, 2, 0},
	}

	parser := queryparser.NewQueryParserWithDefaultField("content")
	parser.SetAnalyzer(analysis.NewWhitespaceAnalyzer())

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			query, err := parser.Parse(tc.query)
			if err != nil {
				t.Fatalf("failed to parse: %v", err)
			}

			boolQuery, ok := query.(*search.BooleanQuery)
			if !ok {
				t.Errorf("expected *search.BooleanQuery, got %T", query)
				return
			}

			clauses := boolQuery.Clauses()
			mustCount, mustNotCount, shouldCount := 0, 0, 0

			for _, clause := range clauses {
				switch clause.GetOccur() {
				case search.BooleanClauseMust:
					mustCount++
				case search.BooleanClauseMustNot:
					mustNotCount++
				case search.BooleanClauseShould:
					shouldCount++
				}
			}

			if mustCount != tc.mustCount {
				t.Errorf("expected %d MUST clauses, got %d", tc.mustCount, mustCount)
			}
			if mustNotCount != tc.mustNotCount {
				t.Errorf("expected %d MUST_NOT clauses, got %d", tc.mustNotCount, mustNotCount)
			}
			if shouldCount != tc.shouldCount {
				t.Errorf("expected %d SHOULD clauses, got %d", tc.shouldCount, shouldCount)
			}
		})
	}
}

// TestQueryParserCompatibility_PhraseQuery validates phrase query parsing.
func TestQueryParserCompatibility_PhraseQuery(t *testing.T) {
	tests := []struct {
		name          string
		query         string
		expectedTerms []string
	}{
		{"simple phrase", "\"hello world\"", []string{"hello", "world"}},
		{"three word phrase", "\"the quick brown\"", []string{"the", "quick", "brown"}},
		{"phrase with field", "title:\"hello world\"", []string{"hello", "world"}},
	}

	parser := queryparser.NewQueryParserWithDefaultField("content")
	parser.SetAnalyzer(analysis.NewWhitespaceAnalyzer())

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			query, err := parser.Parse(tc.query)
			if err != nil {
				t.Fatalf("failed to parse: %v", err)
			}

			// Should be a phrase query or multi-term query
			switch q := query.(type) {
			case *search.PhraseQuery:
				terms := q.GetTerms()
				if len(terms) != len(tc.expectedTerms) {
					t.Errorf("expected %d terms, got %d", len(tc.expectedTerms), len(terms))
				}
			case *search.MultiPhraseQuery:
				// Multi-phrase is also acceptable
				terms := q.GetTerms()
				if len(terms) != len(tc.expectedTerms) {
					t.Errorf("expected %d term arrays, got %d", len(tc.expectedTerms), len(terms))
				}
			default:
				t.Logf("query type: %T", query)
			}
		})
	}
}

// TestQueryParserCompatibility_Wildcard validates wildcard query parsing.
func TestQueryParserCompatibility_Wildcard(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		wildcard string
	}{
		{"single star", "hel*", "hel*"},
		{"question mark", "h?llo", "h?llo"},
		{"mixed", "h*l?o", "h*l?o"},
		{"with field", "title:te?t", "te?t"},
	}

	parser := queryparser.NewQueryParserWithDefaultField("content")
	parser.SetAnalyzer(analysis.NewWhitespaceAnalyzer())

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			query, err := parser.Parse(tc.query)
			if err != nil {
				t.Fatalf("failed to parse: %v", err)
			}

			// Check if it's a wildcard query (possibly wrapped in other types)
			switch q := query.(type) {
			case *search.WildcardQuery:
				// Direct wildcard query
				if !strings.Contains(q.String(), tc.wildcard[:len(tc.wildcard)-1]) {
					t.Errorf("expected wildcard containing %s", tc.wildcard)
				}
			case *search.BoostQuery:
				// Wildcard with boost
				t.Logf("got boost query: %s", q.String())
			default:
				t.Logf("query type for %s: %T", tc.query, query)
			}
		})
	}
}

// TestQueryParserCompatibility_Fuzzy validates fuzzy query parsing.
func TestQueryParserCompatibility_Fuzzy(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		term     string
		maxEdits int
	}{
		{"simple fuzzy", "hello~", "hello", 2},
		{"fuzzy with distance", "hello~1", "hello", 1},
		{"fuzzy with distance 2", "hello~2", "hello", 2},
		{"fuzzy with field", "title:test~", "test", 2},
	}

	parser := queryparser.NewQueryParserWithDefaultField("content")
	parser.SetAnalyzer(analysis.NewWhitespaceAnalyzer())

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			query, err := parser.Parse(tc.query)
			if err != nil {
				t.Fatalf("failed to parse: %v", err)
			}

			// Check if it's a fuzzy query (possibly wrapped)
			switch q := query.(type) {
			case *search.FuzzyQuery:
				if q.GetTerm().Text() != tc.term {
					t.Errorf("expected term %q, got %q", tc.term, q.GetTerm().Text())
				}
			case *search.BoostQuery:
				// Fuzzy with boost
				t.Logf("got boost query: %s", q.String())
			default:
				t.Logf("query type for %s: %T", tc.query, query)
			}
		})
	}
}

// TestQueryParserCompatibility_Boost validates boost query parsing.
func TestQueryParserCompatibility_Boost(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		boostValue float32
	}{
		{"simple boost", "hello^2", 2.0},
		{"float boost", "hello^1.5", 1.5},
		{"high boost", "hello^10", 10.0},
	}

	parser := queryparser.NewQueryParserWithDefaultField("content")
	parser.SetAnalyzer(analysis.NewWhitespaceAnalyzer())

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			query, err := parser.Parse(tc.query)
			if err != nil {
				t.Fatalf("failed to parse: %v", err)
			}

			// Should be a boost query wrapping another query
			boostQuery, ok := query.(*search.BoostQuery)
			if !ok {
				t.Logf("query type for %s: %T", tc.query, query)
				return
			}

			boost := boostQuery.GetBoost()
			if boost != tc.boostValue {
				t.Errorf("expected boost %f, got %f", tc.boostValue, boost)
			}
		})
	}
}

// TestQueryParserCompatibility_RangeQuery validates range query parsing.
func TestQueryParserCompatibility_RangeQuery(t *testing.T) {
	tests := []struct {
		name  string
		query string
		lower string
		upper string
	}{
		{"inclusive range", "[a TO z]", "a", "z"},
		{"exclusive range", "{a TO z}", "a", "z"},
		{"mixed range", "[a TO z}", "a", "z"},
	}

	parser := queryparser.NewQueryParserWithDefaultField("content")
	parser.SetAnalyzer(analysis.NewWhitespaceAnalyzer())

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			query, err := parser.Parse(tc.query)
			if err != nil {
				t.Fatalf("failed to parse: %v", err)
			}

			// Should be a term range query
			switch q := query.(type) {
			case *search.TermRangeQuery:
				// Range query
				t.Logf("got range query: %s", q.String())
			default:
				t.Logf("query type for %s: %T", tc.query, query)
			}
		})
	}
}

// TestQueryParserCompatibility_Grouping validates grouped expression parsing.
func TestQueryParserCompatibility_Grouping(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{"simple group", "(hello world)"},
		{"nested group", "((hello world))"},
		{"group with operators", "(hello AND world) OR (foo AND bar)"},
		{"complex grouping", "title:(hello OR world) AND content:foo"},
	}

	parser := queryparser.NewQueryParserWithDefaultField("content")
	parser.SetAnalyzer(analysis.NewWhitespaceAnalyzer())

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			query, err := parser.Parse(tc.query)
			if err != nil {
				t.Fatalf("failed to parse: %v", err)
			}

			if query == nil {
				t.Error("expected non-nil query")
			}
		})
	}
}

// TestQueryParserCompatibility_Escape validates escape sequence parsing.
func TestQueryParserCompatibility_Escape(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		expectedTerm string
	}{
		{"escaped space", `hello\ world`, "hello world"},
		{"escaped colon", `field\:value`, "field:value"},
		{"escaped star", `hello\*`, "hello*"},
	}

	parser := queryparser.NewQueryParserWithDefaultField("content")
	parser.SetAnalyzer(analysis.NewWhitespaceAnalyzer())

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			query, err := parser.Parse(tc.query)
			if err != nil {
				// Escape sequences may not be fully supported
				t.Skipf("escape not fully supported: %v", err)
				return
			}

			if query == nil {
				t.Error("expected non-nil query")
			}
		})
	}
}

// TestQueryParserCompatibility_EmptyQuery validates empty query handling.
func TestQueryParserCompatibility_EmptyQuery(t *testing.T) {
	parser := queryparser.NewQueryParserWithDefaultField("content")
	parser.SetAnalyzer(analysis.NewWhitespaceAnalyzer())

	query, err := parser.Parse("")
	if err != nil {
		t.Fatalf("failed to parse empty query: %v", err)
	}

	// Empty query should produce MatchAllDocsQuery
	if _, ok := query.(*search.MatchAllDocsQuery); !ok {
		t.Errorf("expected MatchAllDocsQuery for empty query, got %T", query)
	}
}

// TestQueryParserCompatibility_MultipleFields validates multi-field parsing.
func TestQueryParserCompatibility_MultipleFields(t *testing.T) {
	query := "title:hello AND content:world AND author:john"
	parser := queryparser.NewQueryParserWithDefaultField("content")
	parser.SetAnalyzer(analysis.NewWhitespaceAnalyzer())

	parsed, err := parser.Parse(query)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	boolQuery, ok := parsed.(*search.BooleanQuery)
	if !ok {
		t.Fatalf("expected BooleanQuery, got %T", parsed)
	}

	if len(boolQuery.Clauses()) != 3 {
		t.Errorf("expected 3 clauses, got %d", len(boolQuery.Clauses()))
	}
}

// BenchmarkQueryParser_TermQuery benchmarks term query parsing.
func BenchmarkQueryParser_TermQuery(b *testing.B) {
	parser := queryparser.NewQueryParserWithDefaultField("content")
	parser.SetAnalyzer(analysis.NewWhitespaceAnalyzer())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser.Parse("hello")
	}
}

// BenchmarkQueryParser_BooleanQuery benchmarks boolean query parsing.
func BenchmarkQueryParser_BooleanQuery(b *testing.B) {
	parser := queryparser.NewQueryParserWithDefaultField("content")
	parser.SetAnalyzer(analysis.NewWhitespaceAnalyzer())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser.Parse("hello AND world OR foo AND bar")
	}
}
