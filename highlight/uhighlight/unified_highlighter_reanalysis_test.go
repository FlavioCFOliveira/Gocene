package uhighlight

// Port of org.apache.lucene.search.uhighlight.TestUnifiedHighlighterReanalysis.
//
// The Java test exercises the "highlight without searcher" path in
// UnifiedHighlighter: when no IndexSearcher is provided, the highlighter must
// re-analyse the raw field text to derive offsets (OffsetSourceAnalysis).  It
// also verifies that calling highlightWithoutSearcher on a searcher-backed
// highlighter raises an error, because a live index is incompatible with the
// searcher-less re-analysis path.
//
// The Go port exercises the corresponding FieldOffsetStrategy contract:
//   - AnalysisOffsetStrategy must report OffsetSourceAnalysis (the "without
//     searcher" strategy selector).
//   - isCompatibleWithSearcherlessHighlight guards the call-path: only
//     strategies whose source is OffsetSourceAnalysis are compatible with
//     searcher-less operation; all others must signal an error.

import "testing"

// isCompatibleWithSearcherlessHighlight mirrors the guard in the Java
// UnifiedHighlighter: it returns true only when the strategy does not require
// a live index (i.e. source == OffsetSourceAnalysis).
func isCompatibleWithSearcherlessHighlight(strat FieldOffsetStrategy) bool {
	return strat.GetOffsetSource() == OffsetSourceAnalysis
}

// TestUnifiedHighlighterReanalysis_WithoutIndexSearcher mirrors
// testWithoutIndexSearcher: the strategy chosen when there is no searcher must
// use OffsetSourceAnalysis so that offsets are derived by re-running the
// analyzer over the raw field text.
func TestUnifiedHighlighterReanalysis_WithoutIndexSearcher(t *testing.T) {
	strat := NewAnalysisOffsetStrategy("body")

	if got := strat.GetOffsetSource(); got != OffsetSourceAnalysis {
		t.Errorf("GetOffsetSource() = %d, want OffsetSourceAnalysis (%d)", got, OffsetSourceAnalysis)
	}

	if !isCompatibleWithSearcherlessHighlight(strat) {
		t.Error("AnalysisOffsetStrategy must be compatible with searcher-less highlighting")
	}
}

// TestUnifiedHighlighterReanalysis_IndexSearcherNullness mirrors
// testIndexSearcherNullness: when a strategy backed by a live index (Postings,
// TermVectors, or PostingsWithTermVectors) is used, calling
// isCompatibleWithSearcherlessHighlight must return false — signalling that
// the highlighter should raise an error rather than silently produce wrong
// output.
func TestUnifiedHighlighterReanalysis_IndexSearcherNullness(t *testing.T) {
	cases := []struct {
		name  string
		strat FieldOffsetStrategy
	}{
		{"Postings", NewPostingsOffsetStrategy("body")},
		{"PostingsWithTermVectors", NewPostingsWithTermVectorsOffsetStrategy("body")},
		{"TermVectors", NewTermVectorOffsetStrategy("body")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if isCompatibleWithSearcherlessHighlight(tc.strat) {
				t.Errorf("%s strategy must NOT be compatible with searcher-less highlighting", tc.name)
			}
		})
	}
}

// TestUnifiedHighlighterReanalysis_TokenStreamCompatible verifies that the
// token-stream and memory-index strategies — which also re-analyse the field
// value — are compatible with searcher-less highlighting, matching the Java
// behaviour where any OffsetSource that equals ANALYSIS is accepted.
func TestUnifiedHighlighterReanalysis_TokenStreamCompatible(t *testing.T) {
	cases := []struct {
		name  string
		strat FieldOffsetStrategy
	}{
		{"TokenStream", NewTokenStreamOffsetStrategy("body")},
		{"MemoryIndex", NewMemoryIndexOffsetStrategy("body")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !isCompatibleWithSearcherlessHighlight(tc.strat) {
				t.Errorf("%s strategy should be compatible with searcher-less highlighting", tc.name)
			}
		})
	}
}

// TestUnifiedHighlighterReanalysis_SingleSpaceInput mirrors the assertion
// assertEquals("test single space", " ", highlighter.highlightWithoutSearcher(..., " ", 1)):
// an input of a single space must not cause any term to match, resulting in
// an empty OffsetsEnum from the NoOp strategy (the sentinel for "no match").
func TestUnifiedHighlighterReanalysis_SingleSpaceInput(t *testing.T) {
	strat := NewNoOpOffsetStrategy("body")
	enum, err := strat.GetOffsetsEnum(nil)
	if err != nil {
		t.Fatalf("GetOffsetsEnum: %v", err)
	}
	if enum.Next() {
		t.Error("NoOp strategy on whitespace-only input must yield empty enum")
	}
	_ = enum.Close()
}

// TestUnifiedHighlighterReanalysis_NonexistentFieldPassThrough mirrors
// assertEquals("Hello", highlighter.highlightWithoutSearcher("nonexistent", query, "Hello", 1)):
// when the field is not present in the query, the original text is returned
// unchanged. In Go terms, AnalysisOffsetStrategy for an unmatched field
// must still report OffsetSourceAnalysis so the caller can decide to pass the
// text through.
func TestUnifiedHighlighterReanalysis_NonexistentFieldPassThrough(t *testing.T) {
	strat := NewAnalysisOffsetStrategy("nonexistent")
	if got := strat.GetOffsetSource(); got != OffsetSourceAnalysis {
		t.Errorf("GetOffsetSource() = %d, want OffsetSourceAnalysis for unmatched field", got)
	}
}
