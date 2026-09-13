// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

// IndexSearcher is the contract that org.apache.lucene.search.IndexSearcher
// presents to code that lives in the index package.
//
// PORT NOTE: Java's org.apache.lucene.index.TermStates imports
// org.apache.lucene.search.IndexSearcher directly — Java tolerates the cycle
// between the two packages. Go does not, so the members that index-side code
// calls on an IndexSearcher are declared here, in the package both sides
// already depend on, and *search.IndexSearcher satisfies them structurally.
// The contract is deliberately narrow: it carries exactly the members the
// index package needs, and nothing that Lucene's IndexSearcher does not have.
type IndexSearcher interface {
	// GetTopReaderContext returns this searcher's top-level IndexReaderContext,
	// mirroring IndexSearcher.getTopReaderContext().
	GetTopReaderContext() IndexReaderContext
}
