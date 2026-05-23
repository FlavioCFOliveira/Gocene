// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package taxonomywritercache

import "container/list"

// NameIntCacheLRU is an LRU cache from string label to int ordinal. It is
// NOT synchronized; callers are responsible for synchronisation. Mirrors
// org.apache.lucene.facet.taxonomy.writercache.NameIntCacheLRU.
//
// When the cache reaches maxCacheSize, makeRoomLRU evicts the oldest third of
// the entries (matching the Java behaviour).
type NameIntCacheLRU struct {
	maxSize int
	order   *list.List
	entries map[string]*list.Element

	// Statistics (for debugging).
	NMisses int64
	NHits   int64
}

type nameCacheEntry struct {
	label string
	ord   int
}

// NewNameIntCacheLRU creates a cache that can hold up to maxSize entries.
// If maxSize is ≤ 0 the cache is treated as unbounded (no LRU eviction).
func NewNameIntCacheLRU(maxSize int) *NameIntCacheLRU {
	return &NameIntCacheLRU{
		maxSize: maxSize,
		order:   list.New(),
		entries: make(map[string]*list.Element),
	}
}

// MaxSize returns the configured capacity.
func (c *NameIntCacheLRU) MaxSize() int { return c.maxSize }

// Size returns the number of entries currently in the cache.
func (c *NameIntCacheLRU) Size() int { return c.order.Len() }

// Get returns the cached ordinal for label, or -1 when absent. Bumps the
// entry to MRU. Increments NHits or NMisses accordingly.
func (c *NameIntCacheLRU) Get(label string) int {
	e, ok := c.entries[label]
	if !ok {
		c.NMisses++
		return -1
	}
	c.order.MoveToFront(e)
	c.NHits++
	return e.Value.(*nameCacheEntry).ord
}

// Put stores (label, ord) in the cache. Returns true when the cache is over
// capacity (the caller should call MakeRoomLRU or flush).
func (c *NameIntCacheLRU) Put(label string, ord int) bool {
	if e, ok := c.entries[label]; ok {
		e.Value.(*nameCacheEntry).ord = ord
		c.order.MoveToFront(e)
	} else {
		e = c.order.PushFront(&nameCacheEntry{label: label, ord: ord})
		c.entries[label] = e
	}
	return c.isCacheFull()
}

// PutPrefix stores a prefix of label (up to prefixLen components, using '/'
// as separator) in the cache. Returns true when the cache is over capacity.
func (c *NameIntCacheLRU) PutPrefix(label string, prefixLen int, ord int) bool {
	key := labelPrefix(label, prefixLen)
	return c.Put(key, ord)
}

// labelPrefix returns the prefix of label containing prefixLen '/'-delimited
// components, or the full label when prefixLen ≥ component count.
func labelPrefix(label string, prefixLen int) string {
	if prefixLen <= 0 {
		return ""
	}
	count := 0
	for i, ch := range label {
		if ch == '/' {
			count++
			if count == prefixLen {
				return label[:i]
			}
		}
	}
	return label
}

func (c *NameIntCacheLRU) isCacheFull() bool {
	return c.maxSize > 0 && c.order.Len() > c.maxSize
}

// MakeRoomLRU evicts the oldest third of entries when the cache is full.
// Returns true if any entries were removed, false otherwise. Mirrors
// NameIntCacheLRU.makeRoomLRU.
func (c *NameIntCacheLRU) MakeRoomLRU() bool {
	if !c.isCacheFull() {
		return false
	}
	n := c.order.Len() - int(2*int64(c.maxSize)/3)
	if n <= 0 {
		return false
	}
	for i := 0; i < n; i++ {
		oldest := c.order.Back()
		if oldest == nil {
			break
		}
		c.order.Remove(oldest)
		delete(c.entries, oldest.Value.(*nameCacheEntry).label)
	}
	return true
}

// Clear empties the cache.
func (c *NameIntCacheLRU) Clear() {
	c.order.Init()
	c.entries = make(map[string]*list.Element)
}

// Stats returns a debug string showing hit/miss counts.
func (c *NameIntCacheLRU) Stats() string {
	return "#miss=" + itoa(c.NMisses) + " #hit=" + itoa(c.NHits)
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	pos := len(buf)
	for v > 0 {
		pos--
		buf[pos] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
