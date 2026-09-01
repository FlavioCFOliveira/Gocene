package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// PrefixQuery matches documents containing terms with a specified prefix.
type PrefixQuery struct {
	AutomatonQuery
	term *index.Term
}

func NewPrefixQuery(prefix *index.Term) *PrefixQuery {
	return NewPrefixQueryWithRewrite(prefix, CONSTANT_SCORE_BLENDED_REWRITE)
}

func NewPrefixQueryWithRewrite(prefix *index.Term, rewriteMethod RewriteMethod) *PrefixQuery {
	auto := automaton.MakePrefixAutomaton(prefix.Bytes())
	return &PrefixQuery{
		AutomatonQuery: *NewAutomatonQuery(prefix, auto, true, rewriteMethod),
		term:           prefix,
	}
}

func (q *PrefixQuery) GetPrefix() *index.Term {
	return q.term
}

func (q *PrefixQuery) ToString(field string) string {
	if q.field != field {
		return q.field + ":" + q.term.Text() + "*"
	}
	return q.term.Text() + "*"
}

func (q *PrefixQuery) HashCode() int {
	return q.AutomatonQuery.HashCode() ^ q.term.HashCode()
}

func (q *PrefixQuery) Equals(other Query) bool {
	if otherQuery, ok := other.(*PrefixQuery); ok {
		if !q.AutomatonQuery.Equals(otherQuery) {
			return false
		}
		return q.term.Equals(otherQuery.term)
	}
	return false
}
