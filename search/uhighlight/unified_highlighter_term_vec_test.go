package uhighlight

// Port of org.apache.lucene.search.uhighlight.TestUnifiedHighlighterTermVec.
//
// The Java test exercises term-vector highlighting paths: no-position TV,
// position-only TV, and FilterDirectoryReader-based isolation.  The Go port
// exercises the TermVectorFilteredLeafReader, FilteredTermsIterator, and
// TermVectorOffsetStrategy.

import (
	"testing"
)

// TestUnifiedHighlighterTermVec_TermVecButNoPositions mirrors
// testTermVecButNoPositions: when terms are present in both base and filter,
// the FilteredTermsIterator must surface them.
func TestUnifiedHighlighterTermVec_TermVecButNoPositions(t *testing.T) {
	cases := []struct {
		name    string
		aaa     string
		bbb     string
		indexed string
	}{
		{"xy", "x", "y", "y x"},
		{"yx", "y", "x", "y x"},
		{"zzzyyyy", "zzz", "yyy", "zzz yyy"},
		{"yyyyzzz", "zzz", "yyy", "yyy zzz"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate base = all terms, filter = subset.
			base := []TermVectorEntry{
				{Term: tc.aaa, Frequency: 1},
				{Term: tc.bbb, Frequency: 1},
			}
			filter := []TermVectorEntry{
				{Term: tc.aaa, Frequency: 1},
				{Term: tc.bbb, Frequency: 1},
			}
			it := NewFilteredTermsIterator(base, filter)
			count := 0
			for it.Next() {
				e, err := it.Entry()
				if err != nil {
					t.Errorf("Entry(): %v", err)
					continue
				}
				if e.Term == "" {
					t.Error("expected non-empty term")
				}
				count++
			}
			if count != 2 {
				t.Errorf("expected 2 terms, got %d", count)
			}
		})
	}
}

// TestUnifiedHighlighterTermVec_FilteredReader mirrors the filter-leaf-reader
// path: TermVectorFilteredLeafReader must only expose terms that pass the
// filter.
func TestUnifiedHighlighterTermVec_FilteredReader(t *testing.T) {
	base := &simpleTermSource{
		entries: map[string][]TermVectorEntry{
			"body": {
				{Term: "fox", Frequency: 2},
				{Term: "dog", Frequency: 1},
				{Term: "cat", Frequency: 3},
			},
		},
	}
	filter := []TermVectorEntry{
		{Term: "fox"},
		{Term: "dog"},
	}
	reader := NewTermVectorFilteredLeafReader(base, filter, "body")

	// Only "fox" and "dog" should appear for field "body".
	entries := reader.TermEntries("body")
	if len(entries) != 2 {
		t.Fatalf("expected 2 filtered entries, got %d: %v", len(entries), entries)
	}
	seen := map[string]bool{}
	for _, e := range entries {
		seen[e.Term] = true
	}
	if !seen["fox"] || !seen["dog"] {
		t.Errorf("expected fox and dog, got %v", seen)
	}
	if seen["cat"] {
		t.Error("'cat' should have been filtered out")
	}
}

// TestUnifiedHighlighterTermVec_FilteredReaderOtherField verifies that
// non-filtered fields pass through unchanged.
func TestUnifiedHighlighterTermVec_FilteredReaderOtherField(t *testing.T) {
	base := &simpleTermSource{
		entries: map[string][]TermVectorEntry{
			"title": {{Term: "hello"}, {Term: "world"}},
		},
	}
	filter := []TermVectorEntry{{Term: "fox"}} // filter is for "body", not "title"
	reader := NewTermVectorFilteredLeafReader(base, filter, "body")

	// "title" is not the filtered field; all entries must pass through.
	entries := reader.TermEntries("title")
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries for non-filtered field, got %d", len(entries))
	}
}

// TestUnifiedHighlighterTermVec_FilteredTermsMissingBase verifies that an
// entry present in filter but absent from base causes an error.
func TestUnifiedHighlighterTermVec_FilteredTermsMissingBase(t *testing.T) {
	base := []TermVectorEntry{
		{Term: "fox"},
	}
	filter := []TermVectorEntry{
		{Term: "ghost"}, // not in base
	}
	it := NewFilteredTermsIterator(base, filter)
	if !it.Next() {
		t.Fatal("expected at least one filter term")
	}
	_, err := it.Entry()
	if err == nil {
		t.Error("expected error for term missing from base")
	}
}

// TestUnifiedHighlighterTermVec_OffsetSource verifies that
// TermVectorOffsetStrategy reports OffsetSourceTermVectors.
func TestUnifiedHighlighterTermVec_OffsetSource(t *testing.T) {
	strat := NewTermVectorOffsetStrategy("body")
	if got := strat.GetOffsetSource(); got != OffsetSourceTermVectors {
		t.Errorf("GetOffsetSource() = %d, want %d", got, OffsetSourceTermVectors)
	}
}

// -- stubs -------------------------------------------------------------------

// simpleTermSource is a minimal TermSource backed by a map.
type simpleTermSource struct {
	entries map[string][]TermVectorEntry
}

func (s *simpleTermSource) TermEntries(field string) []TermVectorEntry {
	return s.entries[field]
}
