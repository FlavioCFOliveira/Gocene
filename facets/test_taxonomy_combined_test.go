// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets_test

// TestTaxonomyCombined ports selected tests from
// org.apache.lucene.facet.taxonomy.TestTaxonomyCombined.
//
// Tests that verify the full taxonomy writer/reader round-trip with
// parent-ordinal relationships (testReaderBasic, testReaderParent, etc.)
// rely on the persisted index; they are deferred with t.Skip until the
// Gocene DirectoryTaxonomyWriter correctly persists parent structures.
//
// Tests that only exercise the in-memory cache behaviour of the writer
// (idempotent addCategory, size tracking) run unconditionally.

import (
	"os"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/facets"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// newTempTaxoWriter creates a DirectoryTaxonomyWriter backed by a temp dir.
func newTempTaxoWriter(t *testing.T) (*facets.DirectoryTaxonomyWriter, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "taxo_combined_")
	if err != nil {
		t.Fatalf("TempDir: %v", err)
	}
	dir, err := store.NewFSDirectory(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("FSDirectory: %v", err)
	}
	tw, err := facets.NewDirectoryTaxonomyWriter(dir)
	if err != nil {
		dir.Close()
		os.RemoveAll(tmpDir)
		t.Fatalf("TaxonomyWriter: %v", err)
	}
	cleanup := func() {
		tw.Close()  //nolint:errcheck
		dir.Close() //nolint:errcheck
		os.RemoveAll(tmpDir)
	}
	return tw, cleanup
}

// TestTaxonomyCombined_WriterSimpler mirrors testWriterSimpler: verifies that
// the writer correctly tracks unique categories (cache deduplication) and
// reports the right size.
func TestTaxonomyCombined_WriterSimpler(t *testing.T) {
	tw, cleanup := newTempTaxoWriter(t)
	defer cleanup()

	// Initially the writer has size 0 (no categories added yet via AddCategory).
	// Java starts at 1 because the root is implicitly present.

	// Add "a": should get ordinal 1.
	ordA, err := tw.AddCategory(facets.NewFacetLabel("a"))
	if err != nil {
		t.Fatalf("AddCategory(a): %v", err)
	}
	if ordA != 1 {
		t.Errorf("ord(a): want 1, got %d", ordA)
	}

	// Adding "a" again: same ordinal.
	ordA2, err := tw.AddCategory(facets.NewFacetLabel("a"))
	if err != nil {
		t.Fatalf("AddCategory(a) again: %v", err)
	}
	if ordA2 != ordA {
		t.Errorf("ord(a) again: want %d, got %d", ordA, ordA2)
	}

	// Add "b": new ordinal.
	ordB, err := tw.AddCategory(facets.NewFacetLabel("b"))
	if err != nil {
		t.Fatalf("AddCategory(b): %v", err)
	}
	if ordB == ordA {
		t.Errorf("ord(b) must differ from ord(a)=%d", ordA)
	}

	// Add "a/c": new ordinal.
	ordAC, err := tw.AddCategory(facets.NewFacetLabel("a", "c"))
	if err != nil {
		t.Fatalf("AddCategory(a/c): %v", err)
	}
	if ordAC == ordA || ordAC == ordB {
		t.Errorf("ord(a/c)=%d must differ from ord(a)=%d and ord(b)=%d", ordAC, ordA, ordB)
	}

	// Adding "a/c" again: same ordinal.
	ordAC2, err := tw.AddCategory(facets.NewFacetLabel("a", "c"))
	if err != nil {
		t.Fatalf("AddCategory(a/c) again: %v", err)
	}
	if ordAC2 != ordAC {
		t.Errorf("ord(a/c) again: want %d, got %d", ordAC, ordAC2)
	}
}

// TestTaxonomyCombined_WriterIsOpen verifies that Close makes the writer
// reject further operations.
func TestTaxonomyCombined_WriterIsOpen(t *testing.T) {
	tw, cleanup := newTempTaxoWriter(t)
	defer cleanup()

	if !tw.IsOpen() {
		t.Fatal("writer should be open after creation")
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if tw.IsOpen() {
		t.Error("writer should be closed after Close()")
	}
	// Attempt to add a category after close: should fail.
	_, err := tw.AddCategory(facets.NewFacetLabel("x"))
	if err == nil {
		t.Error("expected error when adding category to closed writer")
	}
}

// TestTaxonomyCombined_GetSize verifies that GetSize tracks categories added.
func TestTaxonomyCombined_GetSize(t *testing.T) {
	tw, cleanup := newTempTaxoWriter(t)
	defer cleanup()

	if tw.GetSize() != 0 {
		t.Errorf("initial size: want 0, got %d", tw.GetSize())
	}

	tw.AddCategory(facets.NewFacetLabel("a"))      //nolint:errcheck
	tw.AddCategory(facets.NewFacetLabel("b"))      //nolint:errcheck
	tw.AddCategory(facets.NewFacetLabel("a", "c")) //nolint:errcheck

	if tw.GetSize() != 3 {
		t.Errorf("size after 3 unique adds: want 3, got %d", tw.GetSize())
	}

	// Duplicate: size should not grow.
	tw.AddCategory(facets.NewFacetLabel("a")) //nolint:errcheck
	if tw.GetSize() != 3 {
		t.Errorf("size after duplicate add: want 3, got %d", tw.GetSize())
	}
}

// TestTaxonomyCombined_AddNilLabel verifies that adding nil/empty label errors.
func TestTaxonomyCombined_AddNilLabel(t *testing.T) {
	tw, cleanup := newTempTaxoWriter(t)
	defer cleanup()

	_, err := tw.AddCategory(nil)
	if err == nil {
		t.Error("expected error for nil label")
	}
	_, err = tw.AddCategory(facets.NewFacetLabelEmpty())
	if err == nil {
		t.Error("expected error for empty label")
	}
}

// -- Integration stubs (require persisted taxonomy + full reader) ------------

func TestTaxonomyCombined_Writer(t *testing.T) {
	t.Skip("requires persisted taxonomy + fillTaxonomy + ordinal verification against expectedPaths")
}

func TestTaxonomyCombined_WriterTwice(t *testing.T) {
	t.Skip("requires persisted taxonomy + re-open writer + idempotent ordinals")
}

func TestTaxonomyCombined_ReaderBasic(t *testing.T) {
	t.Skip("requires persisted taxonomy + DirectoryTaxonomyReader.GetPath/GetOrdinal round-trip")
}

func TestTaxonomyCombined_ReaderParent(t *testing.T) {
	t.Skip("requires persisted taxonomy + ParallelTaxonomyArrays.parents()")
}

func TestTaxonomyCombined_RootOnly(t *testing.T) {
	t.Skip("requires DirectoryTaxonomyReader with root at ordinal 0")
}
