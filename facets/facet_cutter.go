// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/facets/iterators"
)

// FacetCutter creates LeafFacetCutter for each leaf.
// Mirrors org.apache.lucene.sandbox.facet.cutters.FacetCutter.
type FacetCutter interface {
	// CreateLeafCutter gets cutter for the leaf.
	CreateLeafCutter(context *index.LeafReaderContext) (LeafFacetCutter, error)

	// GetOrdinalsToRollup returns all top level dimension ordinals that require rollup.
	// Returns nil if rollup is not needed.
	GetOrdinalsToRollup() (iterators.OrdinalIterator, error)

	// GetChildrenOrds returns all children ordinals for given ord.
	GetChildrenOrds(ord int) (iterators.OrdinalIterator, error)
}
