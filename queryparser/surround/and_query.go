package surround

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// AndQuery represents `A AND B AND ...`. Mirrors
// org.apache.lucene.queryparser.surround.query.AndQuery.
type AndQuery struct{ *ComposedQuery }

// NewAndQuery builds an AND composite.
func NewAndQuery(children []SrndQuery, infix bool, operatorName string) *AndQuery {
	return &AndQuery{ComposedQuery: NewComposedQuery(children, infix, operatorName)}
}

// MakeLuceneQueryField produces a BooleanQuery with MUST clauses.
func (q *AndQuery) MakeLuceneQueryField(field string, factory *BasicQueryFactory) (search.Query, error) {
	return makeBooleanQuery(q.children, field, factory, search.MUST)
}
