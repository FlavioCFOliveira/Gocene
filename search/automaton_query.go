// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

const (
	// ScoringBooleanRewrite rewrites to a scoring boolean query
	ScoringBooleanRewrite = "scoring_boolean"
	// ConstantScoreRewrite rewrites to a constant score query
	ConstantScoreRewrite = "constant_score"
	// ConstantScoreBlendedRewrite rewrites to a constant score query with blended term weights
	ConstantScoreBlendedRewrite = "constant_score_blended"
	// ConstantScoreBooleanRewrite rewrites to a constant score boolean query
	ConstantScoreBooleanRewrite = "constant_score_boolean"
)

// AutomatonQuery is a query that matches terms against a finite-state machine.
// This query can be used for complex pattern matching, including:
// - Regular expression matching
// - Wildcard matching
// - Fuzzy matching
// - Range queries on terms
//
// The query works by:
// 1. Compiling the automaton to a deterministic, minimized form
// 2. Enumerating terms from the index that match the automaton
// 3. Creating a scoring query based on the matched terms
//
// Thread-safe: Yes. The compiled automaton is immutable after construction.
type AutomatonQuery struct {
	BaseQuery
	term              *index.Term
	automaton         *automaton.Automaton
	compiledAutomaton *automaton.CompiledAutomaton
	isBinary          bool
	rewriteMethod     string

	// Cached hash code for performance
	hashCodeOnce sync.Once
	hashCodeVal  int
}

// NewAutomatonQuery creates a new AutomatonQuery.
//
// Parameters:
//   - term: The term containing the field to search
//   - automaton: The automaton defining the matching pattern
//
// Returns: A new AutomatonQuery instance
//
// Example:
//
//	// Create an automaton that matches "test"
//	a := automaton.Automata.MakeString("test")
//	query := NewAutomatonQuery(index.NewTerm("field", "test"), a)
func NewAutomatonQuery(term *index.Term, automaton *automaton.Automaton) *AutomatonQuery {
	return NewAutomatonQueryWithBinary(term, automaton, false)
}

// NewAutomatonQueryWithBinary creates a new AutomatonQuery with binary flag.
//
// Parameters:
//   - term: The term containing the field to search
//   - automaton: The automaton defining the matching pattern
//   - isBinary: If true, treat the automaton as binary (byte-level matching)
//
// Returns: A new AutomatonQuery instance
func NewAutomatonQueryWithBinary(term *index.Term, automaton *automaton.Automaton, isBinary bool) *AutomatonQuery {
	return NewAutomatonQueryFull(term, automaton, isBinary, ConstantScoreBlendedRewrite)
}

// NewAutomatonQueryFull creates a new AutomatonQuery with all options.
//
// Parameters:
//   - term: The term containing the field to search
//   - automaton: The automaton defining the matching pattern
//   - isBinary: If true, treat the automaton as binary (byte-level matching)
//   - rewriteMethod: How to rewrite this query (see rewrite constants)
//
// Returns: A new AutomatonQuery instance
func NewAutomatonQueryFull(term *index.Term, auto *automaton.Automaton, isBinary bool, rewriteMethod string) *AutomatonQuery {
	compiled := automaton.Compile(auto)

	return &AutomatonQuery{
		term:              term,
		automaton:         auto,
		compiledAutomaton: compiled,
		isBinary:          isBinary,
		rewriteMethod:     rewriteMethod,
	}
}

// GetTerm returns the term (field) being searched.
func (aq *AutomatonQuery) GetTerm() *index.Term {
	return aq.term
}

// GetAutomaton returns the automaton used by this query.
func (aq *AutomatonQuery) GetAutomaton() *automaton.Automaton {
	return aq.automaton
}

// GetCompiledAutomaton returns the compiled automaton for efficient matching.
func (aq *AutomatonQuery) GetCompiledAutomaton() *automaton.CompiledAutomaton {
	return aq.compiledAutomaton
}

// IsBinary returns true if this automaton is treated as binary.
func (aq *AutomatonQuery) IsBinary() bool {
	return aq.isBinary
}

// GetRewriteMethod returns the rewrite method for this query.
func (aq *AutomatonQuery) GetRewriteMethod() string {
	return aq.rewriteMethod
}

// Rewrite rewrites the query to a simpler form.
// This method implements the Query interface.
func (aq *AutomatonQuery) Rewrite(reader IndexReader) (Query, error) {
	// Check if automaton is empty (matches nothing)
	if aq.compiledAutomaton.Type() == "NONE" {
		return NewMatchNoDocsQuery(), nil
	}

	// Check if automaton matches all strings
	if aq.compiledAutomaton.Type() == "ALL" {
		// Rewrite to field exists query
		return NewFieldExistsQuery(aq.term.Field), nil
	}

	// Check if automaton matches a single term
	if aq.compiledAutomaton.Type() == "SINGLE" {
		singleTerm := aq.compiledAutomaton.GetTerm()
		return NewTermQuery(index.NewTerm(aq.term.Field, singleTerm)), nil
	}

	// For complex automata, return a multi-term query
	// In a full implementation, this would create a
	// MultiTermQuery or BooleanQuery with the matched terms
	switch aq.rewriteMethod {
	case ScoringBooleanRewrite:
		return aq.rewriteScoringBoolean(reader)
	case ConstantScoreRewrite:
		return aq.rewriteConstantScore(reader)
	case ConstantScoreBlendedRewrite:
		return aq.rewriteConstantScoreBlended(reader)
	case ConstantScoreBooleanRewrite:
		return aq.rewriteConstantScoreBoolean(reader)
	default:
		return aq.rewriteConstantScoreBlended(reader)
	}
}

// rewriteScoringBoolean rewrites to a scoring boolean query.
func (aq *AutomatonQuery) rewriteScoringBoolean(reader IndexReader) (Query, error) {
	// In a full implementation, this would:
	// 1. Enumerate all matching terms
	// 2. Create a BooleanQuery with SHOULD clauses for each term
	// 3. Score based on term frequency

	// For now, return the query itself (will use automaton matching)
	return aq, nil
}

// rewriteConstantScore rewrites to a constant score query.
func (aq *AutomatonQuery) rewriteConstantScore(reader IndexReader) (Query, error) {
	// Create a constant score query wrapping this query
	return NewConstantScoreQuery(aq), nil
}

// rewriteConstantScoreBlended rewrites to a constant score query with blended weights.
func (aq *AutomatonQuery) rewriteConstantScoreBlended(reader IndexReader) (Query, error) {
	// This is the default and recommended rewrite method
	// It provides good performance while maintaining reasonable scoring
	return NewConstantScoreQuery(aq), nil
}

// rewriteConstantScoreBoolean rewrites to a constant score boolean query.
func (aq *AutomatonQuery) rewriteConstantScoreBoolean(reader IndexReader) (Query, error) {
	// Similar to constant_score but uses boolean rewrite internally
	return NewConstantScoreQuery(aq), nil
}

// Clone creates a copy of this query.
func (aq *AutomatonQuery) Clone() Query {
	return NewAutomatonQueryFull(
		aq.term,
		aq.automaton.Clone(),
		aq.isBinary,
		aq.rewriteMethod,
	)
}

// Equals checks if this query equals another.
func (aq *AutomatonQuery) Equals(other Query) bool {
	if aq == other {
		return true
	}
	if other == nil {
		return false
	}

	// Check if other is an AutomatonQuery
	aq2, ok := other.(*AutomatonQuery)
	if !ok {
		return false
	}

	// Check type match (exact class match)
	// This ensures WildcardQuery != AutomatonQuery even if automaton matches
	if fmt.Sprintf("%T", aq) != fmt.Sprintf("%T", aq2) {
		return false
	}

	// Check term equality
	if !aq.term.Equals(aq2.term) {
		return false
	}

	// Check automaton equality
	return aq.automaton.Equals(aq2.automaton)
}

// HashCode returns a hash code for this query.
// Thread-safe: Uses sync.Once for caching.
func (aq *AutomatonQuery) HashCode() int {
	aq.hashCodeOnce.Do(func() {
		h := 31
		if aq.compiledAutomaton != nil && aq.compiledAutomaton.GetAutomaton() != nil {
			h = 31*h + aq.compiledAutomaton.GetAutomaton().HashCode()
		}
		if aq.term != nil {
			h = 31*h + aq.term.HashCode()
		}
		aq.hashCodeVal = h
	})
	return aq.hashCodeVal
}

// CreateWeight creates a Weight for this query.
func (aq *AutomatonQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	// Rewrite first
	rewritten, err := aq.Rewrite(searcher.GetIndexReader())
	if err != nil {
		return nil, err
	}

	if rewritten != aq {
		return rewritten.CreateWeight(searcher, needsScores, boost)
	}

	// Create a simple weight for this query
	return NewAutomatonWeight(aq, boost), nil
}

// String returns a string representation of this query.
func (aq *AutomatonQuery) String() string {
	return fmt.Sprintf("AutomatonQuery(field=%s)", aq.term.Field)
}

// ============================================================================
// Weight for AutomatonQuery
// ============================================================================

// AutomatonWeight is the weight for AutomatonQuery.
type AutomatonWeight struct {
	BaseWeight
	query  *AutomatonQuery
	boost  float32
}

// NewAutomatonWeight creates a new AutomatonWeight.
func NewAutomatonWeight(query *AutomatonQuery, boost float32) *AutomatonWeight {
	return &AutomatonWeight{
		query: query,
		boost: boost,
	}
}

// GetValue returns the weight value.
func (w *AutomatonWeight) GetValue() float32 {
	return w.boost
}

// Normalize normalizes the weight.
func (w *AutomatonWeight) Normalize(norm float32) {
	w.boost *= norm
}

// GetQuery returns the parent query.
func (w *AutomatonWeight) GetQuery() Query {
	return w.query
}

// Explain returns an explanation for the score.
func (w *AutomatonWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	return NewExplanation(true, w.boost, "AutomatonQuery, product of:"), nil
}

// Scorer creates a scorer for this weight.
func (w *AutomatonWeight) Scorer(context *index.LeafReaderContext) (Scorer, error) {
	// In a full implementation, this would:
	// 1. Get the terms enum for the field
	// 2. Filter terms using the automaton
	// 3. Create a scorer that matches documents containing those terms
	return nil, nil
}

// IsCacheable returns true if this weight can be cached.
func (w *AutomatonWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return true
}

// ============================================================================
// Scorer for AutomatonQuery
// ============================================================================

// AutomatonScorer is a scorer for AutomatonQuery.
type AutomatonScorer struct {
	BaseScorer
	weight    *AutomatonWeight
	doc       int
	score     float32
}

// NewAutomatonScorer creates a new AutomatonScorer.
func NewAutomatonScorer(weight *AutomatonWeight, score float32) *AutomatonScorer {
	return &AutomatonScorer{
		weight: weight,
		doc:    -1,
		score:  score,
	}
}

// NextDoc advances to the next document.
func (s *AutomatonScorer) NextDoc() (int, error) {
	// In a full implementation, this would iterate through matching documents
	return -1, nil
}

// DocID returns the current document ID.
func (s *AutomatonScorer) DocID() int {
	return s.doc
}

// Score returns the score of the current document.
func (s *AutomatonScorer) Score() (float32, error) {
	return s.score, nil
}

// Advance advances to a target document.
func (s *AutomatonScorer) Advance(target int) (int, error) {
	// In a full implementation, this would advance to the target document
	return -1, nil
}

// Cost returns the estimated cost of this scorer.
func (s *AutomatonScorer) Cost() int64 {
	// In a full implementation, this would estimate based on matching terms
	return 100 // Placeholder
}