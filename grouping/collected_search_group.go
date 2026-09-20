// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

// CollectedSearchGroup is the expert representation of a group in
// FirstPassGroupingCollector, tracking the top doc and the
// search.FieldComparator slot.
//
// Mirrors org.apache.lucene.search.grouping.CollectedSearchGroup<T>, which
// extends SearchGroup<T> and adds two package-private fields.
//
// lucene.internal
type CollectedSearchGroup[T any] struct {
	SearchGroup[T]

	// topDoc mirrors the package-private field int topDoc.
	topDoc int

	// comparatorSlot mirrors the package-private field int comparatorSlot.
	comparatorSlot int
}
