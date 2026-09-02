// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"container/list"
	"sync"
)

// TermCacheEntry represents a cached term with its associated data.
type TermCacheEntry struct {
	Term          *Term
	TermStats     *TermStats
	DocFreq       int64
	TotalTermFreq int64
}

// TermCache implements an LRU cache for frequently accessed terms.
// This reduces I/O overhead for repeated queries.
type TermCache struct {
	maxSize int
	items   map[string]*list.Element
	order   *list.List
	mu      sync.RWMutex
}

// termCacheNode represents a node in the LRU cache.
type termCacheNode struct {
	key   string
	entry *TermCacheEntry
}

// TermStats holds statistics for a term.
type TermStats struct {
	DocFreq       int64
	TotalTermFreq int64
}

// NewTermCache creates a new TermCache with the specified maximum size.
func NewTermCache(maxSize int) *TermCache {
	if maxSize <= 0 {
		maxSize = 1000 // Default size
	}
	return &TermCache{
		maxSize: maxSize,
		items:   make(map[string]*list.Element),
		order:   list.New(),
	}
}

// Get retrieves a term from the cache.
// Returns the entry and true if found, nil and false otherwise.
func (c *TermCache) Get(term *Term) (*TermCacheEntry, bool) {
	if c == nil || term == nil {
		return nil, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	elem, found := c.items[term.String()]
	if !found {
		return nil, false
	}

	// Move to front (most recently used)
	c.order.MoveToFront(elem)

	return elem.Value.(*termCacheNode).entry, true
}

// Put adds or updates a term in the cache.
func (c *TermCache) Put(term *Term, entry *TermCacheEntry) {
	if c == nil || term == nil || entry == nil {
		return
	}

	key := term.String()

	c.mu.Lock()
	defer c.mu.Unlock()

	// If already exists, update and move to front
	if elem, found := c.items[key]; found {
		elem.Value.(*termCacheNode).entry = entry
		c.order.MoveToFront(elem)
		return
	}

	// Add new entry
	node := &termCacheNode{key: key, entry: entry}
	elem := c.order.PushFront(node)
	c.items[key] = elem

	// Evict oldest if over capacity
	if c.order.Len() > c.maxSize {
		c.evictOldest()
	}
}

// evictOldest removes the least recently used item from the cache.
func (c *TermCache) evictOldest() {
	elem := c.order.Back()
	if elem == nil {
		return
	}

	node := elem.Value.(*termCacheNode)
	delete(c.items, node.key)
	c.order.Remove(elem)
}

// Invalidate removes a specific term from the cache.
func (c *TermCache) Invalidate(term *Term) {
	if c == nil || term == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	key := term.String()
	if elem, found := c.items[key]; found {
		delete(c.items, key)
		c.order.Remove(elem)
	}
}

// InvalidateAll clears all entries from the cache.
func (c *TermCache) InvalidateAll() {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.order = list.New()
}

// Size returns the current number of items in the cache.
func (c *TermCache) Size() int {
	if c == nil {
		return 0
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.order.Len()
}

// MaxSize returns the maximum capacity of the cache.
func (c *TermCache) MaxSize() int {
	if c == nil {
		return 0
	}
	return c.maxSize
}

// Resize changes the maximum size of the cache.
// If the new size is smaller, oldest entries are evicted.
func (c *TermCache) Resize(newSize int) {
	if c == nil || newSize <= 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.maxSize = newSize

	// Evict oldest entries if necessary
	for c.order.Len() > c.maxSize {
		c.evictOldest()
	}
}

// GetStats returns cache statistics.
func (c *TermCache) GetStats() TermCacheStats {
	if c == nil {
		return TermCacheStats{}
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	return TermCacheStats{
		Size:    c.order.Len(),
		MaxSize: c.maxSize,
	}
}

// TermCacheStats holds statistics about the cache.
type TermCacheStats struct {
	Size    int
	MaxSize int
}

// HitRate returns the cache hit rate (placeholder for future implementation with counters).
func (s TermCacheStats) HitRate() float64 {
	// Placeholder - would need hit/miss counters for actual implementation
	return 0.0
}

// String returns a string representation of the cache entry.
func (e *TermCacheEntry) String() string {
	if e == nil || e.Term == nil {
		return "nil"
	}
	return e.Term.String()
}
