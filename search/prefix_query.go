package search

import (
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// PrefixQuery matches documents containing terms with a specified prefix.
// A PrefixQuery is built by QueryParser for input like `app*`.
//
// This query uses the MultiTermQuery.CONSTANT_SCORE_BLENDED_REWRITE rewrite method.
//
// Ported from org.apache.lucene.search.PrefixQuery (Lucene 10.5.0).
type PrefixQuery struct {
	AutomatonQuery
}

// NewPrefixQuery constructs a query for terms starting with prefix.
func NewPrefixQuery(prefix *index.Term) *PrefixQuery {
	return NewPrefixQueryWithRewriteMethod(prefix, CONSTANT_SCORE_BLENDED_REWRITE)
}

// NewPrefixQueryWithRewriteMethod constructs a query for terms starting with prefix using a defined RewriteMethod.
func NewPrefixQueryWithRewriteMethod(prefix *index.Term, rewriteMethod RewriteMethod) *PrefixQuery {
	return &PrefixQuery{
		AutomatonQuery: *NewAutomatonQuery(prefix, ToAutomaton(prefix.Bytes()), true, rewriteMethod),
	}
}

// ToAutomaton builds an automaton accepting all terms with the specified prefix.
func ToAutomaton(prefix *util.BytesRef) *automaton.Automaton {
	numStatesAndTransitions := prefix.Length() + 1
	auto := automaton.NewAutomatonWithCapacity(numStatesAndTransitions, numStatesAndTransitions)
	lastState := auto.CreateState()
	for i := 0; i < prefix.Length(); i++ {
		state := auto.CreateState()
		b := prefix.Bytes()[i]
		auto.AddTransition(lastState, state, int(b)&0xff, int(b)&0xff)
		lastState = state
	}
	auto.SetAccept(lastState, true)
	auto.AddTransition(lastState, lastState, 0, 255)
	auto.FinishState()
	return auto
}

// Prefix returns the prefix of this query.
func (q *PrefixQuery) Prefix() *index.Term {
	return q.AutomatonQuery.term
}

// ToString prints a user-readable version of this query.
func (q *PrefixQuery) ToString(field string) string {
	var sb strings.Builder
	if q.GetField() != field {
		sb.WriteString(q.GetField())
		sb.WriteByte(':')
	}
	sb.WriteString(q.Prefix().Text())
	sb.WriteByte('*')
	return sb.String()
}

// HashCode returns the hash code of this query.
func (q *PrefixQuery) HashCode() int {
	prime := 31
	result := q.AutomatonQuery.HashCode()
	result = prime*result + q.Prefix().HashCode()
	return result
}

// Equals reports whether another object is equal to this query.
func (q *PrefixQuery) Equals(other Query) bool {
	if q == other {
		return true
	}
	if !q.AutomatonQuery.Equals(other) {
		return false
	}
	// AutomatonQuery.Equals ensures we are the same class
	otherPQ, ok := other.(*PrefixQuery)
	if !ok {
		return false
	}
	return q.Prefix().Equals(otherPQ.Prefix())
}
