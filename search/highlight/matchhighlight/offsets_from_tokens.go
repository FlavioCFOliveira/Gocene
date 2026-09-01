package matchhighlight

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// OffsetsFromTokens works for fields where we know the match occurred but there are no known positions or offsets.
// We re-analyze field values and return offset ranges for returned tokens that are also returned by the query's term collector.
type OffsetsFromTokens struct {
	field    string
	analyzer analysis.Analyzer
}

// NewOffsetsFromTokens creates a new OffsetsFromTokens.
func NewOffsetsFromTokens(field string, analyzer analysis.Analyzer) *OffsetsFromTokens {
	return &OffsetsFromTokens{
		field:    field,
		analyzer: analyzer,
	}
}

// Get retrieves offset ranges by re-analyzing field values and matching against query terms.
func (o *OffsetsFromTokens) Get(matchesIterator search.MatchesIterator, doc FieldValueProvider) ([]OffsetRange, error) {
	values := doc.GetValues(o.field)

	matchTerms := make(map[string]struct{})
	for {
		next, err := matchesIterator.Next()
		if err != nil {
			return nil, err
		}
		if !next {
			break
		}
		q := matchesIterator.GetQuery()
		q.Visit(func(field string, terms [][]byte) {
			if field == o.field {
				for _, t := range terms {
					matchTerms[string(t)] = struct{}{}
				}
			}
		})
	}

	var ranges []OffsetRange
	valueOffset := 0
	for _, value := range values {
		ts := o.analyzer.TokenStream(o.field, value)
		offsetAttr := ts.OffsetAttribute()
		termAttr := ts.TermAttribute()
		ts.Reset()
		for ts.IncrementToken() {
			if _, ok := matchTerms[string(termAttr.GetTerm())]; ok {
				startOffset := valueOffset + offsetAttr.StartOffset()
				endOffset := valueOffset + offsetAttr.EndOffset()
				ranges = append(ranges, OffsetRange{From: startOffset, To: endOffset})
			}
		}
		ts.End()
		valueOffset += offsetAttr.EndOffset() + o.analyzer.GetOffsetGap(o.field)
		ts.Close()
	}
	return ranges, nil
}
