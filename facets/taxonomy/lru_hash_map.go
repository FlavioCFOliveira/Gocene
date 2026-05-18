// Package taxonomy implements support types from
// org.apache.lucene.facet.taxonomy that sit outside the directory/writercache
// sub-packages.
package taxonomy

import "container/list"

// LRUHashMap is a fixed-capacity LRU map keyed by generic comparable keys.
// Mirrors org.apache.lucene.facet.taxonomy.LRUHashMap.
type LRUHashMap[K comparable, V any] struct {
	capacity int
	order    *list.List
	entries  map[K]*list.Element
}

type lruMapEntry[K comparable, V any] struct {
	key K
	val V
}

// NewLRUHashMap builds a map holding up to capacity entries.
func NewLRUHashMap[K comparable, V any](capacity int) *LRUHashMap[K, V] {
	if capacity < 1 {
		capacity = 1
	}
	return &LRUHashMap[K, V]{
		capacity: capacity,
		order:    list.New(),
		entries:  make(map[K]*list.Element, capacity),
	}
}

// Get returns (value, true) when key is cached and bumps it to MRU.
func (m *LRUHashMap[K, V]) Get(key K) (V, bool) {
	var zero V
	if e, ok := m.entries[key]; ok {
		m.order.MoveToFront(e)
		return e.Value.(*lruMapEntry[K, V]).val, true
	}
	return zero, false
}

// Put records (key, val). Returns the evicted (key, value, true) when one was
// pushed out, or (zero, zero, false) when no eviction happened.
func (m *LRUHashMap[K, V]) Put(key K, val V) (K, V, bool) {
	var zeroK K
	var zeroV V
	if e, ok := m.entries[key]; ok {
		e.Value.(*lruMapEntry[K, V]).val = val
		m.order.MoveToFront(e)
		return zeroK, zeroV, false
	}
	e := m.order.PushFront(&lruMapEntry[K, V]{key: key, val: val})
	m.entries[key] = e
	if m.order.Len() > m.capacity {
		oldest := m.order.Back()
		m.order.Remove(oldest)
		ent := oldest.Value.(*lruMapEntry[K, V])
		delete(m.entries, ent.key)
		return ent.key, ent.val, true
	}
	return zeroK, zeroV, false
}

// Size returns the current number of entries.
func (m *LRUHashMap[K, V]) Size() int { return m.order.Len() }

// Clear empties the map.
func (m *LRUHashMap[K, V]) Clear() {
	m.order.Init()
	m.entries = make(map[K]*list.Element, m.capacity)
}
