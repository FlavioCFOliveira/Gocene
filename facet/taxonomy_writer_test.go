// Copyright 2026 Gocene. All rights reserved.
// Use this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facet

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestTaxonomyWriter_Basic(t *testing.T) {
	dir := store.NewByteDir()
	tw, err := NewTaxonomyWriter(dir, index.OpenModeCreate, nil)
	if err != nil {
		t.Fatalf("failed to create taxonomy writer: %v", err)
	}
	defer tw.Close()

	// Test adding root
	// NewTaxonomyWriter adds the root label by default if nextID is 0
	size := tw.GetSize()
	if size != 1 {
		t.Errorf("expected size 1 (root), got %d", size)
	}

	// Test adding a category
	label1 := NewFacetLabel("category1")
	id1, err := tw.AddCategory(label1)
	if err != nil {
		t.Fatalf("failed to add category: %v", err)
	}
	if id1 == RootOrdinal {
		t.Errorf("category1 should not be root")
	}

	// Test adding existing category
	id1b, err := tw.AddCategory(label1)
	if err != nil {
		t.Fatalf("failed to add existing category: %v", err)
	}
	if id1 != id1b {
		t.Errorf("expected same ID for existing category, got %d and %d", id1, id1b)
	}

	// Test adding sub-category
	label2 := NewFacetLabel("category1", "subcategory1")
	id2, err := tw.AddCategory(label2)
	if err != nil {
		t.Fatalf("failed to add subcategory: %v", err)
	}

	parent, err := tw.GetParent(id2)
	if err != nil {
		t.Fatalf("failed to get parent: %v", err)
	}
	if parent != id1 {
		t.Errorf("expected parent %d, got %d", id1, parent)
	}

	if tw.GetSize() != 3 {
		t.Errorf("expected size 3, got %d", tw.GetSize())
	}
}

func TestTaxonomyWriter_Hierarchy(t *testing.T) {
	dir := store.NewByteDir()
	tw, err := NewTaxonomyWriter(dir, index.OpenModeCreate, nil)
	if err != nil {
		t.Fatalf("failed to create taxonomy writer: %v", err)
	}
	defer tw.Close()

	// Path: a -> b -> c
	idA := func() int {
		id, _ := tw.AddCategory(NewFacetLabel("a"))
		return id
	}()
	idB := func() int {
		id, _ := tw.AddCategory(NewFacetLabel("a", "b"))
		return id
	}()
	idC := func() int {
		id, _ := tw.AddCategory(NewFacetLabel("a", "b", "c"))
		return id
	}()

	pC, _ := tw.GetParent(idC)
	if pC != idB {
		t.Errorf("parent of c should be b, got %d", pC)
	}
	pB, _ := tw.GetParent(idB)
	if pB != idA {
		t.Errorf("parent of b should be a, got %d", pB)
	}
	pA, _ := tw.GetParent(idA)
	if pA != RootOrdinal {
		t.Errorf("parent of a should be root, got %d", pA)
	}
}

func TestTaxonomyWriter_Persistence(t *testing.T) {
	dir := store.NewByteDir()

	// First session
	tw1, _ := NewTaxonomyWriter(dir, index.OpenModeCreate, nil)
	id1, _ := tw1.AddCategory(NewFacetLabel("category1"))
	tw1.Close()

	// Second session - append
	tw2, err := NewTaxonomyWriter(dir, index.OpenModeAppend, nil)
	if err != nil {
		t.Fatalf("failed to open taxonomy writer for append: %v", err)
	}

	id1b, err := tw2.AddCategory(NewFacetLabel("category1"))
	if err != nil {
		t.Fatalf("failed to add existing category in second session: %v", err)
	}
	if id1 != id1b {
		t.Errorf("expected ID %d, got %d", id1, id1b)
	}
	tw2.Close()
}
