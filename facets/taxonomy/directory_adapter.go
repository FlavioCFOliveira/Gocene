// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package taxonomy

import (
	"github.com/FlavioCFOliveira/Gocene/facets"
)

// DirectoryTaxonomyReaderAdapter wraps a facets.DirectoryTaxonomyReader as a
// taxonomy.TaxonomyReaderI, bridging the two reader hierarchies so that
// FastTaxonomyFacetCounts and other TaxonomyFacets-based aggregators can
// consume a directory-backed taxonomy reader.
//
// The adapter lazily builds ParallelTaxonomyArrays (children + siblings) from
// the writer's parent-ordinal data on the first call to
// GetParallelTaxonomyArrays.
type DirectoryTaxonomyReaderAdapter struct {
	reader *facets.DirectoryTaxonomyReader
	arrays ParallelTaxonomyArrays
}

// NewDirectoryTaxonomyReaderAdapter creates an adapter around the supplied
// DirectoryTaxonomyReader. The caller must ensure the reader outlives the
// adapter.
func NewDirectoryTaxonomyReaderAdapter(r *facets.DirectoryTaxonomyReader) *DirectoryTaxonomyReaderAdapter {
	return &DirectoryTaxonomyReaderAdapter{reader: r}
}

// GetSize returns the number of categories (including the root).
func (a *DirectoryTaxonomyReaderAdapter) GetSize() int {
	return a.reader.GetSize()
}

// GetParallelTaxonomyArrays returns the parallel arrays for taxonomy traversal.
// The arrays are built lazily from the reader's parent-ordinal data using the
// same algorithm as TaxonomyIndexArrays.computeChildrenSiblings.
func (a *DirectoryTaxonomyReaderAdapter) GetParallelTaxonomyArrays() ParallelTaxonomyArrays {
	if a.arrays == nil {
		parents := a.reader.GetParentOrdinals()
		n := len(parents)
		ch := make([]int, n)
		sib := make([]int, n)
		for i := range ch {
			ch[i] = InvalidOrdinal
			sib[i] = InvalidOrdinal
		}
		// Build children/siblings from parents.
		for i := 1; i < n; i++ {
			p := parents[i]
			sib[i] = ch[p] // existing youngest child of p becomes older sibling of i
			ch[p] = i      // i becomes the youngest child of p
		}
		a.arrays = NewInMemoryParallelTaxonomyArrays(parents, ch, sib)
	}
	return a.arrays
}

// GetOrdinal returns the ordinal for the given path components, or
// InvalidOrdinal when not found. path[0] is the dimension, path[1..] are
// optional sub-path components.
func (a *DirectoryTaxonomyReaderAdapter) GetOrdinal(path ...string) int {
	return a.reader.GetOrdinalFromPath(path...)
}

// GetPath returns the path components for the given ordinal, or nil when not
// found.
func (a *DirectoryTaxonomyReaderAdapter) GetPath(ord int) []string {
	return a.reader.GetPathComponents(ord)
}
