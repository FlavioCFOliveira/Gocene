// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// CollectorManager is a manager of collectors. This class is useful to parallelize execution of search requests and has
// two main methods:
//
// - NewCollector: which must return a NEW collector which will be used to collect a certain set of leaves.
// - Reduce: which will be used to reduce the results of individual collections into a meaningful result. This method is only called after all leaves have been
//   fully collected.
//
// Note: Multiple LeafCollectors may be requested for the same LeafReaderContext via
// Collector.GetLeafCollector(LeafReaderContext) across the different Collectors returned by NewCollector(). Any computation or logic that needs to
// happen once per segment requires specific handling in the collector manager implementation, because the collection of an entire segment may be split across threads.
type CollectorManager[C Collector, T any] interface {
	// NewCollector returns a new Collector. This must return a different instance on each call.
	NewCollector() (C, error)

	// Reduce reduces the results of individual collectors into a meaningful result. For instance a
	// TopDocsCollector would compute the top docs of each collector and then merge them using
	// TopDocs.Merge. This method must be called after collection is finished on all provided collectors.
	Reduce(collectors []C) (T, error)
}
