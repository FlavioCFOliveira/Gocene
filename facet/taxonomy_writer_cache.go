// Copyright 2026 Gocene. All rights reserved.
// Use this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facet

import (
	"container/list"
	"sync"
)

// TaxonomyWriterCache is a simple interface for a cache of category->ordinal mappings.
type TaxonomyWriterCache interface {
	Close()
	Get(categoryPath *FacetLabel) int
	Put(categoryPath *FacetLabel, ordinal int) bool
	IsFull() bool
	Clear()
	Size() int
}

type cacheEntry struct {
	label   *FacetLabel
	ordinal int
}

// LruTaxonomyWriterCache is an LRU implementation of TaxonomyWriterCache.
type LruTaxonomyWriterCache struct {
	mu       sync.Mutex
	maxSize  int
	list     *list.List
	elements map[*FacetLabel]*list.Element
}

func NewLruTaxonomyWriterCache(cacheSize int) *LruTaxonomyWriterCache {
	return &LruTaxonomyWriterCache{
		maxSize:  cacheSize,
		list:     list.New(),
		elements: make(map[*FacetLabel]*list.Element),
	}
}

func (c *LruTaxonomyWriterCache) Close() {
	c.Clear()
}

func (c *LruTaxonomyWriterCache) Get(categoryPath *FacetLabel) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.elements[categoryPath]; ok {
		c.list.MoveToFront(elem)
		return elem.Value.(*cacheEntry).ordinal
	}
	return -1
}

func (c *LruTaxonomyWriterCache) Put(categoryPath *FacetLabel, ordinal int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.elements[categoryPath]; ok {
		c.list.MoveToFront(elem)
		elem.Value.(*cacheEntry).ordinal = ordinal
		return false
	}

	evicted := false
	if c.list.Len() >= c.maxSize {
		// Evict 1/3 of the oldest entries to match Lucene's makeRoomLRU behaviour
		numToEvict := c.maxSize / 3
		if numToEvict == 0 {
			numToEvict = 1
		}
		for i := 0; i < numToEvict && c.list.Len() > 0; i++ {
			last := c.list.Back()
			if last != nil {
				entry := last.Value.(*cacheEntry)
				delete(c.elements, entry.label)
				c.list.Remove(last)
			}
		}
		evicted = true
	}

	entry := &cacheEntry{label: categoryPath, ordinal: ordinal}
	elem := c.list.PushFront(entry)
	c.elements[categoryPath] = elem

	return evicted
}

func (c *LruTaxonomyWriterCache) IsFull() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.list.Len() >= c.maxSize
}

func (c *LruTaxonomyWriterCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.list.Init()
	c.elements = make(map[*FacetLabel]*list.Element)
}

func (c *LruTaxonomyWriterCache) Size() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.list.Len()
}
