// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import "github.com/FlavioCFOliveira/Gocene/search"

// MatcherFactory creates CandidateMatcher instances for a given IndexSearcher.
//
// Port of org.apache.lucene.monitor.MatcherFactory.
type MatcherFactory[T any] interface {
	// CreateMatcher creates a new CandidateMatcher for the given searcher.
	CreateMatcher(searcher *search.IndexSearcher) CandidateMatcher[T]
}

// MatcherFactoryFunc is a function-based MatcherFactory.
type MatcherFactoryFunc[T any] func(searcher *search.IndexSearcher) CandidateMatcher[T]

// CreateMatcher implements MatcherFactory.
func (f MatcherFactoryFunc[T]) CreateMatcher(searcher *search.IndexSearcher) CandidateMatcher[T] {
	return f(searcher)
}
