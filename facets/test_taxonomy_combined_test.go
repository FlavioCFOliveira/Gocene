// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets_test

// TestTaxonomyCombined ports selected tests from
// org.apache.lucene.facet.taxonomy.TestTaxonomyCombined.
//
// Tests that verify the full taxonomy writer/reader round-trip with
// parent-ordinal relationships (testReaderBasic, testReaderParent, etc.)
// rely on the persisted index via DocValues; they are deferred with t.Skip
// until the SegmentReader core-readers gap is resolved (BinaryDocValues not
// yet readable from disk — see memory ref 'gocene-segmentreader-corereaders-gap').
//
// Tests that only exercise the in-memory state of the writer or the NRT
// writer→reader path run unconditionally.

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/facets"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// newTempTaxoWriter creates a DirectoryTaxonomyWriter backed by an in-memory
// ByteBuffersDirectory. ByteBuffersDirectory avoids the CreateOutput stub
// limitation of the base FSDirectory type.
func newTempTaxoWriter(t *testing.T) (*facets.DirectoryTaxonomyWriter, func()) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	tw, err := facets.NewDirectoryTaxonomyWriter(dir)
	if err != nil {
		t.Fatalf("TaxonomyWriter: %v", err)
	}
	cleanup := func() {
		tw.Close() //nolint:errcheck
	}
	return tw, cleanup
}

// TestTaxonomyCombined_WriterSimpler mirrors testWriterSimpler: verifies that
// the writer correctly tracks unique categories (cache deduplication) and
// reports the right size.
func TestTaxonomyCombined_WriterSimpler(t *testing.T) {
	tw, cleanup := newTempTaxoWriter(t)
	defer cleanup()

	// Add "a": root(0) already exists; "a" gets ordinal 1.
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

	// Add "a/c": "a" is already present; "a/c" gets a new ordinal.
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

// TestTaxonomyCombined_GetSize verifies that GetSize tracks total categories
// including the root that is always at ordinal 0.
func TestTaxonomyCombined_GetSize(t *testing.T) {
	tw, cleanup := newTempTaxoWriter(t)
	defer cleanup()

	// After construction: root at ordinal 0, GetSize() == 1.
	if tw.GetSize() != 1 {
		t.Errorf("initial size: want 1 (root), got %d", tw.GetSize())
	}

	tw.AddCategory(facets.NewFacetLabel("a"))      //nolint:errcheck
	tw.AddCategory(facets.NewFacetLabel("b"))      //nolint:errcheck
	tw.AddCategory(facets.NewFacetLabel("a", "c")) //nolint:errcheck

	// root(0) + a(1) + b(2) + a/c(3) = 4.
	if tw.GetSize() != 4 {
		t.Errorf("size after 3 unique user adds: want 4 (root+a+b+a/c), got %d", tw.GetSize())
	}

	// Duplicate: size should not grow.
	tw.AddCategory(facets.NewFacetLabel("a")) //nolint:errcheck
	if tw.GetSize() != 4 {
		t.Errorf("size after duplicate add: want 4, got %d", tw.GetSize())
	}
}

// TestTaxonomyCombined_AddNilLabel verifies that adding nil label errors, and
// that an empty label (root) returns ordinal 0 without error (Lucene semantics).
func TestTaxonomyCombined_AddNilLabel(t *testing.T) {
	tw, cleanup := newTempTaxoWriter(t)
	defer cleanup()

	_, err := tw.AddCategory(nil)
	if err == nil {
		t.Error("expected error for nil label")
	}

	// Empty label = root; Lucene returns its ordinal (0) rather than an error.
	ordRoot, err := tw.AddCategory(facets.NewFacetLabelEmpty())
	if err != nil {
		t.Errorf("expected no error for empty label (root), got %v", err)
	}
	if ordRoot != 0 {
		t.Errorf("expected root ordinal 0, got %d", ordRoot)
	}
}

// TestTaxonomyCombined_ReaderBasicNRT verifies the NRT writer→reader path.
// This test does not require DocValues disk reads (avoids the SegmentReader
// core-readers gap).
func TestTaxonomyCombined_ReaderBasicNRT(t *testing.T) {
	tw, cleanup := newTempTaxoWriter(t)
	defer cleanup()

	// Populate taxonomy.
	tw.AddCategory(facets.NewFacetLabel("a"))      //nolint:errcheck
	tw.AddCategory(facets.NewFacetLabel("a", "b")) //nolint:errcheck
	tw.AddCategory(facets.NewFacetLabel("c"))      //nolint:errcheck

	// NRT reader from writer's in-memory state.
	tr, err := facets.NewDirectoryTaxonomyReaderFromWriter(tw)
	if err != nil {
		t.Fatalf("NewDirectoryTaxonomyReaderFromWriter: %v", err)
	}
	defer tr.Close() //nolint:errcheck

	// root(0) + a(1) + a/b(2) + c(3) = 4.
	if tr.GetSize() != 4 {
		t.Errorf("GetSize: want 4, got %d", tr.GetSize())
	}

	// Ordinal lookups.
	if ord := tr.GetOrdinal(facets.NewFacetLabel("a")); ord != 1 {
		t.Errorf("GetOrdinal(a): want 1, got %d", ord)
	}
	if ord := tr.GetOrdinal(facets.NewFacetLabel("a", "b")); ord != 2 {
		t.Errorf("GetOrdinal(a/b): want 2, got %d", ord)
	}
	if ord := tr.GetOrdinal(facets.NewFacetLabel("c")); ord != 3 {
		t.Errorf("GetOrdinal(c): want 3, got %d", ord)
	}

	// Non-existent category.
	if ord := tr.GetOrdinal(facets.NewFacetLabel("z")); ord != -1 {
		t.Errorf("GetOrdinal(z): want -1, got %d", ord)
	}

	// Parent verification: a/b's parent is a(1); a's parent is root(0); c's parent is root(0).
	if p := tr.GetParent(2); p != 1 {
		t.Errorf("GetParent(a/b): want 1 (a), got %d", p)
	}
	if p := tr.GetParent(1); p != 0 {
		t.Errorf("GetParent(a): want 0 (root), got %d", p)
	}
	if p := tr.GetParent(3); p != 0 {
		t.Errorf("GetParent(c): want 0 (root), got %d", p)
	}
}

// -- Integration stubs (require the DirectoryTaxonomy persistence pipeline) ----
// The on-disk DocValues read path is wired (rmp #4771), so the original
// "BinaryDocValues not readable from disk" blocker no longer applies. What
// remains is the DirectoryTaxonomyWriter persist + DirectoryTaxonomyReader
// cold-open pipeline, tracked in rmp #4774.

func TestTaxonomyCombined_Writer(t *testing.T) {
	t.Skip("requires persisted taxonomy + fillTaxonomy + ordinal verification against expectedPaths (rmp #4774)")
}

func TestTaxonomyCombined_WriterTwice(t *testing.T) {
	t.Skip("requires persisted taxonomy + re-open writer + idempotent ordinals (rmp #4774)")
}

func TestTaxonomyCombined_ReaderBasic(t *testing.T) {
	t.Skip("requires cold DirectoryTaxonomyReader.GetPath/GetOrdinal from disk (rmp #4774)")
}

func TestTaxonomyCombined_ReaderParent(t *testing.T) {
	t.Skip("requires persisted taxonomy + ParallelTaxonomyArrays.parents() (rmp #4774)")
}

func TestTaxonomyCombined_RootOnly(t *testing.T) {
	t.Skip("requires cold DirectoryTaxonomyReader with root at ordinal 0 from disk (rmp #4774)")
}
