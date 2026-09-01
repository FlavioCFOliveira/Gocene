// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/facets/iterators"
)

// LeafFacetCutter is an interface to be implemented to cut documents into facets for an index segment (leaf).
// Mirrors org.apache.lucene.sandbox.facet.cutters.LeafFacetCutter.
type LeafFacetCutter interface {
	iterators.OrdinalIterator

	// AdvanceExact advances to the next doc.
	AdvanceExact(doc int) (bool, error)
}
