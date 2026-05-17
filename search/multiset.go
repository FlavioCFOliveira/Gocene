// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Multiset is a collection that allows duplicate elements. It is implemented
// on top of a map from element to occurrence count.
//
// Mirrors org.apache.lucene.search.Multiset (lucene.internal).
type Multiset[T comparable] struct {
	counts map[T]int
	size   int
}

// NewMultiset creates an empty multiset.
func NewMultiset[T comparable]() *Multiset[T] {
	return &Multiset[T]{counts: make(map[T]int)}
}

// Add inserts e into the multiset.
func (m *Multiset[T]) Add(e T) {
	m.counts[e]++
	m.size++
}

// Remove removes a single occurrence of e from the multiset. It returns true
// if an occurrence was found and removed.
func (m *Multiset[T]) Remove(e T) bool {
	c, ok := m.counts[e]
	if !ok {
		return false
	}
	if c <= 1 {
		delete(m.counts, e)
	} else {
		m.counts[e] = c - 1
	}
	m.size--
	return true
}

// Contains reports whether e occurs in the multiset.
func (m *Multiset[T]) Contains(e T) bool {
	_, ok := m.counts[e]
	return ok
}

// Count returns the number of occurrences of e.
func (m *Multiset[T]) Count(e T) int { return m.counts[e] }

// Size returns the total number of elements (with multiplicity).
func (m *Multiset[T]) Size() int { return m.size }

// IsEmpty reports whether the multiset has no elements.
func (m *Multiset[T]) IsEmpty() bool { return m.size == 0 }

// ForEach calls fn for each element occurrence in the multiset.
// Order matches the underlying map iteration order; within a single element
// the fn is invoked Count(e) times back-to-back.
func (m *Multiset[T]) ForEach(fn func(T)) {
	for e, c := range m.counts {
		for i := 0; i < c; i++ {
			fn(e)
		}
	}
}

// Distinct returns the set of distinct elements present in the multiset.
func (m *Multiset[T]) Distinct() []T {
	out := make([]T, 0, len(m.counts))
	for e := range m.counts {
		out = append(out, e)
	}
	return out
}
