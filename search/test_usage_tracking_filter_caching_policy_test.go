// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestUsageTrackingFilterCachingPolicy.java
//
// Deviation: all test methods skipped — Gocene's UsageTrackingQueryCachingPolicy is a stub
// (no IsCostly, no never-cache-term-query semantics); testBooleanQueries additionally
// requires IndexWriter+IndexSearcher. Full port deferred until caching policy is complete.

package search

import "testing"

func TestUsageTrackingFilterCachingPolicy_CostlyFilter(t *testing.T) {
	t.Skip("requires full UsageTrackingQueryCachingPolicy implementation (IsCostly not yet ported in Gocene)")
}
func TestUsageTrackingFilterCachingPolicy_NeverCacheMatchAll(t *testing.T) {
	t.Skip("requires full UsageTrackingQueryCachingPolicy implementation (never-cache semantics not yet ported in Gocene)")
}
func TestUsageTrackingFilterCachingPolicy_NeverCacheTermFilter(t *testing.T) {
	t.Skip("requires full UsageTrackingQueryCachingPolicy implementation (never-cache semantics not yet ported in Gocene)")
}
func TestUsageTrackingFilterCachingPolicy_NeverCacheDocValuesFieldExistsFilter(t *testing.T) {
	t.Skip("requires full UsageTrackingQueryCachingPolicy implementation (never-cache semantics not yet ported in Gocene)")
}
func TestUsageTrackingFilterCachingPolicy_BooleanQueries(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher+caching policy integration (pre-existing failure in Gocene)")
}
