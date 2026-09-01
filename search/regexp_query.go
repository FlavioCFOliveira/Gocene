package search

import (
	"fmt"
	"regexp"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// RegexpQuery matches documents containing terms that match a regular expression.
type RegexpQuery struct {
	field     string
	pattern   string
	automaton *automaton.Automaton
}

func NewRegexpQuery(field, pattern string) *RegexpQuery {
	return &RegexpQuery{
		field:     field,
		pattern:   pattern,
		automaton: nil,
	}
}

func (q *RegexpQuery) Rewrite(reader index.IndexReader) (Query, error) {
	terms := reader.GetTerms(q.field)
	if terms == nil {
		return &BooleanQuery{}, nil
	}

	builder, err := NewRegexpAutomatonBuilder(q.pattern)
	if err != nil {
		return nil, err
	}

	auto, err := builder.BuildAutomaton()
	if err != nil {
		return nil, err
	}

	bq := NewBooleanQueryBuilder()
	enum := terms.GetAutomatonEnum(auto)
	for enum.Next() {
		term := enum.Term()
		bq.Add(NewTermQuery(term), MUST)
	}

	return bq.Build(), nil
}

func (q *RegexpQuery) Clone() Query {
	return &RegexpQuery{
		field:     q.field,
		pattern:   q.pattern,
		automaton: q.automaton,
	}
}

func (q *RegexpQuery) Equals(other Query) bool {
	if otherQuery, ok := other.(*RegexpQuery); ok {
		return q.field == otherQuery.field && q.pattern == otherQuery.pattern
	}
	return false
}

func (q *RegexpQuery) HashCode() int {
	return q.field + q.pattern // Simplified
}

func (q *RegexpQuery) CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	// RegexpQuery is always rewritten before weight creation.
	return nil, fmt.Errorf("RegexpQuery must be rewritten before weight creation")
}

func (q *RegexpQuery) ToString(field string) string {
	return fmt.Sprintf("RegexpQuery(%s, %s)", q.field, q.pattern)
}
