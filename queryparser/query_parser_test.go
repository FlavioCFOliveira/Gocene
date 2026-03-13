// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package queryparser

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

func TestQueryParser_Parse(t *testing.T) {
	parser := NewQueryParserWithDefaultField("content")

	tests := []struct {
		name       string
		query      string
		wantNonNil bool
		wantType   string
		wantErr    bool
	}{
		{
			name:       "empty query",
			query:      "",
			wantNonNil: true,
			wantType:   "*search.MatchAllDocsQuery",
		},
		{
			name:       "simple term",
			query:      "hello",
			wantNonNil: true,
			wantType:   "*search.TermQuery",
		},
		{
			name:       "AND operator",
			query:      "hello AND world",
			wantNonNil: true,
			wantType:   "*search.BooleanQuery",
		},
		{
			name:       "OR operator",
			query:      "hello OR world",
			wantNonNil: true,
			wantType:   "*search.BooleanQuery",
		},
		{
			name:       "NOT operator",
			query:      "NOT hello",
			wantNonNil: true,
			wantType:   "*search.BooleanQuery",
		},
		{
			name:       "required term",
			query:      "+hello",
			wantNonNil: true,
			wantType:   "*search.BooleanQuery",
		},
		{
			name:       "prohibited term",
			query:      "-hello",
			wantNonNil: true,
			wantType:   "*search.BooleanQuery",
		},
		{
			name:       "grouped expression",
			query:      "(hello world)",
			wantNonNil: true,
			wantType:   "*search.BooleanQuery",
		},
		{
			name:       "field query",
			query:      "title:hello",
			wantNonNil: true,
			wantType:   "*search.TermQuery",
		},
		{
			name:       "wildcard",
			query:      "hel*",
			wantNonNil: true,
			wantType:   "*search.WildcardQuery",
		},
		{
			name:       "fuzzy",
			query:      "hello~",
			wantNonNil: true,
			wantType:   "*search.FuzzyQuery",
		},
		{
			name:       "fuzzy with distance",
			query:      "hello~2",
			wantNonNil: true,
			wantType:   "*search.FuzzyQuery",
		},
		{
			name:       "boost",
			query:      "hello^2",
			wantNonNil: true,
			wantType:   "*search.BoostQuery",
		},
		{
			name:       "phrase",
			query:      `"hello world"`,
			wantNonNil: true,
			wantType:   "*search.PhraseQuery",
		},
		{
			name:       "range inclusive",
			query:      "[a TO z]",
			wantNonNil: true,
			wantType:   "*search.TermRangeQuery",
		},
		{
			name:       "range exclusive",
			query:      "{a TO z}",
			wantNonNil: true,
			wantType:   "*search.TermRangeQuery",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.Parse(tt.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if tt.wantNonNil && got == nil {
					t.Errorf("Parse() returned nil, want non-nil")
					return
				}
				gotType := getTypeName(got)
				if tt.wantType != "" && gotType != tt.wantType {
					t.Errorf("Parse() returned type %s, want %s", gotType, tt.wantType)
				}
			}
		})
	}
}

func TestQueryParser_ParseComplex(t *testing.T) {
	parser := NewQueryParserWithDefaultField("content")

	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{
			name:  "field with boolean",
			query: "title:quick AND fox",
		},
		{
			name:  "required and prohibited",
			query: "+must -mustnot optional",
		},
		{
			name:  "boosted phrase",
			query: `"exact match"^10`,
		},
		{
			name:  "range with field",
			query: "date:[20240101 TO 20241231]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.Parse(tt.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got == nil && !tt.wantErr {
				t.Errorf("Parse() returned nil")
			}
		})
	}
}

func TestQueryParser_GetSet(t *testing.T) {
	parser := NewQueryParserWithDefaultField("content")

	// Test GetDefaultField
	if parser.GetDefaultField() != "content" {
		t.Errorf("GetDefaultField() = %v, want content", parser.GetDefaultField())
	}

	// Test SetDefaultField
	parser.SetDefaultField("title")
	if parser.GetDefaultField() != "title" {
		t.Errorf("GetDefaultField() after SetDefaultField = %v, want title", parser.GetDefaultField())
	}
}

// getTypeName returns the type name of a query for testing.
func getTypeName(q search.Query) string {
	if q == nil {
		return "nil"
	}
	switch q.(type) {
	case *search.TermQuery:
		return "*search.TermQuery"
	case *search.BooleanQuery:
		return "*search.BooleanQuery"
	case *search.PhraseQuery:
		return "*search.PhraseQuery"
	case *search.WildcardQuery:
		return "*search.WildcardQuery"
	case *search.FuzzyQuery:
		return "*search.FuzzyQuery"
	case *search.BoostQuery:
		return "*search.BoostQuery"
	case *search.TermRangeQuery:
		return "*search.TermRangeQuery"
	case *search.MatchAllDocsQuery:
		return "*search.MatchAllDocsQuery"
	default:
		return "unknown"
	}
}
