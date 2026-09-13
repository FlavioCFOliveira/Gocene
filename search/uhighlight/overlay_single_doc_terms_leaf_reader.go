package uhighlight

// OverlaySingleDocTermsLeafReader is the wrapper used by the unified
// highlighter to expose just one document's terms to a Terms-consuming
// component without re-indexing. Mirrors
// org.apache.lucene.search.uhighlight.OverlaySingleDocTermsLeafReader.
//
// The Go port focuses on the (term, frequency, positions) tuple set since
// the full LeafReader contract lives in the index package.
type OverlaySingleDocTermsLeafReader struct {
	Field string
	Terms []TermVectorEntry
}

// TermVectorEntry mirrors the highlight.TermVectorEntry tuple without
// importing the highlight package.
//
// StartOffsets/EndOffsets carry the per-occurrence character ranges when
// term vectors are stored WITH_OFFSETS (Lucene 10.4.0 indexing option).
// When term vectors are stored WITHOUT_OFFSETS, these slices are nil and
// the term-vector offset strategy must fall back to re-analysis to
// resolve passage windows. The slice length aligns with the per-term
// occurrence count: i.e. len(StartOffsets) == len(EndOffsets) == Frequency
// when offsets are present.
type TermVectorEntry struct {
	Term         string
	Frequency    int
	Positions    []int
	StartOffsets []int
	EndOffsets   []int
}

// NewOverlaySingleDocTermsLeafReader builds the wrapper.
func NewOverlaySingleDocTermsLeafReader(field string, terms []TermVectorEntry) *OverlaySingleDocTermsLeafReader {
	clone := make([]TermVectorEntry, len(terms))
	for i, t := range terms {
		positions := append([]int(nil), t.Positions...)
		starts := append([]int(nil), t.StartOffsets...)
		ends := append([]int(nil), t.EndOffsets...)
		clone[i] = TermVectorEntry{
			Term:         t.Term,
			Frequency:    t.Frequency,
			Positions:    positions,
			StartOffsets: starts,
			EndOffsets:   ends,
		}
	}
	return &OverlaySingleDocTermsLeafReader{Field: field, Terms: clone}
}
