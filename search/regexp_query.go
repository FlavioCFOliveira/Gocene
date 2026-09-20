// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// RegexpQuery is a fast regular expression query based on the automaton package.
//
// Ported from org.apache.lucene.search.RegexpQuery.
type RegexpQuery struct {
	AutomatonQuery
}

// DefaultAutomatonProvider provides no named automata.
type defaultAutomatonProvider struct{}

func (p *defaultAutomatonProvider) GetAutomaton(name string) (*automaton.Automaton, error) {
	return nil, nil
}

// DefaultAutomatonProvider is the default provider that returns no named automata.
var DefaultAutomatonProvider automaton.AutomatonProvider = &defaultAutomatonProvider{}

// ConstantScoreBlendedRewrite, the default rewrite method for RegexpQuery, is
// declared by MultiTermQuery (CONSTANT_SCORE_BLENDED_REWRITE) and therefore
// lives in multi_term_query.go, alongside the other RewriteMethod constants.

// NewRegexpQuery constructs a query for terms matching the regular expression in term.
// By default, all regular expression features are enabled.
func NewRegexpQuery(term *index.Term) *RegexpQuery {
	return NewRegexpQueryWithFlags(term, automaton.RegExpAll)
}

// NewRegexpQueryWithFlags constructs a query for terms matching the regular expression in term.
func NewRegexpQueryWithFlags(term *index.Term, flags int) *RegexpQuery {
	return NewRegexpQueryWithLimit(term, flags, automaton.DefaultDeterminizeWorkLimit)
}

// NewRegexpQueryWithLimit constructs a query for terms matching the regular expression in term.
func NewRegexpQueryWithLimit(term *index.Term, flags int, limit int) *RegexpQuery {
	return NewRegexpQueryWithProvider(term, flags, DefaultAutomatonProvider, limit)
}

// NewRegexpQueryWithMatchFlags constructs a query for terms matching the regular expression in term.
func NewRegexpQueryWithMatchFlags(term *index.Term, syntaxFlags int, matchFlags int, limit int) *RegexpQuery {
	return NewRegexpQueryFull(term, syntaxFlags, matchFlags, DefaultAutomatonProvider, limit, ConstantScoreBlendedRewrite, true)
}

// NewRegexpQueryWithProvider constructs a query for terms matching the regular expression in term.
func NewRegexpQueryWithProvider(term *index.Term, syntaxFlags int, provider automaton.AutomatonProvider, limit int) *RegexpQuery {
	return NewRegexpQueryFull(term, syntaxFlags, 0, provider, limit, ConstantScoreBlendedRewrite, true)
}

// NewRegexpQueryFull constructs a query for terms matching the regular expression in term.
func NewRegexpQueryFull(
	term *index.Term,
	syntaxFlags int,
	matchFlags int,
	provider automaton.AutomatonProvider,
	limit int,
	rewriteMethod RewriteMethod,
	doDeterminization bool,
) *RegexpQuery {
	// Create the RegExp AST
	regexp, err := automaton.NewRegExpFlags(term.Text(), syntaxFlags, matchFlags)
	if err != nil {
		// In Java, the constructor might throw an exception. In Go, we'd normally return an error.
		// However, to match the "translation-only" approach and the existing NewAutomatonQuery signature
		// which doesn't return an error, we must handle this.
		// Since NewRegexpQuery is a constructor for a Query, and Querys are often created in bulk,
		// we will panic here to mirror the Java constructor's behavior of throwing a RuntimeException
		// if the regex is invalid.
		panic(fmt.Sprintf("regexp: %v", err))
	}

	// Convert AST to Automaton
	aut := toAutomaton(regexp, limit, provider, doDeterminization)
	if aut == nil {
		// This should not happen if NewRegExpFlags succeeded, but for safety:
		panic("regexp: failed to produce automaton")
	}

	q := &RegexpQuery{
		AutomatonQuery: *NewAutomatonQuery(term, aut, false, rewriteMethod),
	}
	// The embedded AutomatonQuery was copied by value, so the owner installed
	// by NewAutomatonQuery points at the temporary: re-install it on the final
	// object. See MultiTermQuery.SetOwner.
	q.MultiTermQuery.SetOwner(q)
	return q
}

func toAutomaton(regexp *automaton.RegExp, limit int, provider automaton.AutomatonProvider, doDeterminization bool) *automaton.Automaton {
	if doDeterminization {
		nfa, err := regexp.ToAutomatonWith(nil, provider)
		if err != nil {
			panic(fmt.Sprintf("regexp: %v", err))
		}
		aut, err := automaton.Determinize(nfa, limit)
		if err != nil {
			// Mirror Java's behavior of throwing an exception if determinization fails
			panic(fmt.Sprintf("regexp: %v", err))
		}
		return aut
	}
	aut, err := regexp.ToAutomatonWith(nil, provider)
	if err != nil {
		panic(fmt.Sprintf("regexp: %v", err))
	}
	return aut
}

// GetRegexp returns the regexp of this query wrapped in a Term.
func (q *RegexpQuery) GetRegexp() *index.Term {
	return q.term
}

// ToString prints a user-readable version of this query.
func (q *RegexpQuery) ToString(field string) string {
	var sb strings.Builder
	if q.term.Field != field {
		sb.WriteString(q.term.Field)
		sb.WriteString(":")
	}
	sb.WriteByte('/')
	sb.WriteString(q.term.Text())
	sb.WriteByte('/')
	return sb.String()
}
