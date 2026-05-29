// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// Wildcard syntax characters, mirroring WildcardQuery.WILDCARD_* in Lucene 10.4.0.
const (
	wildcardString = '*'  // matches any character sequence, including empty
	wildcardChar   = '?'  // matches any single character
	wildcardEscape = '\\' // escapes the following character
)

// WildcardQuery matches documents containing terms matching a wildcard pattern.
// ? matches any single character
// * matches any character sequence (including empty)
type WildcardQuery struct {
	*BaseQuery
	term *index.Term
}

// NewWildcardQuery creates a new WildcardQuery.
func NewWildcardQuery(term *index.Term) *WildcardQuery {
	return &WildcardQuery{
		BaseQuery: &BaseQuery{},
		term:      term,
	}
}

// Term returns the wildcard term.
func (q *WildcardQuery) Term() *index.Term {
	return q.term
}

// GetField returns the field name.
func (q *WildcardQuery) GetField() string {
	if q.term != nil {
		return q.term.Field
	}
	return ""
}

// Pattern returns the wildcard pattern.
func (q *WildcardQuery) Pattern() []byte {
	if q.term != nil {
		return q.term.Bytes.Bytes
	}
	return nil
}

// Clone creates a copy of this query.
func (q *WildcardQuery) Clone() Query {
	if q.term == nil {
		return &WildcardQuery{
			BaseQuery: &BaseQuery{},
			term:      nil,
		}
	}
	return &WildcardQuery{
		BaseQuery: &BaseQuery{},
		term:      q.term.Clone(),
	}
}

// Equals checks if this query equals another.
func (q *WildcardQuery) Equals(other Query) bool {
	if o, ok := other.(*WildcardQuery); ok {
		if q.term == nil || o.term == nil {
			return q.term == nil && o.term == nil
		}
		return q.term.Equals(o.term)
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *WildcardQuery) HashCode() int {
	if q.term != nil {
		return q.term.HashCode()
	}
	return 0
}

// Rewrite rewrites the query to a simpler form.
func (q *WildcardQuery) Rewrite(reader IndexReader) (Query, error) {
	return q, nil
}

// String returns the Lucene-canonical representation "field:pattern".
// For a nil term the output is "<nil>".
func (q *WildcardQuery) String() string {
	if q.term == nil {
		return "<nil>"
	}
	return q.term.Field + ":" + q.term.Text()
}

// CreateWeight creates a Weight for this query.
//
// In Lucene 10.4.0 WildcardQuery extends AutomatonQuery and inherits its
// createWeight, which enumerates the field's terms matching the wildcard
// automaton and unions their postings at a constant score
// (MultiTermQuery.CONSTANT_SCORE_BLENDED_REWRITE). Gocene models WildcardQuery
// as a standalone type, so CreateWeight builds the equivalent AutomatonQuery on
// demand and delegates to it. The previous implementation wrapped the
// WildcardQuery in a ConstantScoreQuery around itself, which recursed back into
// this method, so the query never matched anything.
func (q *WildcardQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return q.toAutomatonQuery().CreateWeight(searcher, needsScores, boost)
}

// toAutomatonQuery builds the AutomatonQuery that backs this WildcardQuery,
// mirroring the Java WildcardQuery(Term, DEFAULT_DETERMINIZE_WORK_LIMIT,
// CONSTANT_SCORE_BLENDED_REWRITE) / super(term, toAutomaton(term, ...), false,
// rewriteMethod) chain. The automaton is code-point level (isBinary=false);
// CompileFull converts it to a byte (UTF-8) run automaton internally.
func (q *WildcardQuery) toAutomatonQuery() *AutomatonQuery {
	pattern := ""
	field := ""
	if q.term != nil {
		pattern = q.term.Text()
		field = q.term.Field
	}
	auto := wildcardAutomaton(pattern)
	return NewAutomatonQueryFull(
		index.NewTerm(field, ""),
		auto,
		false, // isBinary=false: code-point automaton, UTF-8 conversion applied on compile
		ConstantScoreBlendedRewrite,
	)
}

// wildcardAutomaton converts Lucene wildcard syntax into a determinized
// code-point automaton. It is the Go port of WildcardQuery.toAutomaton(Term,
// int) from Apache Lucene 10.4.0:
//   - '*' contributes Automata.makeAnyString()
//   - '?' contributes Automata.makeAnyChar()
//   - '\' escapes the next code point (lenient: a trailing '\' is literal)
//   - any other code point contributes Automata.makeChar(cp)
//
// The per-character automata are concatenated and determinized.
func wildcardAutomaton(pattern string) *automaton.Automaton {
	runes := []rune(pattern)
	automata := make([]*automaton.Automaton, 0, len(runes))
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		switch c {
		case wildcardString:
			automata = append(automata, automaton.MakeAnyString())
		case wildcardChar:
			automata = append(automata, automaton.MakeAnyChar())
		case wildcardEscape:
			// Add the next code point instead, if it exists; otherwise the
			// trailing '\' is treated literally (lenient parsing).
			if i+1 < len(runes) {
				i++
				automata = append(automata, automaton.MakeChar(int(runes[i])))
			} else {
				automata = append(automata, automaton.MakeChar(int(c)))
			}
		default:
			automata = append(automata, automaton.MakeChar(int(c)))
		}
	}
	concatenated := automaton.Concatenate(automata)
	det, err := automaton.Determinize(concatenated, automaton.DefaultDeterminizeWorkLimit)
	if err != nil {
		// Determinization can only fail by exceeding the work limit; fall back
		// to the (possibly non-deterministic) concatenation, which CompileFull
		// will run via the NFA path.
		return concatenated
	}
	return det
}

// NewWildcardQueryWithStrings creates a new WildcardQuery using strings.
func NewWildcardQueryWithStrings(field string, pattern string) *WildcardQuery {
	return NewWildcardQuery(index.NewTerm(field, pattern))
}
