package search

import (
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

const (
	WILDCARD_STRING = '*'
	WILDCARD_CHAR    = '?'
	WILDCARD_ESCAPE  = '\\'
)

// WildcardQuery implements the wildcard search query. Supported wildcards are '*',
// which matches any character sequence (including the empty one), and '?',
// which matches any single character. '\' is the escape character.
type WildcardQuery struct {
	AutomatonQuery
	term *index.Term
}

// NewWildcardQuery constructs a query for terms matching term.
func NewWildcardQuery(term *index.Term) *WildcardQuery {
	return NewWildcardQueryWithLimit(term, automaton.DefaultDeterminizeWorkLimit)
}

// NewWildcardQueryWithLimit constructs a query for terms matching term.
// limit is the maximum effort to spend while compiling the automaton from this
// wildcard. Set higher to allow more complex queries and lower to prevent memory exhaustion.
func NewWildcardQueryWithLimit(term *index.Term, limit int) *WildcardQuery {
	return NewWildcardQueryWithRewrite(term, limit, CONSTANT_SCORE_BLENDED_REWRITE)
}

// NewWildcardQueryWithRewrite constructs a query for terms matching term.
// limit is the maximum effort to spend while compiling the automaton from this
// wildcard. rewriteMethod is the rewrite method to use when building the final query.
func NewWildcardQueryWithRewrite(term *index.Term, limit int, rewriteMethod RewriteMethod) *WildcardQuery {
	auto := ToAutomaton(term, limit)
	return &WildcardQuery{
		AutomatonQuery: *NewAutomatonQuery(term, auto, false, rewriteMethod),
		term:           term,
	}
}

// ToAutomaton converts Lucene wildcard syntax into an automaton.
func ToAutomaton(term *index.Term, limit int) *automaton.Automaton {
	var automata []*automaton.Automaton
	text := term.Text()

	runes := []rune(text)
	for i := 0; i < len(runes); {
		r := runes[i]
		switch r {
		case WILDCARD_STRING:
			automata = append(automata, automaton.MakeAnyString())
			i++
		case WILDCARD_CHAR:
			automata = append(automata, automaton.MakeAnyChar())
			i++
		case WILDCARD_ESCAPE:
			// add the next codepoint instead, if it exists
			if i+1 < len(runes) {
				next := runes[i+1]
				automata = append(automata, automaton.MakeChar(int(next)))
				i += 2
			} else {
				// lenient parsing with a trailing \
				automata = append(automata, automaton.MakeChar(int(r)))
				i++
			}
		default:
			automata = append(automata, automaton.MakeChar(int(r)))
			i++
		}
	}

	concatenated := automaton.Concatenate(automata)
	det, err := automaton.Determinize(concatenated, limit)
	if err != nil {
		// In Java, this throws an exception (TooComplexToDeterminizeException).
		// To match that behavior, we panic.
		panic(err)
	}
	return det
}

// GetTerm returns the pattern term.
func (q *WildcardQuery) GetTerm() *index.Term {
	return q.term
}

// ToString prints a user-readable version of this query.
func (q *WildcardQuery) ToString(field string) string {
	var sb strings.Builder
	if q.GetField() != field {
		sb.WriteString(q.GetField())
		sb.WriteString(":")
	}
	sb.WriteString(q.term.Text())
	return sb.String()
}
