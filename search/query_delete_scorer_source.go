// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// This file carries nothing of its own: it is the constructor
// org.apache.lucene.index.FrozenBufferedUpdates#applyQueryDeletes calls
// directly in Java,
//
//	final IndexSearcher searcher = new IndexSearcher(readerContext.reader());
//	searcher.setQueryCache(null);
//
// made reachable from package index, which cannot import this package because
// this package imports it. The algorithm applyQueryDeletes runs — the loop over
// segments, the docIDUpto limit, the sortMap branch, the calls to
// ReadersAndUpdates.delete — stays in FrozenBufferedUpdates, exactly where
// Lucene puts it. See [spi.QueryScorerSource].
//
// It replaces search/query_delete_executor.go, which was the consumer half of a
// registry whose producer no longer existed (so it was unreachable) and whose
// design was not Lucene's: it opened a DirectoryReader over every committed
// segment, searched globally, and mapped global docIDs back per segment. Lucene
// evaluates each query per segment, inside applyQueryDeletes.

func init() {
	spi.RegisterQueryScorerSourceFactory(newLeafQueryScorerSource)
}

// leafQueryScorerSource is one IndexSearcher over one leaf, with its query
// cache disabled, as applyQueryDeletes wants it.
type leafQueryScorerSource struct {
	searcher *IndexSearcher
}

// newLeafQueryScorerSource renders `new IndexSearcher(readerContext.reader())`
// followed by `searcher.setQueryCache(null)`.
func newLeafQueryScorerSource(reader spi.LeafReader) spi.QueryScorerSource {
	searcher := NewIndexSearcher(reader)
	searcher.SetQueryCache(nil)
	return &leafQueryScorerSource{searcher: searcher}
}

// Rewrite renders `query = searcher.rewrite(query)`.
func (l *leafQueryScorerSource) Rewrite(query spi.Query) (spi.Query, error) {
	q, err := asSearchQuery(query)
	if err != nil {
		return nil, err
	}
	rewritten, err := l.searcher.Rewrite(q)
	if err != nil {
		return nil, err
	}
	if rewritten == nil {
		return nil, fmt.Errorf("IndexSearcher.Rewrite returned no query for %v", queryToString(q, ""))
	}
	return rewritten, nil
}

// ScorerIterator renders the three calls that follow the rewrite:
//
//	final Weight weight = searcher.createWeight(query, ScoreMode.COMPLETE_NO_SCORES, 1);
//	final Scorer scorer = weight.scorer(readerContext);
//	if (scorer != null) { final DocIdSetIterator it = scorer.iterator(); ... }
//
// A nil return renders Java's `scorer == null`.
func (l *leafQueryScorerSource) ScorerIterator(query spi.Query, ctx *spi.LeafReaderContext) (util.DocIdSetIterator, error) {
	q, err := asSearchQuery(query)
	if err != nil {
		return nil, err
	}
	weight, err := l.searcher.CreateWeight(q, COMPLETE_NO_SCORES, 1)
	if err != nil {
		return nil, err
	}
	if weight == nil {
		return nil, fmt.Errorf("IndexSearcher.CreateWeight returned no weight for %v", queryToString(q, ""))
	}
	scorer, err := weight.Scorer(ctx)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	it := scorer.Iterator()
	if it == nil {
		return nil, nil
	}
	return it, nil
}

// asSearchQuery narrows the shared [spi.Query] handle back to the full
// org.apache.lucene.search.Query contract. Every Query this module builds is a
// search.Query; a value that is not one cannot be rewritten or scored, and that
// is reported rather than silently dropped — a dropped query delete would
// under-delete.
func asSearchQuery(query spi.Query) (Query, error) {
	if query == nil {
		return nil, fmt.Errorf("query delete: nil query")
	}
	q, ok := query.(Query)
	if !ok {
		return nil, fmt.Errorf("query delete: unsupported query type %T (expected a search.Query)", query)
	}
	return q, nil
}
