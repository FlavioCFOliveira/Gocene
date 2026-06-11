// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queryparser"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestQueryParserTestBase provides shared test logic for the classic
// QueryParser. It exercises the base parser features: term queries,
// phrase queries, boolean queries, field scoping, wildcard, fuzzy, range,
// escape, and config setters/getters.
//
// Port of: org.apache.lucene.queryparser.util.QueryParserTestBase
// (a 1378-line abstract base class in Java; in Gocene shared test logic
// is expressed through table-driven test functions.)
func TestQueryParserTestBase(t *testing.T) {
	analyzer := analysis.NewStandardAnalyzer()
	parser := queryparser.NewQueryParser("content", analyzer)

	t.Run("simple term", func(t *testing.T) {
		q, err := parser.Parse("hello")
		if err != nil {
			t.Fatal(err)
		}
		if q == nil {
			t.Fatal("expected non-nil query")
		}
		if _, ok := q.(*search.TermQuery); !ok {
			t.Errorf("expected TermQuery, got %T", q)
		}
	})

	t.Run("phrase", func(t *testing.T) {
		q, err := parser.Parse(`"hello world"`)
		if err != nil {
			t.Fatal(err)
		}
		if q == nil {
			t.Fatal("expected non-nil query")
		}
	})

	t.Run("boolean AND", func(t *testing.T) {
		q, err := parser.Parse("a AND b")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := q.(*search.BooleanQuery); !ok {
			t.Errorf("expected BooleanQuery, got %T", q)
		}
	})

	t.Run("boolean OR", func(t *testing.T) {
		q, err := parser.Parse("a OR b")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := q.(*search.BooleanQuery); !ok {
			t.Errorf("expected BooleanQuery, got %T", q)
		}
	})

	t.Run("NOT", func(t *testing.T) {
		q, err := parser.Parse("NOT a")
		if err != nil {
			t.Fatal(err)
		}
		if q == nil {
			t.Fatal("expected non-nil query")
		}
	})

	t.Run("fielded term", func(t *testing.T) {
		q, err := parser.Parse("title:hello")
		if err != nil {
			t.Fatal(err)
		}
		if q == nil {
			t.Fatal("expected non-nil query")
		}
	})

	t.Run("wildcard", func(t *testing.T) {
		q, err := parser.Parse("hel*o")
		if err != nil {
			t.Fatal(err)
		}
		if q == nil {
			t.Fatal("expected non-nil query")
		}
	})

	t.Run("fuzzy", func(t *testing.T) {
		q, err := parser.Parse("hello~")
		if err != nil {
			t.Fatal(err)
		}
		if q == nil {
			t.Fatal("expected non-nil query")
		}
	})

	t.Run("range inclusive", func(t *testing.T) {
		q, err := parser.Parse("[a TO z]")
		if err != nil {
			t.Fatal(err)
		}
		if q == nil {
			t.Fatal("expected non-nil query")
		}
	})

	t.Run("range exclusive", func(t *testing.T) {
		q, err := parser.Parse("{a TO z}")
		if err != nil {
			t.Fatal(err)
		}
		if q == nil {
			t.Fatal("expected non-nil query")
		}
	})

	t.Run("grouped expression", func(t *testing.T) {
		q, err := parser.Parse("(a OR b) AND c")
		if err != nil {
			t.Fatal(err)
		}
		if q == nil {
			t.Fatal("expected non-nil query")
		}
	})

	t.Run("boost", func(t *testing.T) {
		q, err := parser.Parse("hello^3.0")
		if err != nil {
			t.Fatal(err)
		}
		if q == nil {
			t.Fatal("expected non-nil query")
		}
	})

	t.Run("empty query", func(t *testing.T) {
		q, err := parser.Parse("")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := q.(*search.MatchAllDocsQuery); !ok {
			t.Errorf("empty query should produce MatchAllDocsQuery, got %T", q)
		}
	})
}

// TestQueryParserBaseSetters verifies that QueryParserBase configuration
// setters and getters work correctly.
func TestQueryParserBaseSetters(t *testing.T) {
	analyzer := analysis.NewStandardAnalyzer()
	base := queryparser.NewQueryParserBase("content", analyzer)

	if base.GetDefaultField() != "content" {
		t.Errorf("default field = %q", base.GetDefaultField())
	}
	base.SetDefaultField("body")
	if base.GetDefaultField() != "body" {
		t.Errorf("SetDefaultField failed, got %q", base.GetDefaultField())
	}

	if base.GetAnalyzer() != analyzer {
		t.Error("GetAnalyzer should return the set analyzer")
	}

	if base.GetAllowLeadingWildcard() {
		t.Error("leading wildcard should be false by default")
	}
	base.SetAllowLeadingWildcard(true)
	if !base.GetAllowLeadingWildcard() {
		t.Error("SetAllowLeadingWildcard failed")
	}

	if !base.GetEnablePositionIncrements() {
		t.Error("position increments should be true by default")
	}
	base.SetEnablePositionIncrements(false)
	if base.GetEnablePositionIncrements() {
		t.Error("SetEnablePositionIncrements failed")
	}

	if base.GetPhraseSlop() != 0 {
		t.Errorf("phrase slop default = %d", base.GetPhraseSlop())
	}
	base.SetPhraseSlop(3)
	if base.GetPhraseSlop() != 3 {
		t.Errorf("SetPhraseSlop failed, got %d", base.GetPhraseSlop())
	}

	if base.GetFuzzyMinSim() != 2.0 {
		t.Errorf("fuzzy min sim default = %f, want 2.0", base.GetFuzzyMinSim())
	}
	base.SetFuzzyMinSim(0.8)
	if base.GetFuzzyMinSim() != 0.8 {
		t.Errorf("SetFuzzyMinSim failed, got %f", base.GetFuzzyMinSim())
	}

	if base.GetFuzzyPrefixLength() != 0 {
		t.Errorf("fuzzy prefix length default = %d", base.GetFuzzyPrefixLength())
	}
	base.SetFuzzyPrefixLength(2)
	if base.GetFuzzyPrefixLength() != 2 {
		t.Errorf("SetFuzzyPrefixLength failed, got %d", base.GetFuzzyPrefixLength())
	}

	if !base.GetLowercaseExpandedTerms() {
		t.Error("lowercase expanded terms should be true by default")
	}
	base.SetLowercaseExpandedTerms(false)
	if base.GetLowercaseExpandedTerms() {
		t.Error("SetLowercaseExpandedTerms failed")
	}
}

// TestQueryParserBaseRangeQuery tests the QueryParserBase range query builder.
func TestQueryParserBaseRangeQuery(t *testing.T) {
	qpb := queryparser.NewQueryParserBase("content", analysis.NewStandardAnalyzer())

	q, err := qpb.GetRangeQuery("field", "a", "z", true, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.TermRangeQuery); !ok {
		t.Errorf("expected TermRangeQuery, got %T", q)
	}
}

// TestQueryParserBaseEscape tests the Escape utility method.
func TestQueryParserBaseEscape(t *testing.T) {
	qpb := queryparser.NewQueryParserBase("content", analysis.NewStandardAnalyzer())
	escaped := qpb.Escape("hello (world)")
	if escaped == "" {
		t.Fatal("Escape should not return empty string")
	}
	if len(escaped) < len("hello (world)") {
		t.Error("Escape should not shorten the string")
	}
}

// TestQueryParserBaseWildcardQuery tests the wildcard query builder.
func TestQueryParserBaseWildcardQuery(t *testing.T) {
	qpb := queryparser.NewQueryParserBase("content", analysis.NewStandardAnalyzer())

	q, err := qpb.GetWildcardQuery("field", "hel*o")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.WildcardQuery); !ok {
		t.Errorf("expected WildcardQuery, got %T", q)
	}
}

// TestQueryParserBasePrefixQuery tests the prefix query builder.
func TestQueryParserBasePrefixQuery(t *testing.T) {
	qpb := queryparser.NewQueryParserBase("content", analysis.NewStandardAnalyzer())

	q, err := qpb.GetPrefixQuery("field", "hel")
	if err != nil {
		t.Fatal(err)
	}
	if q == nil {
		t.Fatal("expected non-nil query")
	}
}

// TestQueryParserBaseFuzzyQuery tests the fuzzy query builder.
func TestQueryParserBaseFuzzyQuery(t *testing.T) {
	qpb := queryparser.NewQueryParserBase("content", analysis.NewStandardAnalyzer())

	q, err := qpb.GetFuzzyQuery("field", "hello", 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.FuzzyQuery); !ok {
		t.Errorf("expected FuzzyQuery, got %T", q)
	}
}

// TestQueryParserBaseMatchAllDocs tests the MatchAllDocs query.
func TestQueryParserBaseMatchAllDocs(t *testing.T) {
	qpb := queryparser.NewQueryParserBase("content", analysis.NewStandardAnalyzer())

	q := qpb.GetMatchAllDocsQuery()
	if _, ok := q.(*search.MatchAllDocsQuery); !ok {
		t.Errorf("expected MatchAllDocsQuery, got %T", q)
	}
}

// TestQueryParserBaseMatchNoDocs tests the MatchNoDocs query.
func TestQueryParserBaseMatchNoDocs(t *testing.T) {
	qpb := queryparser.NewQueryParserBase("content", analysis.NewStandardAnalyzer())

	q := qpb.GetMatchNoDocsQuery()
	if _, ok := q.(*search.MatchNoDocsQuery); !ok {
		t.Errorf("expected MatchNoDocsQuery, got %T", q)
	}
}

// TestQueryParserBaseFieldQuery tests the field query builder.
func TestQueryParserBaseFieldQuery(t *testing.T) {
	qpb := queryparser.NewQueryParserBase("content", analysis.NewStandardAnalyzer())

	q, err := qpb.GetFieldQuery("field", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if q == nil {
		t.Fatal("expected non-nil query")
	}
}
