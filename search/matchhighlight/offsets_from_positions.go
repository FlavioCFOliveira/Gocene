package matchhighlight

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// OffsetsFromPositions applies to fields with stored positions but no offsets.
// We re-analyze the field's value to find out offsets of match positions.
type OffsetsFromPositions struct {
	field    string
	analyzer analysis.Analyzer
}

// NewOffsetsFromPositions creates a new OffsetsFromPositions.
func NewOffsetsFromPositions(field string, analyzer analysis.Analyzer) *OffsetsFromPositions {
	return &OffsetsFromPositions{
		field:    field,
		analyzer: analyzer,
	}
}

// Get computes value offsets from positions by re-analyzing the field value.
func (o *OffsetsFromPositions) Get(matchesIterator search.MatchesIterator, doc FieldValueProvider) ([]OffsetRange, error) {
	var positionRanges []OffsetRange
	for {
		next, err := matchesIterator.Next()
		if err != nil {
			return nil, err
		}
		if !next {
			break
		}
		from := matchesIterator.StartPosition()
		to := matchesIterator.EndPosition()
		if from < 0 || to < 0 {
			return nil, fmt.Errorf("matches API returned negative positions for field: %s", o.field)
		}
		positionRanges = append(positionRanges, OffsetRange{From: from, To: to})
	}

	return o.convertPositionsToOffsets(positionRanges, doc.GetValues(o.field))
}

type positionSpan struct {
	OffsetRange
	leftOffset  int
	rightOffset int
}

func (o *OffsetsFromPositions) convertPositionsToOffsets(positionRanges []OffsetRange, values []string) ([]OffsetRange, error) {
	if len(positionRanges) == 0 {
		return positionRanges, nil
	}

	spans := make([]*positionSpan, len(positionRanges))
	spansTable := make([]*positionSpan, len(positionRanges))
	minPosition := 1<<31 - 1
	maxPosition := -1
	for i, rangeVal := range positionRanges {
		s := &positionSpan{
			OffsetRange: rangeVal,
			leftOffset:  1<<31 - 1,
			rightOffset: -1,
		}
		spans[i] = s
		spansTable[i] = s
		if rangeVal.From < minPosition {
			minPosition = rangeVal.From
		}
		if rangeVal.To > maxPosition {
			maxPosition = rangeVal.To
		}
	}

	position := -1
	valueOffset := 0
	spanCount := len(spansTable)

	for valueIndex, value := range values {
		lastValue := valueIndex+1 == len(values)

		ts, err := o.analyzer.TokenStream(o.field, strings.NewReader(value))
		if err != nil {
			return nil, err
		}
		offsetAttr, err := offsetAttribute(ts)
		if err != nil {
			return nil, err
		}
		posAttr, err := positionIncrementAttribute(ts)
		if err != nil {
			return nil, err
		}
		if err := ts.Reset(); err != nil {
			return nil, err
		}
		for {
			more, err := ts.IncrementToken()
			if err != nil {
				return nil, err
			}
			if !more {
				break
			}
			position += posAttr.GetPositionIncrement()

			if position >= minPosition {
				startOffset := valueOffset + offsetAttr.StartOffset()
				endOffset := valueOffset + offsetAttr.EndOffset()

				j := 0
				for i := 0; i < spanCount; i++ {
					span := spansTable[i]
					if position >= span.From {
						if position <= span.To {
							if startOffset < span.leftOffset {
								span.leftOffset = startOffset
							}
							if endOffset > span.rightOffset {
								span.rightOffset = endOffset
							}
						} else {
							continue
						}
					}
					spansTable[j] = span
					j++
				}
				spanCount = j

				// Only short-circuit if we're on the last value (which should be
				// the common case since most fields would only have a single
				// value anyway). We need to make sure of this because otherwise
				// offsetAttr would have incorrect value.
				if position > maxPosition && lastValue {
					break
				}
			}
		}
		if err := ts.End(); err != nil {
			return nil, err
		}
		position += posAttr.GetPositionIncrement() + positionIncrementGap(o.analyzer, o.field)
		valueOffset += offsetAttr.EndOffset() + offsetGap(o.analyzer, o.field)
		if err := ts.Close(); err != nil {
			return nil, err
		}
	}

	converted := make([]OffsetRange, 0, len(spans))
	for _, span := range spans {
		if span.leftOffset == 1<<31-1 || span.rightOffset == -1 {
			return nil, fmt.Errorf("one of the offsets missing for position range: %v", span)
		}
		converted = append(converted, OffsetRange{From: span.leftOffset, To: span.rightOffset})
	}
	return converted, nil
}
