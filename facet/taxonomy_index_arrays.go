// Copyright 2026 Gocene. All rights reserved.
// Use this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facet

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
)

const (
	RootOrdinal    = 0
	InvalidOrdinal = -1
)

// TaxonomyIndexArrays is a ParallelTaxonomyArrays initialized from the taxonomy index.
type TaxonomyIndexArrays struct {
	mu                  sync.Mutex
	parents             *ChunkedIntArray
	children            *ChunkedIntArray
	siblings            *ChunkedIntArray
	initializedChildren bool
}

func NewTaxonomyIndexArrays(reader *index.IndexReader) error {
	// This is a constructor that should be used internally by TaxonomyWriter.
	// For now, we'll implement the logic inside TaxonomyWriter.
	return nil
}

func newTaxonomyIndexArrays(reader *index.IndexReader) (*TaxonomyIndexArrays, error) {
	maxDoc := reader.MaxDoc()
	parents := NewChunkedIntArray(maxDoc)

	for _, leaf := range reader.Leaves() {
		docBase := leaf.DocBase
		parentValues := leaf.GetNumericDocValues("parent_ordinal") // Consts.FIELD_PARENT_ORDINAL_NDV
		if parentValues == nil {
			return nil, fmt.Errorf("parent data field not found in leaf %d", leaf.ID)
		}

		for doc := 0; doc < leaf.MaxDoc(); doc++ {
			val := parentValues.Get(doc)
			pos := doc + docBase
			if pos < parents.Length() {
				parents.Set(pos, int(val))
			}
		}
	}

	// Root ordinal is always invalid parent
	if parents.Length() > 0 {
		parents.Set(RootOrdinal, InvalidOrdinal)
	}

	return &TaxonomyIndexArrays{
		parents: parents,
	}, nil
}

func (t *TaxonomyIndexArrays) Parents() IntArray {
	return t.parents
}

func (t *TaxonomyIndexArrays) Children() IntArray {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.initializedChildren {
		t.computeChildrenSiblings()
	}
	return t.children
}

func (t *TaxonomyIndexArrays) Siblings() IntArray {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.initializedChildren {
		t.computeChildrenSiblings()
	}
	return t.siblings
}

func (t *TaxonomyIndexArrays) computeChildrenSiblings() {
	length := t.parents.Length()
	t.children = NewChunkedIntArray(length)
	t.siblings = NewChunkedIntArray(length)

	for i := 0; i < length; i++ {
		t.children.Set(i, InvalidOrdinal)
	}

	if length > 0 {
		t.siblings.Set(RootOrdinal, InvalidOrdinal)
	}

	for i := 1; i < length; i++ {
		parent := t.parents.Get(i)
		t.siblings.Set(i, t.children.Get(parent))
		t.children.Set(parent, i)
	}
	t.initializedChildren = true
}

func (t *TaxonomyIndexArrays) Add(ordinal, parentOrdinal int) *TaxonomyIndexArrays {
	if ordinal >= t.parents.Length() {
		// Grow the array
		t.parents.grow(ordinal + 1)
	}
	t.parents.Set(ordinal, parentOrdinal)
	// If children were already initialized, we might need to update them.
	// But in TaxonomyWriter, we usually add categories before computing children.
	return t
}
