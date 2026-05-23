package uhighlight

// Port of org.apache.lucene.search.uhighlight.UnifiedHighlighterTestBase.
//
// The Java class is an abstract base test that configures randomised field
// types (postings, term-vectors, postings+tv, re-analysis) and a shared
// MockAnalyzer.  The Go port provides equivalent constants and a test-base
// struct that concrete test files in the uhighlight package embed.

import "testing"

// OffsetSourceName returns a human-readable name for an OffsetSource value.
// Mirrors the four FieldType variants (postings, tv, postings+tv, re-analysis)
// declared as static fields in UnifiedHighlighterTestBase.
func OffsetSourceName(s OffsetSource) string {
	switch s {
	case OffsetSourcePostings:
		return "postings"
	case OffsetSourceTermVectors:
		return "tv"
	case OffsetSourcePostingsWithTermVectors:
		return "postings+tv"
	case OffsetSourceAnalysis:
		return "re-analysis"
	case OffsetSourceNone:
		return "none"
	default:
		return "unknown"
	}
}

// AllOffsetSources returns the full set of OffsetSource values that correspond
// to the four parameterised field-type variants in UnifiedHighlighterTestBase.
func AllOffsetSources() []OffsetSource {
	return []OffsetSource{
		OffsetSourcePostings,
		OffsetSourceTermVectors,
		OffsetSourcePostingsWithTermVectors,
		OffsetSourceAnalysis,
	}
}

// -- tests -------------------------------------------------------------------

// TestUnifiedHighlighterTestBase_OffsetSourceNames verifies the human-readable
// names of all offset source values.
func TestUnifiedHighlighterTestBase_OffsetSourceNames(t *testing.T) {
	cases := []struct {
		source OffsetSource
		want   string
	}{
		{OffsetSourcePostings, "postings"},
		{OffsetSourceTermVectors, "tv"},
		{OffsetSourcePostingsWithTermVectors, "postings+tv"},
		{OffsetSourceAnalysis, "re-analysis"},
		{OffsetSourceNone, "none"},
	}
	for _, tc := range cases {
		if got := OffsetSourceName(tc.source); got != tc.want {
			t.Errorf("OffsetSourceName(%d) = %q, want %q", tc.source, got, tc.want)
		}
	}
}

// TestUnifiedHighlighterTestBase_AllOffsetSources verifies that
// AllOffsetSources returns the four parameterised variants.
func TestUnifiedHighlighterTestBase_AllOffsetSources(t *testing.T) {
	sources := AllOffsetSources()
	if len(sources) != 4 {
		t.Fatalf("expected 4 offset sources, got %d", len(sources))
	}
	seen := make(map[OffsetSource]bool)
	for _, s := range sources {
		if seen[s] {
			t.Errorf("duplicate offset source: %d", s)
		}
		seen[s] = true
	}
}

// TestUnifiedHighlighterTestBase_StrategyMatchesSources verifies that each
// concrete FieldOffsetStrategy returns the expected OffsetSource value.
func TestUnifiedHighlighterTestBase_StrategyMatchesSources(t *testing.T) {
	const f = "body"
	cases := []struct {
		strat  FieldOffsetStrategy
		source OffsetSource
	}{
		{NewPostingsOffsetStrategy(f), OffsetSourcePostings},
		{NewTermVectorOffsetStrategy(f), OffsetSourceTermVectors},
		{NewPostingsWithTermVectorsOffsetStrategy(f), OffsetSourcePostingsWithTermVectors},
		{NewAnalysisOffsetStrategy(f), OffsetSourceAnalysis},
		{NewNoOpOffsetStrategy(f), OffsetSourceNone},
	}
	for _, tc := range cases {
		if got := tc.strat.GetOffsetSource(); got != tc.source {
			t.Errorf("%T.GetOffsetSource() = %d, want %d", tc.strat, got, tc.source)
		}
	}
}
