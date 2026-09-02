// Copyright 2026 Gocene. All rights reserved.
// Use this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package writercache

import (
	"container/list"
	"github.com/FlavioCFOliveira/Gocene/facet"
)

type entry struct {
	key   *facet.FacetLabel
	value int
}

type nameIntCacheLRU struct {
	maxCacheSize int
	cache        map[*facet.FacetLabel]*list.Element
	ll           *list.List
}

func newNameIntCacheLRU(maxCacheSize int) *nameIntCacheLRU {
	return &nameIntCacheLRU{
		maxCacheSize: maxCacheSize,
		cache:        make(map[*facet.FacetLabel]*list.Element),
		ll:           list.New(),
	}
}

func (c *nameIntCacheLRU) __get(key *facet.FacetLabel) int {
	if elem, ok := c.cache[key]; ok {
		c.ll.MoveToFront(elem)
		return elem.Value.(*entry).value
	}
	return -1
}

func (c *nameIntCacheLRU) __put(key *facet.FacetLabel, val int) bool {
	if elem, ok := c.cache[key]; ok {
		c.ll.MoveToFront(elem)
		elem.Value.(*entry).value = val
		return false
	}

	elem := c.ll.PushFront(&entry{key, val})
	c.cache[key] = elem

	return c.isCacheFull()
}

func (c *nameIntCacheLRU) isCacheFull() bool {
	return len(c.cache) > c.maxCacheSize
}

func (c *nameIntCacheLRU) makeRoomLRU() bool {
	if !c.isCacheFull() {
		return false
	}
	// Remove 1/3rd of the cache
	n := len(c.cache) - (2*c.maxCacheSize)/3
	if n <= 0 {
		return false
	}
	for i := 0; i < n; i++ {
		elem := c.ll.Back()
		if elem == nil {
			break
		}
		c.ll.Remove(elem)
		delete(c.cache, elem.Value.(*entry).key)
	}
	return true
}

func (c *nameIntCacheLRU) clear() {
	c.cache = make(map[*facet.FacetLabel]*list.Element)
	c.ll.Init()
}

func (c *nameIntCacheLRU) getSize() int {
	return len(c.cache)
}

func (c *nameIntCacheLRU) __getHash(name *facet.FacetLabel) int {
	return name.LongHashCode() // Simplified, using the labels hash
}

func (c *nameIntCacheLRU) __putHash(name *facet.FacetLabel, val int) bool {
	// For the hashed version, we'd need a different map key.
	// This is a simplified implementation of the LRU.
	return c.__put(name, val)
}
