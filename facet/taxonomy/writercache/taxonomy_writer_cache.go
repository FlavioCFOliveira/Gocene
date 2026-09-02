// Copyright 2026 Gocene. All rights reserved.
// Use this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package writercache

import "github.com/FlavioCFOliveira/Gocene/facet"

// TaxonomyWriterCache is an interface for a cache of category-to-ordinal mappings.
type TaxonomyWriterCache interface {
	Close()
	Get(categoryPath *facet.FacetLabel) int
	Put(categoryPath *facet.FacetLabel, ordinal int) bool
	IsFull() bool
	Clear()
	Size() int
}
