// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import (
	"time"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// CandidateMatcher matches candidate queries selected by a Presearcher against
// documents held in an IndexSearcher.
//
// Port of org.apache.lucene.monitor.CandidateMatcher.
//
// Deviation: Go does not support abstract classes.  CandidateMatcher is an
// interface with an embedded base type BaseCandidateMatcher that callers compose
// to get the shared state and helpers (errors, matches, finish).
type CandidateMatcher[T any] interface {
	// MatchQuery runs the given query against the documents in the searcher,
	// recording any results.
	MatchQuery(queryID string, matchQuery search.Query, metadata map[string]string) error

	// Resolve combines two matches for the same query (e.g. two branches of a
	// disjunction).
	Resolve(match1, match2 T) T

	// ReportError records an error for the given query.
	ReportError(queryID string, err error)

	// Finish finalises the run and returns the aggregated MultiMatchingQueries.
	Finish(buildTime int64, queryCount int) *MultiMatchingQueries[T]
}

// BaseCandidateMatcher provides the shared state for CandidateMatcher implementations.
type BaseCandidateMatcher[T any] struct {
	Searcher   *search.IndexSearcher
	errors     map[string]error
	matches    []map[string]T // one per doc
	searchTime time.Time
}

// NewBaseCandidateMatcher initialises a BaseCandidateMatcher for the given searcher.
func NewBaseCandidateMatcher[T any](searcher *search.IndexSearcher) BaseCandidateMatcher[T] {
	docCount := 0
	if searcher != nil && searcher.GetIndexReader() != nil {
		docCount = searcher.GetIndexReader().MaxDoc()
	}
	matches := make([]map[string]T, docCount)
	for i := range matches {
		matches[i] = make(map[string]T)
	}
	return BaseCandidateMatcher[T]{
		Searcher:   searcher,
		errors:     make(map[string]error),
		matches:    matches,
		searchTime: time.Now(),
	}
}

// AddMatch records a match for the given document, merging with any existing match
// using the provided resolver.
func (b *BaseCandidateMatcher[T]) AddMatch(match T, doc int, resolve func(T, T) T) {
	if doc < 0 || doc >= len(b.matches) {
		return
	}
	id := any(match)
	var qid string
	if qm, ok := id.(interface{ GetQueryID() string }); ok {
		qid = qm.GetQueryID()
	}
	if existing, ok := b.matches[doc][qid]; ok {
		b.matches[doc][qid] = resolve(match, existing)
	} else {
		b.matches[doc][qid] = match
	}
}

// ReportError records an error for the given query ID.
func (b *BaseCandidateMatcher[T]) ReportError(queryID string, err error) {
	b.errors[queryID] = err
}

// Finish finalises the run and returns a MultiMatchingQueries.
func (b *BaseCandidateMatcher[T]) Finish(buildTime int64, queryCount int) *MultiMatchingQueries[T] {
	elapsed := time.Since(b.searchTime).Milliseconds()
	return newMultiMatchingQueries(
		b.matches, b.errors, buildTime, elapsed, queryCount, len(b.matches),
	)
}

// CopyMatches replaces this matcher's match state with another's (used by wrappers).
func (b *BaseCandidateMatcher[T]) CopyMatches(other *BaseCandidateMatcher[T]) {
	b.matches = other.matches
}
