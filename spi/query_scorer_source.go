// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// QueryScorerSource is org.apache.lucene.search.IndexSearcher as
// org.apache.lucene.index.FrozenBufferedUpdates#applyQueryDeletes uses it.
//
// The Java body constructs and drives a searcher per segment:
//
//	final IndexSearcher searcher = new IndexSearcher(readerContext.reader());
//	searcher.setQueryCache(null);
//	query = searcher.rewrite(query);
//	final Weight weight = searcher.createWeight(query, ScoreMode.COMPLETE_NO_SCORES, 1);
//	final Scorer scorer = weight.scorer(readerContext);
//	if (scorer != null) {
//	  final DocIdSetIterator it = scorer.iterator();
//	  ...
//	}
//
// PORT NOTE — why this contract exists at all. In Java the code above lives in
// package org.apache.lucene.index and simply imports IndexSearcher, ScoreMode,
// Weight and Scorer from org.apache.lucene.search. Go cannot: package search
// imports package index, so the edge only runs one way. Construction is the
// part a structural interface cannot supply — index can hold a
// *search.IndexSearcher through an interface, but it cannot call
// `new IndexSearcher(...)` — so the constructor is installed as a process-wide
// hook by the package that owns the type, exactly as
// [RegisterSegmentInfoReader] installs the .si reader that package codecs owns.
// Lucene has no such registry here (it is a Go necessity, not a Lucene feature),
// and unlike the registry that preceded it this hook carries NO algorithm: the
// per-segment loop, the docIDUpto limit, the sortMap branch and the calls to
// ReadersAndUpdates.Delete all stay in FrozenBufferedUpdates, where Lucene
// puts them.
//
// The contract is deliberately narrow, in the spirit of [IndexSearcher] above
// it: it carries exactly the calls applyQueryDeletes makes and nothing Lucene's
// IndexSearcher does not have.
type QueryScorerSource interface {
	// Rewrite mirrors `query = searcher.rewrite(query)`.
	Rewrite(query Query) (Query, error)

	// ScorerIterator mirrors the three remaining calls in one step:
	//
	//	final Weight weight = searcher.createWeight(query, ScoreMode.COMPLETE_NO_SCORES, 1);
	//	final Scorer scorer = weight.scorer(readerContext);
	//	final DocIdSetIterator it = scorer.iterator();
	//
	// They are fused because ScoreMode, Weight and Scorer are
	// org.apache.lucene.search types this package cannot name; the ScoreMode
	// is fixed at COMPLETE_NO_SCORES and the boost at 1, which is the only
	// combination applyQueryDeletes ever asks for. A nil iterator renders
	// Java's `scorer == null`, which the caller must treat as "no documents
	// match in this leaf".
	ScorerIterator(query Query, ctx *LeafReaderContext) (util.DocIdSetIterator, error)
}

// queryScorerSourceFactory is the hook package search installs at init. It
// renders `new IndexSearcher(readerContext.reader())` followed by
// `searcher.setQueryCache(null)`; the query cache is disabled by the factory
// itself because applyQueryDeletes always disables it.
var (
	queryScorerSourceMu      sync.RWMutex
	queryScorerSourceFactory func(reader LeafReader) QueryScorerSource
)

// RegisterQueryScorerSourceFactory installs the process-wide constructor of
// [QueryScorerSource]. Passing nil clears it. Registering is idempotent and
// safe to call from an init function.
func RegisterQueryScorerSourceFactory(fn func(reader LeafReader) QueryScorerSource) {
	queryScorerSourceMu.Lock()
	queryScorerSourceFactory = fn
	queryScorerSourceMu.Unlock()
}

// NewQueryScorerSource builds a [QueryScorerSource] over one leaf reader. It
// fails when no factory has been registered, which means the program never
// linked package search; a query delete cannot be evaluated in that case and
// silently under-deleting would be worse than reporting it.
func NewQueryScorerSource(reader LeafReader) (QueryScorerSource, error) {
	queryScorerSourceMu.RLock()
	fn := queryScorerSourceFactory
	queryScorerSourceMu.RUnlock()
	if fn == nil {
		return nil, fmt.Errorf("spi: no QueryScorerSource factory registered; " +
			"import package github.com/FlavioCFOliveira/Gocene/search to evaluate query deletes")
	}
	return fn(reader), nil
}
