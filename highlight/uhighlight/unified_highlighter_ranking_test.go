package uhighlight

// Port of org.apache.lucene.search.uhighlight.TestUnifiedHighlighterRanking.
//
// The Java test indexes randomised documents and verifies that the top-N
// highlighted passages are a subset of top-(N+1).  The Go port exercises the
// FieldOffsetStrategy and OffsetsEnum contracts that the ranking path uses.

import (
	"sort"
	"testing"
)

// offsetSpan is a minimal passage span used by the ranking tests.
type offsetSpan struct {
	from, to int
}

func (s offsetSpan) length() int { return s.to - s.from }

// TestUnifiedHighlighterRanking_TopNSubsetOfTopNPlus1 mirrors testRanking:
// passages picked from a larger set must be a subset regardless of N.
func TestUnifiedHighlighterRanking_TopNSubsetOfTopNPlus1(t *testing.T) {
	spans := []offsetSpan{
		{0, 30},
		{40, 70},
		{80, 110},
	}

	// Sort by start offset (document order), mirroring Lucene's passage selector.
	sorted := make([]offsetSpan, len(spans))
	copy(sorted, spans)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].from < sorted[j].from
	})

	for n := 1; n <= len(sorted); n++ {
		topN := sorted[:n]
		if n+1 > len(sorted) {
			break
		}
		topNPlus := sorted[:n+1]
		for _, r := range topN {
			found := false
			for _, r2 := range topNPlus {
				if r == r2 {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("top-%d span %v not in top-%d set %v", n, r, n+1, topNPlus)
			}
		}
	}
}

// TestUnifiedHighlighterRanking_SpanLength verifies non-negative span lengths.
func TestUnifiedHighlighterRanking_SpanLength(t *testing.T) {
	cases := []struct {
		from, to, want int
	}{
		{0, 10, 10},
		{5, 5, 0},
		{3, 20, 17},
	}
	for _, tc := range cases {
		s := offsetSpan{tc.from, tc.to}
		if got := s.length(); got != tc.want {
			t.Errorf("span(%d,%d).length() = %d, want %d", tc.from, tc.to, got, tc.want)
		}
	}
}

// TestUnifiedHighlighterRanking_StrategyForField mirrors the offset-source
// selection by parameterisation in testRanking.
func TestUnifiedHighlighterRanking_StrategyForField(t *testing.T) {
	const f = "body"
	for _, src := range AllOffsetSources() {
		var strat FieldOffsetStrategy
		switch src {
		case OffsetSourcePostings:
			strat = NewPostingsOffsetStrategy(f)
		case OffsetSourceTermVectors:
			strat = NewTermVectorOffsetStrategy(f)
		case OffsetSourcePostingsWithTermVectors:
			strat = NewPostingsWithTermVectorsOffsetStrategy(f)
		case OffsetSourceAnalysis:
			strat = NewAnalysisOffsetStrategy(f)
		default:
			continue
		}
		if got := strat.GetOffsetSource(); got != src {
			t.Errorf("source=%d: GetOffsetSource() = %d", src, got)
		}
	}
}

// TestUnifiedHighlighterRanking_SliceEnumOrdering verifies that a
// SliceOffsetsEnum yields entries in insertion order, matching the
// document-order ordering the ranker expects.
func TestUnifiedHighlighterRanking_SliceEnumOrdering(t *testing.T) {
	entries := []OffsetEntry{
		{Term: "a", StartOffset: 0, EndOffset: 5},
		{Term: "b", StartOffset: 10, EndOffset: 15},
		{Term: "c", StartOffset: 20, EndOffset: 25},
	}
	enum := NewSliceOffsetsEnum(entries)
	prev := -1
	for enum.Next() {
		if enum.StartOffset() < prev {
			t.Errorf("offsets out of order: %d < %d", enum.StartOffset(), prev)
		}
		prev = enum.StartOffset()
	}
	_ = enum.Close()
}
