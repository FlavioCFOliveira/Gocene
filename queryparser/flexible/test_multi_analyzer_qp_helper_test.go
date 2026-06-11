// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queryparser/flexible"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestMultiAnalyzerQPHelper verifies that StandardQueryParser works with a
// StandardAnalyzer for basic term, phrase, and boolean query parsing.
func TestMultiAnalyzerQPHelper(t *testing.T) {
	parser := flexible.NewStandardQueryParser()
	parser.SetDefaultField("content")
	parser.SetAnalyzer(analysis.NewStandardAnalyzer())

	t.Run("simple term", func(t *testing.T) {
		q, err := parser.Parse("hello")
		if err != nil {
			t.Fatal(err)
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
		if _, ok := q.(*search.PhraseQuery); !ok {
			t.Errorf("expected PhraseQuery, got %T", q)
		}
	})

	t.Run("fielded query", func(t *testing.T) {
		q, err := parser.Parse("title:test")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := q.(*search.TermQuery); !ok {
			t.Errorf("expected TermQuery, got %T", q)
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

	t.Run("boolean AND", func(t *testing.T) {
		q, err := parser.Parse("a AND b")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := q.(*search.BooleanQuery); !ok {
			t.Errorf("expected BooleanQuery, got %T", q)
		}
	})

	t.Run("not query", func(t *testing.T) {
		q, err := parser.Parse("hello")
		if err != nil {
			t.Fatal(err)
		}
		if q == nil {
			t.Fatal("expected non-nil query")
		}
	})
}

// TestMultiAnalyzerQPHelperSetDefaultConfig verifies config setters on the
// StandardQueryParser.
func TestMultiAnalyzerQPHelperSetDefaultConfig(t *testing.T) {
	parser := flexible.NewStandardQueryParser()
	parser.SetDefaultField("body")
	parser.SetDefaultOperator("AND")

	q, err := parser.Parse("a b")
	if err != nil {
		t.Fatal(err)
	}
	if q == nil {
		t.Fatal("expected non-nil query")
	}
}
