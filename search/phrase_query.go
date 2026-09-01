package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// PhraseQuery matches documents containing a particular sequence of terms.
type PhraseQuery struct {
	slop      int
	field     string
	terms     []*index.Term
	positions []int
}

func NewPhraseQuery(slop int, field string, terms ...*index.Term) *PhraseQuery {
	positions := make([]int, len(terms))
	for i := range positions {
		positions[i] = i
	}
	return &PhraseQuery{
		slop:      slop,
		field:     field,
		terms:     terms,
		positions: positions,
	}
}

func (q *PhraseQuery) Rewrite(reader IndexReader) (Query, error) {
	if len(q.terms) == 0 {
		return nil, nil // Should return MatchNoDocsQuery
	} else if len(q.terms) == 1 {
		return NewTermQuery(q.terms[0]), nil
	}
	return q, nil
}

func (q *PhraseQuery) Clone() Query {
	t := append([]*index.Term(nil), q.terms...)
	p := append([]int(nil), q.positions...)
	return &PhraseQuery{
		slop:      q.slop,
		field:     q.field,
		terms:     t,
		positions: p,
	}
}

func (q *PhraseQuery) Equals(other Query) bool {
	if otherQuery, ok := other.(*PhraseQuery); ok {
		if q.slop != otherQuery.slop || q.field != otherQuery.field {
			return false
		}
		if len(q.terms) != len(otherQuery.terms) {
			return false
		}
		for i := range q.terms {
			if !q.terms[i].Equals(otherQuery.terms[i]) {
				return false
			}
		}
		for i := range q.positions {
			if q.positions[i] != otherQuery.positions[i] {
				return false
			}
		}
		return true
	}
	return false
}

func (q *PhraseQuery) HashCode() int {
	return 0 // placeholder
}

func (q *PhraseQuery) CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	return NewPhraseWeight(q, searcher, scoreMode, boost), nil
}

func (q *PhraseQuery) ToString(field string) string {
	return fmt.Sprintf("PhraseQuery(%s, slop=%d)", q.field, q.slop)
}
