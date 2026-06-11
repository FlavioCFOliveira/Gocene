// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package queryparser_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queryparser"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestMultiAnalyzer exercises the classic QueryParser with StandardAnalyzer for
// basic term, phrase, boolean, and field-scoped queries.
//
// Port of: org.apache.lucene.queryparser.classic.TestMultiAnalyzer
// The Java original tests multi-token synonym analyzers — Gocene's classic
// QueryParser accepts *analysis.StandardAnalyzer and produces TermQuery,
// PhraseQuery, BooleanQuery, etc. Full synonym/multi-token handling (SynonymQuery
// production from position-increment-0 tokens) is deferred.
func TestMultiAnalyzer(t *testing.T) {
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
	})

	t.Run("multiple terms", func(t *testing.T) {
		q, err := parser.Parse("hello world")
		if err != nil {
			t.Fatal(err)
		}
		if q == nil {
			t.Fatal("expected non-nil query")
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

	t.Run("field scoped term", func(t *testing.T) {
		q, err := parser.Parse("title:test")
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
		if q == nil {
			t.Fatal("expected non-nil query")
		}
	})

	t.Run("boolean OR", func(t *testing.T) {
		q, err := parser.Parse("a OR b")
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

	t.Run("empty query", func(t *testing.T) {
		_, err := parser.Parse("")
		if err != nil {
			t.Fatalf("Parse empty: %v", err)
		}
	})
}

// TestMultiAnalyzerProducesTerm verifies that simple term parsing produces a
// TermQuery.
func TestMultiAnalyzerProducesTerm(t *testing.T) {
	analyzer := analysis.NewStandardAnalyzer()
	parser := queryparser.NewQueryParser("content", analyzer)

	q, err := parser.Parse("hello")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.TermQuery); !ok {
		t.Errorf("expected TermQuery, got %T", q)
	}
}
