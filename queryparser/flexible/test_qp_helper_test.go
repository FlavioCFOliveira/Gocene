// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/queryparser/flexible"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestQPHelper exercises the StandardQueryParser across its available features:
// term queries, boolean combinations, field scoping, phrase, range, wildcard,
// fuzzy, boost, and grouping.
//
// Port of: org.apache.lucene.queryparser.flexible.standard.TestQPHelper
// (subset of 1362-line Java suite — missing date/numeric range, span queries,
// and full analyzer pipeline integration).
func TestQPHelper(t *testing.T) {
	parser := flexible.NewStandardQueryParser()
	parser.SetDefaultField("content")

	t.Run("term query", func(t *testing.T) {
		q, err := parser.Parse("hello")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := q.(*search.TermQuery); !ok {
			t.Errorf("expected TermQuery, got %T", q)
		}
	})

	t.Run("boolean AND", func(t *testing.T) {
		q, err := parser.Parse("a AND b")
		if err != nil {
			t.Fatal(err)
		}
		bq, ok := q.(*search.BooleanQuery)
		if !ok {
			t.Fatalf("expected BooleanQuery, got %T", q)
		}
		if len(bq.Clauses()) != 2 {
			t.Errorf("expected 2 clauses, got %d", len(bq.Clauses()))
		}
	})

	t.Run("boolean OR", func(t *testing.T) {
		q, err := parser.Parse("a OR b")
		if err != nil {
			t.Fatal(err)
		}
		bq, ok := q.(*search.BooleanQuery)
		if !ok {
			t.Fatalf("expected BooleanQuery, got %T", q)
		}
		if len(bq.Clauses()) != 2 {
			t.Errorf("expected 2 clauses, got %d", len(bq.Clauses()))
		}
	})

	t.Run("boolean OR with pipe", func(t *testing.T) {
		q, err := parser.Parse("a || b")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := q.(*search.BooleanQuery); !ok {
			t.Errorf("expected BooleanQuery, got %T", q)
		}
	})

	t.Run("AND with pipe", func(t *testing.T) {
		q, err := parser.Parse("a && b")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := q.(*search.BooleanQuery); !ok {
			t.Errorf("expected BooleanQuery, got %T", q)
		}
	})

	t.Run("required term", func(t *testing.T) {
		q, err := parser.Parse("+a")
		if err != nil {
			t.Fatal(err)
		}
		if q == nil {
			t.Fatal("expected non-nil query")
		}
	})

	t.Run("prohibited term", func(t *testing.T) {
		q, err := parser.Parse("-a")
		if err != nil {
			t.Fatal(err)
		}
		if q == nil {
			t.Fatal("expected non-nil query")
		}
	})

	t.Run("NOT term", func(t *testing.T) {
		q, err := parser.Parse("NOT a")
		if err != nil {
			t.Fatal(err)
		}
		if q == nil {
			t.Fatal("expected non-nil query")
		}
	})

	t.Run("fielded term", func(t *testing.T) {
		q, err := parser.Parse("title:test")
		if err != nil {
			t.Fatal(err)
		}
		tq, ok := q.(*search.TermQuery)
		if !ok {
			t.Fatalf("expected TermQuery, got %T", q)
		}
		_ = tq
	})

	t.Run("phrase query", func(t *testing.T) {
		q, err := parser.Parse(`"hello world"`)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := q.(*search.PhraseQuery); !ok {
			t.Errorf("expected PhraseQuery, got %T", q)
		}
	})

	t.Run("phrase query with slop", func(t *testing.T) {
		q, err := parser.Parse(`"hello world"~2`)
		if err != nil {
			t.Fatal(err)
		}
		pq, ok := q.(*search.PhraseQuery)
		if !ok {
			t.Fatalf("expected PhraseQuery, got %T", q)
		}
		if pq.GetSlop() != 2 {
			t.Errorf("expected slop 2, got %d", pq.GetSlop())
		}
	})

	t.Run("range query inclusive", func(t *testing.T) {
		q, err := parser.Parse("[a TO z]")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := q.(*search.TermRangeQuery); !ok {
			t.Errorf("expected TermRangeQuery, got %T", q)
		}
	})

	t.Run("range query exclusive", func(t *testing.T) {
		q, err := parser.Parse("{a TO z}")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := q.(*search.TermRangeQuery); !ok {
			t.Errorf("expected TermRangeQuery, got %T", q)
		}
	})

	t.Run("fielded range query", func(t *testing.T) {
		q, err := parser.Parse("field:[a TO z]")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := q.(*search.TermRangeQuery); !ok {
			t.Errorf("expected TermRangeQuery, got %T", q)
		}
	})

	t.Run("fuzzy query", func(t *testing.T) {
		q, err := parser.Parse("hello~")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := q.(*search.FuzzyQuery); !ok {
			t.Errorf("expected FuzzyQuery, got %T", q)
		}
	})

	t.Run("boost query", func(t *testing.T) {
		q, err := parser.Parse("hello^2.0")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := q.(*search.BoostQuery); !ok {
			t.Errorf("expected BoostQuery, got %T", q)
		}
	})

	t.Run("wildcard query", func(t *testing.T) {
		q, err := parser.Parse("hel*o")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := q.(*search.WildcardQuery); !ok {
			t.Errorf("expected WildcardQuery, got %T", q)
		}
	})

	t.Run("grouped expression", func(t *testing.T) {
		q, err := parser.Parse("(a OR b)")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := q.(*search.BooleanQuery); !ok {
			t.Errorf("expected BooleanQuery for group, got %T", q)
		}
	})

	t.Run("complex boolean", func(t *testing.T) {
		q, err := parser.Parse("(a AND b) OR c")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := q.(*search.BooleanQuery); !ok {
			t.Errorf("expected BooleanQuery, got %T", q)
		}
	})

	t.Run("empty query error", func(t *testing.T) {
		_, err := parser.Parse("")
		if err == nil {
			t.Error("expected error for empty query")
		}
	})
}

// TestQPHelperConfig verifies that query parser configuration affects parsing.
func TestQPHelperConfig(t *testing.T) {
	t.Run("default operator AND", func(t *testing.T) {
		parser := flexible.NewStandardQueryParser()
		parser.SetDefaultField("f")
		parser.SetDefaultOperator("AND")

		q, err := parser.Parse("a b")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := q.(*search.BooleanQuery); !ok {
			t.Errorf("expected BooleanQuery with AND, got %T", q)
		}
	})

	t.Run("empty query after config", func(t *testing.T) {
		parser := flexible.NewStandardQueryParser()
		parser.SetDefaultField("f")
		_, err := parser.Parse("")
		if err == nil {
			t.Error("expected error for empty query")
		}
	})
}
