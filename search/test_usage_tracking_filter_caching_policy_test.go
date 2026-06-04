// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestUsageTrackingFilterCachingPolicy.java
//
// These tests exercise UsageTrackingQueryCachingPolicy:
//   - testCostlyFilter: PrefixQuery and a point range query are "costly"; a
//     TermQuery is not.
//   - testNeverCacheMatchAll / testNeverCacheTermFilter /
//     testNeverCacheDocValuesFieldExistsFilter: even after 1000 uses, these
//     queries are never cached because shouldNeverCache returns true for them.
//   - testBooleanQueries: drives the policy through IndexSearcher.count + an
//     LRUQueryCache to verify the exact caching timing (the BooleanQuery gets
//     cached on its 4th use; its SHOULD children are not cached because the
//     compound query never pulls their scorers on their own).
//
// The first four are faithful, passing ports of the policy's classification and
// never-cache semantics, driven by the production
// UsageTrackingQueryCachingPolicy. testBooleanQueries additionally requires the
// LRUQueryCache scorer-supplier caching integration wired into
// IndexSearcher.count (per-segment DocIdSet caching, getCacheSize, and the
// "never pull child scorers" contract); that cache-integration subsystem is not
// implemented in Gocene, so that test fails honestly citing it (rather than
// being skipped or weakened).

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// utfcpIntPointRangeQuery builds the Gocene equivalent of
// IntPoint.newRangeQuery(field, lo, hi): a single-dimension PointRangeQuery over
// the sortable 4-byte encodings of lo and hi.
func utfcpIntPointRangeQuery(t *testing.T, field string, lo, hi int32) search.Query {
	t.Helper()
	lower := make([]byte, 4)
	upper := make([]byte, 4)
	util.IntToSortableBytes(lo, lower, 0)
	util.IntToSortableBytes(hi, upper, 0)
	q, err := search.NewPointRangeQuery(field, lower, upper)
	if err != nil {
		t.Fatalf("NewPointRangeQuery: %v", err)
	}
	return q
}

// TestUsageTrackingFilterCachingPolicy_CostlyFilter ports testCostlyFilter.
func TestUsageTrackingFilterCachingPolicy_CostlyFilter(t *testing.T) {
	if !search.IsCostly(search.NewPrefixQuery(index.NewTerm("field", "prefix"))) {
		t.Error("PrefixQuery should be costly")
	}
	if !search.IsCostly(utfcpIntPointRangeQuery(t, "intField", 1, 1000)) {
		t.Error("IntPoint range query should be costly")
	}
	if search.IsCostly(search.NewTermQuery(index.NewTerm("field", "value"))) {
		t.Error("TermQuery should not be costly")
	}
}

// TestUsageTrackingFilterCachingPolicy_NeverCacheMatchAll ports
// testNeverCacheMatchAll.
func TestUsageTrackingFilterCachingPolicy_NeverCacheMatchAll(t *testing.T) {
	q := search.NewMatchAllDocsQuery()
	policy := search.NewUsageTrackingQueryCachingPolicy()
	for i := 0; i < 1000; i++ {
		policy.OnUse(q)
	}
	if policy.ShouldCache(q) {
		t.Error("MatchAllDocsQuery must never be cached")
	}
}

// TestUsageTrackingFilterCachingPolicy_NeverCacheTermFilter ports
// testNeverCacheTermFilter.
func TestUsageTrackingFilterCachingPolicy_NeverCacheTermFilter(t *testing.T) {
	q := search.NewTermQuery(index.NewTerm("foo", "bar"))
	policy := search.NewUsageTrackingQueryCachingPolicy()
	for i := 0; i < 1000; i++ {
		policy.OnUse(q)
	}
	if policy.ShouldCache(q) {
		t.Error("TermQuery must never be cached")
	}
}

// TestUsageTrackingFilterCachingPolicy_NeverCacheDocValuesFieldExistsFilter
// ports testNeverCacheDocValuesFieldExistsFilter.
func TestUsageTrackingFilterCachingPolicy_NeverCacheDocValuesFieldExistsFilter(t *testing.T) {
	q := search.NewFieldExistsQuery("foo")
	policy := search.NewUsageTrackingQueryCachingPolicy()
	for i := 0; i < 1000; i++ {
		policy.OnUse(q)
	}
	if policy.ShouldCache(q) {
		t.Error("FieldExistsQuery must never be cached")
	}
}

// TestUsageTrackingFilterCachingPolicy_BooleanQueries ports testBooleanQueries.
//
// The reference wires the policy and an LRUQueryCache into an IndexSearcher and
// uses IndexSearcher.count to assert the precise caching timing: a SHOULD-over-
// SHOULD BooleanQuery is cached only on its 4th use, and its children are never
// cached because the compound query never pulls scorers on them in isolation.
// This exercises the LRUQueryCache scorer-supplier caching integration
// (per-segment DocIdSet caching, getCacheSize) wired into IndexSearcher.count,
// none of which is implemented in Gocene.
func TestUsageTrackingFilterCachingPolicy_BooleanQueries(t *testing.T) {
	t.Fatalf("blocked: LRUQueryCache scorer-supplier caching integration " +
		"(IndexSearcher.setQueryCache/count, per-segment DocIdSet caching, " +
		"LRUQueryCache.getCacheSize) not implemented in Gocene")
}
