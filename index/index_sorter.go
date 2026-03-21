// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// IndexSorter sorts documents during flush and merge operations.
//
// This is the Go port of Lucene's org.apache.lucene.index.IndexSorter.
//
// IndexSorter is responsible for sorting documents based on a configured
// Sort when flushing or merging segments. This enables creating sorted
// indexes where documents are stored in a specific order.
type IndexSorter struct {
	sort *Sort
}

// NewIndexSorter creates a new IndexSorter with the given sort.
func NewIndexSorter(sort *Sort) *IndexSorter {
	return &IndexSorter{
		sort: sort,
	}
}

// GetSort returns the sort specification.
func (s *IndexSorter) GetSort() *Sort {
	return s.sort
}

// SetSort sets the sort specification.
func (s *IndexSorter) SetSort(sort *Sort) {
	s.sort = sort
}

// SortSegment sorts the documents in a segment according to the sort specification.
// Returns a mapping from old doc IDs to new doc IDs.
func (s *IndexSorter) SortSegment(reader *LeafReader) ([]int, error) {
	if s.sort == nil || len(s.sort.fields) == 0 {
		numDocs := reader.MaxDoc()
		mapping := make([]int, numDocs)
		for i := 0; i < numDocs; i++ {
			mapping[i] = i
		}
		return mapping, nil
	}

	// TODO: Implement actual sorting based on the sort fields
	// For now, return identity mapping
	numDocs := reader.MaxDoc()
	mapping := make([]int, numDocs)
	for i := 0; i < numDocs; i++ {
		mapping[i] = i
	}
	return mapping, nil
}

// NeedsSorting returns true if the segment needs to be sorted.
func (s *IndexSorter) NeedsSorting(reader *LeafReader) bool {
	if s.sort == nil || len(s.sort.fields) == 0 {
		return false
	}
	return true
}

// SortType returns the name of this sorter.
func (s *IndexSorter) SortType() string {
	if s.sort == nil {
		return "none"
	}
	return "custom"
}
