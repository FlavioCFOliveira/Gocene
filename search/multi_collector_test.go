// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Test file: search/multi_collector_test.go
// Source: lucene/core/src/test/org/apache/lucene/search/TestMultiCollector.java
// Purpose: Tests MultiCollector which allows running a search with several Collectors
//
// NOTE: These tests are skipped because the custom collector types in this file
// implement GetLeafCollector(index.IndexReaderInterface) but search.Collector requires
// GetLeafCollector(search.IndexReader). The interfaces need alignment before these
// tests can compile and run.

package search_test

import "testing"

func TestMultiCollector_NullCollectors(t *testing.T) {
	t.Fatal("search.Collector interface mismatch — GetLeafCollector parameter type not aligned")
}

func TestMultiCollector_SingleCollector(t *testing.T) {
	t.Fatal("search.Collector interface mismatch — GetLeafCollector parameter type not aligned")
}

func TestMultiCollector_Delegation(t *testing.T) {
	t.Fatal("search.Collector interface mismatch — GetLeafCollector parameter type not aligned")
}

func TestMultiCollector_MergeScoreModes(t *testing.T) {
	t.Fatal("search.Collector interface mismatch — GetLeafCollector parameter type not aligned")
}

func TestMultiCollector_CollectionTermination(t *testing.T) {
	t.Fatal("search.Collector interface mismatch — GetLeafCollector parameter type not aligned")
}

func TestMultiCollector_SetScorerOnTermination(t *testing.T) {
	t.Fatal("search.Collector interface mismatch — GetLeafCollector parameter type not aligned")
}

func TestMultiCollector_SetScorerAfterTermination(t *testing.T) {
	t.Fatal("search.Collector interface mismatch — GetLeafCollector parameter type not aligned")
}

func TestMultiCollector_MinCompetitiveScore(t *testing.T) {
	t.Fatal("search.Collector interface mismatch — GetLeafCollector parameter type not aligned")
}

func TestMultiCollector_DisablesSetMinScore(t *testing.T) {
	t.Fatal("search.Collector interface mismatch — GetLeafCollector parameter type not aligned")
}

func TestMultiCollector_CollectionTerminatedExceptionHandling(t *testing.T) {
	t.Fatal("search.Collector interface mismatch — GetLeafCollector parameter type not aligned")
}

func TestMultiCollector_CacheScores(t *testing.T) {
	t.Fatal("search.Collector interface mismatch — GetLeafCollector parameter type not aligned")
}

func TestMultiCollector_ScorerWrappingForTopScores(t *testing.T) {
	t.Fatal("search.Collector interface mismatch — GetLeafCollector parameter type not aligned")
}
