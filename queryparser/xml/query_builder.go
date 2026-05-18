package xml

import "github.com/FlavioCFOliveira/Gocene/search"

// QueryBuilder is the contract implemented by every XML query builder. Given
// an XML element representing a query, the builder returns the corresponding
// search.Query. Mirrors org.apache.lucene.queryparser.xml.QueryBuilder.
type QueryBuilder interface {
	GetQuery(e *Element) (search.Query, error)
}

// QueryBuilderFunc lets callers register a plain function as a QueryBuilder.
type QueryBuilderFunc func(e *Element) (search.Query, error)

// GetQuery satisfies the QueryBuilder interface for QueryBuilderFunc.
func (f QueryBuilderFunc) GetQuery(e *Element) (search.Query, error) { return f(e) }
