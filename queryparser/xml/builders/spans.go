package builders

import (
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queryparser/xml"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// SpanQueryBuilder is the contract every span-builder implements. Mirrors
// org.apache.lucene.queryparser.xml.builders.SpanQueryBuilder.
type SpanQueryBuilder interface {
	xml.QueryBuilder
	GetSpanQuery(e *xml.Element) (search.SpanQuery, error)
}

// SpanQueryBuilderFactory dispatches an *Element to a registered SpanQueryBuilder
// keyed by tag name. Mirrors the homonymous Java factory.
type SpanQueryBuilderFactory struct {
	builders map[string]SpanQueryBuilder
}

// NewSpanQueryBuilderFactory builds an empty factory.
func NewSpanQueryBuilderFactory() *SpanQueryBuilderFactory {
	return &SpanQueryBuilderFactory{builders: make(map[string]SpanQueryBuilder)}
}

// AddBuilder registers a builder under a tag name.
func (f *SpanQueryBuilderFactory) AddBuilder(name string, b SpanQueryBuilder) {
	f.builders[name] = b
}

// GetBuilder returns the registered builder for a tag name or nil.
func (f *SpanQueryBuilderFactory) GetBuilder(name string) SpanQueryBuilder {
	return f.builders[name]
}

// GetSpanQuery dispatches to the appropriate registered span builder.
func (f *SpanQueryBuilderFactory) GetSpanQuery(e *xml.Element) (search.SpanQuery, error) {
	b, ok := f.builders[e.TagName]
	if !ok {
		return nil, xml.NewParserException("no SpanQueryBuilder registered for <" + e.TagName + ">")
	}
	return b.GetSpanQuery(e)
}

// GetQuery satisfies xml.QueryBuilder: returns the span query as a generic Query.
func (f *SpanQueryBuilderFactory) GetQuery(e *xml.Element) (search.Query, error) {
	sq, err := f.GetSpanQuery(e)
	if err != nil {
		return nil, err
	}
	return applyBoost(e, sq), nil
}

var _ SpanQueryBuilder = (*SpanQueryBuilderFactory)(nil)

// SpanBuilderBase embeds a SpanQueryBuilderFactory and provides a GetQuery
// wrapper for builders that primarily return a SpanQuery but must also satisfy
// xml.QueryBuilder.
type SpanBuilderBase struct {
	Factory *SpanQueryBuilderFactory
}

// NewSpanBuilderBase initialises the embed.
func NewSpanBuilderBase(factory *SpanQueryBuilderFactory) SpanBuilderBase {
	return SpanBuilderBase{Factory: factory}
}

// getChildSpan resolves the first child element to a SpanQuery via the factory.
func (b SpanBuilderBase) getChildSpan(e *xml.Element) (search.SpanQuery, error) {
	child, err := xml.GetFirstChildOrFail(e)
	if err != nil {
		return nil, err
	}
	return b.Factory.GetSpanQuery(child)
}

// SpanTermBuilder produces a SpanTermQuery from <SpanTerm fieldName="...">text</SpanTerm>.
type SpanTermBuilder struct{}

// GetSpanQuery satisfies SpanQueryBuilder.
func (SpanTermBuilder) GetSpanQuery(e *xml.Element) (search.SpanQuery, error) {
	field, err := xml.GetAttributeOrFail(e, "fieldName")
	if err != nil {
		return nil, err
	}
	text, err := xml.GetNonBlankTextOrFail(e)
	if err != nil {
		return nil, err
	}
	return search.NewSpanTermQuery(index.NewTerm(field, text)), nil
}

// GetQuery satisfies xml.QueryBuilder.
func (b SpanTermBuilder) GetQuery(e *xml.Element) (search.Query, error) {
	sq, err := b.GetSpanQuery(e)
	if err != nil {
		return nil, err
	}
	return applyBoost(e, sq), nil
}

var _ SpanQueryBuilder = SpanTermBuilder{}

// SpanOrBuilder produces a SpanOrQuery whose clauses are the child elements.
type SpanOrBuilder struct{ SpanBuilderBase }

// NewSpanOrBuilder wires the builder to the span factory it dispatches through.
func NewSpanOrBuilder(factory *SpanQueryBuilderFactory) *SpanOrBuilder {
	return &SpanOrBuilder{SpanBuilderBase: NewSpanBuilderBase(factory)}
}

// GetSpanQuery satisfies SpanQueryBuilder.
func (b *SpanOrBuilder) GetSpanQuery(e *xml.Element) (search.SpanQuery, error) {
	clauses := make([]search.SpanQuery, 0, len(e.Children))
	for _, child := range e.Children {
		sub, err := b.Factory.GetSpanQuery(child)
		if err != nil {
			return nil, err
		}
		if sub != nil {
			clauses = append(clauses, sub)
		}
	}
	return search.NewSpanOrQuery(clauses...), nil
}

// GetQuery satisfies xml.QueryBuilder.
func (b *SpanOrBuilder) GetQuery(e *xml.Element) (search.Query, error) {
	sq, err := b.GetSpanQuery(e)
	if err != nil {
		return nil, err
	}
	return applyBoost(e, sq), nil
}

var _ SpanQueryBuilder = (*SpanOrBuilder)(nil)

// SpanOrTermsBuilder produces a SpanOrQuery of SpanTermQuery instances built
// from the whitespace-separated terms in the element body. Mirrors Lucene's
// SpanOrTermsBuilder.
type SpanOrTermsBuilder struct{}

// GetSpanQuery satisfies SpanQueryBuilder.
func (SpanOrTermsBuilder) GetSpanQuery(e *xml.Element) (search.SpanQuery, error) {
	field, err := xml.GetAttributeOrFail(e, "fieldName")
	if err != nil {
		return nil, err
	}
	text, err := xml.GetNonBlankTextOrFail(e)
	if err != nil {
		return nil, err
	}
	tokens := strings.Fields(text)
	if len(tokens) == 0 {
		return nil, xml.NewParserException("SpanOrTerms requires at least one term")
	}
	terms := make([]*index.Term, len(tokens))
	for i, t := range tokens {
		terms[i] = index.NewTerm(field, t)
	}
	return search.NewSpanOrTermsQuery(terms...), nil
}

// GetQuery satisfies xml.QueryBuilder.
func (b SpanOrTermsBuilder) GetQuery(e *xml.Element) (search.Query, error) {
	sq, err := b.GetSpanQuery(e)
	if err != nil {
		return nil, err
	}
	return applyBoost(e, sq), nil
}

var _ SpanQueryBuilder = SpanOrTermsBuilder{}

// SpanNotBuilder produces a SpanNotQuery whose `include` is the first child
// and `exclude` is the second child.
type SpanNotBuilder struct{ SpanBuilderBase }

// NewSpanNotBuilder wires the builder.
func NewSpanNotBuilder(factory *SpanQueryBuilderFactory) *SpanNotBuilder {
	return &SpanNotBuilder{SpanBuilderBase: NewSpanBuilderBase(factory)}
}

// GetSpanQuery satisfies SpanQueryBuilder.
func (b *SpanNotBuilder) GetSpanQuery(e *xml.Element) (search.SpanQuery, error) {
	include, err := xml.GetChildByTagNameOrFail(e, "Include")
	if err != nil {
		return nil, err
	}
	exclude, err := xml.GetChildByTagNameOrFail(e, "Exclude")
	if err != nil {
		return nil, err
	}
	inc, err := b.Factory.GetSpanQuery(xml.GetFirstChildElement(include))
	if err != nil {
		return nil, err
	}
	exc, err := b.Factory.GetSpanQuery(xml.GetFirstChildElement(exclude))
	if err != nil {
		return nil, err
	}
	return search.NewSpanNotQuery(inc, exc), nil
}

// GetQuery satisfies xml.QueryBuilder.
func (b *SpanNotBuilder) GetQuery(e *xml.Element) (search.Query, error) {
	sq, err := b.GetSpanQuery(e)
	if err != nil {
		return nil, err
	}
	return applyBoost(e, sq), nil
}

var _ SpanQueryBuilder = (*SpanNotBuilder)(nil)

// SpanNearBuilder produces a SpanNearQuery whose clauses are the child
// elements and whose slop/inOrder come from the element attributes.
type SpanNearBuilder struct{ SpanBuilderBase }

// NewSpanNearBuilder wires the builder.
func NewSpanNearBuilder(factory *SpanQueryBuilderFactory) *SpanNearBuilder {
	return &SpanNearBuilder{SpanBuilderBase: NewSpanBuilderBase(factory)}
}

// GetSpanQuery satisfies SpanQueryBuilder.
func (b *SpanNearBuilder) GetSpanQuery(e *xml.Element) (search.SpanQuery, error) {
	slop := xml.GetAttributeInt(e, "slop", 0)
	inOrder := xml.GetAttributeBoolean(e, "inOrder", false)
	clauses := make([]search.SpanQuery, 0, len(e.Children))
	for _, child := range e.Children {
		sub, err := b.Factory.GetSpanQuery(child)
		if err != nil {
			return nil, err
		}
		if sub != nil {
			clauses = append(clauses, sub)
		}
	}
	return search.NewSpanNearQuery(clauses, slop, inOrder), nil
}

// GetQuery satisfies xml.QueryBuilder.
func (b *SpanNearBuilder) GetQuery(e *xml.Element) (search.Query, error) {
	sq, err := b.GetSpanQuery(e)
	if err != nil {
		return nil, err
	}
	return applyBoost(e, sq), nil
}

var _ SpanQueryBuilder = (*SpanNearBuilder)(nil)

// SpanFirstBuilder produces a SpanFirstQuery whose match is the first child
// element and whose end position comes from the "end" attribute.
type SpanFirstBuilder struct{ SpanBuilderBase }

// NewSpanFirstBuilder wires the builder.
func NewSpanFirstBuilder(factory *SpanQueryBuilderFactory) *SpanFirstBuilder {
	return &SpanFirstBuilder{SpanBuilderBase: NewSpanBuilderBase(factory)}
}

// GetSpanQuery satisfies SpanQueryBuilder.
func (b *SpanFirstBuilder) GetSpanQuery(e *xml.Element) (search.SpanQuery, error) {
	end, err := xml.GetAttributeIntOrFail(e, "end")
	if err != nil {
		return nil, err
	}
	sub, err := b.getChildSpan(e)
	if err != nil {
		return nil, err
	}
	return search.NewSpanFirstQuery(sub, end), nil
}

// GetQuery satisfies xml.QueryBuilder.
func (b *SpanFirstBuilder) GetQuery(e *xml.Element) (search.Query, error) {
	sq, err := b.GetSpanQuery(e)
	if err != nil {
		return nil, err
	}
	return applyBoost(e, sq), nil
}

var _ SpanQueryBuilder = (*SpanFirstBuilder)(nil)

// SpanPositionRangeBuilder produces a SpanPositionRangeQuery.
type SpanPositionRangeBuilder struct{ SpanBuilderBase }

// NewSpanPositionRangeBuilder wires the builder.
func NewSpanPositionRangeBuilder(factory *SpanQueryBuilderFactory) *SpanPositionRangeBuilder {
	return &SpanPositionRangeBuilder{SpanBuilderBase: NewSpanBuilderBase(factory)}
}

// GetSpanQuery satisfies SpanQueryBuilder.
func (b *SpanPositionRangeBuilder) GetSpanQuery(e *xml.Element) (search.SpanQuery, error) {
	start, err := xml.GetAttributeIntOrFail(e, "start")
	if err != nil {
		return nil, err
	}
	end, err := xml.GetAttributeIntOrFail(e, "end")
	if err != nil {
		return nil, err
	}
	sub, err := b.getChildSpan(e)
	if err != nil {
		return nil, err
	}
	return search.NewSpanPositionRangeQuery(sub, start, end), nil
}

// GetQuery satisfies xml.QueryBuilder.
func (b *SpanPositionRangeBuilder) GetQuery(e *xml.Element) (search.Query, error) {
	sq, err := b.GetSpanQuery(e)
	if err != nil {
		return nil, err
	}
	return applyBoost(e, sq), nil
}

var _ SpanQueryBuilder = (*SpanPositionRangeBuilder)(nil)

// BoostingTermBuilder produces a SpanTermQuery wrapped in a BoostQuery from
// <BoostingTermQuery fieldName="..." boost="...">text</BoostingTermQuery>.
// Mirrors the SpanPayloadCheck-flavoured BoostingTermBuilder.
type BoostingTermBuilder struct{}

// GetSpanQuery satisfies SpanQueryBuilder.
func (BoostingTermBuilder) GetSpanQuery(e *xml.Element) (search.SpanQuery, error) {
	field, err := xml.GetAttributeOrFail(e, "fieldName")
	if err != nil {
		return nil, err
	}
	text, err := xml.GetNonBlankTextOrFail(e)
	if err != nil {
		return nil, err
	}
	return search.NewSpanTermQuery(index.NewTerm(field, text)), nil
}

// GetQuery satisfies xml.QueryBuilder. Always wraps the SpanTermQuery in a
// BoostQuery (default boost 1.0 collapses back to the raw query).
func (b BoostingTermBuilder) GetQuery(e *xml.Element) (search.Query, error) {
	sq, err := b.GetSpanQuery(e)
	if err != nil {
		return nil, err
	}
	return applyBoost(e, sq), nil
}

var _ SpanQueryBuilder = BoostingTermBuilder{}
