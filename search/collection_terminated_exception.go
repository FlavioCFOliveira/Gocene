// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "errors"

// The Sprint 51 search-module port stubs have been resolved:
//   - CollectionTerminatedException → promoted to a proper error type (below).
//   - AbstractDocIdSetIterator → removed (unused; Go uses BaseDocIdSetIterator).
//   - AbstractKnnCollector → removed (unused; real impl. in util/hnsw/knn_collector.go).
//   - BoostAttribute → removed (unused; real impl. in analysis/boost_attribute.go).
//   - BoostAttributeImpl → removed (unused; real impl. in analysis/boost_attribute.go).
//   - ByteVectorSimilarityQuery → removed (unused; KNN queries use the codecs path).
//   - CachingCollector → removed (unused; caching lives in lru_query_cache.go).
//   - CheckedIntConsumer → removed (unused; Go uses closure-based iteration).

// CollectionTerminatedException mirrors
// org.apache.lucene.search.CollectionTerminatedException. In Java it is
// a checked exception used purely as a control-flow signal: a collector
// throws it from getLeafCollector/collect to tell the search loop that it
// no longer needs the current segment (or any further documents).
//
// Go has no exceptions, so following the convention already established in
// this package (see errCollectionTerminated in spatial_query.go and the
// errors.Is/errors.As-driven control flow there) the type is modelled as a
// plain error value that callers return up the stack and detect with
// IsCollectionTerminated. The search loop swallows it exactly where Lucene's
// IndexSearcher.search catches the exception, so it never escapes to the
// caller as a real failure.
type CollectionTerminatedException struct{}

// NewCollectionTerminatedException builds a
// CollectionTerminatedException.
func NewCollectionTerminatedException() *CollectionTerminatedException {
	return &CollectionTerminatedException{}
}

// Error implements the error interface so CollectionTerminatedException can be
// returned through the (LeafCollector, error) and error signatures used by the
// collector hierarchy and detected with errors.As / IsCollectionTerminated.
func (e *CollectionTerminatedException) Error() string {
	return "collection terminated"
}

// IsCollectionTerminated reports whether err is, or wraps, a
// CollectionTerminatedException. It is the Go equivalent of catching
// CollectionTerminatedException in Java and lets callers distinguish the
// terminate-early control-flow signal from genuine I/O errors.
func IsCollectionTerminated(err error) bool {
	if err == nil {
		return false
	}
	var cte *CollectionTerminatedException
	return errors.As(err, &cte)
}
