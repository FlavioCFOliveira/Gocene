package taxonomywritercache

import "container/list"

// LruTaxonomyWriterCache is a bounded LRU label-to-ordinal cache. Mirrors
// org.apache.lucene.facet.taxonomy.writercache.LruTaxonomyWriterCache.
type LruTaxonomyWriterCache struct {
	capacity int
	order    *list.List
	entries  map[string]*list.Element
}

type lruEntry struct {
	label string
	ord   int
}

// NewLruTaxonomyWriterCache builds a cache holding up to capacity entries.
func NewLruTaxonomyWriterCache(capacity int) *LruTaxonomyWriterCache {
	if capacity < 1 {
		capacity = 1
	}
	return &LruTaxonomyWriterCache{
		capacity: capacity,
		order:    list.New(),
		entries:  make(map[string]*list.Element, capacity),
	}
}

// Get returns the ordinal for label and bumps it to MRU; -1 when absent.
func (c *LruTaxonomyWriterCache) Get(label string) int {
	if e, ok := c.entries[label]; ok {
		c.order.MoveToFront(e)
		return e.Value.(*lruEntry).ord
	}
	return -1
}

// Put records (label, ord). Returns true when the cache is full after the
// insertion so callers can flush.
func (c *LruTaxonomyWriterCache) Put(label string, ord int) bool {
	if e, ok := c.entries[label]; ok {
		e.Value.(*lruEntry).ord = ord
		c.order.MoveToFront(e)
		return c.order.Len() >= c.capacity
	}
	e := c.order.PushFront(&lruEntry{label: label, ord: ord})
	c.entries[label] = e
	if c.order.Len() > c.capacity {
		oldest := c.order.Back()
		c.order.Remove(oldest)
		delete(c.entries, oldest.Value.(*lruEntry).label)
	}
	return c.order.Len() >= c.capacity
}

// Size returns the number of entries currently cached.
func (c *LruTaxonomyWriterCache) Size() int { return c.order.Len() }

// Clear empties the cache.
func (c *LruTaxonomyWriterCache) Clear() {
	c.order.Init()
	c.entries = make(map[string]*list.Element, c.capacity)
}

// IsFull reports whether the cache holds capacity entries.
func (c *LruTaxonomyWriterCache) IsFull() bool { return c.order.Len() >= c.capacity }

var _ TaxonomyWriterCache = (*LruTaxonomyWriterCache)(nil)
