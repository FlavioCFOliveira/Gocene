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
	if aq.compiledAutomaton.TypeName() == "NONE" {
		return NewMatchNoDocsQuery(), nil
	}

	// Check if automaton matches all strings
	if aq.compiledAutomaton.TypeName() == "ALL" {
		// Rewrite to field exists query
		return NewFieldExistsQuery(aq.term.Field), nil
	}

	// Check if automaton matches a single term
	if aq.compiledAutomaton.TypeName() == "SINGLE" {
		singleTerm := aq.compiledAutomaton.GetTerm()
		return NewTermQuery(index.NewTerm(aq.term.Field, singleTerm)), nil
	}

	// Enumerate matching terms via the index and build the requested
	// rewrite shape.  When no reader is available we degrade to wrapping
	// the original query in a ConstantScoreQuery (the safe choice that
	// preserves the contract without inventing a synthetic term set).
	matched, err := aq.enumerateMatchedTerms(reader)
	if err != nil {
		return nil, err
	}

	switch aq.rewriteMethod {
	case ScoringBooleanRewrite:
		return aq.rewriteScoringBoolean(matched)
	case ConstantScoreRewrite:
		return aq.rewriteConstantScore(matched)
	case ConstantScoreBlendedRewrite:
		return aq.rewriteConstantScoreBlended(matched)
	case ConstantScoreBooleanRewrite:
		return aq.rewriteConstantScoreBoolean(matched)
	default:
		return aq.rewriteConstantScoreBlended(matched)
	}
}

// enumerateMatchedTerms walks every term in aq.term.Field and returns the
// ones accepted by the compiled automaton, capped by maxClauseCount.
// Returns nil when the reader cannot expose a Terms accessor for the
// field, which the rewrite paths interpret as "no expansion possible".
func (aq *AutomatonQuery) enumerateMatchedTerms(reader IndexReader) ([][]byte, error) {
	if reader == nil {
		return nil, nil
	}
	// LeafReader / SegmentReader expose Terms(field string) (index.Terms,
	// error); we use a narrow interface so the rewrite does not depend
	// on which concrete reader implementation is wired in.
	type schemaTermsProvider interface {
		Terms(field string) (index.Terms, error)
	}
	stp, ok := interface{}(reader).(schemaTermsProvider)
	if !ok {
		return nil, nil
	}
	terms, err := stp.Terms(aq.term.Field)
	if err != nil || terms == nil {
		return nil, err
	}
	it, err := terms.GetIterator()
	if err != nil || it == nil {
		return nil, err
	}
	out := make([][]byte, 0, 16)
	limit := GetMaxClauseCount()
	for {
		t, err := it.Next()
		if err != nil {
			return nil, err
		}
		if t == nil {
			break
		}
		bv := t.BytesValue()
		if bv == nil {
			continue
		}
		bytes := bv.ValidBytes()
		if !aq.compiledAutomaton.Run(bytes) {
			continue
		}
		// Copy the bytes; the iterator may reuse the buffer.
		dup := make([]byte, len(bytes))
		copy(dup, bytes)
		out = append(out, dup)
		if len(out) >= limit {
			return nil, ErrTooManyClauses
		}
	}
	return out, nil
}

// matchedTermsToBoolean materialises a BooleanQuery whose SHOULD clauses
// are TermQueries over the matched-term set.  Returns nil when no terms
// matched, signalling MatchNoDocs to the caller.
func (aq *AutomatonQuery) matchedTermsToBoolean(terms [][]byte) Query {
	if len(terms) == 0 {
		return nil
	}
	bq := NewBooleanQuery()
	for _, b := range terms {
		bq.Add(NewTermQuery(index.NewTermFromBytes(aq.term.Field, b)), SHOULD)
	}
	return bq
}

// rewriteScoringBoolean expands matched terms into a SHOULD BooleanQuery.
// Mirrors MultiTermQuery.SCORING_BOOLEAN_REWRITE (Lucene 10.4.0).
func (aq *AutomatonQuery) rewriteScoringBoolean(matched [][]byte) (Query, error) {
	if matched == nil {
		// Could not enumerate (no reader); fall back to wrapping the
		// query so the caller still gets a usable Query.
		return NewConstantScoreQuery(aq), nil
	}
	bq := aq.matchedTermsToBoolean(matched)
	if bq == nil {
		return NewMatchNoDocsQuery(), nil
	}
	return bq, nil
}

// rewriteConstantScore wraps the query in a ConstantScoreQuery.
func (aq *AutomatonQuery) rewriteConstantScore(matched [][]byte) (Query, error) {
	if matched != nil {
		bq := aq.matchedTermsToBoolean(matched)
		if bq == nil {
			return NewMatchNoDocsQuery(), nil
		}
		return NewConstantScoreQuery(bq), nil
	}
	return NewConstantScoreQuery(aq), nil
}

// rewriteConstantScoreBlended is the recommended rewrite.  It produces
// the same shape as CONSTANT_SCORE for now; the "blended" optimisation
// (per-leaf doc-id sets with blended boosts) requires postings-level
// access that Gocene defers in line with the ScoringRewrite degradation.
func (aq *AutomatonQuery) rewriteConstantScoreBlended(matched [][]byte) (Query, error) {
	return aq.rewriteConstantScore(matched)
}

// rewriteConstantScoreBoolean expands matched terms into a SHOULD
// BooleanQuery and wraps it in a ConstantScoreQuery.
func (aq *AutomatonQuery) rewriteConstantScoreBoolean(matched [][]byte) (Query, error) {
	if matched == nil {
		return NewConstantScoreQuery(aq), nil
	}
	bq := aq.matchedTermsToBoolean(matched)
	if bq == nil {
		return NewMatchNoDocsQuery(), nil
	}
	return NewConstantScoreQuery(bq), nil
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
//
// The query is rewritten against the searcher's reader; in the normal
// case Rewrite produces a BooleanQuery / ConstantScoreQuery whose own
// CreateWeight handles the heavy lifting.  The fall-through "no rewrite"
// branch produces an AutomatonWeight that yields no matches and serves
// as a placeholder for readers that cannot expose a Terms accessor.
func (aq *AutomatonQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	if searcher == nil {
		return NewAutomatonWeight(aq, boost), nil
	}
	rewritten, err := aq.Rewrite(searcher.GetIndexReader())
	if err != nil {
		return nil, err
	}
	if rewritten != nil && rewritten != aq {
		return rewritten.CreateWeight(searcher, needsScores, boost)
	}
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
	query *AutomatonQuery
	boost float32
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

// Scorer returns nil for the placeholder AutomatonWeight.
//
// AutomatonQuery.CreateWeight always rewrites through Terms enumeration
// when a reader is available, so the placeholder path is only entered
// when no reader (and therefore no postings) is reachable. Returning
// nil mirrors Lucene's "no source" fast path for that situation.
func (w *AutomatonWeight) Scorer(context *index.LeafReaderContext) (Scorer, error) {
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
	weight *AutomatonWeight
	doc    int
	score  float32
}

// NewAutomatonScorer creates a new AutomatonScorer.
func NewAutomatonScorer(weight *AutomatonWeight, score float32) *AutomatonScorer {
	return &AutomatonScorer{
		weight: weight,
		doc:    -1,
		score:  score,
	}
}

// NextDoc returns NO_MORE_DOCS — the placeholder scorer never positions
// to a matching document.  Real iteration happens via the rewritten
// BooleanQuery / ConstantScoreQuery produced by AutomatonQuery.Rewrite.
func (s *AutomatonScorer) NextDoc() (int, error) {
	s.doc = NO_MORE_DOCS
	return NO_MORE_DOCS, nil
}

// DocID returns the current document ID.
func (s *AutomatonScorer) DocID() int {
	return s.doc
}

// Score returns the score of the current document.
func (s *AutomatonScorer) Score() float32 {
	return s.score
}

// Advance returns NO_MORE_DOCS for the placeholder scorer.
func (s *AutomatonScorer) Advance(target int) (int, error) {
	s.doc = NO_MORE_DOCS
	return NO_MORE_DOCS, nil
}

// Cost returns 0 for the placeholder scorer, indicating that it never
// produces any matches.
func (s *AutomatonScorer) Cost() int64 {
	return 0
}

// GetMaxScore returns the constant boost score; the placeholder scorer
// never positions to a document, but the contract requires returning the
// upper bound the underlying query would otherwise score.
func (s *AutomatonScorer) GetMaxScore(upTo int) float32 { return s.score }

// DocIDRunEnd returns the doc+1 sentinel for the placeholder scorer.
func (s *AutomatonScorer) DocIDRunEnd() int { return s.doc + 1 }
