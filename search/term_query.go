package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TermQuery matches documents containing a term.
type TermQuery struct {
	term              *index.Term
	perReaderTermState *index.TermStates
}

func NewTermQuery(t *index.Term) *TermQuery {
	return &TermQuery{
		term: t,
	}
}

func NewTermQueryWithStates(t *index.Term, states *index.TermStates) *TermQuery {
	return &TermQuery{
		term:              t,
		perReaderTermState: states,
	}
}

func (q *TermQuery) GetTerm() *index.Term {
	return q.term
}

func (q *TermQuery) Rewrite(reader index.IndexReader) (Query, error) {
	return q, nil
}

func (q *TermQuery) Clone() Query {
	return q
}

func (q *TermQuery) Equals(other Query) bool {
	if otherQuery, ok := other.(*TermQuery); ok {
		return q.term.Equals(otherQuery.term)
	}
	return false
}

func (q *TermQuery) HashCode() int {
	return q.term.HashCode()
}

func (q *TermQuery) CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	context := searcher.GetReader().GetTopReaderContext()
	var termState *index.TermStates
	if q.perReaderTermState == nil || !q.perReaderTermState.WasBuiltFor(context) {
		var err error
		termState, err = index.BuildTermStates(searcher.GetReader(), q.term, scoreMode.NeedsScores())
		if err != nil {
			return nil, err
		}
	} else {
		termState = q.perReaderTermState
	}

	return NewTermWeight(searcher, q.term, scoreMode, boost, termState), nil
}

func (q *TermQuery) ToString(field string) string {
	if q.term.Field() != field {
		return fmt.Sprintf("%s:%s", q.term.Field(), q.term.Text())
	}
	return q.term.Text()
}
