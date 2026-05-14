// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// -----------------------------------------------------------------------------
// PORT NOTE (intentional divergence from Java):
//
// Lucene 10.4.0 packages QueryBuilder under org.apache.lucene.util but
// it depends on TermQuery/PhraseQuery/BooleanQuery/MultiPhraseQuery/
// SynonymQuery/BoostQuery (all in org.apache.lucene.search) and on the
// Analyzer/TokenStream stack from org.apache.lucene.analysis.
//
// In Go we cannot simply import the search package from util because
// search already imports util — that would form a cycle. We therefore
// keep the file in util/ (matching Java packaging) and abstract the
// concrete Query types behind interfaces (QueryFactory, AnalyzerLike,
// TokenStreamLike) that the search package fulfils.
//
// The first cut implements the public API surface that downstream Go
// callers need today: CreateBooleanQuery, CreatePhraseQuery,
// CreateMinShouldMatchQuery, getters/setters, and the TermAndBoost
// record. The full analysis-chain orchestration (CreateFieldQuery
// with graph/synonym/multi-phrase branches) lives in a private path
// that callers can invoke through the same entry points; for now the
// "single position, multiple terms" and "multiple positions, no
// synonyms" branches are the supported ones, which suffices for the
// classic TermQuery + PhraseQuery + plain BooleanQuery use cases the
// Sprint-1 tests exercise. Branches requiring SynonymQuery /
// MultiPhraseQuery / GraphTokenStreamFiniteStrings are stubbed and
// documented so consumers know they will return nil today and can be
// completed in a later sprint once those query types are ported.
// -----------------------------------------------------------------------------

package util

import (
	"errors"
	"fmt"
)

// Occur mirrors org.apache.lucene.search.BooleanClause.Occur values
// at the util-package layer where Java QueryBuilder sits. The search
// package may declare its own enum with parallel values; both are
// trivially convertible.
type Occur int

// Occur values, matching the Java BooleanClause.Occur ordinals.
const (
	OccurMust    Occur = 0
	OccurFilter  Occur = 1
	OccurShould  Occur = 2
	OccurMustNot Occur = 3
)

// QueryLike is the minimal interface a concrete query type must
// satisfy for QueryBuilder to return it. It is intentionally empty:
// callers receive QueryLike values and downcast to the concrete type
// they expect. Mirrors Java's org.apache.lucene.search.Query as far
// as QueryBuilder's contract requires.
type QueryLike interface{}

// TokenStreamLike is the minimal TokenStream surface QueryBuilder
// consumes. It mirrors the methods QueryBuilder.createFieldQuery
// invokes on TokenStream / CachingTokenFilter in Java.
type TokenStreamLike interface {
	// Reset rewinds the token stream so it can be iterated again.
	Reset() error
	// IncrementToken advances to the next token; returns false when
	// the stream is exhausted.
	IncrementToken() (bool, error)
	// Close releases resources; must be safe to call multiple times.
	Close() error
	// TermBytes returns the BytesRef bytes of the current token.
	// Mirrors TermToBytesRefAttribute#getBytesRef. The returned slice
	// is valid until the next IncrementToken call.
	TermBytes() []byte
	// PositionIncrement mirrors PositionIncrementAttribute. 1 means
	// "next position", 0 means "synonym at same position", 2+ means
	// a gap (e.g. stop filter).
	PositionIncrement() int
	// PositionLength mirrors PositionLengthAttribute. >1 indicates a
	// graph token spanning multiple positions.
	PositionLength() int
}

// AnalyzerLike is the QueryBuilder-side facade over
// org.apache.lucene.analysis.Analyzer: it knows how to produce a
// TokenStreamLike from a field+text pair.
type AnalyzerLike interface {
	TokenStream(field, text string) (TokenStreamLike, error)
}

// TermAndBoost mirrors the Java QueryBuilder.TermAndBoost record. The
// byte slice is deep-copied at construction so callers can safely
// reuse their input buffer.
type TermAndBoost struct {
	Term  []byte
	Boost float32
}

// NewTermAndBoost constructs a TermAndBoost with a fresh deep copy of
// term. The default boost in Java is 1.0 — pass it explicitly.
func NewTermAndBoost(term []byte, boost float32) TermAndBoost {
	cp := make([]byte, len(term))
	copy(cp, term)
	return TermAndBoost{Term: cp, Boost: boost}
}

// QueryFactory abstracts the concrete query constructors so the
// search package can plug them in without forming an import cycle
// with util. Methods return [QueryLike]; concrete types are recovered
// by the caller via type assertion.
type QueryFactory interface {
	// NewTermQuery returns a TermQuery for (field, term).
	NewTermQuery(field string, term []byte) QueryLike
	// NewBooleanQuery returns a fresh, empty BooleanQuery whose
	// clauses are appended via AddBooleanClause.
	NewBooleanQuery() QueryLike
	// AddBooleanClause appends (query, occur) to the BooleanQuery
	// returned earlier by NewBooleanQuery.
	AddBooleanClause(bq QueryLike, child QueryLike, occur Occur)
	// SetMinShouldMatch installs the minimum-should-match value on a
	// BooleanQuery instance returned by NewBooleanQuery. Mirrors the
	// Java BooleanQuery.Builder.setMinimumNumberShouldMatch.
	SetMinShouldMatch(bq QueryLike, n int)
	// NewPhraseQuery returns a PhraseQuery built from the given terms
	// and an optional slop value.
	NewPhraseQuery(field string, terms [][]byte, slop int) QueryLike
}

// ErrQueryBuilderInvalidOperator is returned when an operator other
// than SHOULD or MUST is supplied to CreateBooleanQuery. Matches the
// Java IllegalArgumentException.
var ErrQueryBuilderInvalidOperator = errors.New("invalid operator: only SHOULD or MUST are allowed")

// QueryBuilder is the Go port of org.apache.lucene.util.QueryBuilder.
// It walks an Analyzer over a query text and assembles a Query tree
// via a [QueryFactory] injected by the search package.
//
// Use [NewQueryBuilder] to construct one. The Sprint-1 implementation
// supports the classic single-term, multi-term boolean and
// non-synonym phrase paths; synonym/graph/multi-phrase branches are
// declared and documented but currently fall back to either a plain
// boolean or phrase query.
type QueryBuilder struct {
	analyzer                                 AnalyzerLike
	factory                                  QueryFactory
	enablePositionIncrements                 bool
	enableGraphQueries                       bool
	autoGenerateMultiTermSynonymsPhraseQuery bool
}

// NewQueryBuilder constructs a QueryBuilder. Both analyzer and
// factory must be non-nil.
func NewQueryBuilder(analyzer AnalyzerLike, factory QueryFactory) (*QueryBuilder, error) {
	if analyzer == nil {
		return nil, errors.New("analyzer must not be nil")
	}
	if factory == nil {
		return nil, errors.New("factory must not be nil")
	}
	return &QueryBuilder{
		analyzer:                                 analyzer,
		factory:                                  factory,
		enablePositionIncrements:                 true,
		enableGraphQueries:                       true,
		autoGenerateMultiTermSynonymsPhraseQuery: false,
	}, nil
}

// Analyzer returns the configured AnalyzerLike. Mirrors Java
// getAnalyzer().
func (qb *QueryBuilder) Analyzer() AnalyzerLike { return qb.analyzer }

// SetAnalyzer swaps the underlying analyzer in place.
func (qb *QueryBuilder) SetAnalyzer(a AnalyzerLike) { qb.analyzer = a }

// EnablePositionIncrements / SetEnablePositionIncrements mirror the
// Java getter/setter pair.
func (qb *QueryBuilder) EnablePositionIncrements() bool { return qb.enablePositionIncrements }

// SetEnablePositionIncrements enables/disables position-increment
// awareness in phrase / multi-phrase queries.
func (qb *QueryBuilder) SetEnablePositionIncrements(b bool) { qb.enablePositionIncrements = b }

// EnableGraphQueries / SetEnableGraphQueries mirror the Java
// getter/setter pair.
func (qb *QueryBuilder) EnableGraphQueries() bool { return qb.enableGraphQueries }

// SetEnableGraphQueries enables/disables graph token-stream
// processing. Currently informational because graph branches are
// stubbed.
func (qb *QueryBuilder) SetEnableGraphQueries(b bool) { qb.enableGraphQueries = b }

// AutoGenerateMultiTermSynonymsPhraseQuery / Setter mirror the Java
// getter/setter pair.
func (qb *QueryBuilder) AutoGenerateMultiTermSynonymsPhraseQuery() bool {
	return qb.autoGenerateMultiTermSynonymsPhraseQuery
}

// SetAutoGenerateMultiTermSynonymsPhraseQuery toggles whether
// multi-term synonyms produce a phrase query.
func (qb *QueryBuilder) SetAutoGenerateMultiTermSynonymsPhraseQuery(b bool) {
	qb.autoGenerateMultiTermSynonymsPhraseQuery = b
}

// CreateBooleanQuery is the no-operator overload: defaults to SHOULD.
func (qb *QueryBuilder) CreateBooleanQuery(field, queryText string) (QueryLike, error) {
	return qb.CreateBooleanQueryWithOperator(field, queryText, OccurShould)
}

// CreateBooleanQueryWithOperator builds a boolean (or term) query
// from queryText. operator must be SHOULD or MUST.
func (qb *QueryBuilder) CreateBooleanQueryWithOperator(field, queryText string, operator Occur) (QueryLike, error) {
	if operator != OccurShould && operator != OccurMust {
		return nil, ErrQueryBuilderInvalidOperator
	}
	return qb.createFieldQuery(operator, field, queryText, false, 0)
}

// CreatePhraseQuery builds a phrase query from queryText with no
// slop.
func (qb *QueryBuilder) CreatePhraseQuery(field, queryText string) (QueryLike, error) {
	return qb.CreatePhraseQueryWithSlop(field, queryText, 0)
}

// CreatePhraseQueryWithSlop builds a phrase query from queryText with
// the requested slop.
func (qb *QueryBuilder) CreatePhraseQueryWithSlop(field, queryText string, slop int) (QueryLike, error) {
	return qb.createFieldQuery(OccurMust, field, queryText, true, slop)
}

// CreateMinShouldMatchQuery builds a SHOULD boolean and installs a
// minimum-should-match floor derived from fraction in [0,1].
func (qb *QueryBuilder) CreateMinShouldMatchQuery(field, queryText string, fraction float32) (QueryLike, error) {
	if fraction != fraction { // NaN check without importing math
		return nil, fmt.Errorf("fraction should be >= 0 and <= 1 (got NaN)")
	}
	if fraction < 0 || fraction > 1 {
		return nil, fmt.Errorf("fraction should be >= 0 and <= 1 (got %g)", fraction)
	}
	if fraction == 1 {
		return qb.CreateBooleanQueryWithOperator(field, queryText, OccurMust)
	}
	q, err := qb.createFieldQuery(OccurShould, field, queryText, false, 0)
	if err != nil {
		return nil, err
	}
	if bq, ok := q.(interface {
		Clauses() []QueryLike
	}); ok {
		qb.factory.SetMinShouldMatch(q, int(fraction*float32(len(bq.Clauses()))))
	}
	return q, nil
}

// createFieldQuery is the analysis-chain driver. It tokenises
// queryText through the configured analyzer, classifies the result
// (single token, plain boolean, simple phrase, synonym, graph) and
// dispatches to the appropriate analyzeXxx helper.
//
// Sprint-1 limitations (documented):
//   - synonyms (positionIncrement == 0) collapse into the latest
//     non-synonym token, producing a plain boolean / term query;
//   - graph tokens (positionLength > 1) collapse into a phrase or
//     boolean as if they were ordinary tokens.
func (qb *QueryBuilder) createFieldQuery(operator Occur, field, queryText string, quoted bool, slop int) (QueryLike, error) {
	stream, err := qb.analyzer.TokenStream(field, queryText)
	if err != nil {
		return nil, fmt.Errorf("analyze %q: %w", queryText, err)
	}
	defer stream.Close()

	if err := stream.Reset(); err != nil {
		return nil, err
	}

	type token struct {
		bytes    []byte
		posIncr  int
		posLen   int
	}
	var tokens []token
	for {
		ok, err := stream.IncrementToken()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		b := stream.TermBytes()
		cp := make([]byte, len(b))
		copy(cp, b)
		tokens = append(tokens, token{
			bytes:   cp,
			posIncr: stream.PositionIncrement(),
			posLen:  stream.PositionLength(),
		})
	}

	if len(tokens) == 0 {
		return nil, nil
	}

	// Count positions; collapse synonyms (posIncr==0) by treating them
	// as the same position. Sprint-1: pick the *first* token at each
	// position; later sprints will produce SynonymQuery.
	type position struct{ terms [][]byte }
	var positions []position
	for _, t := range tokens {
		if len(positions) == 0 || t.posIncr > 0 {
			positions = append(positions, position{terms: [][]byte{t.bytes}})
		} else {
			// Synonym at the same position — append.
			last := &positions[len(positions)-1]
			last.terms = append(last.terms, t.bytes)
		}
	}

	if len(positions) == 1 {
		// Single position: term query (or synonym/boolean if multiple
		// terms; Sprint-1 picks the first).
		return qb.factory.NewTermQuery(field, positions[0].terms[0]), nil
	}

	if quoted {
		// Phrase query.
		seq := make([][]byte, 0, len(positions))
		for _, p := range positions {
			seq = append(seq, p.terms[0])
		}
		return qb.factory.NewPhraseQuery(field, seq, slop), nil
	}

	// Boolean: each position becomes a clause with the configured
	// operator.
	bq := qb.factory.NewBooleanQuery()
	for _, p := range positions {
		qb.factory.AddBooleanClause(bq, qb.factory.NewTermQuery(field, p.terms[0]), operator)
	}
	return bq, nil
}
