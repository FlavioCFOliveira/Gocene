// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

// TopOrdAndNumberQueue is the abstract priority queue that pairs an ordinal
// with a numeric value, ordering entries by value (ascending — the smallest
// values sit at the top so a bounded queue keeps the top-N largest).
// Mirrors org.apache.lucene.facet.TopOrdAndNumberQueue.
type TopOrdAndNumberQueue interface {
	// Insert adds the entry (ord, value) when it would land within the top N
	// slots, returning whether the insertion actually happened. value is the
	// boxed numeric value as float64 — concrete subtypes carry the typed value.
	Insert(ord int, value float64) bool

	// Size returns the number of entries currently held.
	Size() int

	// Capacity returns the maximum number of entries the queue can hold.
	Capacity() int

	// Pop removes and returns the smallest entry.
	Pop() (ord int, value float64, ok bool)

	// Top returns (without removing) the smallest entry.
	Top() (ord int, value float64, ok bool)

	// Clear empties the queue.
	Clear()
}
