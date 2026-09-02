// Copyright 2026 Gocene. All rights reserved.
// Use this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package taxonomy

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/facet"
	"github.com/FlavioCFOliveira/Gocene/facet/taxonomy/writercache"
	"github.com/FlavioCFOliveira/Gocene/internal/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestDirectoryTaxonomyWriter_Basic(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	dtw, err := NewDirectoryTaxonomyWriter(dir, index.OpenModeCreate, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer dtw.Close()

	// Root category should be at 0
	if dtw.GetSize() != 1 {
		t.Errorf("expected size 1, got %d", dtw.GetSize())
	}

	// Add category
	cat1 := facet.NewFacetLabel("Electronics")
	ord1, err := dtw.AddCategory(cat1)
	if err != nil {
		t.Fatal(err)
	}
	if ord1 != 1 {
		t.Errorf("expected ordinal 1, got %d", ord1)
	}

	// Add sub-category
	cat2 := facet.NewFacetLabel("Electronics", "Laptops")
	ord2, err := dtw.AddCategory(cat2)
	if err != nil {
		t.Fatal(err)
	}
	if ord2 != 2 {
		t.Errorf("expected ordinal 2, got %d", ord2)
	}

	// Verify parent
	parent, err := dtw.GetParent(ord2)
	if err != nil {
		t.Fatal(err)
	}
	if parent != ord1 {
		t.Errorf("expected parent %d, got %d", ord1, parent)
	}

	// Verify size
	if dtw.GetSize() != 3 {
		t.Errorf("expected size 3, got %d", dtw.GetSize())
	}
}

func TestDirectoryTaxonomyWriter_Duplicates(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	dtw, err := NewDirectoryTaxonomyWriter(dir, index.OpenModeCreate, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer dtw.Close()

	cat := facet.NewFacetLabel("Books")
	ord1, _ := dtw.AddCategory(cat)
	ord2, _ := dtw.AddCategory(cat)

	if ord1 != ord2 {
		t.Errorf("expected same ordinal for same category, got %d and %d", ord1, ord2)
	}
}

func TestDirectoryTaxonomyWriter_NestedAutoCreation(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	dtw, err := NewDirectoryTaxonomyWriter(dir, index.OpenModeCreate, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer dtw.Close()

	// Add deep category directly
	cat := facet.NewFacetLabel("A", "B", "C")
	ord, err := dtw.AddCategory(cat)
	if err != nil {
		t.Fatal(err)
	}

	// Size should be 4 (Root, A, B, C)
	if dtw.GetSize() != 4 {
		t.Errorf("expected size 4, got %d", dtw.GetSize())
	}

	// Verify chain
	pB, _ := dtw.GetParent(ord)
	pA, _ := dtw.GetParent(pB)
	pRoot, _ := dtw.GetParent(pA)

	if pRoot != 0 {
		t.Errorf("expected root parent 0, got %d", pRoot)
	}
}
