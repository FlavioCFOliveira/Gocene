package highlight

// TermVectorLeafReader is a thin wrapper around a single document's term
// vectors so highlighters can iterate (term, frequency, positions) tuples
// without coupling to the full LeafReader interface. Mirrors
// org.apache.lucene.search.highlight.TermVectorLeafReader.
type TermVectorLeafReader struct {
	field string
	terms []TermVectorEntry
}

// TermVectorEntry is a single term + occurrence record from the vector.
type TermVectorEntry struct {
	Term      string
	Frequency int
	Positions []int
}

// NewTermVectorLeafReader builds the reader for the supplied field and
// term-vector entries.
func NewTermVectorLeafReader(field string, terms []TermVectorEntry) *TermVectorLeafReader {
	clone := make([]TermVectorEntry, len(terms))
	for i, t := range terms {
		positions := make([]int, len(t.Positions))
		copy(positions, t.Positions)
		clone[i] = TermVectorEntry{Term: t.Term, Frequency: t.Frequency, Positions: positions}
	}
	return &TermVectorLeafReader{field: field, terms: clone}
}

// Field returns the field name.
func (r *TermVectorLeafReader) Field() string { return r.field }

// Terms returns a copy of the term-vector entries.
func (r *TermVectorLeafReader) Terms() []TermVectorEntry {
	out := make([]TermVectorEntry, len(r.terms))
	copy(out, r.terms)
	return out
}
