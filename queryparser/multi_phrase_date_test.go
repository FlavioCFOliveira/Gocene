// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package queryparser_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/queryparser"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestStandardQueryParser_MultiPhrase verifies that phrases containing
// synonym groups (separated by '|') produce MultiPhraseQuery.
func TestStandardQueryParser_MultiPhrase(t *testing.T) {
	parser := queryparser.NewStandardQueryParser()
	parser.SetDefaultField("text")

	tests := []struct {
		name     string
		input    string
		wantType string // "phrase" or "multi"
	}{
		{"simple phrase", `"quick brown fox"`, "phrase"},
		{"synonyms", `"quick|fast brown|red fox"`, "multi"},
		{"mixed", `"hello|hi world|earth"`, "multi"},
		{"slop with synonyms", `"quick|fast brown fox"~2`, "multi"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, err := parser.Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tt.input, err)
			}
			switch tt.wantType {
			case "phrase":
				if _, ok := q.(*search.PhraseQuery); !ok {
					t.Errorf("expected PhraseQuery, got %T", q)
				}
			case "multi":
				if _, ok := q.(*search.MultiPhraseQuery); !ok {
					t.Errorf("expected MultiPhraseQuery, got %T", q)
				}
			}
		})
	}

	t.Logf("MultiPhrase tests passed")
}

// TestStandardQueryParser_DateRange verifies date resolution in range queries.
func TestStandardQueryParser_DateRange(t *testing.T) {
	parser := queryparser.NewStandardQueryParser()
	parser.SetDefaultField("date")

	// Without date resolution, dates are treated as raw strings.
	q1, err := parser.Parse("[2024-01-01 TO 2024-12-31]")
	if err != nil {
		t.Fatalf("Parse date range: %v", err)
	}
	trq, ok := q1.(*search.TermRangeQuery)
	if !ok {
		t.Fatalf("expected TermRangeQuery, got %T", q1)
	}
	if string(trq.LowerTerm()) != "2024-01-01" {
		t.Errorf("without resolution, lower = %q, want '2024-01-01'", trq.LowerTerm())
	}

	// With DAY resolution, dates are resolved to compact terms.
	res := queryparser.ResolutionDay
	parser.SetDateResolution(&res)
	q2, err := parser.Parse("[2024-01-01 TO 2024-12-31]")
	if err != nil {
		t.Fatalf("Parse date range with resolution: %v", err)
	}
	trq2, ok := q2.(*search.TermRangeQuery)
	if !ok {
		t.Fatalf("expected TermRangeQuery, got %T", q2)
	}
	if string(trq2.LowerTerm()) != "20240101000000" {
		t.Errorf("with DAY resolution, lower = %q, want '20240101000000'", trq2.LowerTerm())
	}
	if string(trq2.UpperTerm()) != "20241231000000" {
		t.Errorf("with DAY resolution, upper = %q, want '20241231000000'", trq2.UpperTerm())
	}

	// Open-ended range with date resolution.
	q3, err := parser.Parse("[2024-01-01 TO *]")
	if err != nil {
		t.Fatalf("Parse open-ended date range: %v", err)
	}
	trq3, ok := q3.(*search.TermRangeQuery)
	if !ok {
		t.Fatalf("expected TermRangeQuery, got %T", q3)
	}
	if trq3.UpperTerm() != nil {
		t.Errorf("expected nil upper term for open range, got %q", trq3.UpperTerm())
	}

	t.Logf("DateRange tests passed")
}

// TestStandardQueryParser_DateResolution_Year verifies YEAR granularity.
func TestStandardQueryParser_DateResolution_Year(t *testing.T) {
	parser := queryparser.NewStandardQueryParser()
	parser.SetDefaultField("date")
	res := queryparser.ResolutionYear
	parser.SetDateResolution(&res)

	q, err := parser.Parse("[2024-06-15 TO 2025-03-20]")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	trq := q.(*search.TermRangeQuery)
	// Both rounded to Jan 1st of their respective years.
	if string(trq.LowerTerm()) != "20240101000000" {
		t.Errorf("YEAR resolution lower = %q, want '20240101000000'", trq.LowerTerm())
	}
	if string(trq.UpperTerm()) != "20250101000000" {
		t.Errorf("YEAR resolution upper = %q, want '20250101000000'", trq.UpperTerm())
	}

	t.Logf("Year resolution test passed")
}
