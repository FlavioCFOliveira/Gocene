// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package directory

import (
	"sync"

	"github.com/FlavioCFOliveira/Gocene/facets/taxonomy"
)

const (
	// chunkSizeBits and ChunkSize mirror the Java constants that control the
	// size of each page in the chunked parent array.
	chunkSizeBits = 13
	// ChunkSize is the number of ints in each page.
	ChunkSize = 1 << chunkSizeBits
)

// InvalidOrdinal is the sentinel ordinal value, mirroring TaxonomyReader.INVALID_ORDINAL.
const InvalidOrdinal = -1

// TaxonomyIndexArrays is the ParallelTaxonomyArrays implementation used by the
// directory taxonomy reader. It stores parents eagerly and lazily computes the
// children/siblings arrays on first access. Mirrors
// org.apache.lucene.facet.taxonomy.directory.TaxonomyIndexArrays.
type TaxonomyIndexArrays struct {
	parents []int

	mu                  sync.Mutex
	initializedChildren bool
	children            []int
	siblings            []int
}

// NewTaxonomyIndexArraysFromParents constructs the arrays directly from a
// pre-built parents slice (used in tests and incremental updates).
func NewTaxonomyIndexArraysFromParents(parents []int) *TaxonomyIndexArrays {
	p := make([]int, len(parents))
	copy(p, parents)
	return &TaxonomyIndexArrays{parents: p}
}

// Add grows the parents slice by one, recording newParentOrdinal as the
// parent of the new leaf. Mirrors TaxonomyIndexArrays.add.
func (t *TaxonomyIndexArrays) Add(newOrdinal, parentOrdinal int) {
	if newOrdinal < len(t.parents) {
		t.parents[newOrdinal] = parentOrdinal
		return
	}
	// Grow.
	grown := make([]int, newOrdinal+1)
	copy(grown, t.parents)
	grown[newOrdinal] = parentOrdinal
	t.parents = grown

	t.mu.Lock()
	t.initializedChildren = false // invalidate lazy arrays
	t.children = nil
	t.siblings = nil
	t.mu.Unlock()
}

// Parents implements ParallelTaxonomyArrays.
func (t *TaxonomyIndexArrays) Parents() []int {
	out := make([]int, len(t.parents))
	copy(out, t.parents)
	return out
}

// Children implements ParallelTaxonomyArrays. The array is computed lazily
// on the first call and cached.
func (t *TaxonomyIndexArrays) Children() []int {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.initializedChildren {
		t.computeChildrenSiblings()
		t.initializedChildren = true
	}
	out := make([]int, len(t.children))
	copy(out, t.children)
	return out
}

// Siblings implements ParallelTaxonomyArrays. Computed together with Children.
func (t *TaxonomyIndexArrays) Siblings() []int {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.initializedChildren {
		t.computeChildrenSiblings()
		t.initializedChildren = true
	}
	out := make([]int, len(t.siblings))
	copy(out, t.siblings)
	return out
}

// computeChildrenSiblings fills the children and siblings slices from parents.
// Must be called with t.mu held. Mirrors computeChildrenSiblings in the Java.
func (t *TaxonomyIndexArrays) computeChildrenSiblings() {
	n := len(t.parents)
	ch := make([]int, n)
	sib := make([]int, n)
	for i := range ch {
		ch[i] = InvalidOrdinal
		sib[i] = InvalidOrdinal
	}
	// Root has no siblings.
	// Process in ordinal order; parents[i] < i always holds.
	for i := 1; i < n; i++ {
		p := t.parents[i]
		sib[i] = ch[p] // existing youngest child of p becomes older sibling of i
		ch[p] = i      // i becomes the youngest child of p
	}
	t.children = ch
	t.siblings = sib
}

// Size returns the number of entries (ordinals) tracked.
func (t *TaxonomyIndexArrays) Size() int { return len(t.parents) }

var _ taxonomy.ParallelTaxonomyArrays = (*TaxonomyIndexArrays)(nil)
