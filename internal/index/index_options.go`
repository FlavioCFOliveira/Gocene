package index

// IndexOptions controls how much information is stored in the postings lists.
type IndexOptions int

const (
	IndexOptionsNone IndexOptions = iota
	IndexOptionsDocs
	IndexOptionsDocsAndFreqs
	IndexOptionsDocsAndFreqsAndPositions
	IndexOptionsDocsAndFreqsAndPositionsAndOffsets
	IndexOptionsDocsAndCustomFreqs
)

// Subsumes returns true if the index format encoding this IndexOptions includes the bits
// required for the other index format.
func (io IndexOptions) Subsumes(other IndexOptions) bool {
	if io == IndexOptionsDocsAndCustomFreqs {
		return IndexOptionsDocsAndFreqs.Subsumes(other)
	}
	if other == IndexOptionsDocsAndCustomFreqs {
		return io.subsumes(IndexOptionsDocsAndFreqs)
	}
	return int(io) >= int(other)
}

func (io IndexOptions) subsumes(other IndexOptions) bool {
	return int(io) >= int(other)
}
