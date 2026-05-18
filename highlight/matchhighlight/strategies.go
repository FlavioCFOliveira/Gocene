package matchhighlight

// OffsetsFromMatchIterator is the default strategy: it reads the character
// offsets directly from the iterator (one OffsetRange per match). Mirrors
// org.apache.lucene.search.matchhighlight.OffsetsFromMatchIterator.
type OffsetsFromMatchIterator struct{}

// Retrieve returns the offsets reported by the iterator verbatim.
func (OffsetsFromMatchIterator) Retrieve(iter MatchIterator) ([]OffsetRange, error) {
	var out []OffsetRange
	for iter.Next() {
		out = append(out, NewOffsetRange(iter.StartOffset(), iter.EndOffset()))
	}
	return out, nil
}

var _ OffsetsRetrievalStrategy = OffsetsFromMatchIterator{}

// OffsetsFromPositions is the position-based strategy: positions are
// converted to character offsets via the supplied token-position-to-offset
// table. Mirrors org.apache.lucene.search.matchhighlight.OffsetsFromPositions.
type OffsetsFromPositions struct {
	Positions []OffsetRange // index = token position
}

// Retrieve walks position ranges and translates them to character offsets.
func (s *OffsetsFromPositions) Retrieve(iter MatchIterator) ([]OffsetRange, error) {
	var out []OffsetRange
	for iter.Next() {
		startPos := iter.StartPosition()
		endPos := iter.EndPosition()
		if startPos < 0 || endPos > len(s.Positions) {
			continue
		}
		from := s.Positions[startPos].From
		to := s.Positions[endPos-1].To
		out = append(out, NewOffsetRange(from, to))
	}
	return out, nil
}

var _ OffsetsRetrievalStrategy = (*OffsetsFromPositions)(nil)

// OffsetsFromTokens walks a pre-tokenised list of OffsetRanges and emits the
// ranges whose positions appear in the iterator. Mirrors
// org.apache.lucene.search.matchhighlight.OffsetsFromTokens.
type OffsetsFromTokens struct {
	TokenOffsets []OffsetRange
}

// Retrieve walks the iterator and returns the token offsets at the matching
// positions.
func (s *OffsetsFromTokens) Retrieve(iter MatchIterator) ([]OffsetRange, error) {
	var out []OffsetRange
	for iter.Next() {
		pos := iter.StartPosition()
		if pos < 0 || pos >= len(s.TokenOffsets) {
			continue
		}
		out = append(out, s.TokenOffsets[pos])
	}
	return out, nil
}

var _ OffsetsRetrievalStrategy = (*OffsetsFromTokens)(nil)

// OffsetsFromValues is the strategy backed by the original field values.
// Mirrors org.apache.lucene.search.matchhighlight.OffsetsFromValues.
type OffsetsFromValues struct {
	// Values is the slice of source-text strings the highlighter is rendering.
	Values []string
}

// Retrieve is a passthrough: the matches API already provides character
// offsets so values are only consulted at render time.
func (s *OffsetsFromValues) Retrieve(iter MatchIterator) ([]OffsetRange, error) {
	return OffsetsFromMatchIterator{}.Retrieve(iter)
}

var _ OffsetsRetrievalStrategy = (*OffsetsFromValues)(nil)
