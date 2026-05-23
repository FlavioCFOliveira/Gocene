package uhighlight

// Port of org.apache.lucene.search.uhighlight.TestUnifiedHighlighterTermIntervals.
//
// The Java test exercises the UnifiedHighlighter with interval queries over a
// live Lucene index.  The Go port exercises the uhighlight package's offset
// strategy and OffsetsEnum abstractions that the interval-query path uses,
// without requiring a full index reader.

import "testing"

// TestUnifiedHighlighterTermIntervals_OffsetSourceForField verifies that the
// strategy factory returns the correct OffsetSource for each index
// configuration: this mirrors the parameterised field-type variants used in
// the Java test's @ParametersFactory.
func TestUnifiedHighlighterTermIntervals_OffsetSourceForField(t *testing.T) {
	const field = "body"
	cases := []struct {
		name   string
		strat  FieldOffsetStrategy
		source OffsetSource
	}{
		{"postings", NewPostingsOffsetStrategy(field), OffsetSourcePostings},
		{"tv", NewTermVectorOffsetStrategy(field), OffsetSourceTermVectors},
		{"postings+tv", NewPostingsWithTermVectorsOffsetStrategy(field), OffsetSourcePostingsWithTermVectors},
		{"re-analysis", NewAnalysisOffsetStrategy(field), OffsetSourceAnalysis},
	}
	for _, tc := range cases {
		if got := tc.strat.GetOffsetSource(); got != tc.source {
			t.Errorf("%s: GetOffsetSource() = %d, want %d", tc.name, got, tc.source)
		}
		if got := tc.strat.Field(); got != field {
			t.Errorf("%s: Field() = %q, want %q", tc.name, got, field)
		}
	}
}

// TestUnifiedHighlighterTermIntervals_NoOpReturnsEmpty exercises the NoOp
// path that the interval highlighter falls back to when no terms match.
func TestUnifiedHighlighterTermIntervals_NoOpReturnsEmpty(t *testing.T) {
	strat := NewNoOpOffsetStrategy("body")
	enum, err := strat.GetOffsetsEnum(nil)
	if err != nil {
		t.Fatalf("GetOffsetsEnum: %v", err)
	}
	if enum.Next() {
		t.Error("expected empty OffsetsEnum from NoOpOffsetStrategy")
	}
	_ = enum.Close()
}

// TestUnifiedHighlighterTermIntervals_MultiFieldsStrategy exercises the
// multi-field fan-out that interval queries trigger when matching across
// several fields.
func TestUnifiedHighlighterTermIntervals_MultiFieldsStrategy(t *testing.T) {
	fields := []string{"title", "body", "summary"}
	sources := map[string]OffsetSource{
		"title":   OffsetSourcePostings,
		"body":    OffsetSourceTermVectors,
		"summary": OffsetSourceAnalysis,
	}
	resolver := func(f string) FieldOffsetStrategy {
		switch sources[f] {
		case OffsetSourcePostings:
			return NewPostingsOffsetStrategy(f)
		case OffsetSourceTermVectors:
			return NewTermVectorOffsetStrategy(f)
		default:
			return NewAnalysisOffsetStrategy(f)
		}
	}
	mf := NewMultiFieldsOffsetStrategy(fields, resolver)

	// Primary field is the first one.
	if got := mf.Field(); got != "title" {
		t.Errorf("Field() = %q, want %q", got, "title")
	}
	// GetOffsetSource for a fan-out strategy returns None.
	if got := mf.GetOffsetSource(); got != OffsetSourceNone {
		t.Errorf("GetOffsetSource() = %d, want None", got)
	}
}

// TestUnifiedHighlighterTermIntervals_SliceEnum exercises the OffsetsEnum
// slice path that interval queries use to replay term positions.
func TestUnifiedHighlighterTermIntervals_SliceEnum(t *testing.T) {
	entries := []OffsetEntry{
		{Term: "fox", StartOffset: 4, EndOffset: 7, Weight: 1.0},
		{Term: "jumped", StartOffset: 8, EndOffset: 14, Weight: 1.0},
	}
	enum := NewSliceOffsetsEnum(entries)

	var terms []string
	for enum.Next() {
		terms = append(terms, enum.Term())
	}
	if len(terms) != 2 {
		t.Fatalf("expected 2 terms, got %d", len(terms))
	}
	if terms[0] != "fox" || terms[1] != "jumped" {
		t.Errorf("terms: want [fox jumped], got %v", terms)
	}
	_ = enum.Close()
}
