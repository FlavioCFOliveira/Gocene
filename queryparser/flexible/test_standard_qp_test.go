// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/queryparser/flexible"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestStandardQP is a broad regression test for the StandardQueryParser covering
// operator precedence, field scoping, boost, and basic syntax parsing.
//
// Port of: org.apache.lucene.queryparser.flexible.standard.TestStandardQP
// (subset — range query type selection, FuzzyQuery production, and full analyzer
// pipeline integration are not yet wired in Gocene.)
func TestStandardQP(t *testing.T) {
	parser := flexible.NewStandardQueryParser()

	t.Run("default field set", func(t *testing.T) {
		parser.SetDefaultField("text")
		if parser.GetConfig().GetDefaultField() != "text" {
			t.Errorf("default field should be 'text', got %s", parser.GetConfig().GetDefaultField())
		}
	})

	t.Run("operator precedence: AND before OR", func(t *testing.T) {
		parser.SetDefaultField("f")
		// "a OR b AND c" should parse as "a OR (b AND c)" (AND binds tighter)
		q, err := parser.Parse("a OR b AND c")
		if err != nil {
			t.Fatal(err)
		}
		bq, ok := q.(*search.BooleanQuery)
		if !ok {
			t.Fatalf("expected BooleanQuery, got %T", q)
		}
		if len(bq.Clauses()) < 2 {
			t.Errorf("expected at least 2 clauses in outer boolean, got %d", len(bq.Clauses()))
		}
	})

	t.Run("field scoping", func(t *testing.T) {
		parser.SetDefaultField("f")

		t.Run("simple field scoped", func(t *testing.T) {
			q, err := parser.Parse("title:hello")
			if err != nil {
				t.Fatal(err)
			}
			tq, ok := q.(*search.TermQuery)
			if !ok {
				t.Fatalf("expected TermQuery, got %T", q)
			}
			_ = tq
		})

		t.Run("field scoped phrase", func(t *testing.T) {
			q, err := parser.Parse(`title:"hello world"`)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := q.(*search.PhraseQuery); !ok {
				t.Errorf("expected PhraseQuery for fielded phrase, got %T", q)
			}
		})

		t.Run("multiple field scoped terms", func(t *testing.T) {
			q, err := parser.Parse("title:a AND body:b")
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := q.(*search.BooleanQuery); !ok {
				t.Errorf("expected BooleanQuery, got %T", q)
			}
		})
	})

	t.Run("boolean expressions", func(t *testing.T) {
		parser.SetDefaultField("f")

		t.Run("simple AND", func(t *testing.T) {
			q, err := parser.Parse("a AND b")
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := q.(*search.BooleanQuery); !ok {
				t.Errorf("expected BooleanQuery, got %T", q)
			}
		})

		t.Run("simple OR", func(t *testing.T) {
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
				t.Error("expected non-nil query")
			}
		})

		t.Run("negation", func(t *testing.T) {
			q, err := parser.Parse("-a")
			if err != nil {
				t.Fatal(err)
			}
			if q == nil {
				t.Error("expected non-nil query")
			}
		})

		t.Run("required", func(t *testing.T) {
			q, err := parser.Parse("+a")
			if err != nil {
				t.Fatal(err)
			}
			if q == nil {
				t.Error("expected non-nil query")
			}
		})
	})

	t.Run("grouped expression", func(t *testing.T) {
		parser.SetDefaultField("f")

		q, err := parser.Parse("(a OR b) AND c")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := q.(*search.BooleanQuery); !ok {
			t.Errorf("expected BooleanQuery, got %T", q)
		}
	})

	t.Run("boost", func(t *testing.T) {
		parser.SetDefaultField("f")

		t.Run("term boost", func(t *testing.T) {
			q, err := parser.Parse("a^3.0")
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := q.(*search.BoostQuery); !ok {
				t.Errorf("expected BoostQuery, got %T", q)
			}
		})
	})

	t.Run("wildcard queries", func(t *testing.T) {
		parser.SetDefaultField("f")

		q, err := parser.Parse("test*")
		if err != nil {
			t.Fatal(err)
		}
		if q == nil {
			t.Fatal("expected non-nil query for prefix wildcard")
		}
	})

	t.Run("range queries", func(t *testing.T) {
		parser.SetDefaultField("f")

		t.Run("inclusive", func(t *testing.T) {
			q, err := parser.Parse("[1 TO 10]")
			if err != nil {
				t.Fatal(err)
			}
			trq, ok := q.(*search.TermRangeQuery)
			if !ok {
				t.Fatalf("expected TermRangeQuery, got %T", q)
			}
			if !trq.IncludesLower() || !trq.IncludesUpper() {
				t.Error("expected both bounds inclusive")
			}
		})

		t.Run("exclusive", func(t *testing.T) {
			q, err := parser.Parse("{a TO z}")
			if err != nil {
				t.Fatal(err)
			}
			trq, ok := q.(*search.TermRangeQuery)
			if !ok {
				t.Fatalf("expected TermRangeQuery, got %T", q)
			}
			if trq.IncludesLower() || trq.IncludesUpper() {
				t.Error("expected both bounds exclusive")
			}
		})
	})

	t.Run("phrase queries", func(t *testing.T) {
		parser.SetDefaultField("f")

		t.Run("basic phrase", func(t *testing.T) {
			q, err := parser.Parse(`"hello world"`)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := q.(*search.PhraseQuery); !ok {
				t.Errorf("expected PhraseQuery, got %T", q)
			}
		})
	})

	t.Run("empty query", func(t *testing.T) {
		_, err := parser.Parse("")
		if err == nil {
			t.Error("expected error for empty query")
		}
	})

	t.Run("error handling - whitespace only", func(t *testing.T) {
		_, err := parser.Parse("   ")
		if err == nil {
			t.Error("expected error for whitespace-only query")
		}
	})
}
