// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"container/list"
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// LRUQueryCache is a Least Recently Used query cache implementation.
// This is the Go port of Lucene's org.apache.lucene.search.LRUQueryCache.
//
// The cache stores query results and evicts the least recently used entries
// when the cache reaches its maximum size. It is thread-safe and can be
// used concurrently by multiple search threads.
type LRUQueryCache struct {
	*BaseQueryCache

	// maxSize is the maximum number of entries in the cache
	maxSize int

	// maxRamBytes is the maximum RAM usage in bytes (0 = unlimited)
	maxRamBytes int64

	// cache is the underlying LRU cache
	cache map[Query]*list.Element

	// lru is the LRU list
	lru *list.List

	// mu protects the cache
	mu sync.RWMutex

	// stats tracks cache statistics
	hits   int64
	misses int64
	evicts int64
}

// cacheEntry represents a single entry in the cache.
type cacheEntry struct {
	query   Query
	weight  Weight
	context index.IndexReaderInterface
}

// NewLRUQueryCache creates a new LRUQueryCache with the given maximum size.
//
// Parameters:
//   - maxSize: The maximum number of queries to cache. When the cache is full,
//     the least recently used entry is evicted.
//   - maxRamBytes: The maximum RAM usage in bytes. Use 0 for unlimited.
func NewLRUQueryCache(maxSize int, maxRamBytes int64) *LRUQueryCache {
	return &LRUQueryCache{
		BaseQueryCache: NewBaseQueryCache(),
		maxSize:        maxSize,
		maxRamBytes:    maxRamBytes,
		cache:          make(map[Query]*list.Element),
		lru:            list.New(),
	}
}

// DoCache wraps the given Weight with a caching layer if the query should be cached.
func (c *LRUQueryCache) DoCache(weight Weight, policy QueryCachingPolicy) Weight {
	if weight == nil {
		return nil
	}

	query := weight.GetQuery()
	if query == nil {
		return weight
	}

	// Check if this query should be cached
	if !policy.ShouldCache(query) {
		return weight
	}

	// Track usage
	policy.OnUse(query)

	// Return a cached weight wrapper
	return &CachedWeight{
		Weight: weight,
		cache:  c,
		query:  query,
	}
}

// Get retrieves a cached Weight for the given query and reader.
// Returns nil if not found.
func (c *LRUQueryCache) Get(query Query, reader index.IndexReaderInterface) Weight {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.cache[query]
	if !ok {
		c.misses++
		return nil
	}

	entry := elem.Value.(*cacheEntry)

	// Check if the context matches
	if entry.context != reader {
		c.misses++
		return nil
	}

	// Move to front (most recently used)
	c.lru.MoveToFront(elem)
	c.hits++

	return entry.weight
}

// Put adds a Weight to the cache.
func (c *LRUQueryCache) Put(query Query, reader index.IndexReaderInterface, weight Weight) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already in cache
	if elem, ok := c.cache[query]; ok {
		// Update existing entry
		entry := elem.Value.(*cacheEntry)
		entry.weight = weight
		entry.context = reader
		c.lru.MoveToFront(elem)
		return
	}

	// Evict if necessary
	for c.shouldEvict() {
		c.evictLRU()
	}

	// Add new entry
	entry := &cacheEntry{
		query:   query,
		weight:  weight,
		context: reader,
	}
	elem := c.lru.PushFront(entry)
	c.cache[query] = elem
}

// shouldEvict returns true if we should evict an entry.
func (c *LRUQueryCache) shouldEvict() bool {
	if c.maxSize > 0 && len(c.cache) >= c.maxSize {
		return true
	}
	// TODO: Check maxRamBytes
	return false
}

// evictLRU evicts the least recently used entry.
func (c *LRUQueryCache) evictLRU() {
	elem := c.lru.Back()
	if elem == nil {
		return
	}

	entry := elem.Value.(*cacheEntry)
	delete(c.cache, entry.query)
	c.lru.Remove(elem)
	c.evicts++
}

// Clear removes all entries from the cache.
func (c *LRUQueryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[Query]*list.Element)
	c.lru = list.New()
}

// GetStats returns cache statistics.
func (c *LRUQueryCache) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return CacheStats{
		Hits:   c.hits,
		Misses: c.misses,
		Evicts: c.evicts,
		Size:   int64(len(c.cache)),
	}
}

// CacheStats holds cache statistics.
type CacheStats struct {
	Hits   int64
	Misses int64
	Evicts int64
	Size   int64
}

// HitRate returns the cache hit rate as a percentage.
func (s CacheStats) HitRate() float64 {
	total := s.Hits + s.Misses
	if total == 0 {
		return 0
	}
	return float64(s.Hits) / float64(total) * 100
}

// String returns a string representation of the cache stats.
func (s CacheStats) String() string {
	return fmt.Sprintf("CacheStats{hits=%d, misses=%d, evicts=%d, size=%d, hitRate=%.2f%%}",
		s.Hits, s.Misses, s.Evicts, s.Size, s.HitRate())
}

// CachedWeight wraps a Weight with caching functionality.
type CachedWeight struct {
	Weight
	cache *LRUQueryCache
	query Query
}

// Scorer creates a scorer for this weight, using the cache if available.
func (w *CachedWeight) Scorer(context *index.LeafReaderContext) (Scorer, error) {
	// Try to get from cache
	if cached := w.cache.Get(w.query, context.Reader()); cached != nil {
		return cached.Scorer(context)
	}

	// Create the scorer
	scorer, err := w.Weight.Scorer(context)
	if err != nil {
		return nil, err
	}

	// Cache the weight for future use
	// Note: We cache the weight, not the scorer, as scorers are per-segment
	w.cache.Put(w.query, context.Reader(), w.Weight)

	return scorer, nil
}

// ScorerSupplier creates a scorer supplier for this weight.
func (w *CachedWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	return w.Weight.ScorerSupplier(context)
}

// Explain returns an explanation of the score for the given document.
func (w *CachedWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	return w.Weight.Explain(context, doc)
}

// BulkScorer creates a bulk scorer for efficient bulk scoring.
func (w *CachedWeight) BulkScorer(context *index.LeafReaderContext) (BulkScorer, error) {
	return w.Weight.BulkScorer(context)
}

// IsCacheable returns true if this weight can be cached for the given leaf.
func (w *CachedWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return w.Weight.IsCacheable(ctx)
}

// Count returns the count of matching documents in sub-linear time.
func (w *CachedWeight) Count(context *index.LeafReaderContext) (int, error) {
	return w.Weight.Count(context)
}

// Matches returns the matches for a specific document.
func (w *CachedWeight) Matches(context *index.LeafReaderContext, doc int) (Matches, error) {
	return w.Weight.Matches(context, doc)
}

// Ensure CachedWeight implements Weight
var _ Weight = (*CachedWeight)(nil)

// Ensure LRUQueryCache implements QueryCache
var _ QueryCache = (*LRUQueryCache)(nil)
