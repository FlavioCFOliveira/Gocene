// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// PrefixQuery matches documents containing terms with the given prefix.
type PrefixQuery struct {
	*BaseQuery
	prefix *index.Term
}

// NewPrefixQuery creates a new PrefixQuery.
func NewPrefixQuery(prefix *index.Term) *PrefixQuery {
	return &PrefixQuery{
		BaseQuery: &BaseQuery{},
		prefix:    prefix,
	}
}

// Prefix returns the prefix term.
func (q *PrefixQuery) Prefix() *index.Term {
	return q.prefix
}

// GetField returns the field name.
func (q *PrefixQuery) GetField() string {
	if q.prefix != nil {
		return q.prefix.Field
	}
	return ""
}

// Clone creates a copy of this query.
func (q *PrefixQuery) Clone() Query {
	return NewPrefixQuery(q.prefix.Clone())
}

// Equals checks if this query equals another.
func (q *PrefixQuery) Equals(other Query) bool {
	if o, ok := other.(*PrefixQuery); ok {
		return q.prefix.Equals(o.prefix)
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *PrefixQuery) HashCode() int {
	return q.prefix.HashCode()
}

// Rewrite rewrites the query to a simpler form.
func (q *PrefixQuery) Rewrite(reader IndexReader) (Query, error) {
	return q, nil
}

// String returns the Lucene-canonical string representation: "field:prefix*".
// For a nil prefix term the output is "<nil>:*".
func (q *PrefixQuery) String() string {
	if q.prefix == nil {
		return "<nil>:*"
	}
	return q.prefix.Field + ":" + q.prefix.Text() + "*"
}

// CreateWeight creates a Weight for this query.
//
// In Lucene 10.4.0 PrefixQuery extends AutomatonQuery and inherits its
// createWeight, which enumerates the field's terms sharing the prefix via the
// compiled automaton and unions their postings at a constant score
// (MultiTermQuery.CONSTANT_SCORE_BLENDED_REWRITE). Gocene models PrefixQuery
// as a standalone type, so CreateWeight builds the equivalent AutomatonQuery
// on demand and delegates to it. The previous implementation wrapped the
// PrefixQuery in a ConstantScoreQuery around itself, which recursed back into
// this method (and inherited the nil-Weight BaseQuery.CreateWeight before the
// CSQ fix), so the query never matched anything.
func (q *PrefixQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return q.toAutomatonQuery().CreateWeight(searcher, needsScores, boost)
}

// toAutomatonQuery builds the AutomatonQuery that backs this PrefixQuery,
// mirroring the Java PrefixQuery(Term, CONSTANT_SCORE_BLENDED_REWRITE) /
// super(prefix, toAutomaton(prefix.bytes()), true, rewriteMethod) chain. The
// automaton is byte-level (isBinary=true), matching the bytes() of the prefix
// term directly.
func (q *PrefixQuery) toAutomatonQuery() *AutomatonQuery {
	var prefixBytes []byte
	if q.prefix != nil {
		prefixBytes = q.prefix.BytesValue().ValidBytes()
	}
	auto := prefixAutomaton(prefixBytes)
	field := ""
	if q.prefix != nil {
		field = q.prefix.Field
	}
	return NewAutomatonQueryFull(
		index.NewTerm(field, ""),
		auto,
		true, // isBinary: the automaton operates over raw term bytes
		ConstantScoreBlendedRewrite,
	)
}

// prefixAutomaton builds a byte-level automaton accepting every term that
// starts with prefix. It is the Go port of PrefixQuery.toAutomaton(BytesRef)
// from Apache Lucene 10.4.0: a linear chain over the prefix bytes followed by
// an accepting state with a [0,255] self-loop that consumes any suffix.
func prefixAutomaton(prefix []byte) *automaton.Automaton {
	a := automaton.NewAutomaton()
	lastState := a.CreateState()
	for _, b := range prefix {
		state := a.CreateState()
		a.AddTransitionSingle(lastState, state, int(b))
		lastState = state
	}
	a.SetAccept(lastState, true)
	a.AddTransition(lastState, lastState, 0, 255)
	a.FinishState()
	return a
}

// NewPrefixQueryWithStrings creates a new PrefixQuery using strings.
func NewPrefixQueryWithStrings(field string, prefix string) *PrefixQuery {
	return NewPrefixQuery(index.NewTerm(field, prefix))
}
