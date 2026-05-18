package xml

import "github.com/FlavioCFOliveira/Gocene/search"

// QueryBuilderFactory dispatches an *Element to a QueryBuilder keyed by the
// element's tag name. It mirrors org.apache.lucene.queryparser.xml.QueryBuilderFactory.
type QueryBuilderFactory struct {
	builders map[string]QueryBuilder
}

// NewQueryBuilderFactory returns an empty factory ready to accept builder
// registrations.
func NewQueryBuilderFactory() *QueryBuilderFactory {
	return &QueryBuilderFactory{builders: make(map[string]QueryBuilder)}
}

// AddBuilder registers a builder under a tag name. A subsequent call with the
// same name replaces the existing entry.
func (f *QueryBuilderFactory) AddBuilder(name string, builder QueryBuilder) {
	f.builders[name] = builder
}

// GetBuilder returns the QueryBuilder registered for the tag name or nil if
// no builder has been registered.
func (f *QueryBuilderFactory) GetBuilder(name string) QueryBuilder {
	return f.builders[name]
}

// GetQuery dispatches the element to the registered builder. If no builder is
// registered for the element's tag, a ParserException is returned.
func (f *QueryBuilderFactory) GetQuery(e *Element) (search.Query, error) {
	b, ok := f.builders[e.TagName]
	if !ok {
		return nil, NewParserException("no QueryBuilder registered for <" + e.TagName + ">")
	}
	return b.GetQuery(e)
}

var _ QueryBuilder = (*QueryBuilderFactory)(nil)
