package taxonomywritercache

import (
	"container/list"
	"hash/fnv"
)

// NameHashIntCacheLRU stores label hashes instead of the labels themselves to
// keep memory usage low. Collisions are unlikely but possible — the caller is
// responsible for verifying the ordinal returned by Get against the index.
// Mirrors org.apache.lucene.facet.taxonomy.writercache.NameHashIntCacheLRU.
type NameHashIntCacheLRU struct {
	capacity int
	order    *list.List
	entries  map[uint64]*list.Element
}

type hashEntry struct {
	hash uint64
	ord  int
}

// NewNameHashIntCacheLRU builds a hash-keyed LRU cache.
func NewNameHashIntCacheLRU(capacity int) *NameHashIntCacheLRU {
	if capacity < 1 {
		capacity = 1
	}
	return &NameHashIntCacheLRU{
		capacity: capacity,
		order:    list.New(),
		entries:  make(map[uint64]*list.Element, capacity),
	}
}

// Get returns the ordinal cached for label or -1 when absent; bumps the
// entry to MRU.
func (c *NameHashIntCacheLRU) Get(label string) int {
	h := hashLabel(label)
	if e, ok := c.entries[h]; ok {
		c.order.MoveToFront(e)
		return e.Value.(*hashEntry).ord
	}
	return -1
}

// Put records (label, ord). Returns true when the cache is full.
func (c *NameHashIntCacheLRU) Put(label string, ord int) bool {
	h := hashLabel(label)
	if e, ok := c.entries[h]; ok {
		e.Value.(*hashEntry).ord = ord
		c.order.MoveToFront(e)
		return c.order.Len() >= c.capacity
	}
	e := c.order.PushFront(&hashEntry{hash: h, ord: ord})
	c.entries[h] = e
	if c.order.Len() > c.capacity {
		oldest := c.order.Back()
		c.order.Remove(oldest)
		delete(c.entries, oldest.Value.(*hashEntry).hash)
	}
	return c.order.Len() >= c.capacity
}

// Size returns the number of cached entries.
func (c *NameHashIntCacheLRU) Size() int { return c.order.Len() }

// Clear empties the cache.
func (c *NameHashIntCacheLRU) Clear() {
	c.order.Init()
	c.entries = make(map[uint64]*list.Element, c.capacity)
}

// IsFull reports whether the cache holds capacity entries.
func (c *NameHashIntCacheLRU) IsFull() bool { return c.order.Len() >= c.capacity }

func hashLabel(label string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(label))
	return h.Sum64()
}

var _ TaxonomyWriterCache = (*NameHashIntCacheLRU)(nil)
