package matchhighlight

import "sort"

// PassageSelector picks the best N passages from a list of OffsetRanges and
// a desired character budget. Mirrors
// org.apache.lucene.search.matchhighlight.PassageSelector.
type PassageSelector struct {
	MaxPassages int
	MaxLength   int
}

// NewPassageSelector builds a selector.
func NewPassageSelector(maxPassages, maxLength int) *PassageSelector {
	if maxPassages < 1 {
		maxPassages = 1
	}
	if maxLength < 1 {
		maxLength = 1
	}
	return &PassageSelector{MaxPassages: maxPassages, MaxLength: maxLength}
}

// Select picks up to MaxPassages OffsetRanges from regions, sorted by
// document order, ensuring the cumulative length stays within MaxLength.
func (s *PassageSelector) Select(regions []OffsetRange) []OffsetRange {
	if len(regions) == 0 {
		return nil
	}
	clone := make([]OffsetRange, len(regions))
	copy(clone, regions)
	sort.SliceStable(clone, func(i, j int) bool { return clone[i].From < clone[j].From })
	out := make([]OffsetRange, 0, s.MaxPassages)
	total := 0
	for _, r := range clone {
		if len(out) >= s.MaxPassages {
			break
		}
		if total+r.Length() > s.MaxLength {
			break
		}
		out = append(out, r)
		total += r.Length()
	}
	return out
}
