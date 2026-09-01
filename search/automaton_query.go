package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// AutomatonQuery matches terms against a finite-state machine.
type AutomatonQuery struct {
	MultiTermQuery
	automaton *automaton.Automaton
	compiled  *automaton.CompiledAutomaton
	term      *index.Term
	isBinary   bool
}

func NewAutomatonQuery(term *index.Term, automaton *automaton.Automaton, isBinary bool, rewriteMethod RewriteMethod) *AutomatonQuery {
	return &AutomatonQuery{
		MultiTermQuery: *NewMultiTermQuery(term.Field(), rewriteMethod, func(terms index.Terms) (index.TermsEnum, error) {
			return nil, nil
		}),
		automaton: automaton,
		compiled:  automaton.NewCompiledAutomaton(automaton, false, true, isBinary),
		term:      term,
		isBinary:  isBinary,
	}
}

func (q *AutomatonQuery) GetTermsEnum(terms index.Terms) (index.TermsEnum, error) {
	return q.compiled.GetTermsEnum(terms), nil
}

func (q *AutomatonQuery) HashCode() int {
	return q.MultiTermQuery.HashCode() ^ q.compiled.HashCode()
}

func (q *AutomatonQuery) Equals(other Query) bool {
	if otherQuery, ok := other.(*AutomatonQuery); ok {
		if !q.MultiTermQuery.Equals(otherQuery) {
			return false
		}
		if !q.compiled.Equals(otherQuery.compiled) {
			return false
		}
		return q.term.Equals(otherQuery.term)
	}
	return false
}

func (q *AutomatonQuery) ToString(field string) string {
	return "AutomatonQuery{" + q.automaton.String() + "}"
}

func (q *AutomatonQuery) Visit(visitor QueryVisitor) {
	if visitor.AcceptField(q.field) {
		q.compiled.Visit(visitor, q, q.field)
	}
}

func (q *AutomatonQuery) GetAutomaton() *automaton.Automaton {
	return q.automaton
}

func (q *AutomatonQuery) GetCompiled() *automaton.CompiledAutomaton {
	return q.compiled
}

func (q *AutomatonQuery) IsAutomatonBinary() bool {
	return q.isBinary
}
