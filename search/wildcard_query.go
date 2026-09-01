package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

const (
	WILDCARD_STRING = '*'
	WILDCARD_CHAR    = '?'
	WILDCARD_ESCAPE  = '\\'
)

type WildcardQuery struct {
	AutomatonQuery
	term *index.Term
}

func NewWildcardQuery(term *index.Term) *WildcardQuery {
	return NewWildcardQueryWithLimit(term, 1000) // Default limit
}

func NewWildcardQueryWithLimit(term *index.Term, limit int) *WildcardQuery {
	return NewWildcardQueryWithRewrite(term, limit, CONSTANT_SCORE_BLENDED_REWRITE)
}

func NewWildcardQueryWithRewrite(term *index.Term, limit int, rewriteMethod RewriteMethod) *WildcardQuery {
	auto := ToAutomaton(term, limit)
	return &WildcardQuery{
		AutomatonQuery: *NewAutomatonQuery(term, auto, false, rewriteMethod),
		term:           term,
	}
}

func ToAutomaton(term *index.Term, limit int) *automaton.Automaton {
	text := term.Text()
	var automata []*automaton.Automaton

	for i := 0; i < len(text); {
		c := text[i]
		switch c {
		case WILDCARD_STRING:
			automata = append(automata, automaton.MakeAnyString())
			i++
		case WILDCARD_CHAR:
			automata = append(automata, automaton.MakeAnyChar())
			i++
		case WILDCARD_ESCAPE:
			if i+1 < len(text) {
				automata = append(automata, automaton.MakeChar(int(text[i+1])))
				i += 2
			} else {
				automata = append(automata, automaton.MakeChar(int(c)))
				i++
			}
		default:
			automata = append(automata, automaton.MakeChar(int(c)))
			i++
		}
	}

	return automaton.Concatenate(automata)
}

func (q *WildcardQuery) GetTerm() *index.Term {
	return q.term
}

func (q *WildcardQuery) ToString(field string) string {
	if q.field != field {
		return q.field + ":" + q.term.Text()
	}
	return q.term.Text()
}
