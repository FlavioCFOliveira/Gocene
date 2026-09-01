package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

type FuzzyQuery struct {
	field         string
	term          string
	maxEdits      int
	prefixLength  int
	transpositions bool
}

func NewFuzzyQuery(field, term string, maxEdits int) *FuzzyQuery {
	return NewFuzzyQueryAdvanced(field, term, maxEdits, 0, true)
}

func NewFuzzyQueryAdvanced(field, term string, maxEdits, prefixLength int, transpositions bool) *FuzzyQuery {
	return &FuzzyQuery{
		field:          field,
		term:           term,
		maxEdits:       maxEdits,
		prefixLength:   prefixLength,
		transpositions: transpositions,
	}
}

func (q *FuzzyQuery) Rewrite(reader index.IndexReader) (Query, error) {
	terms := reader.GetTerms(q.field)
	if terms == nil {
		return &BooleanQuery{}, nil
	}

	builder, err := NewFuzzyAutomatonBuilder(q.term, q.maxEdits, q.prefixLength, q.transpositions)
	if err != nil {
		return nil, err
	}

	auto := builder.BuildMaxEditAutomaton()

	bq := NewBooleanQueryBuilder()
	enum := terms.GetAutomatonEnum(auto)
	for enum.Next() {
		term := enum.Term()
		bq.Add(NewTermQuery(term), MUST)
	}

	return bq.Build(), nil
}

func (q *FuzzyQuery) Clone() Query {
	return &FuzzyQuery{
		field:    q.field,
		term:     q.term,
		maxEdits: q.maxEdits,
	}
}

func (q *FuzzyQuery) Equals(other Query) bool {
	if otherQuery, ok := other.(*FuzzyQuery); ok {
		return q.field == otherQuery.field && q.term == otherQuery.term && q.maxEdits == otherQuery.maxEdits
	}
	return false
}

func (q *FuzzyQuery) HashCode() int {
	return q.field + q.term // Simplified
}

func (q *FuzzyQuery) CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	return nil, fmt.Errorf("FuzzyQuery must be rewritten before weight creation")
}

func (q *FuzzyQuery) ToString(field string) string {
	return fmt.Sprintf("FuzzyQuery(%s, %s, %d)", q.field, q.term, q.maxEdits)
}
