// Package builders implements the standard set of XML query builders bundled
// with org.apache.lucene.queryparser.xml.builders. Each builder converts a
// single XML element into a search.Query and registers itself with a
// CoreParser (or any QueryBuilderFactory).
package builders

import (
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queryparser/xml"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// MatchAllDocsQueryBuilder builds a search.MatchAllDocsQuery.
type MatchAllDocsQueryBuilder struct{}

// GetQuery returns a fresh MatchAllDocsQuery; the element is ignored.
func (MatchAllDocsQueryBuilder) GetQuery(_ *xml.Element) (search.Query, error) {
	return search.NewMatchAllDocsQuery(), nil
}

var _ xml.QueryBuilder = MatchAllDocsQueryBuilder{}

// TermQueryBuilder builds a search.TermQuery from a <TermQuery fieldName="...">text</TermQuery>.
type TermQueryBuilder struct{}

// GetQuery returns the TermQuery defined by the element.
func (TermQueryBuilder) GetQuery(e *xml.Element) (search.Query, error) {
	field, err := xml.GetAttributeOrFail(e, "fieldName")
	if err != nil {
		return nil, err
	}
	text, err := xml.GetNonBlankTextOrFail(e)
	if err != nil {
		return nil, err
	}
	q := search.NewTermQuery(index.NewTerm(field, text))
	return applyBoost(e, q), nil
}

var _ xml.QueryBuilder = TermQueryBuilder{}

// TermsQueryBuilder builds a BooleanQuery containing one SHOULD TermQuery per
// whitespace-separated term in the element's text body. fieldName is required.
type TermsQueryBuilder struct{}

// GetQuery returns the BooleanQuery encoded by the element.
func (TermsQueryBuilder) GetQuery(e *xml.Element) (search.Query, error) {
	field, err := xml.GetAttributeOrFail(e, "fieldName")
	if err != nil {
		return nil, err
	}
	text, err := xml.GetNonBlankTextOrFail(e)
	if err != nil {
		return nil, err
	}
	bq := search.NewBooleanQuery()
	for _, tok := range strings.Fields(text) {
		bq.Add(search.NewTermQuery(index.NewTerm(field, tok)), search.SHOULD)
	}
	if mm := xml.GetAttributeInt(e, "minimumNumberShouldMatch", 0); mm > 0 {
		bq.SetMinimumNumberShouldMatch(mm)
	}
	return applyBoost(e, bq), nil
}

var _ xml.QueryBuilder = TermsQueryBuilder{}

// BooleanQueryBuilder builds a search.BooleanQuery from a <BooleanQuery>
// containing one <Clause occurs="must|should|mustNot|filter"> per sub-query.
type BooleanQueryBuilder struct {
	Factory *xml.QueryBuilderFactory
}

// NewBooleanQueryBuilder wires the builder to its sub-query factory.
func NewBooleanQueryBuilder(factory *xml.QueryBuilderFactory) *BooleanQueryBuilder {
	return &BooleanQueryBuilder{Factory: factory}
}

// GetQuery walks the element's <Clause> children and assembles a BooleanQuery.
func (b *BooleanQueryBuilder) GetQuery(e *xml.Element) (search.Query, error) {
	bq := search.NewBooleanQuery()
	for _, clause := range xml.GetChildrenByTagName(e, "Clause") {
		sub, err := b.parseClause(clause)
		if err != nil {
			return nil, err
		}
		if sub == nil {
			continue
		}
		occur, err := parseOccurAttribute(clause)
		if err != nil {
			return nil, err
		}
		bq.Add(sub, occur)
	}
	if mm := xml.GetAttributeInt(e, "minimumNumberShouldMatch", 0); mm > 0 {
		bq.SetMinimumNumberShouldMatch(mm)
	}
	return applyBoost(e, bq), nil
}

func (b *BooleanQueryBuilder) parseClause(clause *xml.Element) (search.Query, error) {
	child := xml.GetFirstChildElement(clause)
	if child == nil {
		return nil, nil
	}
	return b.Factory.GetQuery(child)
}

func parseOccurAttribute(e *xml.Element) (search.Occur, error) {
	v := strings.ToLower(xml.GetAttribute(e, "occurs", "should"))
	switch v {
	case "must":
		return search.MUST, nil
	case "should":
		return search.SHOULD, nil
	case "mustnot":
		return search.MUST_NOT, nil
	case "filter":
		return search.FILTER, nil
	default:
		return search.SHOULD, xml.NewParserException("unknown occurs value: " + v)
	}
}

var _ xml.QueryBuilder = (*BooleanQueryBuilder)(nil)

// ConstantScoreQueryBuilder wraps the child query in a ConstantScoreQuery.
type ConstantScoreQueryBuilder struct {
	Factory *xml.QueryBuilderFactory
}

// NewConstantScoreQueryBuilder wires the builder to its sub-query factory.
func NewConstantScoreQueryBuilder(factory *xml.QueryBuilderFactory) *ConstantScoreQueryBuilder {
	return &ConstantScoreQueryBuilder{Factory: factory}
}

// GetQuery wraps the first child element in a ConstantScoreQuery.
func (b *ConstantScoreQueryBuilder) GetQuery(e *xml.Element) (search.Query, error) {
	child, err := xml.GetFirstChildOrFail(e)
	if err != nil {
		return nil, err
	}
	sub, err := b.Factory.GetQuery(child)
	if err != nil {
		return nil, err
	}
	csq := search.NewConstantScoreQuery(sub)
	if boost := xml.GetAttributeFloat(e, "boost", 1.0); boost != 1.0 {
		return search.NewBoostQuery(csq, boost), nil
	}
	return csq, nil
}

var _ xml.QueryBuilder = (*ConstantScoreQueryBuilder)(nil)

// DisjunctionMaxQueryBuilder builds a search.DisjunctionMaxQuery whose
// disjuncts are the immediate element children.
type DisjunctionMaxQueryBuilder struct {
	Factory *xml.QueryBuilderFactory
}

// NewDisjunctionMaxQueryBuilder wires the builder.
func NewDisjunctionMaxQueryBuilder(factory *xml.QueryBuilderFactory) *DisjunctionMaxQueryBuilder {
	return &DisjunctionMaxQueryBuilder{Factory: factory}
}

// GetQuery returns the DisjunctionMaxQuery encoded by the element.
func (b *DisjunctionMaxQueryBuilder) GetQuery(e *xml.Element) (search.Query, error) {
	tb := xml.GetAttributeFloat(e, "tieBreaker", 0.0)
	disjuncts := make([]search.Query, 0, len(e.Children))
	for _, child := range e.Children {
		sub, err := b.Factory.GetQuery(child)
		if err != nil {
			return nil, err
		}
		if sub != nil {
			disjuncts = append(disjuncts, sub)
		}
	}
	q := search.NewDisjunctionMaxQueryWithTieBreaker(disjuncts, tb)
	return applyBoost(e, q), nil
}

var _ xml.QueryBuilder = (*DisjunctionMaxQueryBuilder)(nil)

// RangeQueryBuilder builds a TermRangeQuery from
// <RangeQuery fieldName="..." lowerTerm="a" upperTerm="z" includeLower="true" includeUpper="false"/>.
type RangeQueryBuilder struct{}

// GetQuery decodes the element's attributes into a TermRangeQuery.
func (RangeQueryBuilder) GetQuery(e *xml.Element) (search.Query, error) {
	field, err := xml.GetAttributeOrFail(e, "fieldName")
	if err != nil {
		return nil, err
	}
	lower := xml.GetAttribute(e, "lowerTerm", "")
	upper := xml.GetAttribute(e, "upperTerm", "")
	includeLower := xml.GetAttributeBoolean(e, "includeLower", true)
	includeUpper := xml.GetAttributeBoolean(e, "includeUpper", true)
	q := search.NewTermRangeQueryWithStrings(field, lower, upper, includeLower, includeUpper)
	return applyBoost(e, q), nil
}

var _ xml.QueryBuilder = RangeQueryBuilder{}

// PointRangeQueryBuilder builds a PointRangeQuery from
// <PointRangeQuery fieldName="..." lowerTerm="..." upperTerm="..."/>. The
// builder treats the term text as raw bytes; callers that want numeric
// semantics must encode the bytes themselves prior to serialisation.
type PointRangeQueryBuilder struct{}

// GetQuery decodes the element into a PointRangeQuery.
func (PointRangeQueryBuilder) GetQuery(e *xml.Element) (search.Query, error) {
	field, err := xml.GetAttributeOrFail(e, "fieldName")
	if err != nil {
		return nil, err
	}
	lower := []byte(xml.GetAttribute(e, "lowerTerm", ""))
	upper := []byte(xml.GetAttribute(e, "upperTerm", ""))
	q, err := search.NewPointRangeQuery(field, lower, upper)
	if err != nil {
		return nil, xml.NewParserExceptionWithCause("invalid PointRangeQuery", err)
	}
	return applyBoost(e, q), nil
}

var _ xml.QueryBuilder = PointRangeQueryBuilder{}

// UserInputQueryBuilder delegates parsing back to the supplied parser
// callback. Mirrors org.apache.lucene.queryparser.xml.builders.UserInputQueryBuilder.
type UserInputQueryBuilder struct {
	Parse func(field, queryString string) (search.Query, error)
}

// NewUserInputQueryBuilder wires the builder to a parsing callback.
func NewUserInputQueryBuilder(parse func(field, queryString string) (search.Query, error)) *UserInputQueryBuilder {
	return &UserInputQueryBuilder{Parse: parse}
}

// GetQuery dispatches the element body to the wrapped parser.
func (b *UserInputQueryBuilder) GetQuery(e *xml.Element) (search.Query, error) {
	if b.Parse == nil {
		return nil, xml.NewParserException("UserInputQueryBuilder has no parser configured")
	}
	field := xml.GetAttribute(e, "fieldName", "")
	queryString, err := xml.GetNonBlankTextOrFail(e)
	if err != nil {
		return nil, err
	}
	q, err := b.Parse(field, queryString)
	if err != nil {
		return nil, err
	}
	return applyBoost(e, q), nil
}

var _ xml.QueryBuilder = (*UserInputQueryBuilder)(nil)

// applyBoost wraps q in a BoostQuery when the element carries a boost
// attribute different from 1.0.
func applyBoost(e *xml.Element, q search.Query) search.Query {
	if q == nil {
		return nil
	}
	boost := xml.GetAttributeFloat(e, "boost", 1.0)
	if boost == 1.0 {
		return q
	}
	return search.NewBoostQuery(q, boost)
}
