// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// FieldComparatorSource provides a FieldComparator for custom field sorting.
//
// Mirrors org.apache.lucene.search.FieldComparatorSource.
type FieldComparatorSource interface {
	// NewComparator creates a comparator for the field in the given index.
	NewComparator(fieldname string, numHits int, pruning Pruning, reversed bool) FieldComparator
}
