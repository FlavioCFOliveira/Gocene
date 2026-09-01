package matchhighlight

import (
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
		ts := o.analyzer.TokenStream(o.field, value)
		offsetAttr := ts.OffsetAttribute()
		ts.Reset()
		for ts.IncrementToken() {
			// Go through all tokens to increment offset attribute properly.
		}
		ts.End()
		startOffset := valueOffset
		valueOffset += offsetAttr.EndOffset()
		ranges = append(ranges, OffsetRange{From: startOffset, To: valueOffset})
		valueOffset += o.analyzer.GetOffsetGap(o.field)
		ts.Close()
	}
	return ranges, nil
}
