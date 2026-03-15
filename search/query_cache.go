// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// QueryCache is a cache for query results.
// This is the Go port of Lucene's org.apache.lucene.search.QueryCache.
//
// Query caching can improve performance for frequently used queries by caching
// their results and reusing them across searches. The cache is typically
// associated with an IndexReader and is invalidated when the reader changes.
type QueryCache interface {
	// DoCache wraps the given Weight with a caching layer.
	// The caching policy determines whether this query should be cached.
	DoCache(weight Weight, policy QueryCachingPolicy) Weight
}

// QueryCachingPolicy determines which queries should be cached.
// This is the Go port of Lucene's org.apache.lucene.search.QueryCachingPolicy.
type QueryCachingPolicy interface {
	// ShouldCache returns true if the given query should be cached.
	ShouldCache(query Query) bool

	// OnUse is called when a query is used.
	OnUse(query Query)
}

// BaseQueryCache provides common functionality for query caches.
type BaseQueryCache struct{}

// NewBaseQueryCache creates a new BaseQueryCache.
func NewBaseQueryCache() *BaseQueryCache {
	return &BaseQueryCache{}
}

// DoCache wraps the given Weight with a caching layer.
// Subclasses should override this method.
func (c *BaseQueryCache) DoCache(weight Weight, policy QueryCachingPolicy) Weight {
	// Default implementation: no caching
	return weight
}

// BaseQueryCachingPolicy provides common functionality for caching policies.
type BaseQueryCachingPolicy struct{}

// NewBaseQueryCachingPolicy creates a new BaseQueryCachingPolicy.
func NewBaseQueryCachingPolicy() *BaseQueryCachingPolicy {
	return &BaseQueryCachingPolicy{}
}

// ShouldCache returns true if the given query should be cached.
// Default implementation: cache nothing.
func (p *BaseQueryCachingPolicy) ShouldCache(query Query) bool {
	return false
}

// OnUse is called when a query is used.
// Default implementation: do nothing.
func (p *BaseQueryCachingPolicy) OnUse(query Query) {}

// Ensure BaseQueryCachingPolicy implements QueryCachingPolicy
var _ QueryCachingPolicy = (*BaseQueryCachingPolicy)(nil)

// UsageTrackingQueryCachingPolicy tracks query usage and caches frequently used queries.
type UsageTrackingQueryCachingPolicy struct {
	*BaseQueryCachingPolicy
	minFrequency int
	usageCounts  map[Query]int
}

// NewUsageTrackingQueryCachingPolicy creates a new UsageTrackingQueryCachingPolicy.
func NewUsageTrackingQueryCachingPolicy(minFrequency int) *UsageTrackingQueryCachingPolicy {
	return &UsageTrackingQueryCachingPolicy{
		BaseQueryCachingPolicy: NewBaseQueryCachingPolicy(),
		minFrequency:           minFrequency,
		usageCounts:            make(map[Query]int),
	}
}

// ShouldCache returns true if the query has been used frequently enough.
func (p *UsageTrackingQueryCachingPolicy) ShouldCache(query Query) bool {
	return p.usageCounts[query] >= p.minFrequency
}

// OnUse increments the usage count for the query.
func (p *UsageTrackingQueryCachingPolicy) OnUse(query Query) {
	p.usageCounts[query]++
}

// Ensure UsageTrackingQueryCachingPolicy implements QueryCachingPolicy
var _ QueryCachingPolicy = (*UsageTrackingQueryCachingPolicy)(nil)

// CacheHelper provides access to the query cache for an IndexReader.
// This is the Go port of Lucene's CacheHelper.
type CacheHelper struct {
	cache       QueryCache
	policy      QueryCachingPolicy
	indexReader index.IndexReaderInterface
}

// NewCacheHelper creates a new CacheHelper.
func NewCacheHelper(reader index.IndexReaderInterface, cache QueryCache, policy QueryCachingPolicy) *CacheHelper {
	return &CacheHelper{
		cache:       cache,
		policy:      policy,
		indexReader: reader,
	}
}

// GetQueryCache returns the query cache.
func (h *CacheHelper) GetQueryCache() QueryCache {
	return h.cache
}

// GetQueryCachingPolicy returns the query caching policy.
func (h *CacheHelper) GetQueryCachingPolicy() QueryCachingPolicy {
	return h.policy
}

// GetIndexReader returns the index reader.
func (h *CacheHelper) GetIndexReader() index.IndexReaderInterface {
	return h.indexReader
}
