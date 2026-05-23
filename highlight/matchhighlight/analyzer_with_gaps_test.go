package matchhighlight

// Port of org.apache.lucene.search.matchhighlight.AnalyzerWithGaps.
//
// In Java this is a test-helper class that wraps an Analyzer and returns
// configurable offset/position gaps.  The Go port provides an equivalent
// AnalyzerWithGaps struct used by other matchhighlight test files, together
// with verification tests for its gap accessors.

import "testing"

// AnalyzerWithGaps is a test helper that stores configurable gap values for
// use by the match-highlight test pipeline.  It mirrors
// org.apache.lucene.search.matchhighlight.AnalyzerWithGaps.
type AnalyzerWithGaps struct {
	OffsetGap   int
	PositionGap int
}

// NewAnalyzerWithGaps builds the helper.
func NewAnalyzerWithGaps(offsetGap, positionGap int) *AnalyzerWithGaps {
	return &AnalyzerWithGaps{OffsetGap: offsetGap, PositionGap: positionGap}
}

// GetOffsetGap returns the offset gap for the given field.
func (a *AnalyzerWithGaps) GetOffsetGap(_ string) int { return a.OffsetGap }

// GetPositionIncrementGap returns the position increment gap for the given
// field.
func (a *AnalyzerWithGaps) GetPositionIncrementGap(_ string) int { return a.PositionGap }

// -- tests -------------------------------------------------------------------

func TestAnalyzerWithGaps_OffsetGap(t *testing.T) {
	cases := []struct {
		offsetGap   int
		positionGap int
	}{
		{1, 0},
		{0, 1},
		{10, 5},
		{100, 200},
	}
	for _, tc := range cases {
		a := NewAnalyzerWithGaps(tc.offsetGap, tc.positionGap)
		if got := a.GetOffsetGap("anyField"); got != tc.offsetGap {
			t.Errorf("offsetGap=%d positionGap=%d: GetOffsetGap got %d, want %d",
				tc.offsetGap, tc.positionGap, got, tc.offsetGap)
		}
		if got := a.GetPositionIncrementGap("anyField"); got != tc.positionGap {
			t.Errorf("offsetGap=%d positionGap=%d: GetPositionIncrementGap got %d, want %d",
				tc.offsetGap, tc.positionGap, got, tc.positionGap)
		}
	}
}

func TestAnalyzerWithGaps_FieldNameIsIgnored(t *testing.T) {
	// The gap values must be the same regardless of field name, mirroring the
	// Java DelegatingAnalyzerWrapper behaviour.
	a := NewAnalyzerWithGaps(7, 3)
	for _, field := range []string{"", "title", "body", "f1", "f2"} {
		if got := a.GetOffsetGap(field); got != 7 {
			t.Errorf("field=%q: GetOffsetGap got %d, want 7", field, got)
		}
		if got := a.GetPositionIncrementGap(field); got != 3 {
			t.Errorf("field=%q: GetPositionIncrementGap got %d, want 3", field, got)
		}
	}
}
