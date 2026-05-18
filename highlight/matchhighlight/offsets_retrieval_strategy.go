package matchhighlight

// OffsetsRetrievalStrategy chooses how the matches API extracts character
// offsets from a per-document Matches iterator. Mirrors the abstract
// org.apache.lucene.search.matchhighlight.OffsetsRetrievalStrategy.
type OffsetsRetrievalStrategy interface {
	// Retrieve returns the merged OffsetRanges for the supplied iterator.
	Retrieve(iter MatchIterator) ([]OffsetRange, error)
}

// MatchIterator is the minimal iterator the matchhighlight strategies need.
// It mirrors the slice of methods the Lucene MatchesIterator exposes.
type MatchIterator interface {
	// Next returns false once the iterator is exhausted.
	Next() bool
	// StartOffset returns the character offset of the current match.
	StartOffset() int
	// EndOffset returns the exclusive end character offset.
	EndOffset() int
	// StartPosition returns the token position of the match.
	StartPosition() int
	// EndPosition returns the token position past the match.
	EndPosition() int
}
