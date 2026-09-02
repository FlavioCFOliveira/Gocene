package matchhighlight

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// OffsetsFromMatchIterator retrieves offsets directly from search.MatchesIterator,
// if they are available, otherwise it falls back to using OffsetsFromPositions.
type OffsetsFromMatchIterator struct {
	field             string
	noOffsetsFallback *OffsetsFromPositions
}

// NewOffsetsFromMatchIterator creates a new OffsetsFromMatchIterator.
func NewOffsetsFromMatchIterator(field string, noOffsetsFallback *OffsetsFromPositions) *OffsetsFromMatchIterator {
	return &OffsetsFromMatchIterator{
		field:             field,
		noOffsetsFallback: noOffsetsFallback,
	}
}

// Get retrieves offsets from the MatchesIterator or falls back to the position-based strategy.
func (o *OffsetsFromMatchIterator) Get(matchesIterator search.MatchesIterator, doc FieldValueProvider) ([]OffsetRange, error) {
	var positionRanges []OffsetRange
	var offsetRanges []OffsetRange
	offsetsAvailable := true

	for {
		next, err := matchesIterator.Next()
		if err != nil {
			return nil, err
		}
		if !next {
			break
		}

		fromPos := matchesIterator.StartPosition()
		toPos := matchesIterator.EndPosition()
		if fromPos < 0 || toPos < 0 {
			return nil, fmt.Errorf("matches API returned negative positions for field: %s", o.field)
		}
		positionRanges = append(positionRanges, OffsetRange{From: fromPos, To: toPos})

		if offsetsAvailable {
			fromOff, errFrom := matchesIterator.StartOffset()
			toOff, errTo := matchesIterator.EndOffset()
			if errFrom != nil || errTo != nil || fromOff < 0 || toOff < 0 {
				offsetsAvailable = false
			} else {
				offsetRanges = append(offsetRanges, OffsetRange{From: fromOff, To: toOff})
			}
		}
	}

	if !offsetsAvailable {
		return o.noOffsetsFallback.convertPositionsToOffsets(positionRanges, doc.GetValues(o.field))
	}
	return offsetRanges, nil
}
