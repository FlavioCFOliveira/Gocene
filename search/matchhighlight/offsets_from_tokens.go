package matchhighlight

import (
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// OffsetsFromTokens works for fields where we know the match occurred but
// there are no known positions or offsets. We re-analyze field values and
// return offset ranges for returned tokens that are also returned by the
// query's term collector.
//
// Mirrors org.apache.lucene.search.matchhighlight.OffsetsFromTokens of
// Apache Lucene 10.5.0.
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

// matchTermsCollector renders the anonymous QueryVisitor Java declares inline
// in OffsetsFromTokens#get, which collects the bytes of every visited term
// belonging to the highlighted field.
type matchTermsCollector struct {
	search.EmptyQueryVisitorBase
	field string
	terms map[string]struct{}
}

// ConsumeTerms mirrors the overridden consumeTerms(Query, Term...).
func (c *matchTermsCollector) ConsumeTerms(_ search.Query, terms ...*index.Term) {
	for _, t := range terms {
		if t == nil || t.Bytes == nil {
			continue
		}
		if c.field == t.Field {
			c.terms[string(t.Bytes.ValidBytes())] = struct{}{}
		}
	}
}

// Get retrieves offset ranges by re-analyzing field values and matching
// against the query's terms.
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
		collector := &matchTermsCollector{field: o.field, terms: matchTerms}
		collector.Visitor = collector
		if v, ok := q.(interface{ Visit(search.QueryVisitor) }); ok {
			v.Visit(collector)
		}
	}

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
		termAttr, err := termToBytesRefAttribute(ts)
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
			if _, ok := matchTerms[string(termAttr.GetBytesRef().ValidBytes())]; ok {
				startOffset := valueOffset + offsetAttr.StartOffset()
				endOffset := valueOffset + offsetAttr.EndOffset()
				ranges = append(ranges, OffsetRange{From: startOffset, To: endOffset})
			}
		}
		if err := ts.End(); err != nil {
			return nil, err
		}
		valueOffset += offsetAttr.EndOffset() + offsetGap(o.analyzer, o.field)
		if err := ts.Close(); err != nil {
			return nil, err
		}
	}
	return ranges, nil
}
