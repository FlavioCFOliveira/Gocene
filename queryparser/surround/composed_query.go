package surround

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/queries/spans"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// ComposedQuery is the abstract parent of every surround node that contains
// other SrndQuery children (AND, OR, NOT, distance). Mirrors
// org.apache.lucene.queryparser.surround.query.ComposedQuery.
type ComposedQuery struct {
	SrndQueryBase
	children     []SrndQuery
	operatorName string
	infix        bool
}

// NewComposedQuery initialises a ComposedQuery with the given children and
// rendering hints.
func NewComposedQuery(children []SrndQuery, infix bool, operatorName string) *ComposedQuery {
	return &ComposedQuery{
		children:     children,
		operatorName: operatorName,
		infix:        infix,
	}
}

// GetChildren returns the immediate sub-queries.
func (c *ComposedQuery) GetChildren() []SrndQuery { return c.children }

// SetChildren replaces the sub-queries.
func (c *ComposedQuery) SetChildren(children []SrndQuery) { c.children = children }

// GetOperatorName returns the JavaCC operator label (e.g. "AND", "OR", "5W").
func (c *ComposedQuery) GetOperatorName() string { return c.operatorName }

// IsInfix reports whether the operator was used in infix position.
func (c *ComposedQuery) IsInfix() bool { return c.infix }

// IsFieldsSubQueryAcceptable reports whether at least one subquery is acceptable.
// Mirrors org.apache.lucene.queryparser.surround.query.ComposedQuery.
func (c *ComposedQuery) IsFieldsSubQueryAcceptable() bool {
	for _, child := range c.children {
		if child.IsFieldsSubQueryAcceptable() {
			return true
		}
	}
	return false
}

// String returns the string representation of the composed query.
// Mirrors org.apache.lucene.queryparser.surround.query.ComposedQuery.
func (c *ComposedQuery) String() string {
	var sb strings.Builder
	if c.IsInfix() {
		c.infixToString(&sb)
	} else {
		c.prefixToString(&sb)
	}
	c.SrndQueryBase.WeightToString(&sb)
	return sb.String()
}

func (c *ComposedQuery) getBracketOpen() string { return "(" }

func (c *ComposedQuery) getBracketClose() string { return ")" }

func (c *ComposedQuery) getPrefixSeparator() string { return ", " }

func (c *ComposedQuery) infixToString(sb *strings.Builder) {
	sb.WriteString(c.getBracketOpen())
	if len(c.children) > 0 {
		sb.WriteString(c.children[0].String())
		for _, child := range c.children[1:] {
			sb.WriteString(" ")
			sb.WriteString(c.GetOperatorName())
			sb.WriteString(" ")
			sb.WriteString(child.String())
		}
	}
	sb.WriteString(c.getBracketClose())
}

func (c *ComposedQuery) prefixToString(sb *strings.Builder) {
	sb.WriteString(c.GetOperatorName())
	sb.WriteString(c.getBracketOpen())
	if len(c.children) > 0 {
		sb.WriteString(c.children[0].String())
		for _, child := range c.children[1:] {
			sb.WriteString(c.getPrefixSeparator())
			sb.WriteString(child.String())
		}
	}
	sb.WriteString(c.getBracketClose())
}

// OrQuery represents `A OR B OR ...`. Mirrors
// org.apache.lucene.queryparser.surround.query.OrQuery.
type OrQuery struct{ *ComposedQuery }

// NewOrQuery builds an OR composite.
func NewOrQuery(children []SrndQuery, infix bool, operatorName string) *OrQuery {
	return &OrQuery{ComposedQuery: NewComposedQuery(children, infix, operatorName)}
}

// MakeLuceneQueryField produces a BooleanQuery with SHOULD clauses.
func (q *OrQuery) MakeLuceneQueryField(field string, factory *BasicQueryFactory) (search.Query, error) {
	return makeBooleanQuery(q.children, field, factory, search.SHOULD)
}

// DistanceSubQueryNotAllowed returns the reason why this node cannot
// participate in a DistanceQuery, or the empty string when the node is
// acceptable. Mirrors org.apache.lucene.queryparser.surround.query.OrQuery.
func (q *OrQuery) DistanceSubQueryNotAllowed() string {
	for _, child := range q.children {
		if dsq, ok := child.(DistanceSubQuery); ok {
			if reason := dsq.DistanceSubQueryNotAllowed(); reason != "" {
				return reason
			}
		} else {
			return fmt.Sprintf("subquery not allowed: %v", child)
		}
	}
	return ""
}

// AddSpanQueries contributes one or more SpanQuery clauses to the supplied
// factory. Mirrors org.apache.lucene.queryparser.surround.query.OrQuery.
func (q *OrQuery) AddSpanQueries(factory *SpanNearClauseFactory) error {
	for _, child := range q.children {
		dsq, ok := child.(DistanceSubQuery)
		if !ok {
			return NewParseException("distance query child is not a SimpleTerm/DistanceSubQuery")
		}
		if err := dsq.AddSpanQueries(factory); err != nil {
			return err
		}
	}
	return nil
}

// NotQuery represents `A NOT B NOT ...`. Mirrors
// org.apache.lucene.queryparser.surround.query.NotQuery: the first child is
// MUST, the remainder are MUST_NOT.
type NotQuery struct{ *ComposedQuery }

// NewNotQuery builds a NOT composite.
func NewNotQuery(children []SrndQuery, infix bool, operatorName string) *NotQuery {
	return &NotQuery{ComposedQuery: NewComposedQuery(children, infix, operatorName)}
}

// MakeLuceneQueryField produces a BooleanQuery with the first clause as MUST
// and the rest as MUST_NOT.
func (q *NotQuery) MakeLuceneQueryField(field string, factory *BasicQueryFactory) (search.Query, error) {
	if len(q.children) == 0 {
		return search.NewBooleanQueryBuilder().Build(), nil
	}
	clauses := make([]*search.BooleanClause, 0, len(q.children))
	first, err := q.children[0].MakeLuceneQueryField(field, factory)
	if err != nil {
		return nil, err
	}
	if first != nil {
		clauses = append(clauses, search.NewBooleanClause(first, search.MUST))
	}
	for _, child := range q.children[1:] {
		sub, err := child.MakeLuceneQueryField(field, factory)
		if err != nil {
			return nil, err
		}
		if sub == nil {
			continue
		}
		clauses = append(clauses, search.NewBooleanClause(sub, search.MUST_NOT))
	}
	return newBooleanQueryFromClauses(clauses), nil
}

// DistanceQuery represents proximity operators `kW` (ordered, slop k-1) and
// `kN` (unordered, slop k-1). Mirrors
// org.apache.lucene.queryparser.surround.query.DistanceQuery.
type DistanceQuery struct {
	*ComposedQuery
	opDistance    int
	subQueryField string
	ordered       bool
}

// NewDistanceQuery builds a distance composite. `opDistance` is the integer
// distance parsed from the operator (e.g. 3 for "3W"); `ordered` selects
// SpanNearQuery's inOrder flag.
func NewDistanceQuery(children []SrndQuery, infix bool, opDistance int, operatorName string, ordered bool) *DistanceQuery {
	return &DistanceQuery{
		ComposedQuery: NewComposedQuery(children, infix, operatorName),
		opDistance:    opDistance,
		ordered:       ordered,
	}
}

// GetOpDistance returns the distance parameter.
func (q *DistanceQuery) GetOpDistance() int { return q.opDistance }

// IsOrdered reports whether the distance is order-sensitive.
func (q *DistanceQuery) IsOrdered() bool { return q.ordered }

// MakeLuceneQueryField produces a SpanNearQuery from the children.
func (q *DistanceQuery) MakeLuceneQueryField(field string, factory *BasicQueryFactory) (search.Query, error) {
	clauseFactory := NewSpanNearClauseFactory(field, factory)
	for _, child := range q.children {
		dsq, ok := child.(DistanceSubQuery)
		if !ok {
			return nil, NewParseException("distance query child is not a SimpleTerm/DistanceSubQuery")
		}
		if reason := dsq.DistanceSubQueryNotAllowed(); reason != "" {
			return nil, NewParseException(reason)
		}
		if err := dsq.AddSpanQueries(clauseFactory); err != nil {
			return nil, err
		}
	}
	clauses := clauseFactory.MakeSpanClauses()
	if len(clauses) == 0 {
		return search.NewBooleanQueryBuilder().Build(), nil
	}
	slop := q.opDistance - 1
	if slop < 0 {
		slop = 0
	}
	return spans.NewSpanNearQuery(clauses, slop, q.ordered)
}

// AddSpanQueries makes DistanceQuery itself a DistanceSubQuery so it can be
// nested. Each child contributes its own spans to the outer factory.
func (q *DistanceQuery) AddSpanQueries(factory *SpanNearClauseFactory) error {
	for _, child := range q.children {
		dsq, ok := child.(DistanceSubQuery)
		if !ok {
			return NewParseException("distance query child is not a SimpleTerm/DistanceSubQuery")
		}
		if err := dsq.AddSpanQueries(factory); err != nil {
			return err
		}
	}
	return nil
}

// DistanceSubQueryNotAllowed returns the empty string — DistanceQuery is
// itself permitted as a sub-query of another DistanceQuery.
func (q *DistanceQuery) DistanceSubQueryNotAllowed() string { return "" }

// FieldsQuery applies a SrndQuery against one or more fields. Mirrors
// org.apache.lucene.queryparser.surround.query.FieldsQuery.
type FieldsQuery struct {
	SrndQueryBase
	sub        SrndQuery
	fieldNames []string
	delim      rune
}

// NewFieldsQuery builds a multi-field wrapper around a sub-query.
func NewFieldsQuery(sub SrndQuery, fieldNames []string, delim rune) *FieldsQuery {
	return &FieldsQuery{sub: sub, fieldNames: fieldNames, delim: delim}
}

// GetFieldNames returns the field list.
func (q *FieldsQuery) GetFieldNames() []string { return q.fieldNames }

// GetFieldOperator returns the JavaCC delimiter character (default '/').
func (q *FieldsQuery) GetFieldOperator() rune { return q.delim }

// IsFieldsSubQueryAcceptable returns false — Lucene forbids nested FieldsQuery.
func (q *FieldsQuery) IsFieldsSubQueryAcceptable() bool { return false }

// MakeLuceneQueryField ignores the supplied field and instead applies the
// inner query to every configured field, OR'ing the resulting queries.
func (q *FieldsQuery) MakeLuceneQueryField(_ string, factory *BasicQueryFactory) (search.Query, error) {
	if !q.sub.IsFieldsSubQueryAcceptable() {
		return nil, NewParseException("FieldsQuery sub-query is not acceptable here: " + q.fieldOperatorString())
	}
	if len(q.fieldNames) == 1 {
		return q.sub.MakeLuceneQueryField(q.fieldNames[0], factory)
	}
	clauses := make([]*search.BooleanClause, 0, len(q.fieldNames))
	for _, fn := range q.fieldNames {
		sub, err := q.sub.MakeLuceneQueryField(fn, factory)
		if err != nil {
			return nil, err
		}
		if sub != nil {
			clauses = append(clauses, search.NewBooleanClause(sub, search.SHOULD))
		}
	}
	return newBooleanQueryFromClauses(clauses), nil
}

// String returns the string representation of the fields query. Mirrors
// org.apache.lucene.queryparser.surround.query.FieldsQuery#toString.
func (q *FieldsQuery) String() string {
	var sb strings.Builder
	sb.WriteByte('(')
	q.fieldNamesToString(&sb)
	sb.WriteString(q.sub.String())
	sb.WriteByte(')')
	return sb.String()
}

// fieldNamesToString appends each field name followed by the field operator.
// Mirrors FieldsQuery#fieldNamesToString.
func (q *FieldsQuery) fieldNamesToString(sb *strings.Builder) {
	for _, fn := range q.fieldNames {
		sb.WriteString(fn)
		sb.WriteRune(q.delim)
	}
}

func (q *FieldsQuery) fieldOperatorString() string {
	return string(q.delim) + strings.Join(q.fieldNames, string(q.delim))
}

func newBooleanQueryFromClauses(clauses []*search.BooleanClause) *search.BooleanQuery {
	bq := search.NewBooleanQueryBuilder()
	for _, c := range clauses {
		bq.AddClause(c)
	}
	return bq.Build()
}

func makeBooleanQuery(children []SrndQuery, field string, factory *BasicQueryFactory, occur search.Occur) (search.Query, error) {
	if len(children) == 0 {
		return search.NewBooleanQueryBuilder().Build(), nil
	}
	if len(children) == 1 {
		return children[0].MakeLuceneQueryField(field, factory)
	}
	clauses := make([]*search.BooleanClause, 0, len(children))
	for _, child := range children {
		sub, err := child.MakeLuceneQueryField(field, factory)
		if err != nil {
			return nil, err
		}
		if sub == nil {
			continue
		}
		clauses = append(clauses, search.NewBooleanClause(sub, occur))
	}
	return newBooleanQueryFromClauses(clauses), nil
}
