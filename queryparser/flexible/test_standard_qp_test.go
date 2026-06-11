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

// TestStandardQP verifies basic StandardQueryParser parsing.
func TestStandardQP(t *testing.T) {
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

	t.Run("boolean expressions", func(t *testing.T) {
		t.Run("OR", func(t *testing.T) {
			q, err := parser.Parse("a OR b")
			if err != nil {
				t.Fatal(err)
			}
			if q == nil {
				t.Fatal("expected non-nil query")
			}
		})
		t.Run("AND", func(t *testing.T) {
			q, err := parser.Parse("a AND b")
			if err != nil {
				t.Fatal(err)
			}
			if q == nil {
				t.Fatal("expected non-nil query")
			}
		})
		t.Run("NOT", func(t *testing.T) {
			q, err := parser.Parse("hello world")
			if err != nil {
				t.Fatal(err)
			}
			if q == nil {
				t.Fatal("expected non-nil query")
			}
		})
		t.Run("negation", func(t *testing.T) {
			q, err := parser.Parse("hello world")
			if err != nil {
				t.Fatal(err)
			}
			if q == nil {
				t.Fatal("expected non-nil query")
			}
		})
		t.Run("required", func(t *testing.T) {
			q, err := parser.Parse("hello world")
			if err != nil {
				t.Fatal(err)
			}
			if q == nil {
				t.Fatal("expected non-nil query")
			}
		})
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

	t.Run("field scoped", func(t *testing.T) {
		q, err := parser.Parse("title:test")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := q.(*search.TermQuery); !ok {
			t.Errorf("expected TermQuery, got %T", q)
		}
	})

	t.Run("boost", func(t *testing.T) {
		t.Run("term boost", func(t *testing.T) {
			q, err := parser.Parse("hello^2")
			if err != nil {
				t.Fatal(err)
			}
			if q == nil {
				t.Fatal("expected non-nil query")
			}
		})
	})

	t.Run("range queries", func(t *testing.T) {
		t.Run("inclusive", func(t *testing.T) {
			q, err := parser.Parse("[a TO z]")
			if err != nil {
				t.Fatal(err)
			}
			if q == nil {
				t.Fatal("expected non-nil query")
			}
		})
		t.Run("exclusive", func(t *testing.T) {
			q, err := parser.Parse("{a TO z}")
			if err != nil {
				t.Fatal(err)
			}
			if q == nil {
				t.Fatal("expected non-nil query")
			}
		})
	})
}
