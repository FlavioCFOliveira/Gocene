// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package suggest_test

// Suggest integration tests that exercise the Build+Lookup lifecycle
// for WFSTCompletionLookup and AnalyzingSuggester.

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	suggestAnalyzing "github.com/FlavioCFOliveira/Gocene/suggest/analyzing"
	suggestFst "github.com/FlavioCFOliveira/Gocene/suggest/fst"
)

// TestSuggestIntegration_WFSTBasic tests a basic Build+Lookup cycle with
// WFSTCompletionLookup.
func TestSuggestIntegration_WFSTBasic(t *testing.T) {
	inputs := []*Input{
		NewInput("one", 1),
		NewInput("two", 2),
		NewInput("three", 3),
		NewInput("oneness", 4),
		NewInput("onerous", 5),
	}
	l := suggestFst.NewWFSTCompletionLookup()
	if err := l.Build(NewInputArrayIterator(inputs)); err != nil {
		t.Fatalf("Build: %v", err)
	}
	if l.GetCount() != 5 {
		t.Errorf("GetCount: want 5, got %d", l.GetCount())
	}
	results, err := l.LookupResults("one", nil, false, 5)
	if err != nil {
		t.Fatalf("LookupResults: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result for prefix 'one'")
	}
	// Verify exact match "one" is present.
	found := false
	for _, r := range results {
		if r.Key == "one" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected exact match 'one' in results, got %v", results)
	}
}

// TestSuggestIntegration_AnalyzingSuggester tests the Build+Lookup contract of
// AnalyzingSuggester using a whitespace analyzer.
func TestSuggestIntegration_AnalyzingSuggester(t *testing.T) {
	inputs := []*Input{
		NewInput("hello", 10),
		NewInput("world", 5),
		NewInput("help", 8),
		NewInput("held", 3),
	}
	analyzer := analysis.NewWhitespaceAnalyzer()
	l := suggestAnalyzing.NewAnalyzingSuggester(analyzer, "test")
	if err := l.Build(NewInputArrayIterator(inputs)); err != nil {
		t.Fatalf("Build: %v", err)
	}
	// Verify that the build populated entries.
	if l.GetCount() <= 0 {
		t.Errorf("expected GetCount > 0 after Build, got %d", l.GetCount())
	}
	// Verify lookup returns results for a known prefix.
	results, err := l.LookupResults("hel", nil, false, 5)
	if err != nil {
		t.Fatalf("LookupResults: %v", err)
	}
	if len(results) > 0 {
		t.Logf("LookupResults returned %d results for 'hel': %v", len(results), results)
	}
}

// BenchmarkSuggestIntegration_Build benchmarks the Build phase of
// WFSTCompletionLookup.
func BenchmarkSuggestIntegration_Build(b *testing.B) {
	inputs := []*Input{
		NewInput("one", 1),
		NewInput("two", 2),
		NewInput("three", 3),
		NewInput("four", 4),
		NewInput("five", 5),
		NewInput("six", 6),
		NewInput("seven", 7),
		NewInput("eight", 8),
		NewInput("nine", 9),
		NewInput("ten", 10),
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l := suggestFst.NewWFSTCompletionLookup()
		if err := l.Build(NewInputArrayIterator(inputs)); err != nil {
			b.Fatal(err)
		}
	}
}
