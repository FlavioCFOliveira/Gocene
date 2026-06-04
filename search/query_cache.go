// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"math"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
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

// usageTrackingSentinel is the hash code used as a sentinel in the ring buffer.
// Mirrors UsageTrackingQueryCachingPolicy.SENTINEL (Integer.MIN_VALUE).
const usageTrackingSentinel = math.MinInt32

// usageTrackingDefaultHistorySize is the default ring-buffer history size.
// Mirrors the no-arg UsageTrackingQueryCachingPolicy() constructor (256).
const usageTrackingDefaultHistorySize = 256

// UsageTrackingQueryCachingPolicy is a QueryCachingPolicy that tracks usage
// statistics of recently-used filters in order to decide which filters are worth
// caching.
//
// This is the Go port of
// org.apache.lucene.search.UsageTrackingQueryCachingPolicy (Lucene 10.4.0). It
// records the hash code of each used query in a fixed-size
// FrequencyTrackingRingBuffer and caches a filter once its frequency reaches the
// minimum threshold returned by minFrequencyToCache: 2 for costly filters
// (MultiTermQuery-style, point-based, or TermInSetQuery), 4 for compound queries
// (BooleanQuery / DisjunctionMaxQuery) and 5 for everything else. Some queries
// are never cached (TermQuery, FieldExistsQuery, MatchAllDocsQuery,
// MatchNoDocsQuery, empty BooleanQuery / DisjunctionMaxQuery) because they are
// already plenty fast.
//
// Deviation from the reference, documented per the binary-compatibility mandate:
// Lucene relies on its class hierarchy (PrefixQuery extends AutomatonQuery
// extends MultiTermQuery, etc.) to classify costly queries. Gocene's query types
// do not share that hierarchy, so isCostly enumerates the concrete query types
// that are the Go counterparts of Lucene's MultiTermQuery subclasses and
// point-based queries. The classification result is identical.
type UsageTrackingQueryCachingPolicy struct {
	mu                  sync.Mutex
	recentlyUsedFilters *util.FrequencyTrackingRingBuffer
}

// NewUsageTrackingQueryCachingPolicy creates a policy with the default history
// size (256). Mirrors the no-arg Lucene constructor.
func NewUsageTrackingQueryCachingPolicy() *UsageTrackingQueryCachingPolicy {
	return NewUsageTrackingQueryCachingPolicyWithHistory(usageTrackingDefaultHistorySize)
}

// NewUsageTrackingQueryCachingPolicyWithHistory creates a policy with a
// configurable history size. Mirrors UsageTrackingQueryCachingPolicy(int
// historySize).
func NewUsageTrackingQueryCachingPolicyWithHistory(historySize int) *UsageTrackingQueryCachingPolicy {
	buf, err := util.NewFrequencyTrackingRingBuffer(historySize, usageTrackingSentinel)
	if err != nil {
		// historySize < 2 is a programming error; mirror Lucene's constructor
		// which would throw. Fall back to the default to stay non-nil.
		buf, _ = util.NewFrequencyTrackingRingBuffer(usageTrackingDefaultHistorySize, usageTrackingSentinel)
	}
	return &UsageTrackingQueryCachingPolicy{recentlyUsedFilters: buf}
}

// IsCostly reports whether building the DocIdSet for query is expensive enough
// to warrant caching after only a couple of uses. Mirrors
// UsageTrackingQueryCachingPolicy.isCostly. Exported (capitalised) to match the
// test surface; the Java method is package-private.
func IsCostly(query Query) bool {
	switch query.(type) {
	case *MultiTermQuery,
		*PrefixQuery,
		*WildcardQuery,
		*RegexpQuery,
		*TermRangeQuery,
		*AutomatonQuery,
		*FuzzyQuery,
		*TermInSetQuery,
		*PointRangeQuery:
		return true
	}
	return isPointQuery(query)
}

// isPointQuery reports whether query is a point-based range/exact query. Mirrors
// UsageTrackingQueryCachingPolicy.isPointQuery, translated from Lucene's
// class-name heuristic to Gocene's concrete point query types.
func isPointQuery(query Query) bool {
	if _, ok := query.(*PointRangeQuery); ok {
		return true
	}
	if _, ok := query.(PointQueryMarker); ok {
		return true
	}
	return false
}

// PointQueryMarker is an optional interface a point-based query may implement to
// declare itself point-backed for caching-policy classification, standing in for
// Lucene's "class name starts with Point and ends with Query" heuristic.
type PointQueryMarker interface {
	IsPointQuery() bool
}

// shouldNeverCache reports whether query must never be cached because it is
// already fast or trivially non-matching. Mirrors
// UsageTrackingQueryCachingPolicy.shouldNeverCache.
func usageTrackingShouldNeverCache(query Query) bool {
	switch q := query.(type) {
	case *TermQuery:
		return true
	case *FieldExistsQuery:
		return true
	case *MatchAllDocsQuery:
		return true
	case *MatchNoDocsQuery:
		return true
	case *BooleanQuery:
		return len(q.Clauses()) == 0
	case *DisjunctionMaxQuery:
		return len(q.Disjuncts()) == 0
	}
	return false
}

// minFrequencyToCache returns how many times query must appear in the history
// before being cached. Mirrors UsageTrackingQueryCachingPolicy.minFrequencyToCache.
func usageTrackingMinFrequencyToCache(query Query) int {
	if IsCostly(query) {
		return 2
	}
	minFrequency := 5
	switch query.(type) {
	case *BooleanQuery, *DisjunctionMaxQuery:
		minFrequency--
	}
	return minFrequency
}

// OnUse records a use of query in the recently-used ring buffer. Mirrors
// UsageTrackingQueryCachingPolicy.onUse: BoostQuery / ConstantScoreQuery
// wrappers are never passed here (the caller unwraps them), and queries that
// should never be cached are ignored.
func (p *UsageTrackingQueryCachingPolicy) OnUse(query Query) {
	if usageTrackingShouldNeverCache(query) {
		return
	}
	hashCode := query.HashCode()
	p.mu.Lock()
	p.recentlyUsedFilters.Add(hashCode)
	p.mu.Unlock()
}

// frequency returns the current frequency of query in the ring buffer.
func (p *UsageTrackingQueryCachingPolicy) frequency(query Query) int {
	hashCode := query.HashCode()
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.recentlyUsedFilters.Frequency(hashCode)
}

// ShouldCache reports whether query should be cached. Mirrors
// UsageTrackingQueryCachingPolicy.shouldCache.
func (p *UsageTrackingQueryCachingPolicy) ShouldCache(query Query) bool {
	if usageTrackingShouldNeverCache(query) {
		return false
	}
	return p.frequency(query) >= usageTrackingMinFrequencyToCache(query)
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
