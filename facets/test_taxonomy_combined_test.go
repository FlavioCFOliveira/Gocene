// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets_test

// TestTaxonomyCombined ports selected tests from
// org.apache.lucene.facet.taxonomy.TestTaxonomyCombined.
//
// The full writer→disk→cold-reader round-trip (testWriter, testWriter2,
// testReaderBasic, testReaderParent, testRootOnly) is exercised against a
// persisted taxonomy: DirectoryTaxonomyWriter writes each category as a
// term + BinaryDocValues path + NumericDocValues parent ordinal, and
// DirectoryTaxonomyReader cold-opens the committed index and reads the
// ordinals/paths/parents back. The on-disk DocValues read path landed in
// rmp #4771; the persist + cold-open pipeline landed in rmp #4774.

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/facets"
	"github.com/FlavioCFOliveira/Gocene/store"

	// Blank-import the codec registry so the taxonomy IndexWriter has a default
	// codec able to persist the taxonomy segment (term + BinaryDocValues +
	// NumericDocValues) to disk for cold-open.
	_ "github.com/FlavioCFOliveira/Gocene/codecs"
	_ "github.com/FlavioCFOliveira/Gocene/codecs/lucene90/compressing"
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

// -- Integration tests (DirectoryTaxonomy persist + cold-open pipeline) --------
// The on-disk DocValues read path landed in rmp #4771; the persist +
// DirectoryTaxonomyReader cold-open pipeline landed in rmp #4774. These tests
// port the TestTaxonomyCombined fixtures from Lucene and exercise the full
// writer→disk→cold-reader round-trip.

// taxoCategories mirrors TestTaxonomyCombined.categories: the user-added
// categories, in addition order.
var taxoCategories = [][]string{
	{"Author", "Tom Clancy"},
	{"Author", "Richard Dawkins"},
	{"Author", "Richard Adams"},
	{"Price", "10", "11"},
	{"Price", "10", "12"},
	{"Price", "20", "27"},
	{"Date", "2006", "05"},
	{"Date", "2005"},
	{"Date", "2006"},
	{"Subject", "Nonfiction", "Children", "Animals"},
	{"Author", "Stephen Jay Gould"},
	{"Author", "נדבあب"},
}

// taxoExpectedPaths mirrors TestTaxonomyCombined.expectedPaths: the ordinal of
// each added category is the last element of the corresponding row.
var taxoExpectedPaths = [][]int{
	{1, 2},
	{1, 3},
	{1, 4},
	{5, 6, 7},
	{5, 6, 8},
	{5, 9, 10},
	{11, 12, 13},
	{11, 14},
	{11, 12},
	{15, 16, 17, 18},
	{1, 19},
	{1, 20},
}

// taxoExpectedCategories mirrors TestTaxonomyCombined.expectedCategories: every
// category the taxonomy index is expected to contain, in increasing ordinal
// order (parents are added automatically). Index 0 is the root.
var taxoExpectedCategories = [][]string{
	{}, // the root category
	{"Author"},
	{"Author", "Tom Clancy"},
	{"Author", "Richard Dawkins"},
	{"Author", "Richard Adams"},
	{"Price"},
	{"Price", "10"},
	{"Price", "10", "11"},
	{"Price", "10", "12"},
	{"Price", "20"},
	{"Price", "20", "27"},
	{"Date"},
	{"Date", "2006"},
	{"Date", "2006", "05"},
	{"Date", "2005"},
	{"Subject"},
	{"Subject", "Nonfiction"},
	{"Subject", "Nonfiction", "Children"},
	{"Subject", "Nonfiction", "Children", "Animals"},
	{"Author", "Stephen Jay Gould"},
	{"Author", "נדבあب"},
}

// fillTaxonomy adds taxoCategories to tw and asserts each AddCategory returns
// the expected ordinal. Mirrors TestTaxonomyCombined.fillTaxonomy.
func fillTaxonomy(t *testing.T, tw *facets.DirectoryTaxonomyWriter) {
	t.Helper()
	for i, cat := range taxoCategories {
		ord, err := tw.AddCategory(facets.NewFacetLabel(cat...))
		if err != nil {
			t.Fatalf("AddCategory(%v): %v", cat, err)
		}
		want := taxoExpectedPaths[i][len(taxoExpectedPaths[i])-1]
		if ord != want {
			t.Fatalf("AddCategory(%v): want ordinal %d, got %d", cat, want, ord)
		}
	}
}

// TestTaxonomyCombined_Writer mirrors testWriter: fillTaxonomy returns the
// expected ordinals and the writer reports the expected total size.
func TestTaxonomyCombined_Writer(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	tw, err := facets.NewDirectoryTaxonomyWriter(dir)
	if err != nil {
		t.Fatalf("NewDirectoryTaxonomyWriter: %v", err)
	}
	fillTaxonomy(t, tw)
	if got := tw.GetSize(); got != len(taxoExpectedCategories) {
		t.Errorf("GetSize: want %d, got %d", len(taxoExpectedCategories), got)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestTaxonomyCombined_WriterTwice mirrors testWriter2 (close + reopen + refill):
// re-adding the same categories to a writer reopened over the committed index
// yields the same ordinals (the writer cold-loads its cache from disk) and does
// not create extraneous categories.
func TestTaxonomyCombined_WriterTwice(t *testing.T) {
	dir := store.NewByteBuffersDirectory()

	tw, err := facets.NewDirectoryTaxonomyWriter(dir)
	if err != nil {
		t.Fatalf("NewDirectoryTaxonomyWriter: %v", err)
	}
	fillTaxonomy(t, tw)
	if err := tw.Close(); err != nil {
		t.Fatalf("Close (first): %v", err)
	}

	// Reopen over the committed index: the writer must cold-load its ordinal
	// cache from disk so the second fillTaxonomy reproduces the same ordinals.
	tw2, err := facets.NewDirectoryTaxonomyWriter(dir)
	if err != nil {
		t.Fatalf("NewDirectoryTaxonomyWriter (reopen): %v", err)
	}
	fillTaxonomy(t, tw2)
	if got := tw2.GetSize(); got != len(taxoExpectedCategories) {
		t.Errorf("GetSize after reopen+refill: want %d, got %d", len(taxoExpectedCategories), got)
	}
	if err := tw2.Close(); err != nil {
		t.Fatalf("Close (second): %v", err)
	}
}

// TestTaxonomyCombined_ReaderBasic mirrors testReaderBasic: after writing and
// closing the taxonomy, a cold DirectoryTaxonomyReader reproduces every
// ordinal⇔category mapping from disk.
func TestTaxonomyCombined_ReaderBasic(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	tw, err := facets.NewDirectoryTaxonomyWriter(dir)
	if err != nil {
		t.Fatalf("NewDirectoryTaxonomyWriter: %v", err)
	}
	fillTaxonomy(t, tw)
	if err := tw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	tr, err := facets.NewDirectoryTaxonomyReader(dir)
	if err != nil {
		t.Fatalf("NewDirectoryTaxonomyReader: %v", err)
	}
	defer tr.Close() //nolint:errcheck

	if got := tr.GetSize(); got != len(taxoExpectedCategories) {
		t.Fatalf("GetSize: want %d, got %d", len(taxoExpectedCategories), got)
	}

	// ordinal => category => ordinal round-trips.
	for i := 0; i < tr.GetSize(); i++ {
		path := tr.GetPath(i)
		if path == nil {
			t.Fatalf("GetPath(%d) = nil", i)
		}
		if ord := tr.GetOrdinal(path); ord != i {
			t.Errorf("GetOrdinal(GetPath(%d)): want %d, got %d", i, i, ord)
		}
	}

	// Each non-root ordinal maps to its expected category path.
	for i := 1; i < len(taxoExpectedCategories); i++ {
		want := facets.NewFacetLabel(taxoExpectedCategories[i]...)
		got := tr.GetPath(i)
		if got == nil || !want.Equals(got) {
			t.Errorf("GetPath(%d): want %v, got %v", i, taxoExpectedCategories[i], got)
		}
	}

	// Each expected category maps to its expected ordinal.
	for i := 1; i < len(taxoExpectedCategories); i++ {
		got := tr.GetOrdinal(facets.NewFacetLabel(taxoExpectedCategories[i]...))
		if got != i {
			t.Errorf("GetOrdinal(%v): want %d, got %d", taxoExpectedCategories[i], i, got)
		}
	}

	// Non-existent categories return the invalid ordinal (-1).
	if ord := tr.GetOrdinal(facets.NewFacetLabel("non-existant")); ord != -1 {
		t.Errorf("GetOrdinal(non-existant): want -1, got %d", ord)
	}
	if ord := tr.GetOrdinal(facets.NewFacetLabel("Author", "Jules Verne")); ord != -1 {
		t.Errorf("GetOrdinal(Author/Jules Verne): want -1, got %d", ord)
	}
}

// TestTaxonomyCombined_ReaderParent mirrors testReaderParent: every non-root
// category's persisted parent ordinal matches the parent implied by its path.
func TestTaxonomyCombined_ReaderParent(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	tw, err := facets.NewDirectoryTaxonomyWriter(dir)
	if err != nil {
		t.Fatalf("NewDirectoryTaxonomyWriter: %v", err)
	}
	fillTaxonomy(t, tw)
	if err := tw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	tr, err := facets.NewDirectoryTaxonomyReader(dir)
	if err != nil {
		t.Fatalf("NewDirectoryTaxonomyReader: %v", err)
	}
	defer tr.Close() //nolint:errcheck

	// Root's parent is the invalid ordinal.
	if p := tr.GetParent(0); p != -1 {
		t.Errorf("GetParent(root): want -1, got %d", p)
	}

	// Each non-root ordinal's parent path equals the ordinal's path minus its
	// last component.
	for ord := 1; ord < tr.GetSize(); ord++ {
		me := tr.GetPath(ord)
		if me == nil {
			t.Fatalf("GetPath(%d) = nil", ord)
		}
		parentOrd := tr.GetParent(ord)
		parent := tr.GetPath(parentOrd)
		if parent == nil {
			t.Fatalf("ordinal %d: parent %d is not a valid category", ord, parentOrd)
		}
		wantParent := me.SubPath(0, me.Length()-1)
		if !wantParent.Equals(parent) {
			t.Errorf("ordinal %d (%v): parent %d is %v, want %v",
				ord, me.Components, parentOrd, parent.Components, wantParent.Components)
		}
	}
}

// TestTaxonomyCombined_RootOnly mirrors testRootOnly: an empty taxonomy, once
// committed and cold-opened, contains exactly the root at ordinal 0 with the
// invalid parent ordinal.
func TestTaxonomyCombined_RootOnly(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	tw, err := facets.NewDirectoryTaxonomyWriter(dir)
	if err != nil {
		t.Fatalf("NewDirectoryTaxonomyWriter: %v", err)
	}
	if got := tw.GetSize(); got != 1 {
		t.Errorf("writer GetSize (root only): want 1, got %d", got)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	tr, err := facets.NewDirectoryTaxonomyReader(dir)
	if err != nil {
		t.Fatalf("NewDirectoryTaxonomyReader: %v", err)
	}
	defer tr.Close() //nolint:errcheck

	if got := tr.GetSize(); got != 1 {
		t.Errorf("reader GetSize (root only): want 1, got %d", got)
	}
	root := tr.GetPath(0)
	if root == nil || root.Length() != 0 {
		t.Errorf("GetPath(0): want empty root label, got %v", root)
	}
	if p := tr.GetParent(0); p != -1 {
		t.Errorf("GetParent(root): want -1, got %d", p)
	}
	if ord := tr.GetOrdinal(facets.NewFacetLabelEmpty()); ord != 0 {
		t.Errorf("GetOrdinal(root): want 0, got %d", ord)
	}
}
