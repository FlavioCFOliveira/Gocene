// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

// CollectedSearchGroup extends SearchGroup with the bookkeeping the
// first-pass grouping collector keeps internally — the position of the most
// recently seen document in the segment and the order in which the group
// joined the priority queue. Mirrors
// org.apache.lucene.search.grouping.CollectedSearchGroup.
type CollectedSearchGroup[T any] struct {
	*SearchGroup[T]

	// TopDoc is the docID of the most recent hit observed for this group.
	TopDoc int

	// ComparatorSlot is the slot the FieldComparator owns for this group.
	ComparatorSlot int
}

// NewCollectedSearchGroup builds the collector-side view of a SearchGroup.
func NewCollectedSearchGroup[T any](value T, sortValues []any, topDoc, comparatorSlot int) *CollectedSearchGroup[T] {
	return &CollectedSearchGroup[T]{
		SearchGroup:    NewSearchGroup(value, sortValues),
		TopDoc:         topDoc,
		ComparatorSlot: comparatorSlot,
	}
}
