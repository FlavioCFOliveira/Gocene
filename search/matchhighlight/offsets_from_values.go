package matchhighlight

import (
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// OffsetsFromValues works for fields where we know the match occurred but there are no known positions or offsets.
// We re-analyze field values and return offset ranges for entire values (not individual tokens).
type OffsetsFromValues struct {
	field    string
	analyzer analysis.Analyzer
}

// NewOffsetsFromValues creates a new OffsetsFromValues.
func NewOffsetsFromValues(field string, analyzer analysis.Analyzer) *OffsetsFromValues {
	return &OffsetsFromValues{
		field:    field,
		analyzer: analyzer,
	}
}

// Get retrieves offset ranges for entire field values.
func (o *OffsetsFromValues) Get(matchesIterator search.MatchesIterator, doc FieldValueProvider) ([]OffsetRange, error) {
	values := doc.GetValues(o.field)

	var ranges []OffsetRange
	valueOffset := 0
	for _, value := range values {
		ts, err := o.analyzer.TokenStream(o.field, strings.NewReader(value))
		if err != nil {
			return nil, err
		}
		offsetAttr, err := offsetAttribute(ts)
		if err != nil {
			return nil, err
		}
		if err := ts.Reset(); err != nil {
			return nil, err
		}
		startOffset := valueOffset
		for {
			// Go through all tokens to increment offset attribute properly.
			more, err := ts.IncrementToken()
			if err != nil {
				return nil, err
			}
			if !more {
				break
			}
		}
		if err := ts.End(); err != nil {
			return nil, err
		}
		valueOffset += offsetAttr.EndOffset()
		ranges = append(ranges, OffsetRange{From: startOffset, To: valueOffset})
		valueOffset += offsetGap(o.analyzer, o.field)
		if err := ts.Close(); err != nil {
			return nil, err
		}
	}
	return ranges, nil
}
