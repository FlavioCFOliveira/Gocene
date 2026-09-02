// Copyright 2026 Gocene. All rights reserved.
// Use this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package writercache

import (
	"sync"
	"github.com/FlavioCFOliveira/Gocene/facet"
)

type LruTaxonomyWriterCache struct {
	mu    sync.Mutex
	cache *nameIntCacheLRU
}

func NewLruTaxonomyWriterCache(cacheSize int) *LruTaxonomyWriterCache {
	return &LruTaxonomyWriterCache{
		cache: newNameIntCacheLRU(cacheSize),
	}
}

func (c *LruTaxonomyWriterCache) IsFull() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cache.isCacheFull()
}

func (c *LruTaxonomyWriterCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache.clear()
}

func (c *LruTaxonomyWriterCache) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache.clear()
	c.cache = nil
}

func (c *LruTaxonomyWriterCache) Size() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cache.getSize()
}

func (c *LruTaxonomyWriterCache) Get(categoryPath *facet.FacetLabel) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cache.__get(categoryPath)
}

func (c *LruTaxonomyWriterCache) Put(categoryPath *facet.FacetLabel, ordinal int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	ret := c.cache.__put(categoryPath, ordinal)
	if ret {
		c.cache.makeRoomLRU()
	}
	return ret
}
