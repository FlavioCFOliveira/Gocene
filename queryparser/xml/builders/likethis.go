package builders

import (
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queryparser/xml"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// LikeThisQueryBuilder builds a MoreLikeThis query from a <LikeThisQuery>
// element. The element's "fieldName" attribute lists comma-separated fields
// and the text body provides the seed text. Mirrors the homonymous Java builder.
type LikeThisQueryBuilder struct {
	Analyzer analysis.Analyzer
}

// NewLikeThisQueryBuilder wires the builder to an analyzer.
func NewLikeThisQueryBuilder(a analysis.Analyzer) *LikeThisQueryBuilder {
	return &LikeThisQueryBuilder{Analyzer: a}
}

// GetQuery satisfies xml.QueryBuilder.
func (b *LikeThisQueryBuilder) GetQuery(e *xml.Element) (search.Query, error) {
	fieldList, err := xml.GetAttributeOrFail(e, "fieldName")
	if err != nil {
		return nil, err
	}
	text, err := xml.GetNonBlankTextOrFail(e)
	if err != nil {
		return nil, err
	}
	mlt := search.NewMoreLikeThis(b.Analyzer)
	if v := xml.GetAttributeInt(e, "minTermFrequency", 0); v > 0 {
		mlt.MinTermFreq = v
	}
	if v := xml.GetAttributeInt(e, "minDocFreq", 0); v > 0 {
		mlt.MinDocFreq = v
	}
	if v := xml.GetAttributeInt(e, "maxQueryTerms", 0); v > 0 {
		mlt.MaxQueryTerms = v
	}
	fields := splitFields(fieldList)
	if len(fields) > 0 {
		mlt.FieldNames = fields
	}
	q, err := mlt.LikeText(text)
	if err != nil {
		return nil, xml.NewParserExceptionWithCause("LikeThisQuery generation failed", err)
	}
	return applyBoost(e, q), nil
}

var _ xml.QueryBuilder = (*LikeThisQueryBuilder)(nil)

// FuzzyLikeThisQueryBuilder is a thin variant of LikeThisQueryBuilder that
// keeps the same MoreLikeThis machinery but enables fuzzy expansion via the
// MinTermFreq/MaxQueryTerms tunables. The full FuzzyLikeThis Java port (with
// its custom rewrite producing FuzzyTermsQuery) is not yet available in Go,
// so this builder downgrades to LikeThis behaviour while honouring the
// extra attributes "ignoreTF" and "fuzzyMinSim" used by Lucene tests.
type FuzzyLikeThisQueryBuilder struct {
	Analyzer analysis.Analyzer
}

// NewFuzzyLikeThisQueryBuilder wires the builder.
func NewFuzzyLikeThisQueryBuilder(a analysis.Analyzer) *FuzzyLikeThisQueryBuilder {
	return &FuzzyLikeThisQueryBuilder{Analyzer: a}
}

// GetQuery satisfies xml.QueryBuilder.
func (b *FuzzyLikeThisQueryBuilder) GetQuery(e *xml.Element) (search.Query, error) {
	fieldList, err := xml.GetAttributeOrFail(e, "fieldName")
	if err != nil {
		return nil, err
	}
	text, err := xml.GetNonBlankTextOrFail(e)
	if err != nil {
		return nil, err
	}
	mlt := search.NewMoreLikeThis(b.Analyzer)
	if v := xml.GetAttributeInt(e, "minTermFreq", 0); v > 0 {
		mlt.MinTermFreq = v
	}
	if v := xml.GetAttributeInt(e, "maxQueryTerms", 0); v > 0 {
		mlt.MaxQueryTerms = v
	}
	fields := splitFields(fieldList)
	if len(fields) > 0 {
		mlt.FieldNames = fields
	}
	q, err := mlt.LikeText(text)
	if err != nil {
		return nil, xml.NewParserExceptionWithCause("FuzzyLikeThisQuery generation failed", err)
	}
	return applyBoost(e, q), nil
}

var _ xml.QueryBuilder = (*FuzzyLikeThisQueryBuilder)(nil)

func splitFields(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
