package uhighlight

// OffsetSource is the source of term offsets.
type OffsetSource int

const (
	SourcePostings OffsetSource = iota
	SourceTermVectors
	SourceAnalysis
	SourcePostingsWithTermVectors
	SourceNoneNeeded
)

// HighlightFlag is a flag for controlling highlighting behavior.
type HighlightFlag int

const (
	FlagPhrases HighlightFlag = iota
	FlagMultiTermQuery
	FlagPassageRelevancyOverSpeed
	FlagWeightMatches
)

// FlagSet is a set of HighlightFlags.
type FlagSet map[HighlightFlag]struct{}

func (s FlagSet) Contains(f HighlightFlag) bool {
	_, ok := s[f]
	return ok
}

func (s FlagSet) Add(f HighlightFlag) {
	s[f] = struct{}{}
}
