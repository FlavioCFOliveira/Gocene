package search

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// AutomatonQuery matches terms against a finite-state machine.
type AutomatonQuery struct {
	MultiTermQuery
	automaton *automaton.Automaton
	compiled  *automaton.CompiledAutomaton
	term      *index.Term
	isBinary  bool
}

// NewAutomatonQuery creates a new AutomatonQuery from an Automaton.
//
// The Java field is also named automaton; Go's flat file-level namespace makes
// that identifier shadow the util/automaton package, so the parameter carries
// the abbreviated name while the field keeps Lucene's.
func NewAutomatonQuery(term *index.Term, a *automaton.Automaton, isBinary bool, rewriteMethod RewriteMethod) *AutomatonQuery {
	q := &AutomatonQuery{
		MultiTermQuery: *NewMultiTermQuery(term.Field, rewriteMethod),
		automaton:      a,
		compiled:       automaton.NewCompiledAutomaton(a, false, true, isBinary),
		term:           term,
		isBinary:       isBinary,
	}
	q.MultiTermQuery.SetOwner(q)
	return q
}

// GetTermsEnumWithAttributes returns a TermsEnum over the terms of this field
// that the automaton accepts.
//
// Mirrors AutomatonQuery.getTermsEnum(Terms, AttributeSource), whose body is
// compiled.getTermsEnum(terms); that member of CompiledAutomaton is rendered
// as index.CompiledAutomatonTermsEnum (see index/compiled_automaton.go).
func (q *AutomatonQuery) GetTermsEnumWithAttributes(terms index.Terms, atts *util.AttributeSource) (index.TermsEnum, error) {
	return index.CompiledAutomatonTermsEnum(q.compiled, terms)
}

// Compile-time assertion that AutomatonQuery supplies the abstract
// MultiTermQuery#getTermsEnum(Terms, AttributeSource) body.
var _ MultiTermQueryOwner = (*AutomatonQuery)(nil)

func (q *AutomatonQuery) HashCode() int {
	return q.MultiTermQuery.HashCode() ^ q.compiled.HashCode()
}

func (q *AutomatonQuery) Equals(other spi.Query) bool {
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
		CompiledAutomatonVisit(q.compiled, visitor, q, q.field)
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
