// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"math/bits"
)

// ApproximatePriorityQueue is an approximate priority queue, which attempts to poll items
// by decreasing log of the weight, though exact ordering is not guaranteed.
// This class doesn't support null elements (handled by Go's zero values/generics).
type ApproximatePriorityQueue[T any] struct {
	// slots between 0 and 63 are sparsely populated, and indexes that are
	// greater than or equal to 64 are densely populated.
	// Items close to the beginning of this list are more likely to have a
	// higher weight.
	slots []T

	// usedSlots is a bitset where ones indicate that the corresponding index in `slots` is taken.
	usedSlots uint64
}

// NewApproximatePriorityQueue creates a new ApproximatePriorityQueue.
func NewApproximatePriorityQueue[T any]() *ApproximatePriorityQueue[T] {
	return &ApproximatePriorityQueue[T]{
		slots: make([]T, 64),
	}
}

// Add an entry to this queue that has the provided weight.
func (q *ApproximatePriorityQueue[T]) Add(entry T, weight int64) {
	// The expected slot of an item is the number of leading zeros of its weight,
	// ie. the larger the weight, the closer an item is to the start of the array.
	expectedSlot := bits.LeadingZeros64(uint64(weight))

	// If the slot is already taken, we look for the next one that is free.
	// The above bitwise operation is equivalent to looping over slots until finding one that is
	// free.
	freeSlots := ^q.usedSlots
	destinationSlot := expectedSlot + bits.TrailingZeros64(freeSlots>>expectedSlot)

	if destinationSlot < 64 {
		q.usedSlots |= 1 << destinationSlot
		q.slots[destinationSlot] = entry
	} else {
		q.slots = append(q.slots, entry)
	}
}

// Poll return an entry matching the predicate. This will usually be one of the available entries that
// have the highest weight, though this is not guaranteed. This method returns the zero value of T
// and false if no free entries are available.
func (q *ApproximatePriorityQueue[T]) Poll(predicate func(T) bool) (T, bool) {
	var zero T
	// Look at indexes 0..63 first, which are sparsely populated.
	nextSlot := 0
	for {
		if nextSlot >= 64 {
			break
		}
		nextUsedSlot := nextSlot + bits.TrailingZeros64(q.usedSlots>>nextSlot)
		if nextUsedSlot >= 64 {
			break
		}
		entry := q.slots[nextUsedSlot]
		if predicate(entry) {
			q.usedSlots &= ^(1 << nextUsedSlot)
			q.slots[nextUsedSlot] = zero
			return entry, true
		} else {
			nextSlot = nextUsedSlot + 1
		}
	}

	// Then look at indexes 64.. which are densely populated.
	// Poll in descending order so that if the number of indexing threads
	// decreases, we keep using the same entry over and over again.
	// Resizing operations are also less costly on slices when items are closer
	// to the end of the slice.
	for i := len(q.slots) - 1; i >= 64; i-- {
		entry := q.slots[i]
		if predicate(entry) {
			// In Go, removing an element from a slice while maintaining order is costly.
			// To mirror the Java List.remove(index) behaviour exactly, we would need to shift.
			// However, for the densely populated part, we can just shift.
			q.slots = append(q.slots[:i], q.slots[i+1:]...)
			return entry, true
		}
	}

	return zero, false
}

// IsEmpty returns true if the queue is empty.
func (q *ApproximatePriorityQueue[T]) IsEmpty() bool {
	return q.usedSlots == 0 && len(q.slots) == 64
}

// Remove removes the entry from the queue. Returns true if the entry was removed.
func (q *ApproximatePriorityQueue[T]) Remove(entry T, equals func(T, T) bool) bool {
	var zero T
	for i, v := range q.slots {
		if equals(v, entry) {
			if i < 64 {
				q.usedSlots &= ^(1 << i)
				q.slots[i] = zero
			} else {
				q.slots = append(q.slots[:i], q.slots[i+1:]...)
			}
			return true
		}
	}
	return false
}
