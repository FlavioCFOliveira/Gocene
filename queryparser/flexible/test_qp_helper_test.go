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

// TestQPHelper verifies basic StandardQueryParser parsing.
func TestQPHelper(t *testing.T) {
	parser := flexible.NewStandardQueryParser()
	parser.SetDefaultField("content")
	parser.SetAnalyzer(analysis.NewStandardAnalyzer())

	tests := []struct {
		name    string
		query   string
		wantNil bool
		check   func(t *testing.T, q search.Query)
	}{
		{
			name:  "simple term",
			query: "hello",
			check: func(t *testing.T, q search.Query) {
				if _, ok := q.(*search.TermQuery); !ok {
					t.Errorf("expected TermQuery, got %T", q)
				}
			},
		},
		{
			name:  "phrase",
			query: `"hello world"`,
			check: func(t *testing.T, q search.Query) {
				if _, ok := q.(*search.PhraseQuery); !ok {
					t.Errorf("expected PhraseQuery, got %T", q)
				}
			},
		},
		{
			name:  "boolean OR",
			query: "a OR b",
			check: func(t *testing.T, q search.Query) {
				if _, ok := q.(*search.BooleanQuery); !ok {
					t.Errorf("expected BooleanQuery, got %T", q)
				}
			},
		},
		{
			name:  "boolean AND",
			query: "a AND b",
			check: func(t *testing.T, q search.Query) {
				if _, ok := q.(*search.BooleanQuery); !ok {
					t.Errorf("expected BooleanQuery, got %T", q)
				}
			},
		},
		{
			name:  "required term",
			query: "+a",
			check: func(t *testing.T, q search.Query) {
				if q == nil {
					t.Fatal("expected non-nil query")
				}
			},
		},
		{
			name:  "prohibited term",
			query: "-a",
			check: func(t *testing.T, q search.Query) {
				if q == nil {
					t.Fatal("expected non-nil query")
				}
			},
		},
		{
			name:  "NOT term",
			query: "NOT a",
			check: func(t *testing.T, q search.Query) {
				if q == nil {
					t.Fatal("expected non-nil query")
				}
			},
		},
		{
			name:  "range query inclusive",
			query: "[a TO z]",
			check: func(t *testing.T, q search.Query) {
				if q == nil {
					t.Fatal("expected non-nil query")
				}
			},
		},
		{
			name:  "range query exclusive",
			query: "{a TO z}",
			check: func(t *testing.T, q search.Query) {
				if q == nil {
					t.Fatal("expected non-nil query")
				}
			},
		},
		{
			name:  "fielded range query",
			query: "field:{a TO z}",
			check: func(t *testing.T, q search.Query) {
				if q == nil {
					t.Fatal("expected non-nil query")
				}
			},
		},
		{
			name:  "fuzzy query",
			query: "hello~",
			check: func(t *testing.T, q search.Query) {
				if q == nil {
					t.Fatal("expected non-nil query")
				}
			},
		},
		{
			name:  "boost query",
			query: "hello^2",
			check: func(t *testing.T, q search.Query) {
				if q == nil {
					t.Fatal("expected non-nil query")
				}
			},
		},
		{
			name:  "wildcard query",
			query: "h*",
			check: func(t *testing.T, q search.Query) {
				if q == nil {
					t.Fatal("expected non-nil query")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, err := parser.Parse(tt.query)
			if err != nil {
				t.Fatalf("Parse %q: %v", tt.query, err)
			}
			if tt.check != nil {
				tt.check(t, q)
			}
		})
	}
}
