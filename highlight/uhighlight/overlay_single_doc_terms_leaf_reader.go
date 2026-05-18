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
type TermVectorEntry struct {
	Term      string
	Frequency int
	Positions []int
}

// NewOverlaySingleDocTermsLeafReader builds the wrapper.
func NewOverlaySingleDocTermsLeafReader(field string, terms []TermVectorEntry) *OverlaySingleDocTermsLeafReader {
	clone := make([]TermVectorEntry, len(terms))
	for i, t := range terms {
		positions := make([]int, len(t.Positions))
		copy(positions, t.Positions)
		clone[i] = TermVectorEntry{Term: t.Term, Frequency: t.Frequency, Positions: positions}
	}
	return &OverlaySingleDocTermsLeafReader{Field: field, Terms: clone}
}
