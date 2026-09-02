// Copyright 2026 Gocene. All rights reserved.
// Use this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package taxonomy

// ParallelTaxonomyArrays returns 3 arrays for traversing the taxonomy:
// - parents: parents[i] denotes the parent of category ordinal i.
// - children: children[i] denotes a child of category ordinal i.
// - siblings: siblings[i] denotes the sibling of category ordinal i.
type ParallelTaxonomyArrays interface {
	Parents() IntArray
	Children() IntArray
	Siblings() IntArray
}

// IntArray is an abstraction that looks like an int[], but read-only.
type IntArray interface {
	Get(i int) int
	Length() int
}
